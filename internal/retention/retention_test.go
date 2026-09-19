package retention

import (
	"context"
	"testing"
	"time"
)

func boolPtr(b bool) *bool { return &b }

func TestSettingsDefaultsAndOverrides(t *testing.T) {
	s := NewSettings(0, false) // 0 → DefaultRetentionDays
	if s.RetentionDays("") != DefaultRetentionDays {
		t.Errorf("default days = %d, want %d", s.RetentionDays(""), DefaultRetentionDays)
	}
	if s.PromptLoggingEnabled("anyone") {
		t.Error("prompt logging should default to off")
	}

	s.SetTenant("tn_keep", TenantSettings{RetentionDays: 90, PromptLogging: boolPtr(true)})
	s.SetTenant("tn_strict", TenantSettings{RetentionDays: 7})

	if got := s.RetentionDays("tn_keep"); got != 90 {
		t.Errorf("tn_keep days = %d, want 90", got)
	}
	if !s.PromptLoggingEnabled("tn_keep") {
		t.Error("tn_keep should have prompt logging on")
	}
	if got := s.RetentionDays("tn_strict"); got != 7 {
		t.Errorf("tn_strict days = %d, want 7", got)
	}
	if s.PromptLoggingEnabled("tn_strict") {
		t.Error("tn_strict should inherit prompt logging off")
	}
	if got := s.RetentionDays("tn_unknown"); got != DefaultRetentionDays {
		t.Errorf("unknown tenant days = %d, want default", got)
	}
}

func TestSettingsGlobalPromptLoggingDefaultOn(t *testing.T) {
	s := NewSettings(30, true)
	if !s.PromptLoggingEnabled("tn_x") {
		t.Error("global default should enable prompt logging")
	}
	s.SetTenant("tn_off", TenantSettings{PromptLogging: boolPtr(false)})
	if s.PromptLoggingEnabled("tn_off") {
		t.Error("per-tenant override should disable prompt logging")
	}
}

func TestSettingsCutoff(t *testing.T) {
	s := NewSettings(10, false)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	want := now.AddDate(0, 0, -10)
	if got := s.Cutoff("", now); !got.Equal(want) {
		t.Errorf("cutoff = %v, want %v", got, want)
	}
}

func TestNilSettingsAreSafe(t *testing.T) {
	var s *Settings
	if s.RetentionDays("x") != DefaultRetentionDays {
		t.Error("nil settings should return default days")
	}
	if s.PromptLoggingEnabled("x") {
		t.Error("nil settings should report prompt logging off")
	}
	if len(s.TenantIDs()) != 0 {
		t.Error("nil settings should have no tenant ids")
	}
}

// memoryPurger records purge requests and reports a fixed row count.
type memoryPurger struct {
	requests []PurgeRequest
	rows     int64
}

func (m *memoryPurger) Purge(_ context.Context, req PurgeRequest) (int64, error) {
	m.requests = append(m.requests, req)
	return m.rows, nil
}

func TestCleanerSweepBuildsPerTenantAndDefaultPlan(t *testing.T) {
	s := NewSettings(30, false)
	s.SetTenant("tn_a", TenantSettings{RetentionDays: 7})
	s.SetTenant("tn_b", TenantSettings{RetentionDays: 90})

	mp := &memoryPurger{rows: 3}
	cleaner := NewCleaner(s, mp, nil)
	now := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

	res := cleaner.Sweep(context.Background(), now)

	// 2 tenant tables × (2 overrides + 1 default) + 1 no-tenant table × 1 = 7.
	if res.Requests != 7 {
		t.Fatalf("requests = %d, want 7", res.Requests)
	}
	if res.Errors != 0 {
		t.Fatalf("errors = %d, want 0", res.Errors)
	}
	if res.RowsPurged != 7*3 {
		t.Fatalf("rows purged = %d, want 21", res.RowsPurged)
	}

	// Verify the per-tenant cutoff honours the tenant's own retention window.
	var sawTenantA bool
	for _, req := range mp.requests {
		if req.Table == "request_logs" && req.TenantID == "tn_a" {
			sawTenantA = true
			if !req.Cutoff.Equal(now.AddDate(0, 0, -7)) {
				t.Errorf("tn_a cutoff = %v, want -7d", req.Cutoff)
			}
		}
		// The default sweep must exclude the override tenants.
		if req.Table == "request_logs" && req.TenantID == "" {
			if len(req.ExcludeTenants) != 2 {
				t.Errorf("default sweep should exclude 2 tenants, got %v", req.ExcludeTenants)
			}
		}
	}
	if !sawTenantA {
		t.Error("expected a per-tenant request_logs sweep for tn_a")
	}
}

func TestCleanerNoTenantTableSweptByTimeOnly(t *testing.T) {
	s := NewSettings(30, false)
	s.SetTenant("tn_a", TenantSettings{RetentionDays: 7})
	mp := &memoryPurger{}
	cleaner := NewCleaner(s, mp, nil)
	cleaner.Sweep(context.Background(), time.Now())

	for _, req := range mp.requests {
		if req.Table == "route_attempts" {
			if req.TenantID != "" || len(req.ExcludeTenants) != 0 {
				t.Errorf("route_attempts sweep should be tenant-agnostic, got %+v", req)
			}
			if req.TimestampColumn != "attempted_at" {
				t.Errorf("route_attempts timestamp column = %q, want attempted_at", req.TimestampColumn)
			}
		}
	}
}

// errPurger always fails, to exercise the cleaner's error counting.
type errPurger struct{}

func (errPurger) Purge(context.Context, PurgeRequest) (int64, error) {
	return 0, context.DeadlineExceeded
}

func TestCleanerCountsErrorsWithoutAborting(t *testing.T) {
	s := NewSettings(30, false)
	cleaner := NewCleaner(s, errPurger{}, nil)
	res := cleaner.Sweep(context.Background(), time.Now())
	if res.Requests == 0 || res.Errors != res.Requests {
		t.Fatalf("expected all requests to error, got requests=%d errors=%d", res.Requests, res.Errors)
	}
	if res.RowsPurged != 0 {
		t.Fatalf("rows purged = %d, want 0", res.RowsPurged)
	}
}

func TestNilCleanerSweepIsNoop(t *testing.T) {
	var c *Cleaner
	if got := c.Sweep(context.Background(), time.Now()); got.Requests != 0 {
		t.Errorf("nil cleaner should do nothing, got %+v", got)
	}
}
