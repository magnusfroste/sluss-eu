package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

func TestModelServedListsWhatTheProviderServes(t *testing.T) {
	opts := labOpts(t, labProvider(t))
	rec := postFormRaw(ModelServedHandler(opts), "/router/models/served", "id=glm-local")
	var out struct {
		Slug   string   `json:"slug"`
		Served []string `json:"served"`
		Error  string   `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v %s", err, rec.Body.String())
	}
	if out.Error != "" || out.Slug != "glm-5.3-flash" || len(out.Served) != 1 || out.Served[0] != "mimo" {
		t.Fatalf("served = %+v", out)
	}
	if rec := postFormRaw(ModelServedHandler(opts), "/router/models/served", "id=nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown model: status %d, want 404", rec.Code)
	}
}

func TestModelActivityCountsOnlyThisModel(t *testing.T) {
	now := time.Now()
	reqs := eventlog.NewRequestLogTracker(0)
	ring := eventlog.NewErrorRing(10)
	ctx := context.Background()
	for i, m := range []string{"glm-local", "glm-local", "other", "glm-local"} {
		reqs.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: "r" + itoa(i), SelectedModel: m, TaskType: "coding", Sensitivity: "none",
			PromptTokens: 10, EstimatedCostUSD: 0.01, DecidedAt: now.Add(-time.Minute), Blocked: i == 3,
		}})
	}
	ring.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		ProviderID: "dgx", ModelID: "glm-local", ErrorCode: "model_unavailable", AttemptedAt: now.Add(-time.Minute)}})
	ring.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		ProviderID: "dgx", ModelID: "glm-local", ErrorCode: "timeout", AttemptedAt: now.Add(-48 * time.Hour)}})

	opts := ModelsOptions{RequestLog: reqs, ErrorRing: ring}
	rec := httptest.NewRecorder()
	ModelActivityHandler(opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/models/activity?id=glm-local", nil))
	var a modelActivity
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v %s", err, rec.Body.String())
	}
	// Two routed requests (the blocked one never reached the model), one
	// failure inside the window (the 48h-old one is outside it).
	if a.Count != 2 || len(a.Recent) != 2 || a.Failed != 1 || a.Durable {
		t.Fatalf("activity = %+v", a)
	}
	if a.Recent[0].Task != taskLabel("coding") {
		t.Errorf("task label = %q", a.Recent[0].Task)
	}
}

func TestModelsPageRendersOnePanelPerModel(t *testing.T) {
	opts := modelsTestOpts(t)
	rec := httptest.NewRecorder()
	ModelsPageHandler(opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/models", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`id="mp-cheap-general"`, `role="dialog"`, `name="panel" value="cheap-general"`,
		`/router/log?model=cheap-general`, "Does it answer?", "Last 24 hours", `data-model="cheap-general" tabindex="0"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(body, "editrow") {
		t.Error("the old inline edit row is still rendered")
	}
}

func TestModelSaveFromPanelReopensIt(t *testing.T) {
	opts := modelsTestOpts(t)
	form := url.Values{
		"id": {"cheap-general"}, "provider_id": {"openrouter"}, "provider_model_id": {"x/y"},
		"tier": {"cheap"}, "enabled": {"on"}, "panel": {"cheap-general"},
	}
	rec := postFormRaw(ModelsAddHandler(opts), "/router/models", form.Encode())
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "&m=cheap-general") {
		t.Fatalf("redirect = %q, want the panel reopened", loc)
	}
}

func TestLogPageFiltersByModel(t *testing.T) {
	reqs := eventlog.NewRequestLogTracker(0)
	for i, m := range []string{"glm-local", "other-model"} {
		reqs.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: "r" + itoa(i), SelectedModel: m, TaskType: "coding", DecidedAt: time.Now()}})
	}
	rec := httptest.NewRecorder()
	LogPageHandler(LogOptions{RequestLog: reqs}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/log?model=glm-local", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "model glm-local ×") || strings.Contains(body, "other-model") {
		t.Fatalf("log not filtered to glm-local")
	}
}
