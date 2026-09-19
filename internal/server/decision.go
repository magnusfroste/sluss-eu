package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/router"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// DecisionOptions configures the /router/decision handler.
type DecisionOptions struct {
	Engine      *engine.Engine
	PolicyCache *policy.Cache
	Logger      *slog.Logger
}

// decisionResponse extends RouteDecision with the computed JobDescriptor so
// callers can see exactly what the engine classified from the request.
type decisionResponse struct {
	engine.RouteDecision
	Job *router.JobDescriptor `json:"job,omitempty"`
}

// DecisionHandler handles POST /router/decision — a dry-run that returns the
// routing decision without making any provider call.
func DecisionHandler(opts DecisionOptions) http.HandlerFunc {
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

		var auth router.AuthTenantContext
		if t, ok := tenant.FromContext(r.Context()); ok {
			auth.TenantID = t.ID
			auth.ProjectID = t.Project
		}

		if opts.Engine == nil {
			writeError(w, http.StatusServiceUnavailable, "engine_unavailable", "routing engine not configured")
			return
		}

		dec, job, err := explainRoute(opts.Engine, opts.PolicyCache, explainInput{
			RequestID: middleware.RequestIDFromContext(r.Context()),
			Auth:      auth,
			Headers:   r.Header,
			Request:   &req,
		})
		if err != nil {
			if errors.Is(err, engine.ErrBlocked) {
				status := dec.BlockStatus
				if status == 0 {
					status = http.StatusForbidden
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(decisionResponse{RouteDecision: dec})
				return
			}
			if errors.Is(err, engine.ErrModelNotFound) {
				writeError(w, http.StatusNotFound, "model_not_found", err.Error())
				return
			}
			writeError(w, http.StatusUnprocessableEntity, "no_route", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Router-Selected-Model", dec.SelectedModel)
		if dec.PolicyVersion != "" {
			w.Header().Set("X-Router-Policy-Version", dec.PolicyVersion)
		}

		resp := decisionResponse{RouteDecision: dec}
		if policy.ExplainEnabled(r.Header) {
			resp.Job = job
		}

		_ = json.NewEncoder(w).Encode(resp)

		if l := opts.Logger; l != nil {
			l.InfoContext(r.Context(), "router_decision",
				"request_id", middleware.RequestIDFromContext(r.Context()),
				"selected_model", dec.SelectedModel,
				"selected_provider", dec.SelectedProvider,
				"policy_version", dec.PolicyVersion,
				"timeout_ms", dec.TimeoutMS,
				"fallback_count", len(dec.Fallbacks),
			)
		}
	}
}

// explainInput carries what explainRoute needs to classify and route a request.
// Auth/Headers are optional (empty for MCP dry-runs, where there is no HTTP
// request context).
type explainInput struct {
	RequestID string
	Auth      router.AuthTenantContext
	Headers   http.Header
	Request   *openai.ChatRequest
}

// explainRoute runs the dry-run routing path shared by POST /router/decision and
// the MCP route_explain tool: build the JobDescriptor, look up policy, and score
// with the engine assuming full provider health. It never calls a provider. The
// caller must ensure eng != nil.
func explainRoute(eng *engine.Engine, cache *policy.Cache, in explainInput) (engine.RouteDecision, *router.JobDescriptor, error) {
	job := router.NewJobDescriptor(router.JobDescriptorInput{
		RequestID: in.RequestID,
		Auth:      in.Auth,
		Headers:   in.Headers,
		Request:   in.Request,
	})
	pol := lookupPolicy(cache, job)
	dec, err := eng.Decide(job, pol, engine.FullyHealthy, in.Request.Stream)
	return dec, job, err
}

func lookupPolicy(cache *policy.Cache, job *router.JobDescriptor) *policy.CompiledPolicy {
	if cache == nil {
		return nil
	}
	scope := policy.Scope{TenantID: job.TenantID, ProjectID: job.ProjectID}
	pol, _ := cache.Active(scope)
	return pol
}
