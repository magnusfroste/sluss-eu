package main

import (
	"strings"
	"testing"
)

func TestBannerRelease(t *testing.T) {
	var sb strings.Builder
	writeBanner(&sb, bootInfo{Version: "v1.0.0", Commit: "4f5a346", Built: "2026-10-30T21:05:06Z",
		Addr: ":8080", PublicURL: "https://demo.sluss.eu", DataDir: "/data", Providers: "openrouter",
		Deployment: "easypanel", UpdateCheck: true})
	out := sb.String()
	for _, want := range []string{
		"v1.0.0 (commit 4f5a346, built 2026-10-30T21:05:06Z)",
		"https://demo.sluss.eu/router/dashboard",
		"https://demo.sluss.eu/manual",
		"/data", "easypanel", "openrouter", "EU LLM gateway",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("banner lacks %q:\n%s", want, out)
		}
	}
}

func TestBannerMainBuildLocal(t *testing.T) {
	b := bootInfo{Commit: "ce95f1a", Built: "2026-10-30T21:10:00Z", Addr: "0.0.0.0:9090"}
	if got := b.versionLine(); got != "main build ce95f1a (built 2026-10-30T21:10:00Z)" {
		t.Errorf("versionLine = %q", got)
	}
	if got := b.consoleURL(); got != "http://localhost:9090/router/dashboard" {
		t.Errorf("consoleURL = %q", got)
	}
	var sb strings.Builder
	writeBanner(&sb, b)
	out := sb.String()
	for _, want := range []string{"ROUTER_DATA_DIR unset", "ROUTER_UPDATE_CHECK=off", "not set (ROUTER_DEPLOYMENT)"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner lacks %q:\n%s", want, out)
		}
	}
}
