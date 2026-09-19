package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runModelTestRequest(t *testing.T, opts ModelsOptions, id string) (modelTestResult, int) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/models/test", strings.NewReader("id="+id))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ModelTestHandler(opts)(rec, req)
	var res modelTestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return res, rec.Code
}

// A healthy provider → ✅: ok, HTTP status, latency and the row's exact slug.
func TestModelTestReachable(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	opts := modelsTestOpts(t)
	opts.ProbeClient = &http.Client{Transport: rewriteHost(srv.URL)}

	res, code := runModelTestRequest(t, opts, "cheap-general")
	if code != http.StatusOK || !res.OK {
		t.Fatalf("want ok, got code=%d res=%+v", code, res)
	}
	if res.Slug == "" || res.ProviderID == "" {
		t.Fatalf("result should carry slug + provider: %+v", res)
	}
	// The test call must use the row's exact slug — that is the whole point.
	if !strings.Contains(gotBody, `"model":"`+res.Slug+`"`) {
		t.Fatalf("test call did not send the row's slug %q: %s", res.Slug, gotBody)
	}
	if !strings.Contains(gotBody, `"max_tokens":1`) {
		t.Fatalf("test call should be a 1-token completion: %s", gotBody)
	}
}

// A 400 from the provider → ❌ with the slug diagnosis (the zai/glm-4.6 case).
func TestModelTestWrongSlugDiagnosis(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"1211","message":"Unknown Model"}}`))
	}))
	defer srv.Close()

	opts := modelsTestOpts(t)
	opts.ProbeClient = &http.Client{Transport: rewriteHost(srv.URL)}

	res, _ := runModelTestRequest(t, opts, "cheap-general")
	if res.OK {
		t.Fatalf("a 400 should not read ok: %+v", res)
	}
	if res.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Status)
	}
	if !strings.Contains(res.Diagnosis, "slug") {
		t.Fatalf("400 diagnosis should point at the slug, got %q", res.Diagnosis)
	}
	if !strings.Contains(res.Error, "Unknown Model") {
		t.Fatalf("upstream error body should be surfaced, got %q", res.Error)
	}
}

func TestModelTestUnknownModel(t *testing.T) {
	opts := modelsTestOpts(t)
	res, code := runModelTestRequest(t, opts, "nope")
	if code != http.StatusNotFound || res.OK {
		t.Fatalf("unknown id should 404, got code=%d res=%+v", code, res)
	}
}

// The Models page carries a Test button per row.
func TestModelsPageHasTestButton(t *testing.T) {
	opts := modelsTestOpts(t)
	rec := httptest.NewRecorder()
	ModelsPageHandler(opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/models", nil))
	body := rec.Body.String()
	for _, want := range []string{`data-model="cheap-general"`, "/router/models/test"} {
		if !strings.Contains(body, want) {
			t.Errorf("models page missing %q", want)
		}
	}
}
