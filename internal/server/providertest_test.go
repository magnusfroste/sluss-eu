package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/providercfg"
)

// labProvider serves /models listing only "mimo", and answers chat calls with
// 404 for anything else — the DGX case after the lab swapped the model.
func labProvider(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/models") {
			_, _ = w.Write([]byte(`{"data":[{"id":"mimo"}]}`))
			return
		}
		var body struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "mimo" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func labOpts(t *testing.T, srv *httptest.Server) ModelsOptions {
	t.Helper()
	store := newUserStore(t)
	_ = store.UpsertRosterProvider(providercfg.Provider{ID: "dgx", Name: "DGX", BaseURL: srv.URL + "/v1", ComplianceTags: []string{"local"}})
	_ = store.UpsertRosterModel(providercfg.Model{ID: "glm-local", ProviderID: "dgx", ProviderModelID: "glm-5.3-flash", Tier: "balanced", Enabled: true})
	return ModelsOptions{Roster: store}
}

func postFormRaw(h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h(rec, req)
	return rec
}

func TestProviderTestFlagsARenamedModel(t *testing.T) {
	opts := labOpts(t, labProvider(t))
	rec := postFormRaw(ProviderTestHandler(opts), "/router/providers/test", "id=dgx")
	var res providerTestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v %s", err, rec.Body.String())
	}
	if len(res.Served) != 1 || res.Served[0] != "mimo" {
		t.Fatalf("served = %v, want [mimo]", res.Served)
	}
	if len(res.Models) != 1 || res.Models[0].OK || res.Models[0].Slug != "glm-5.3-flash" {
		t.Fatalf("the stale model should fail its live test: %+v", res.Models)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "glm-5.3-flash" {
		t.Fatalf("missing = %v, want the stale slug", res.Missing)
	}
	if postFormRaw(ProviderTestHandler(opts), "/router/providers/test", "id=nope").Code != http.StatusNotFound {
		t.Fatal("unknown provider should 404")
	}
}

func TestUseServedSlugFixesTheModel(t *testing.T) {
	opts := labOpts(t, labProvider(t))
	// A slug the provider does not serve is refused — no typos saved.
	if rec := postFormRaw(ModelSlugHandler(opts), "/router/models/slug", "id=glm-local&slug=mim0"); rec.Code != http.StatusBadRequest {
		t.Fatalf("unserved slug: want 400, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := postFormRaw(ModelSlugHandler(opts), "/router/models/slug", "id=glm-local&slug=mimo"); rec.Code != http.StatusOK {
		t.Fatalf("served slug: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	ms, _ := opts.Roster.LoadRosterModels()
	if ms[0].ProviderModelID != "mimo" {
		t.Fatalf("slug not saved: %+v", ms[0])
	}
	var res providerTestResult
	_ = json.Unmarshal(postFormRaw(ProviderTestHandler(opts), "/router/providers/test", "id=dgx").Body.Bytes(), &res)
	if len(res.Models) != 1 || !res.Models[0].OK || len(res.Missing) != 0 {
		t.Fatalf("after the switch the test should pass: %+v", res)
	}
}
