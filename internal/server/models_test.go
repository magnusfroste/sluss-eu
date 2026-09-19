package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/server"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func modelsServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	store2 := auth.NewInMemoryKeyStore()
	store2.Add("k", &tenant.Tenant{ID: "tn", Project: "prj", KeyID: "key"})

	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store2,
		Engine:   engine.New(store),
	})
	srv := httptest.NewServer(h)
	return srv, srv.Close
}

func TestModelsEndpointRequiresAuth(t *testing.T) {
	srv, done := modelsServer(t)
	defer done()

	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestModelsEndpointListsModels(t *testing.T) {
	srv, done := modelsServer(t)
	defer done()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer k")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var list openai.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Object != "list" {
		t.Errorf("object = %q, want list", list.Object)
	}

	byID := map[string]openai.ModelObject{}
	for _, m := range list.Data {
		if m.Object != "model" {
			t.Errorf("%s object = %q, want model", m.ID, m.Object)
		}
		byID[m.ID] = m
	}
	for _, want := range []string{"auto", "cheap-general", "balanced-coder", "premium-reasoning"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("models list missing %q (got %v)", want, keys(byID))
		}
	}
	// Registry models advertise their provider; the auto sentinel is router-owned.
	if byID["auto"].OwnedBy != "sluss" {
		t.Errorf("auto owned_by = %q", byID["auto"].OwnedBy)
	}
	if byID["cheap-general"].OwnedBy == "" {
		t.Error("registry model should have owned_by set")
	}
}

func keys(m map[string]openai.ModelObject) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
