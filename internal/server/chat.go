package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/budget"
	"github.com/magnusfroste/sluss/internal/contextproc"
	"github.com/magnusfroste/sluss/internal/cost"
	"github.com/magnusfroste/sluss/internal/council"
	"github.com/magnusfroste/sluss/internal/decisioncache"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/experiment"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/retention"
	"github.com/magnusfroste/sluss/internal/router"
	"github.com/magnusfroste/sluss/internal/secrets"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// firstTokenTimeoutMS is how long to wait for the first streaming chunk before
// attempting the next fallback provider.
const firstTokenTimeoutMS = 10_000

var firstTokenTimeout = firstTokenTimeoutMS * time.Millisecond

// reasoningTimeout is the raised per-request timeout for reasoning-capable
// models: a long think (or a slow local GPU) must not be cut off by the
// adapter's default 60s. Applied whether thinking is on or off — these models
// tend to run on slower on-prem hardware.
const reasoningTimeout = 120 * time.Second

// ChatOptions configures the chat completions handler.
type ChatOptions struct {
	ContextPipeline        *contextproc.Pipeline
	ContextPipelineEnabled bool
	PromptAdapter          *provider.PromptAdapter
	Logger                 *slog.Logger

	// Routing (Sprint 05). Optional — if Engine is nil the handler uses the
	// single Provider passed to ChatCompletionsHandler.
	Engine            *engine.Engine
	Adapters          map[string]provider.Adapter // provider ID → adapter
	PolicyCache       *policy.Cache
	ShadowPolicyCache *policy.Cache

	// Observability (Sprint 06). All optional.
	EventQueue    *eventlog.Queue
	HealthTracker *health.Tracker

	// Security audit trail (ISSUE-044). Optional; records blocked requests.
	Auditor audit.Sink

	// Retention/privacy settings (ISSUE-045). Optional; gates prompt logging.
	Retention *retention.Settings

	// Budget caps (ISSUE-051). Optional; blocks or downgrades over-budget scopes.
	Budget *budget.Evaluator

	// Route decision cache (ISSUE-052). Optional; caches low-risk decisions.
	// RegistryVersion versions cache keys alongside the policy version.
	DecisionCache   *decisioncache.Cache
	RegistryVersion string

	// Council mode (Fusion-inspired). For task classes in CouncilTasks the
	// handler runs a deliberation panel of the top-N routed models concurrently
	// (non-streaming only) and returns the consensus answer. Empty CouncilTasks
	// disables it. Slow path only — never touches fast-path routing.
	CouncilTasks map[string]bool
	CouncilSize  int

	// Live policy A/B (Fas 4). When Experiment is enabled and ExperimentPolicyCache
	// is set, a deterministic share of traffic (bucketed by tenant+project) is
	// served the experiment policy variant instead of the primary policy, and the
	// assigned arm is tagged on the response and decision event. Disabled by
	// default (Percentage 0).
	ExperimentPolicyCache *policy.Cache
	Experiment            experiment.Config

	// Bandit (Fas 4), when set, receives per-(task, model) realized rewards so the
	// routing engine (via engine.Quality) can learn online which model performs
	// best per task class. Optional; nil disables reward recording.
	Bandit *bandit.Bandit
}

// streamCandidate is one entry in the ordered streaming attempt list.
type streamCandidate struct {
	adapter         provider.StreamingAdapter
	providerModelID string
	modelID         string
	providerID      string
	// Reasoning-capable models get a per-candidate thinking directive and a
	// raised timeout (set from the job after candidates are built).
	enableThinking *bool
	timeoutHint    time.Duration
}

