package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/magnusfroste/sluss/internal/decisioncache"
	"github.com/magnusfroste/sluss/internal/openai"
)

func TestChatUnknownPinnedModelIsModelNotFound(t *testing.T) {
	h := engineHandler(t, decisioncache.New(0, 0)) // cache disabled

	rec := postChat(t, h, openai.ChatRequest{
		Model:    "nonexistent-model-xyz",
		Messages: []openai.Message{{Role: "user", Content: "hi"}},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
	var env openai.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if env.Error.Code != "model_not_found" {
		t.Errorf("error code = %q, want model_not_found", env.Error.Code)
	}
}
