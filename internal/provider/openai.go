package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

const chatCompletionsPath = "/v1/chat/completions"

// OpenAIAdapter calls an OpenAI-compatible chat completions endpoint. It
// expects req.Model to already contain the provider model id selected upstream.
type OpenAIAdapter struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
	Timeout time.Duration
	// Referer and Title are optional attribution headers (HTTP-Referer / X-Title)
	// recommended by OpenRouter; sent only when non-empty so native OpenAI/
	// Anthropic endpoints are unaffected.
	Referer string
	Title   string
	// ProviderID names the provider connection in logs (optional).
	ProviderID string
	// Logger, when set, emits a WARN with the upstream status, Server header and a
	// bounded body snippet on any non-2xx — so a live instance's container log
	// (e.g. EasyPanel) shows a Cloudflare error page vs an origin/vLLM error
	// without any extra tooling. Nil disables it.
	Logger *slog.Logger
}

func (a *OpenAIAdapter) Name() string { return "openai" }

// handleErrorStatus reads the upstream error body once, logs a WARN with the
// status, Server header and a bounded snippet (so the container log shows a
// Cloudflare error page vs an origin/vLLM error), and maps it to a typed error.
// The snippet is server-side only — the caller's clean typed error is unchanged.
func (a *OpenAIAdapter) handleErrorStatus(req *NormalizedModelRequest, resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if a.Logger != nil {
		snippet := string(bytes.TrimSpace(raw))
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		model := ""
		if req != nil {
			model = req.Model
		}
		a.Logger.Warn("provider_upstream_error",
			"provider_id", a.ProviderID,
			"model", model,
			"status", resp.StatusCode,
			"server", resp.Header.Get("Server"),
			"content_type", resp.Header.Get("Content-Type"),
			"body_snippet", snippet,
		)
	}
	return mapOpenAIStatus(resp.StatusCode, bytes.NewReader(raw))
}

// effectiveTimeout returns the per-request timeout: the router's TimeoutHint
// (raised for reasoning-capable models) wins over the adapter default.
func (a *OpenAIAdapter) effectiveTimeout(req *NormalizedModelRequest) time.Duration {
	if req != nil && req.TimeoutHint > 0 {
		return req.TimeoutHint
	}
	return a.Timeout
}

// httpClient returns the adapter's client, with the hard client-level timeout
// lifted when the per-request deadline exceeds it — the context deadline still
// bounds the request, and the shared Transport (connection pool) is kept.
func (a *OpenAIAdapter) httpClient(req *NormalizedModelRequest) *http.Client {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	if req != nil && req.TimeoutHint > 0 && client.Timeout > 0 && client.Timeout < req.TimeoutHint {
		c2 := *client
		c2.Timeout = 0
		client = &c2
	}
	return client
}

// setCommonHeaders applies content-type, bearer auth and optional OpenRouter
// attribution headers shared by the Complete and Stream paths.
func (a *OpenAIAdapter) setCommonHeaders(httpReq *http.Request) {
	httpReq.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	}
	if a.Referer != "" {
		httpReq.Header.Set("HTTP-Referer", a.Referer)
	}
	if a.Title != "" {
		httpReq.Header.Set("X-Title", a.Title)
	}
}

func (a *OpenAIAdapter) Complete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error) {
	if t := a.effectiveTimeout(req); t > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}

	endpoint, err := chatCompletionsURL(a.BaseURL)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(req.ToOpenAI())
	if err != nil {
		return nil, fmt.Errorf("openai adapter: marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai adapter: build request: %w", err)
	}
	a.setCommonHeaders(httpReq)

	resp, err := a.httpClient(req).Do(httpReq)
	if err != nil {
		if isTimeoutError(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: request timed out", ErrProviderTimeout)
		}
		return nil, fmt.Errorf("openai adapter: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, a.handleErrorStatus(req, resp)
	}

	var out openai.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: decode response", ErrProviderBadResp)
	}
	return &out, nil
}