func ChatCompletionsHandler(p provider.Adapter, opts ...ChatOptions) http.HandlerFunc {
	var cfg ChatOptions
	if len(opts) > 0 {
		cfg = opts[0]
	}
	setMaskLogger(cfg.Logger)
	return func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		if len(req.Messages) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "messages cannot be empty")
			return
		}

		normalized := provider.NormalizeChatRequest(&req)

		// Build JobDescriptor for every request (fast, pure in-memory).
		var auth router.AuthTenantContext
		if t, ok := tenant.FromContext(r.Context()); ok {
			auth.TenantID = t.ID
			auth.ProjectID = t.Project
		}
		job := router.NewJobDescriptor(router.JobDescriptorInput{
			RequestID: middleware.RequestIDFromContext(r.Context()),
			Auth:      auth,
			Headers:   r.Header,
			Request:   &req,
		})

		// Optional prompt logging — off by default, gated per tenant (ISSUE-045).
		cfg.logPrompt(r.Context(), job, &req)

		// Routing engine path.
		if cfg.Engine != nil && len(cfg.Adapters) > 0 {
			// Budget caps (ISSUE-051): block or downgrade before routing.
			if !cfg.applyBudget(w, r, job) {
				return
			}

			pol := lookupPolicy(cfg.PolicyCache, job)
			shadowPol := lookupPolicy(cfg.ShadowPolicyCache, job)

			// Live policy A/B (Fas 4): deterministically bucket by tenant+project.
			// Treatment is served the experiment policy variant; control keeps the
			// primary policy. The arm is tagged on the response and decision event so
			// outcomes can be sliced by arm. Off by default.
			arm := experiment.ArmControl
			if cfg.Experiment.Enabled() && cfg.ExperimentPolicyCache != nil {
				arm = cfg.Experiment.Assign(job.TenantID + "\x1f" + job.ProjectID)
				if arm == experiment.ArmTreatment {
					if vpol := lookupPolicy(cfg.ExperimentPolicyCache, job); vpol != nil {
						pol = vpol
					}
				}
				w.Header().Set("X-Router-Experiment-Arm", string(arm))
			}

			// Route decision cache (ISSUE-052): low-risk requests may reuse a
			// versioned, previously-computed decision instead of re-scoring.
			var (
				dec       engine.RouteDecision
				decErr    error
				cacheKey  string
				cacheable = cfg.DecisionCache != nil && decisioncache.Cacheable(job)
				cacheHit  bool
			)
			health := cfg.healthSnapshot()
			if cacheable {
				cacheKey = decisioncache.Key(&req, job, policyVersion(pol), cfg.RegistryVersion)
				if cached, ok := cfg.DecisionCache.Get(cacheKey); ok {
					dec, cacheHit = cached, true
				}
			}

			routeStart := time.Now()
			if !cacheHit {
				dec, decErr = cfg.Engine.Decide(job, pol, health, req.Stream)
			}
			routingDur := time.Since(routeStart)
			shadowComparison := cfg.compareShadowDecision(r.Context(), job, dec, decErr, shadowPol, health, req.Stream)

			if cacheable && !cacheHit && decErr == nil && !dec.Blocked {
				cfg.DecisionCache.Put(cacheKey, dec)
			}
			if cfg.DecisionCache != nil {
				w.Header().Set("X-Router-Cache", cacheStatus(cacheable, cacheHit))
			}

			// Always enqueue a decision event (blocked or not).
			cfg.enqueueDecision(job, dec, shadowComparison, routingDur, decErr, arm)

			if decErr != nil {
				if errors.Is(decErr, engine.ErrBlocked) {
					status := dec.BlockStatus
					if status == 0 {
						status = http.StatusForbidden
					}
					cfg.auditBlocked(r.Context(), job, dec)
					writeError(w, status, dec.BlockCode, dec.BlockReason)
					return
				}
				if errors.Is(decErr, engine.ErrModelNotFound) {
					// Client pinned a model that does not exist — a client error,
					// not an upstream routing failure.
					writeError(w, http.StatusNotFound, "model_not_found", decErr.Error())
					return
				}
				writeError(w, http.StatusBadGateway, "no_route", decErr.Error())
				return
			}

			w.Header().Set("X-Router-Selected-Model", dec.SelectedModel)
			w.Header().Set("X-Router-Policy-Version", dec.PolicyVersion)
			w.Header().Set("X-Router-Route-Class", string(job.TaskType))
			// Visible-switch metadata (ISSUE-083): a safe, human reason when the
			// route was driven by content sensitivity, plus the selected model's
			// compliance tags so a demo/client can show "stayed in the house"
			// when it routed to a local-tagged provider. Types/reasons only —
			// never PII values.
			cfg.setRouteReasonHeaders(w, job, dec)

			if req.Stream {
				candidates := buildStreamCandidates(dec, cfg.Adapters)
				// Reasoning-capable candidates get a per-candidate thinking
				// directive (from requires_reasoning) and a raised timeout.
				for i := range candidates {
					candidates[i].enableThinking, candidates[i].timeoutHint =
						cfg.reasoningDirective(candidates[i].modelID, job)
				}
				if len(candidates) > 0 {
					normalized.Model = candidates[0].providerModelID
					streamWithFallback(w, r, candidates, normalized, cfg.Logger, cfg.HealthTracker, cfg.EventQueue, job.RequestID, attemptMeta{
						tenantID:         job.TenantID,
						projectID:        job.ProjectID,
						taskType:         string(job.TaskType),
						estimatedCostUSD: dec.EstimatedCostUSD,
						costFn:           cfg.actualCostUSD,
					})
					return
				}
			}

			if adapter, ok := cfg.Adapters[dec.SelectedProvider]; ok {
				normalized.Model = dec.ProviderModelID
				normalized.EnableThinking, normalized.TimeoutHint =
					cfg.reasoningDirective(dec.SelectedModel, job)
				cfg.maybeRunContextPipeline(w, r, normalized, job, pol, req.Stream)
				cfg.maybeRunPromptAdapter(r, normalized, dec.SelectedModel, dec.ProviderModelID)

				// Council mode: high-risk task classes deliberate across a panel
				// instead of trusting a single model. Non-streaming only; falls
				// through to the single-model path if a panel can't be formed.
				if cfg.councilEnabled(job, req.Stream) {
					if cfg.runCouncil(w, r, dec, normalized, job) {
						return
					}
				}

				start := time.Now()
				resp, err := adapter.Complete(r.Context(), normalized)
				durationMs := time.Since(start).Milliseconds()
				cfg.recordAttempt(job.RequestID, dec.SelectedProvider, dec.SelectedModel, 0, resp, err, durationMs, 0, attemptMeta{
					tenantID:         job.TenantID,
					projectID:        job.ProjectID,
					taskType:         string(job.TaskType),
					estimatedCostUSD: dec.EstimatedCostUSD,
				})
				if err != nil {
					status, code := mapProviderError(err)
					writeError(w, status, code, err.Error())
					return
				}
				cfg.setUsageHeaders(w, dec.SelectedModel, dec.SelectedProvider, resp)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Fall through if adapter not found.
		}

		// Legacy path: single provider, no routing.
		if req.Stream {
			streamChatCompletion(w, r, p, normalized, cfg.Logger)
			return
		}
		cfg.maybeRunContextPipeline(w, r, normalized, job, lookupPolicy(cfg.PolicyCache, job), req.Stream)
		cfg.maybeRunPromptAdapter(r, normalized, normalized.Model, normalized.Model)
		resp, err := p.Complete(r.Context(), normalized)
		if err != nil {
			status, code := mapProviderError(err)
			writeError(w, status, code, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Router-Selected-Model", resp.Model)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// buildStreamCandidates converts a RouteDecision into an ordered attempt list
// for streaming. Primary is always first.
func buildStreamCandidates(dec engine.RouteDecision, adapters map[string]provider.Adapter) []streamCandidate {
	var result []streamCandidate
	add := func(providerID, providerModelID, modelID string) {
		a, ok := adapters[providerID]
		if !ok {
			return
		}
		sa, ok := a.(provider.StreamingAdapter)
		if !ok {
			return
		}
		result = append(result, streamCandidate{
			adapter:         sa,
			providerModelID: providerModelID,
			modelID:         modelID,
			providerID:      providerID,
		})
	}
	add(dec.SelectedProvider, dec.ProviderModelID, dec.SelectedModel)
	for _, fb := range dec.Fallbacks {
		add(fb.ProviderID, fb.ProviderModelID, fb.ModelID)
	}
	return result
}

// streamWithFallback attempts streaming from each candidate in order.
// It falls back to the next candidate if the connection fails or if no first
// token arrives within firstTokenTimeoutMS (ISSUE-030).
func streamWithFallback(
	w http.ResponseWriter,
	r *http.Request,
	candidates []streamCandidate,
	normalized *provider.NormalizedModelRequest,
	logger *slog.Logger,
	healthTracker *health.Tracker,
	queue *eventlog.Queue,
	requestID string,
	meta attemptMeta,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unavailable", "response writer does not support streaming")
		return
	}

	recordStream := func(c streamCandidate, i int, firstTokenMs, durationMs int64, inputTokens, outputTokens int, err error) {
		if healthTracker != nil {
			if err == nil {
				healthTracker.RecordSuccess(c.providerID)
			} else {
				healthTracker.RecordFailure(c.providerID)
			}
		}
		if queue != nil {
			code := ""
			if err != nil {
				_, code = mapProviderError(err)
			}
			// When usage is known, attribute realized cost from real tokens so
			// streamed requests don't fall back to the (max-tokens) estimate —
			// which would make spend and the savings baseline inconsistent.
			var actualCost float64
			if (inputTokens > 0 || outputTokens > 0) && meta.costFn != nil {
				actualCost = meta.costFn(c.modelID, c.providerID, inputTokens, outputTokens)
			}
			queue.Enqueue(eventlog.Event{
				Type: eventlog.EventTypeAttempt,
				Attempt: &eventlog.AttemptEvent{
					RequestID:        requestID,
					TenantID:         meta.tenantID,
					ProjectID:        meta.projectID,
					ProviderID:       c.providerID,
					ModelID:          c.modelID,
					AttemptIndex:     i,
					Success:          err == nil,
					ErrorCode:        code,
					DurationMs:       durationMs,
					FirstTokenMs:     firstTokenMs,
					InputTokens:      inputTokens,
					OutputTokens:     outputTokens,
					ActualCostUSD:    actualCost,
					EstimatedCostUSD: meta.estimatedCostUSD,
					AttemptedAt:      time.Now(),
				},
			})
		}
	}

	for i, c := range candidates {
		req := normalized.Clone()
		req.Model = c.providerModelID
		req.EnableThinking = c.enableThinking
		req.TimeoutHint = c.timeoutHint
		attemptStart := time.Now()
		var usageIn, usageOut int // captured from the provider's final usage chunk
		attemptCtx, cancelAttempt := context.WithCancel(r.Context())

		chunks, err := c.adapter.Stream(attemptCtx, req)
		if err != nil {
			cancelAttempt()
			logStreamAttempt(r, logger, c, i, "stream_open_error", err)
			recordStream(c, i, 0, time.Since(attemptStart).Milliseconds(), 0, 0, err)
			continue
		}

		// Wait for the first chunk within the first-token timeout.
		timer := time.NewTimer(firstTokenTimeout)
		var firstChunk provider.StreamChunk
		var gotFirst bool
		select {
		case chunk, chanOk := <-chunks:
			timer.Stop()
			if !chanOk {
				cancelAttempt()
				logStreamAttempt(r, logger, c, i, "stream_channel_closed", nil)
				recordStream(c, i, 0, time.Since(attemptStart).Milliseconds(), 0, 0, fmt.Errorf("channel closed"))
				continue
			}
			firstChunk = chunk
			gotFirst = true
		case <-timer.C:
			cancelAttempt()
			logStreamAttempt(r, logger, c, i, "first_token_timeout", nil)
			recordStream(c, i, 0, time.Since(attemptStart).Milliseconds(), 0, 0, provider.ErrProviderTimeout)
			continue
		case <-r.Context().Done():
			timer.Stop()
			cancelAttempt()
			return
		}

		if !gotFirst {
			cancelAttempt()
			continue
		}

		firstTokenMs := time.Since(attemptStart).Milliseconds()

		if firstChunk.Err != nil {
			cancelAttempt()
			if i+1 < len(candidates) {
				logStreamAttempt(r, logger, c, i, "first_chunk_error", firstChunk.Err)
				recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), 0, 0, firstChunk.Err)
				continue
			}
			recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), 0, 0, firstChunk.Err)
			status, code := mapProviderError(firstChunk.Err)
			writeError(w, status, code, firstChunk.Err.Error())
			return
		}

		// First token received — write headers and stream to completion.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Router-Selected-Model", c.modelID)
		w.Header().Set("X-Router-First-Token-Sent", "true")
		w.Header().Set("X-Router-First-Token-Ms", strconv.FormatInt(firstTokenMs, 10))
		started := time.Now()
		if firstChunk.InputTokens > 0 || firstChunk.OutputTokens > 0 {
			usageIn, usageOut = firstChunk.InputTokens, firstChunk.OutputTokens
		}

		if firstChunk.Done {
			writeSSEData(w, []byte("[DONE]"))
			flusher.Flush()
			recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), usageIn, usageOut, nil)
			cancelAttempt()
			return
		}
		if len(firstChunk.Data) > 0 {
			writeSSEData(w, firstChunk.Data)
			flusher.Flush()
		}

		// Drain the rest.
		for chunk := range chunks {
			if chunk.InputTokens > 0 || chunk.OutputTokens > 0 {
				usageIn, usageOut = chunk.InputTokens, chunk.OutputTokens
			}
			if chunk.Err != nil {
				writeSSEError(w, provider.ErrStreamInterrupted.Error(), chunk.Err.Error())
				flusher.Flush()
				logStreamResult(r, logger, c.providerID, c.modelID, true, time.Since(started).Milliseconds(), chunk.Err)
				recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), 0, 0, chunk.Err)
				cancelAttempt()
				return
			}
			if chunk.Done {
				writeSSEData(w, []byte("[DONE]"))
				flusher.Flush()
				logStreamResult(r, logger, c.providerID, c.modelID, true, time.Since(started).Milliseconds(), nil)
				recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), usageIn, usageOut, nil)
				cancelAttempt()
				return
			}
			if len(chunk.Data) > 0 {
				writeSSEData(w, chunk.Data)
				flusher.Flush()
			}
		}
		logStreamResult(r, logger, c.providerID, c.modelID, true, time.Since(started).Milliseconds(), nil)
		recordStream(c, i, firstTokenMs, time.Since(attemptStart).Milliseconds(), usageIn, usageOut, nil)
		cancelAttempt()
		return
	}

	// All candidates exhausted.
	writeError(w, http.StatusBadGateway, "provider_error", "all streaming candidates failed or timed out")
}

