package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestInspectorHeadersAreASCIITokens(t *testing.T) {
	rec := httptest.NewRecorder()
	job := &router.JobDescriptor{RiskLevel: "high"}
	setInspectorHeaders(rec, job, engine.RouteDecision{
		SelectedProvider: "dgx",
		MatchedRules:     []string{"pii-local-only", "bad rule\r\nX-Evil: 1", "ö-rule"},
	})
	h := rec.Header()
	if h.Get("X-Router-Risk") != "high" || h.Get("X-Router-Selected-Provider") != "dgx" {
		t.Fatalf("headers = %v", h)
	}
	if got := h.Get("X-Router-Policy-Rules"); got != "pii-local-only,badruleX-Evil:1,-rule" {
		t.Fatalf("rules = %q", got)
	}
}

func TestChatRequestInfoOnlyAnswersTheCallersOwnRequest(t *testing.T) {
	rl := eventlog.NewRequestLogTracker(0)
	rl.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: "req_1", SelectedModel: "glm-local", PromptTokens: 812, EstimatedCostUSD: 0.002,
		Actor: "anna@example.eu", DecidedAt: time.Now()}})
	get := func(user string) chatRequestInfo {
		req := httptest.NewRequest(http.MethodGet, "/chat/request?id=req_1", nil)
		if user != "" {
			req = req.WithContext(withAdminUser(req.Context(), user))
		}
		rec := httptest.NewRecorder()
		ChatRequestInfoHandler(nil, rl)(rec, req)
		var out chatRequestInfo
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	if got := get("anna@example.eu"); !got.Found || got.InputTokens != 812 || got.Model != "glm-local" {
		t.Fatalf("own request = %+v", got)
	}
	if got := get("bob@example.eu"); got.Found {
		t.Fatalf("someone else's request must not be answered: %+v", got)
	}
	if got := get(""); got.Found {
		t.Fatalf("anonymous caller must not be answered: %+v", got)
	}
}