func (a *OpenAIAdapter) Stream(ctx context.Context, req *NormalizedModelRequest) (<-chan StreamChunk, error) {
	var cancel context.CancelFunc
	if t := a.effectiveTimeout(req); t > 0 {
		ctx, cancel = context.WithTimeout(ctx, t)
	}

	endpoint, err := chatCompletionsURL(a.BaseURL)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, err
	}
	outbound := req.Clone()
	outbound.Stream = true
	oreq := outbound.ToOpenAI()
	// Ask the provider to emit a final usage chunk so streamed requests still
	// report real token counts (needed for spend/savings accounting).
	oreq.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
	body, err := json.Marshal(oreq)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("openai adapter: marshal stream request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("openai adapter: build stream request: %w", err)
	}
	a.setCommonHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient(req).Do(httpReq)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		if isTimeoutError(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: stream request timed out", ErrProviderTimeout)
		}
		return nil, fmt.Errorf("openai adapter: stream request failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		if cancel != nil {
			cancel()
		}
		return nil, a.handleErrorStatus(req, resp)
	}

	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		defer resp.Body.Close()
		if cancel != nil {
			defer cancel()
		}

		reader := bufio.NewReader(resp.Body)
		doneSeen := false
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				if chunk, ok := parseSSEDataLine(line); ok {
					if chunk.Done {
						doneSeen = true
					}
					if !sendStreamChunk(ctx, chunks, chunk) {
						return
					}
					if chunk.Done {
						return
					}
				}
			}
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				if !doneSeen {
					sendStreamChunk(ctx, chunks, StreamChunk{Err: fmt.Errorf("%w: stream ended before done frame", ErrStreamInterrupted)})
				}
				return
			}
			sendStreamChunk(ctx, chunks, StreamChunk{Err: fmt.Errorf("%w: read stream: %v", ErrStreamInterrupted, err)})
			return
		}
	}()
	return chunks, nil
}

func sendStreamChunk(ctx context.Context, chunks chan<- StreamChunk, chunk StreamChunk) bool {
	select {
	case chunks <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

func parseSSEDataLine(line string) (StreamChunk, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return StreamChunk{}, false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "[DONE]" {
		return StreamChunk{Done: true}, true
	}
	if payload == "" {
		return StreamChunk{}, false
	}
	chunk := StreamChunk{Data: []byte(payload)}
	// The final usage chunk (requested via stream_options.include_usage) carries
	// token counts; capture them so the router can account streamed requests.
	var u struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(payload), &u) == nil && u.Usage != nil {
		chunk.InputTokens = u.Usage.PromptTokens
		chunk.OutputTokens = u.Usage.CompletionTokens
	}
	return chunk, true
}

// ChatCompletionsURL exposes the adapter's endpoint derivation so out-of-package
// callers (e.g. the provider_probe MCP tool) target exactly what a real request
// would hit, for any base_url shape.
func ChatCompletionsURL(baseURL string) (string, error) { return chatCompletionsURL(baseURL) }

func chatCompletionsURL(baseURL string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("%w: missing base url", ErrProviderBadReq)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base url", ErrProviderBadReq)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: invalid base url", ErrProviderBadReq)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	switch {
	case strings.HasSuffix(basePath, chatCompletionsPath):
		parsed.Path = basePath
	case endsWithAPIVersion(basePath):
		// Base already carries the API version segment (/v1, /v4, ...) — append
		// only the resource. z.ai's coding endpoint is .../paas/v4, not /v4/v1.
		parsed.Path = basePath + "/chat/completions"
	default:
		parsed.Path = basePath + chatCompletionsPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// endsWithAPIVersion reports whether the path's last segment is a version like
// v1, v4, v2beta — so the adapter appends "/chat/completions" instead of the
// default "/v1/chat/completions" and doesn't double up the version.
func endsWithAPIVersion(path string) bool {
	seg := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		seg = path[i+1:]
	}
	if len(seg) < 2 || seg[0] != 'v' || seg[1] < '0' || seg[1] > '9' {
		return false
	}
	return true
}

func mapOpenAIStatus(status int, body io.Reader) error {
	envelope := decodeErrorEnvelope(body)
	if isModelUnavailable(envelope) || status == http.StatusNotFound {
		return fmt.Errorf("%w: provider status %d", ErrModelUnavailable, status)
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w: provider status %d", ErrProviderAuth, status)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: provider status %d", ErrProviderRateLimit, status)
	case status >= 500:
		return fmt.Errorf("%w: provider status %d", ErrProvider5xx, status)
	case status >= 400:
		return fmt.Errorf("%w: provider status %d", ErrProviderBadReq, status)
	default:
		return fmt.Errorf("%w: provider status %d", ErrProviderBadResp, status)
	}
}

func decodeErrorEnvelope(body io.Reader) openai.ErrorEnvelope {
	var envelope openai.ErrorEnvelope
	limited := io.LimitReader(body, 64*1024)
	_ = json.NewDecoder(limited).Decode(&envelope)
	return envelope
}

func isModelUnavailable(envelope openai.ErrorEnvelope) bool {
	errBody := envelope.Error
	haystack := strings.ToLower(strings.Join([]string{
		errBody.Code,
		errBody.Type,
		errBody.Message,
	}, " "))
	return strings.Contains(haystack, "model_not_found") ||
		strings.Contains(haystack, "model_not_available") ||
		strings.Contains(haystack, "model unavailable") ||
		strings.Contains(haystack, "does not exist") ||
		strings.Contains(haystack, "not found")
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