// streamChatCompletion is the legacy single-provider streaming path.
func streamChatCompletion(w http.ResponseWriter, r *http.Request, p provider.Adapter, normalized *provider.NormalizedModelRequest, logger *slog.Logger) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unavailable", "response writer does not support streaming")
		return
	}
	streamer, ok := p.(provider.StreamingAdapter)
	if !ok {
		err := fmt.Errorf("%w: streaming not supported by provider %s", provider.ErrProviderBadReq, p.Name())
		status, code := mapProviderError(err)
		writeError(w, status, code, err.Error())
		return
	}

	started := time.Now()
	chunks, err := streamer.Stream(r.Context(), normalized)
	if err != nil {
		status, code := mapProviderError(err)
		writeError(w, status, code, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if normalized.Model != "" {
		w.Header().Set("X-Router-Selected-Model", normalized.Model)
	}

	firstChunkSent := false
	var firstTokenMs int64
	for chunk := range chunks {
		if chunk.Err != nil {
			if !firstChunkSent {
				status, code := mapProviderError(chunk.Err)
				writeError(w, status, code, chunk.Err.Error())
				return
			}
			writeSSEError(w, provider.ErrStreamInterrupted.Error(), chunk.Err.Error())
			flusher.Flush()
			logStreamResult(r, logger, p.Name(), normalized.Model, true, firstTokenMs, chunk.Err)
			return
		}
		if !firstChunkSent {
			firstChunkSent = true
			firstTokenMs = time.Since(started).Milliseconds()
			w.Header().Set("X-Router-First-Token-Sent", "true")
			w.Header().Set("X-Router-First-Token-Ms", strconv.FormatInt(firstTokenMs, 10))
		}
		if chunk.Done {
			writeSSEData(w, []byte("[DONE]"))
			flusher.Flush()
			logStreamResult(r, logger, p.Name(), normalized.Model, true, firstTokenMs, nil)
			return
		}
		if len(chunk.Data) == 0 {
			continue
		}
		writeSSEData(w, chunk.Data)
		flusher.Flush()
	}
	logStreamResult(r, logger, p.Name(), normalized.Model, firstChunkSent, firstTokenMs, nil)
}

func writeSSEData(w http.ResponseWriter, data []byte) {
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n\n"))
}

func writeSSEError(w http.ResponseWriter, code, msg string) {
	msg = maskOutbound("sse_error", code, msg)
	body, _ := json.Marshal(openai.ErrorEnvelope{
		Error: openai.ErrorBody{Message: msg, Type: code, Code: code},
	})
	_, _ = w.Write([]byte("event: error\n"))
	writeSSEData(w, body)
}

func logStreamAttempt(r *http.Request, logger *slog.Logger, c streamCandidate, attempt int, reason string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []any{
		"request_id", middleware.RequestIDFromContext(r.Context()),
		"attempt", attempt,
		"model", c.modelID,
		"provider", c.providerID,
		"reason", reason,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	logger.WarnContext(r.Context(), "stream_fallback", attrs...)
}

func logStreamResult(r *http.Request, logger *slog.Logger, providerName, model string, firstChunkSent bool, firstTokenMs int64, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []any{
		"request_id", middleware.RequestIDFromContext(r.Context()),
		"provider", providerName,
		"model", model,
		"first_token_sent", firstChunkSent,
		"first_token_ms", firstTokenMs,
	}
	if err != nil {
		attrs = append(attrs, "stream_error", err.Error())
		logger.WarnContext(r.Context(), "chat_stream_interrupted", attrs...)
		return
	}
	logger.InfoContext(r.Context(), "chat_stream_completed", attrs...)
}

func logContextPipeline(r *http.Request, logger *slog.Logger, result contextproc.PipelineResult) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(r.Context(), "context_pipeline",
		"request_id", middleware.RequestIDFromContext(r.Context()),
		"context_processors_applied", result.Applied,
		"context_processors_skipped", result.Skipped,
		"context_tokens_saved", result.TotalTokensSaved,
	)
}

func (o *ChatOptions) maybeRunContextPipeline(
	w http.ResponseWriter,
	r *http.Request,
	normalized *provider.NormalizedModelRequest,
	job *router.JobDescriptor,
	pol *policy.CompiledPolicy,
	streaming bool,
) {
	if !contextPipelineAllowed(o.ContextPipelineEnabled, pol, job, streaming) {
		return
	}

	pipeline := o.ContextPipeline
	if pipeline == nil {
		pipeline = contextproc.NewNoopPipeline()
	}
	if pipeline.Logger == nil && o.Logger != nil {
		pipelineCopy := *pipeline
		pipelineCopy.Logger = o.Logger
		pipeline = &pipelineCopy
	}

	result := pipeline.Run(r.Context(), normalized, job)
	if result.TotalTokensSaved > 0 {
		w.Header().Set("X-Router-Context-Savings", strconv.Itoa(result.TotalTokensSaved))
	}
	if len(result.Applied) > 0 || len(result.Skipped) > 0 {
		logContextPipeline(r, o.Logger, result)
	}
}

func contextPipelineAllowed(operatorEnabled bool, pol *policy.CompiledPolicy, job *router.JobDescriptor, streaming bool) bool {
	if !operatorEnabled || streaming || pol == nil || job == nil {
		return false
	}
	return pol.Evaluate(policy.EvaluationInput{
		TenantID:             job.TenantID,
		ProjectID:            job.ProjectID,
		TaskType:             string(job.TaskType),
		RiskLevel:            string(job.RiskLevel),
		Sensitivity:          string(job.Sensitivity),
		PromptTokensEstimate: job.PromptTokensEstimate,
		Keywords:             append([]string(nil), job.Keywords...),
		FilesTouched:         append([]string(nil), job.FilesTouched...),
		RequiresToolUse:      job.RequiresToolUse,
		RequiresJSONSchema:   job.RequiresJSONSchema,
		RequiresVision:       job.RequiresVision,
		RouterMode:           string(job.RouterMode),
		ToolRisk:             job.ToolRisk,
		ToolsDeclared:        append([]string(nil), job.ToolsDeclared...),
	}).Route.ContextPipelineEnabled()
}

func (o *ChatOptions) maybeRunPromptAdapter(
	r *http.Request,
	normalized *provider.NormalizedModelRequest,
	modelID string,
	providerModelID string,
) {
	if o.PromptAdapter == nil || normalized == nil {
		return
	}
	adapted, result := o.PromptAdapter.Apply(normalized, provider.PromptAdapterContext{
		ModelID:         modelID,
		ProviderModelID: providerModelID,
	})
	if adapted == nil {
		return
	}
	*normalized = *adapted
	if len(result.AppliedRules) == 0 {
		return
	}
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(r.Context(), "prompt_adapter",
		"request_id", middleware.RequestIDFromContext(r.Context()),
		"selected_model", modelID,
		"provider_model_id", providerModelID,
		"applied_rules", result.AppliedRules,
	)
}

// --- Observability helpers ---

// healthSnapshot returns the current health snapshot; if no tracker is
// configured returns engine.FullyHealthy.
func (o *ChatOptions) healthSnapshot() engine.HealthSnapshot {
	if o.HealthTracker != nil {
		return o.HealthTracker
	}
	return engine.FullyHealthy
}

// enqueueDecision enqueues a DecisionEvent if a queue is configured.
func (o *ChatOptions) enqueueDecision(job *router.JobDescriptor, dec engine.RouteDecision, shadowComparison *engine.DecisionComparison, routingDur time.Duration, decErr error, arm experiment.Arm) {
	if o.EventQueue == nil {
		return
	}
	d := &eventlog.DecisionEvent{
		RequestID:             job.RequestID,
		TenantID:              job.TenantID,
		ProjectID:             job.ProjectID,
		TaskType:              string(job.TaskType),
		RiskLevel:             string(job.RiskLevel),
		Sensitivity:           string(job.Sensitivity),
		SelectedModel:         dec.SelectedModel,
		SelectedProvider:      dec.SelectedProvider,
		ProviderModelID:       dec.ProviderModelID,
		PolicyVersion:         dec.PolicyVersion,
		PromptTokens:          job.PromptTokensEstimate,
		EstimatedCostUSD:      dec.EstimatedCostUSD,
		RoutingDurationMs:     routingDur.Milliseconds(),
		RoutingDurationMicros: routingDur.Microseconds(),
		Blocked:               dec.Blocked,
		BlockCode:             dec.BlockCode,
		ShadowComparison:      shadowComparison,
		ExperimentArm:         string(arm),
		DecidedAt:             time.Now(),
	}
	o.EventQueue.Enqueue(eventlog.Event{Type: eventlog.EventTypeDecision, Decision: d})
}

func (o *ChatOptions) compareShadowDecision(
	ctx context.Context,
	job *router.JobDescriptor,
	primary engine.RouteDecision,
	primaryErr error,
	shadowPol *policy.CompiledPolicy,
	health engine.HealthSnapshot,
	streaming bool,
) *engine.DecisionComparison {
	if o.Engine == nil || shadowPol == nil || job == nil {
		return nil
	}
	if primaryErr != nil && !errors.Is(primaryErr, engine.ErrBlocked) {
		return nil
	}

	shadowJob := cloneJobDescriptor(job)
	shadowDecision, shadowErr := o.Engine.Decide(shadowJob, shadowPol, health, streaming)
	if shadowErr != nil && !errors.Is(shadowErr, engine.ErrBlocked) {
		logger := o.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.WarnContext(ctx, "shadow_decision_failed",
			"request_id", job.RequestID,
			"policy_version", policyVersion(shadowPol),
			"error", shadowErr.Error(),
		)
		return nil
	}

	comparison := engine.CompareDecisions(primary, shadowDecision)
	return &comparison
}

// logPrompt logs prompt message content when retention settings enable prompt
// logging for the tenant (ISSUE-045). It is a no-op by default. Content is run
// through secret masking first so credentials never reach the logs, and it is
// emitted at debug level so normal operation stays quiet.
func (o *ChatOptions) logPrompt(ctx context.Context, job *router.JobDescriptor, req *openai.ChatRequest) {
	if o.Retention == nil || !o.Retention.PromptLoggingEnabled(job.TenantID) {
		return
	}
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	for i, m := range req.Messages {
		logger.DebugContext(ctx, "prompt_message",
			"request_id", job.RequestID,
			"tenant_id", job.TenantID,
			"index", i,
			"role", m.Role,
			"content", secrets.Mask(m.Content).Text,
		)
	}
}

// applyBudget evaluates budget caps for the request's scope (ISSUE-051). It
// returns false when the request has been rejected (response already written),
// true to continue. A warning sets a header; a downgrade forces cheap routing.
func (o *ChatOptions) applyBudget(w http.ResponseWriter, r *http.Request, job *router.JobDescriptor) bool {
	if o.Budget == nil {
		return true
	}
	v := o.Budget.Check(job.TenantID, job.ProjectID)
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	switch {
	case v.Blocked():
		logger.WarnContext(r.Context(), "budget_block",
			"request_id", job.RequestID, "scope", v.Scope,
			"spent_micro_usd", v.SpentMicroUSD, "limit_micro_usd", v.LimitMicroUSD)
		writeError(w, http.StatusPaymentRequired, "budget_exceeded",
			"budget cap exceeded for "+v.Scope)
		return false
	case v.Downgrade():
		job.RouterMode = router.RouterModeCheap
		w.Header().Set("X-Router-Budget-Action", "downgrade")
		logger.WarnContext(r.Context(), "budget_downgrade",
			"request_id", job.RequestID, "scope", v.Scope,
			"spent_micro_usd", v.SpentMicroUSD, "limit_micro_usd", v.LimitMicroUSD)
	case v.Status == budget.StatusWarn:
		w.Header().Set("X-Router-Budget-Warning", "true")
		logger.InfoContext(r.Context(), "budget_warning",
			"request_id", job.RequestID, "scope", v.Scope,
			"fraction", v.Fraction, "limit_micro_usd", v.LimitMicroUSD)
	}
	return true
}

// policyVersion returns the compiled policy's version, or "" when no policy
// applies. It versions the decision cache key (ISSUE-052).
func policyVersion(pol *policy.CompiledPolicy) string {
	if pol == nil {
		return ""
	}
	return pol.Version()
}

// cacheStatus renders the X-Router-Cache header value.
func cacheStatus(cacheable, hit bool) string {
	switch {
	case !cacheable:
		return "bypass"
	case hit:
		return "hit"
	default:
		return "miss"
	}
}

func cloneJobDescriptor(in *router.JobDescriptor) *router.JobDescriptor {
	if in == nil {
		return nil
	}
	out := *in
	out.FilesTouched = append([]string(nil), in.FilesTouched...)
	out.Keywords = append([]string(nil), in.Keywords...)
	out.PIITypes = append([]string(nil), in.PIITypes...)
	if in.Metadata != nil {
		out.Metadata = make(map[string]any, len(in.Metadata))
		for k, v := range in.Metadata {
			out.Metadata[k] = v
		}
	}
	if in.ExplicitModel != nil {
		model := *in.ExplicitModel
		out.ExplicitModel = &model
	}
	return &out
}

// auditBlocked records a security-audit entry when policy blocks a request
// before any provider call (ISSUE-044). No-op when no auditor is configured.
func (o *ChatOptions) auditBlocked(ctx context.Context, job *router.JobDescriptor, dec engine.RouteDecision) {
	if o.Auditor == nil {
		return
	}
	detail := map[string]string{
		"block_code": dec.BlockCode,
		"task_type":  string(job.TaskType),
		"risk_level": string(job.RiskLevel),
	}
	// PII context on the audit entry: types + implied count only, never values.
	if len(job.PIITypes) > 0 {
		detail["pii_types"] = strings.Join(job.PIITypes, ",")
	}
	target := ""
	if job.ExplicitModel != nil {
		target = *job.ExplicitModel
	}
	audit.Record(ctx, o.Auditor, audit.Entry{
		Action:    audit.ActionRequestBlocked,
		Actor:     job.TenantID,
		TenantID:  job.TenantID,
		ProjectID: job.ProjectID,
		Target:    target,
		Outcome:   audit.OutcomeBlocked,
		RequestID: job.RequestID,
		Reason:    dec.BlockReason,
		Detail:    detail,
	})
}

// attemptMeta carries the per-request attributes an AttemptEvent needs for spend
// accounting: who the request belongs to and the decision-time cost estimate
// (used as a fallback when provider usage is unavailable).
type attemptMeta struct {
	tenantID         string
	projectID        string
	taskType         string // for per-(task,model) bandit reward attribution
	estimatedCostUSD float64
	// costFn computes realized USD from actual token usage (registry pricing).
	// Used by the streaming path, where usage arrives in a final chunk.
	costFn func(modelID, providerID string, inTok, outTok int) float64
}

// recordAttempt updates the health tracker and enqueues an AttemptEvent. When
// provider usage is present it computes the realized cost from the registry so
// spend reflects actual, not estimated, spend.
func (o *ChatOptions) recordAttempt(requestID, providerID, modelID string, attemptIdx int, resp *openai.ChatResponse, err error, durationMs, firstTokenMs int64, meta attemptMeta) {
	success := err == nil
	if o.HealthTracker != nil {
		if success {
			o.HealthTracker.RecordSuccess(providerID)
		} else {
			o.HealthTracker.RecordFailure(providerID)
		}
	}
	// Bandit reward: 1 for a successful attempt, 0 otherwise. Lets the router
	// learn per-(task, model) which model actually delivers, online.
	if o.Bandit != nil && meta.taskType != "" {
		reward := 0.0
		if success {
			reward = 1.0
		}
		o.Bandit.Record(meta.taskType, modelID, reward)
	}
	if o.EventQueue == nil {
		return
	}
	a := &eventlog.AttemptEvent{
		RequestID:        requestID,
		TenantID:         meta.tenantID,
		ProjectID:        meta.projectID,
		ProviderID:       providerID,
		ModelID:          modelID,
		AttemptIndex:     attemptIdx,
		Success:          success,
		DurationMs:       durationMs,
		FirstTokenMs:     firstTokenMs,
		EstimatedCostUSD: meta.estimatedCostUSD,
		AttemptedAt:      time.Now(),
	}
	if err != nil {
		a.ErrorCode = mapProviderErrorCode(err)
	}
	if resp != nil {
		a.InputTokens = resp.Usage.PromptTokens
		a.OutputTokens = resp.Usage.CompletionTokens
		a.ActualCostUSD = o.actualCostUSD(modelID, providerID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	o.EventQueue.Enqueue(eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: a})
}

// setUsageHeaders surfaces per-request token usage and realized cost as response
// headers so clients and tooling can see what the call cost. No-op for a nil
// response.
func (o *ChatOptions) setUsageHeaders(w http.ResponseWriter, modelID, providerID string, resp *openai.ChatResponse) {
	if resp == nil {
		return
	}
	w.Header().Set("X-Router-Input-Tokens", strconv.Itoa(resp.Usage.PromptTokens))
	w.Header().Set("X-Router-Output-Tokens", strconv.Itoa(resp.Usage.CompletionTokens))
	costUSD := o.actualCostUSD(modelID, providerID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	w.Header().Set("X-Router-Cost-USD", strconv.FormatFloat(costUSD, 'f', 6, 64))
}

// localTags mark a provider/model as staying inside the house — the "cloud →
// local" switch the demo makes visible (ISSUE-083). "private" covers a private
// on-prem/VPC endpoint (e.g. a self-hosted DGX inference server).
var localTags = map[string]bool{"local": true, "private": true, "on-prem": true, "on-premise": true, "air-gapped": true, "self-hosted": true}

// setRouteReasonHeaders surfaces why a sensitive request routed where it did,
// safely (types/reasons only, never PII values). Header VALUES are ASCII: HTTP
// headers are not UTF-8 safe (browsers decode them as Latin-1), so the reason is
// a stable machine code and the client renders the localized sentence.
//   - X-Router-Route-Reason: "" | "pii" | "pii:<type,type>" (types are ASCII).
//   - X-Router-Selected-Tags: the selected model's compliance tags (ASCII).
//   - X-Router-Egress: "local" when the selected model carries a local tag, so a
//     client can say "data stayed in the house"; otherwise "cloud".
func (o *ChatOptions) setRouteReasonHeaders(w http.ResponseWriter, job *router.JobDescriptor, dec engine.RouteDecision) {
	if len(job.PIITypes) > 0 {
		w.Header().Set("X-Router-Route-Reason", "pii:"+strings.Join(job.PIITypes, ","))
	} else if job.Sensitivity == router.SensitivityPII {
		w.Header().Set("X-Router-Route-Reason", "pii")
	}
	tags := o.selectedModelTags(dec.SelectedModel)
	if len(tags) > 0 {
		w.Header().Set("X-Router-Selected-Tags", strings.Join(tags, ", "))
	}
	egress := "cloud"
	for _, t := range tags {
		if localTags[t] {
			egress = "local"
			break
		}
	}
	w.Header().Set("X-Router-Egress", egress)
}

// selectedModelTags looks up the effective compliance tags of the selected
// model from the live registry. Returns nil when unavailable.
func (o *ChatOptions) selectedModelTags(modelID string) []string {
	if o.Engine == nil || o.Engine.Registry == nil {
		return nil
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return nil
	}
	if m, ok := snap.Model(modelID); ok {
		return m.ComplianceTags
	}
	return nil
}

// reasoningDirective returns the enable_thinking directive and timeout hint for
// a selected model. Nil/0 unless the model is flagged reasoning-capable — then
// thinking follows the job's requires_reasoning signal (hard tasks think,
// simple ones answer directly; one model, deterministically controlled) and the
// timeout is raised so a long think on slow on-prem hardware isn't cut off.
func (o *ChatOptions) reasoningDirective(modelID string, job *router.JobDescriptor) (*bool, time.Duration) {
	if o.Engine == nil || o.Engine.Registry == nil || job == nil {
		return nil, 0
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return nil, 0
	}
	m, ok := snap.Model(modelID)
	if !ok || !m.ReasoningCapable {
		return nil, 0
	}
	think := job.RequiresReasoning
	return &think, reasoningTimeout
}

// councilEnabled reports whether the request should use council (deliberation
// panel) mode: the task class is configured for council and the request is not
// streaming (the panel needs full answers to judge).
func (o *ChatOptions) councilEnabled(job *router.JobDescriptor, streaming bool) bool {
	return !streaming && len(o.CouncilTasks) > 0 && job != nil && o.CouncilTasks[string(job.TaskType)]
}

// buildCouncilPanel assembles the deliberation panel from the decision's
// selected model plus its fallbacks, keeping distinct (provider, model) pairs
// with a resolvable adapter, capped at CouncilSize (default 3, min 2).
func (o *ChatOptions) buildCouncilPanel(dec engine.RouteDecision) []council.Candidate {
	size := o.CouncilSize
	if size <= 0 {
		size = 3
	}
	seen := make(map[string]bool)
	var panel []council.Candidate
	add := func(providerID, providerModelID, modelID string) {
		if len(panel) >= size {
			return
		}
		key := providerID + "\x1f" + providerModelID
		if seen[key] {
			return
		}
		adapter, ok := o.Adapters[providerID]
		if !ok {
			return
		}
		seen[key] = true
		panel = append(panel, council.Candidate{
			ProviderID:      providerID,
			ModelID:         modelID,
			ProviderModelID: providerModelID,
			Adapter:         adapter,
		})
	}
	add(dec.SelectedProvider, dec.ProviderModelID, dec.SelectedModel)
	for _, fb := range dec.Fallbacks {
		add(fb.ProviderID, fb.ProviderModelID, fb.ModelID)
	}
	return panel
}

// runCouncil executes the deliberation panel and writes the consensus answer.
// It returns false (handled=false) when a panel of at least two distinct models
// cannot be formed, so the caller falls through to the single-model path.
func (o *ChatOptions) runCouncil(w http.ResponseWriter, r *http.Request, dec engine.RouteDecision, normalized *provider.NormalizedModelRequest, job *router.JobDescriptor) bool {
	panel := o.buildCouncilPanel(dec)
	if len(panel) < 2 {
		return false // not enough diversity for a council — use the single model
	}

	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Record every member so health and spend account for the full panel cost —
	// council trades cost for reliability, and that trade must be visible.
	record := func(res council.Result) {
		o.recordAttempt(job.RequestID, res.Candidate.ProviderID, res.Candidate.ModelID, res.Index, res.Response, res.Err, res.DurationMs, 0, attemptMeta{
			tenantID:         job.TenantID,
			projectID:        job.ProjectID,
			taskType:         string(job.TaskType),
			estimatedCostUSD: dec.EstimatedCostUSD,
		})
	}

	results, verdict, ok := council.Run(r.Context(), panel, normalized, council.ConsensusJudge{}, record)
	if !ok {
		// Every panel member failed — surface a gateway error like a normal
		// exhausted fallback chain would.
		writeError(w, http.StatusBadGateway, "provider_5xx", "all council members failed")
		return true
	}

	winner := results[verdict.WinnerIndex]
	logger.InfoContext(r.Context(), "council_decision",
		"request_id", job.RequestID,
		"task_type", string(job.TaskType),
		"panel_size", verdict.PanelSize,
		"success_count", verdict.SuccessCount,
		"agreement", verdict.Agreement,
		"winner_model", winner.Candidate.ModelID,
		"reason", verdict.Reason,
	)

	w.Header().Set("X-Router-Council", "true")
	w.Header().Set("X-Router-Council-Size", strconv.Itoa(verdict.PanelSize))
	w.Header().Set("X-Router-Council-Success", strconv.Itoa(verdict.SuccessCount))
	w.Header().Set("X-Router-Council-Agreement", strconv.FormatFloat(verdict.Agreement, 'f', 2, 64))
	w.Header().Set("X-Router-Council-Reason", verdict.Reason)
	// The winner may differ from the primary routed model; reflect it.
	w.Header().Set("X-Router-Selected-Model", winner.Candidate.ModelID)
	o.setUsageHeaders(w, winner.Candidate.ModelID, winner.Candidate.ProviderID, winner.Response)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(winner.Response)
	return true
}

// actualCostUSD prices real token usage against the selected model's registry
// cost metadata. Returns 0 when the model or its cost is unavailable, in which
// case spend falls back to the decision-time estimate.
func (o *ChatOptions) actualCostUSD(modelID, providerID string, inTok, outTok int) float64 {
	if o.Engine == nil || o.Engine.Registry == nil {
		return 0
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return 0
	}
	model, ok := snap.Model(modelID)
	if !ok {
		return 0
	}
	est, err := cost.EstimateCost(modelID, providerID, model.Cost, cost.TokenUsage{
		InputTokens:  int64(inTok),
		OutputTokens: int64(outTok),
		Mode:         cost.ModeActual,
	})
	if err != nil {
		return 0
	}
	return float64(est.TotalMicroUSD) / 1_000_000
}

func mapProviderErrorCode(err error) string {
	_, code := mapProviderError(err)
	return code
}

func mapProviderError(err error) (status int, code string) {
	switch {
	case errors.Is(err, provider.ErrProviderTimeout):
		return http.StatusGatewayTimeout, "provider_timeout"
	case errors.Is(err, provider.ErrProviderRateLimit):
		return http.StatusTooManyRequests, "provider_rate_limit"
	case errors.Is(err, provider.ErrProviderAuth):
		return http.StatusBadGateway, "provider_auth_error"
	case errors.Is(err, provider.ErrProvider5xx):
		return http.StatusBadGateway, "provider_5xx"
	case errors.Is(err, provider.ErrProviderBadReq):
		return http.StatusBadRequest, "provider_bad_request"
	case errors.Is(err, provider.ErrProviderBadResp):
		return http.StatusBadGateway, "provider_bad_response"
	case errors.Is(err, provider.ErrModelUnavailable):
		return http.StatusBadGateway, "model_unavailable"
	case errors.Is(err, provider.ErrStreamInterrupted):
		return http.StatusBadGateway, "stream_interrupted"
	default:
		return http.StatusBadGateway, "provider_error"
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	msg = maskOutbound("error_response", code, msg)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
		Error: openai.ErrorBody{Message: msg, Type: code, Code: code},
	})
}
