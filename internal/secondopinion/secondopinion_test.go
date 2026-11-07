package secondopinion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// fakeModel answers like an OpenAI-compatible server with a fixed text, after
// an optional delay, and remembers the last request so the test can see what
// was sent.
func fakeModel(t *testing.T, answer string, delay time.Duration) (*httptest.Server, *openai.ChatRequest) {
	t.Helper()
	var last openai.ChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&last)
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{
			ID: "x", Object: "chat.completion", Model: last.Model,
			Choices: []openai.Choice{{Message: openai.Message{Role: "assistant", Content: answer}, FinishReason: "stop"}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func rosterWith(t *testing.T, srv *httptest.Server, modelTags []string) (*registry.Store, func() map[string]provider.Adapter) {
	t.Helper()
	def := registry.Definition{
		RegistryVersion: "test",
		CreatedAt:       time.Now(),
		Providers: []registry.Provider{
			{ID: "gpu", Name: "GPU box", Status: registry.ProviderStatusActive, BaseURL: srv.URL, ComplianceTags: modelTags},
		},
		Models: []registry.Model{{
			ID: "small-local", ProviderID: "gpu", ProviderModelID: "qwen-small", Tier: registry.TierCheap, Enabled: true,
			Capabilities: registry.Capabilities{Chat: true}, ContextWindowTokens: 8000,
		}},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	adapters := map[string]provider.Adapter{"gpu": &provider.OpenAIAdapter{BaseURL: srv.URL, Client: srv.Client(), ProviderID: "gpu"}}
	return store, func() map[string]provider.Adapter { return adapters }
}

func plainJob() *router.JobDescriptor {
	return router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{Model: "auto",
		Messages: []openai.Message{{Role: "user", Content: "Write a kind note: Erik is away until March because of what happened in the spring."}}}})
}

func TestSecondOpinionRaisesButNeverLowers(t *testing.T) {
	srv, last := fakeModel(t, `["health","pii"]`, 0)
	store, adapters := rosterWith(t, srv, []string{"local", "on-prem"})
	c := New(Config{ModelID: "small-local"}, store, adapters, nil)

	job := plainJob()
	if job.Sensitivity != router.SensitivityNone {
		t.Fatalf("precondition: the rules see nothing here, got %q", job.Sensitivity)
	}
	res := c.Apply(context.Background(), job, []openai.Message{{Role: "user", Content: "Erik is away until March because of what happened in the spring."}})
	if res.Err != nil || res.Skipped != "" {
		t.Fatalf("unexpected: %+v", res)
	}
	if strings.Join(res.Added, ",") != "pii,health" {
		t.Fatalf("added = %v, want pii then health (strongest first)", res.Added)
	}
	if job.Sensitivity != router.SensitivityPII || job.RiskLevel != router.RiskMedium {
		t.Fatalf("job should be raised to pii/medium, got %s/%s", job.Sensitivity, job.RiskLevel)
	}
	if job.SecondOpinionModel != "small-local" || len(job.SecondOpinion) != 2 {
		t.Fatalf("second opinion not recorded on the job: %+v", job)
	}
	if last.Model != "qwen-small" || len(last.Messages) != 2 || last.Messages[0].Role != "system" {
		t.Fatalf("the model should get the system prompt and the text under its provider slug: %+v", last)
	}

	// A job the rules already rate as secrets/high stays exactly that when
	// the model answers with something weaker or nothing at all.
	srv2, _ := fakeModel(t, `[]`, 0)
	store2, adapters2 := rosterWith(t, srv2, []string{"local"})
	c2 := New(Config{ModelID: "small-local"}, store2, adapters2, nil)
	strict := router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{Model: "auto",
		Messages: []openai.Message{{Role: "user", Content: "rotate the api key, the secret leaked"}}}})
	before, beforeRisk := strict.Sensitivity, strict.RiskLevel
	res = c2.Apply(context.Background(), strict, []openai.Message{{Role: "user", Content: "rotate the api key, the secret leaked"}})
	if len(res.Added) != 0 || strict.Sensitivity != before || strict.RiskLevel != beforeRisk {
		t.Fatalf("a silent second opinion must change nothing: %+v %s/%s", res, strict.Sensitivity, strict.RiskLevel)
	}
}

func TestSecondOpinionRefusesACloudModel(t *testing.T) {
	srv, last := fakeModel(t, `["pii"]`, 0)
	store, adapters := rosterWith(t, srv, []string{"cloud", "eu-resident"})
	c := New(Config{ModelID: "small-local"}, store, adapters, nil)
	job := plainJob()
	res := c.Apply(context.Background(), job, []openai.Message{{Role: "user", Content: "hello"}})
	if !strings.Contains(res.Skipped, "not tagged local") || len(res.Added) != 0 || last.Model != "" {
		t.Fatalf("a model without a local tag must never be called: %+v (called=%q)", res, last.Model)
	}
}

func TestSecondOpinionTimesOutQuietly(t *testing.T) {
	srv, _ := fakeModel(t, `["pii"]`, 300*time.Millisecond)
	store, adapters := rosterWith(t, srv, []string{"local"})
	c := New(Config{ModelID: "small-local", Timeout: 50 * time.Millisecond}, store, adapters, nil)
	job := plainJob()
	start := time.Now()
	res := c.Apply(context.Background(), job, []openai.Message{{Role: "user", Content: "hello"}})
	if res.Err == nil || len(res.Added) != 0 || job.Sensitivity != router.SensitivityNone {
		t.Fatalf("a timeout must leave the job as the rules left it: %+v", res)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("the timeout did not bound the call: %v", time.Since(start))
	}
}

func TestParseLabelsIsStrict(t *testing.T) {
	got := parseLabels(`Sure! ["health", "financial"] — the text mentions a classified piiza.`)
	if strings.Join(got, ",") != "health,financial" {
		t.Fatalf("parseLabels = %v", got)
	}
	if len(parseLabels("[]")) != 0 || len(parseLabels("nothing sensitive")) != 0 {
		t.Fatalf("empty answers must yield nothing")
	}
	if got := parseLabels(`security_classified`); strings.Join(got, ",") != "security_classified" {
		t.Fatalf("security_classified alone: %v", got)
	}
}

func TestNilClassifierIsOff(t *testing.T) {
	var c *Classifier
	if c.Enabled() {
		t.Fatal("nil must be off")
	}
	job := plainJob()
	if res := c.Apply(context.Background(), job, nil); res.Skipped != "off" || job.Sensitivity != router.SensitivityNone {
		t.Fatalf("nil must change nothing: %+v", res)
	}
	if New(Config{}, nil, nil, nil) != nil {
		t.Fatal("no model id must give nil")
	}
}
