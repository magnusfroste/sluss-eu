package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
)

// decisionResult decodes only the fields routerctl renders from the
// /router/decision response. It deliberately mirrors a subset of the server's
// contract (engine.RouteDecision) rather than importing it: a client decodes
// the parts of a contract it cares about.
type decisionResult struct {
	SelectedModel    string   `json:"selected_model"`
	SelectedProvider string   `json:"selected_provider"`
	ProviderModelID  string   `json:"provider_model_id"`
	PolicyVersion    string   `json:"policy_version"`
	TimeoutMS        int      `json:"timeout_ms"`
	RequiresVerifier bool     `json:"requires_verifier"`
	DecisionReasons  []string `json:"decision_reasons"`
	Fallbacks        []struct {
		ModelID    string `json:"model_id"`
		ProviderID string `json:"provider_id"`
	} `json:"fallbacks"`
	Blocked     bool   `json:"blocked"`
	BlockCode   string `json:"block_code"`
	BlockReason string `json:"block_reason"`
}

// fetchDecision posts a dry-run request to /router/decision and returns the
// parsed decision. A policy-blocked decision is returned as a normal result
// (Blocked=true), not an error; only auth/transport/router errors are errors.
func fetchDecision(ctx context.Context, client *http.Client, baseURL, apiKey string, req *openai.ChatRequest, explain bool) (decisionResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return decisionResult{}, fmt.Errorf("encode request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/router/decision"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return decisionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if explain {
		httpReq.Header.Set("X-Router-Explain", "true")
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return decisionResult{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var out decisionResult
	decodeErr := json.Unmarshal(raw, &out)

	// A successful decision (200) or a policy block (non-200 but carries a
	// decision body) is a valid result. Everything else is an error.
	if resp.StatusCode == http.StatusOK || (decodeErr == nil && out.Blocked) {
		if decodeErr != nil {
			return decisionResult{}, fmt.Errorf("decode decision: %w", decodeErr)
		}
		return out, nil
	}
	return decisionResult{}, fmt.Errorf("router returned %d: %s", resp.StatusCode, errorMessage(raw))
}

// errorMessage extracts the message from an OpenAI-style error envelope, falling
// back to the raw (trimmed) body.
func errorMessage(raw []byte) string {
	var env openai.ErrorEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Error.Message != "" {
		return env.Error.Message
	}
	return strings.TrimSpace(string(raw))
}

// render writes a human-readable summary of a decision to w.
func render(w io.Writer, d decisionResult) {
	if d.Blocked {
		fmt.Fprintf(w, "BLOCKED: %s — %s\n", d.BlockCode, d.BlockReason)
		return
	}
	fmt.Fprintf(w, "Selected model:    %s\n", d.SelectedModel)
	fmt.Fprintf(w, "Selected provider: %s\n", d.SelectedProvider)
	if d.ProviderModelID != "" {
		fmt.Fprintf(w, "Provider model id: %s\n", d.ProviderModelID)
	}
	if d.PolicyVersion != "" {
		fmt.Fprintf(w, "Policy version:    %s\n", d.PolicyVersion)
	}
	fmt.Fprintf(w, "Timeout:           %d ms\n", d.TimeoutMS)
	fmt.Fprintf(w, "Requires verifier: %t\n", d.RequiresVerifier)

	if len(d.Fallbacks) > 0 {
		fmt.Fprintln(w, "Fallback chain:")
		for i, f := range d.Fallbacks {
			fmt.Fprintf(w, "  %d. %s (%s)\n", i+1, f.ModelID, f.ProviderID)
		}
	}
	if len(d.DecisionReasons) > 0 {
		fmt.Fprintln(w, "Explanations:")
		for _, r := range d.DecisionReasons {
			fmt.Fprintf(w, "  - %s\n", r)
		}
	}
}
