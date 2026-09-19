package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// /chat/models lists routable models with tier, egress and output price — the
// data the admin's model picker needs to make the cost/CO₂ trade visible.
func TestChatModelsHandler(t *testing.T) {
	eng := localRegistryStore(t)
	rec := httptest.NewRecorder()
	ChatModelsHandler(eng)(rec, httptest.NewRequest(http.MethodGet, "/chat/models", nil))
	var got []chatModelOption
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	byID := map[string]chatModelOption{}
	for _, m := range got {
		byID[m.ID] = m
	}
	if len(byID) != 3 {
		t.Fatalf("want 3 models, got %+v", got)
	}
	if byID["local-model"].Egress != "local" || byID["private-model"].Egress != "local" {
		t.Fatalf("local/private-tagged models should read egress=local: %+v", got)
	}
	if byID["cloud-model"].Egress != "cloud" {
		t.Fatalf("untagged model should read egress=cloud: %+v", got)
	}
	if byID["cloud-model"].Tier != "balanced" || byID["cloud-model"].OutUSDPerMTok != 1.0 {
		t.Fatalf("tier/price wrong: %+v", byID["cloud-model"])
	}
}

func TestChatModelsHandlerNilEngine(t *testing.T) {
	rec := httptest.NewRecorder()
	ChatModelsHandler(nil)(rec, httptest.NewRequest(http.MethodGet, "/chat/models", nil))
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("nil engine should return an empty list, got %q", body)
	}
}

// The model picker is an admin control-panel feature: present on the admin chat
// page, absent on the shareable guest view (guests always route with Auto).
func TestModelPickerAdminOnly(t *testing.T) {
	if !strings.Contains(demoChatHTML, `id="modelsel"`) {
		t.Error("admin chat page should carry the model picker")
	}
	if !strings.Contains(demoChatHTML, "Auto (router)") {
		t.Error("picker should default to Auto (router)")
	}
	if strings.Contains(demoChatHTMLGuest, `id="modelsel"`) {
		t.Error("guest chat page must NOT carry the model picker")
	}
	// The pinned-mode copy: classification bypassed, policy still enforced.
	if !strings.Contains(demoChatHTML, "policy is still enforced") {
		t.Error("pinned hint should say policy is still enforced")
	}
}
