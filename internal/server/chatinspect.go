package server

// Chat inspector data (ISSUE-145): after a streamed answer the token counts
// and cost are not in the response headers yet (they are known only when the
// stream ends), so the inspector asks for them by request id. Only the
// caller's own requests are answered — the record's actor must be the signed-
// in person or demo link — and only counts and the model, never content,
// tenant or person.

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
)

type chatRequestInfo struct {
	Found        bool    `json:"found"`
	Model        string  `json:"model,omitempty"`
	ProviderSlug string  `json:"provider_model_id,omitempty"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// ChatRequestInfoHandler serves GET /chat/request?id=<request id>.
func ChatRequestInfoHandler(h *history.Store, rl *eventlog.RequestLogTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		out := chatRequestInfo{}
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		var rows []eventlog.RequestLogRecord
		if h != nil {
			rows = h.Recent(500)
		} else if rl != nil {
			rows = rl.Recent(500)
		}
		me := AdminUserFromContext(r.Context())
		for _, rec := range rows {
			if rec.RequestID != id {
				continue
			}
			if me == "" || rec.Actor != me {
				break // not the caller's request: answer as if unknown
			}
			out = chatRequestInfo{Found: true, Model: rec.Model, ProviderSlug: rec.ProviderModelID,
				InputTokens: rec.InputTokens, OutputTokens: rec.OutputTokens, CostUSD: rec.CostUSD}
			break
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}
