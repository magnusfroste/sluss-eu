package decisioncache

import (
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

func lowRiskJob() *router.JobDescriptor {
	return &router.JobDescriptor{
		TenantID:    "tn",
		ProjectID:   "prj",
		TaskType:    router.TaskSimpleChat,
		RiskLevel:   router.RiskLow,
		Sensitivity: router.SensitivityNone,
	}
}

func TestCacheableGating(t *testing.T) {
	if !Cacheable(lowRiskJob()) {
		t.Error("low-risk non-sensitive job should be cacheable")
	}
	high := lowRiskJob()
	high.RiskLevel = router.RiskHigh
	if Cacheable(high) {
		t.Error("high-risk job must not be cacheable")
	}
	sensitive := lowRiskJob()
	sensitive.Sensitivity = router.SensitivitySourceCode
	if Cacheable(sensitive) {
		t.Error("sensitive job must not be cacheable")
	}
	if Cacheable(nil) {
		t.Error("nil job must not be cacheable")
	}
}

func TestKeyIsStableAndVersioned(t *testing.T) {
	req := &openai.ChatRequest{Model: "auto", Messages: []openai.Message{{Role: "user", Content: "hi"}}}
	job := lowRiskJob()

	base := Key(req, job, "pv_1", "rv_1")
	if base != Key(req, job, "pv_1", "rv_1") {
		t.Error("identical inputs must produce identical keys")
	}
	if base == Key(req, job, "pv_2", "rv_1") {
		t.Error("policy version change must change the key (invalidation)")
	}
	if base == Key(req, job, "pv_1", "rv_2") {
		t.Error("registry version change must change the key")
	}

	other := &openai.ChatRequest{Model: "auto", Messages: []openai.Message{{Role: "user", Content: "different"}}}
	if base == Key(other, job, "pv_1", "rv_1") {
		t.Error("different prompt must change the key")
	}

	jt := lowRiskJob()
	jt.TenantID = "tn_other"
	if base == Key(req, jt, "pv_1", "rv_1") {
		t.Error("different tenant must change the key")
	}
}

func TestGetPutAndExpiry(t *testing.T) {
	c := New(time.Minute, 0)
	cur := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return cur }

	dec := engine.RouteDecision{SelectedModel: "cheap-general", SelectedProvider: "openai"}
	c.Put("k", dec)

	got, ok := c.Get("k")
	if !ok || got.SelectedModel != "cheap-general" {
		t.Fatalf("expected hit, got %+v ok=%v", got, ok)
	}

	// Advance past the TTL → miss, and the entry is evicted.
	cur = cur.Add(2 * time.Minute)
	if _, ok := c.Get("k"); ok {
		t.Error("expired entry should miss")
	}
	if c.Len() != 0 {
		t.Errorf("expired entry should be evicted, len=%d", c.Len())
	}
}

func TestDisabledCache(t *testing.T) {
	c := New(0, 0) // TTL 0 disables
	c.Put("k", engine.RouteDecision{SelectedModel: "x"})
	if _, ok := c.Get("k"); ok {
		t.Error("disabled cache must always miss")
	}
	if c.Len() != 0 {
		t.Error("disabled cache must store nothing")
	}
}

func TestNilCacheSafe(t *testing.T) {
	var c *Cache
	if _, ok := c.Get("k"); ok {
		t.Error("nil cache Get should miss")
	}
	c.Put("k", engine.RouteDecision{}) // must not panic
	if c.Len() != 0 {
		t.Error("nil cache Len should be 0")
	}
}

func TestMaxEntriesBound(t *testing.T) {
	c := New(time.Minute, 2)
	c.Put("a", engine.RouteDecision{})
	c.Put("b", engine.RouteDecision{})
	c.Put("c", engine.RouteDecision{}) // triggers clear-then-insert
	if c.Len() > 2 {
		t.Errorf("cache should stay within max, len=%d", c.Len())
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("most recent insert should be present")
	}
}
