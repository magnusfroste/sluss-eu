// Package retention implements per-tenant log-retention settings and a cleanup
// sweeper that purges expired log rows (ISSUE-045).
//
// It also owns the prompt-logging switch: prompt content is never logged unless
// explicitly enabled, and the switch can be turned off globally or per tenant.
// All reads are lock-free and side-effect free, so consulting settings on the
// request path never touches the routing latency budget.
package retention

import (
	"sort"
	"time"
)

// DefaultRetentionDays is the fallback retention window when none is configured.
// It matches the tenants.retention_days default in migration 001.
const DefaultRetentionDays = 30

// TenantSettings overrides retention behaviour for one tenant. A zero
// RetentionDays inherits the global default; a nil PromptLogging inherits the
// global prompt-logging default.
type TenantSettings struct {
	RetentionDays int
	PromptLogging *bool
}

// Settings holds global defaults plus per-tenant overrides. Construct with
// NewSettings and apply overrides via SetTenant before serving traffic; once
// in use it is read-only and safe for concurrent reads.
type Settings struct {
	defaultDays          int
	defaultPromptLogging bool
	tenants              map[string]TenantSettings
}

// NewSettings builds settings with the given global defaults. A non-positive
// defaultDays falls back to DefaultRetentionDays.
func NewSettings(defaultDays int, defaultPromptLogging bool) *Settings {
	if defaultDays <= 0 {
		defaultDays = DefaultRetentionDays
	}
	return &Settings{
		defaultDays:          defaultDays,
		defaultPromptLogging: defaultPromptLogging,
		tenants:              make(map[string]TenantSettings),
	}
}

// SetTenant records a per-tenant override. Call during setup only.
func (s *Settings) SetTenant(tenantID string, ts TenantSettings) {
	if s == nil || tenantID == "" {
		return
	}
	s.tenants[tenantID] = ts
}

// RetentionDays returns the effective retention window for a tenant.
func (s *Settings) RetentionDays(tenantID string) int {
	if s == nil {
		return DefaultRetentionDays
	}
	if ts, ok := s.tenants[tenantID]; ok && ts.RetentionDays > 0 {
		return ts.RetentionDays
	}
	return s.defaultDays
}

// PromptLoggingEnabled reports whether prompt content may be logged for a
// tenant. It defaults to off unless a global or per-tenant override enables it.
func (s *Settings) PromptLoggingEnabled(tenantID string) bool {
	if s == nil {
		return false
	}
	if ts, ok := s.tenants[tenantID]; ok && ts.PromptLogging != nil {
		return *ts.PromptLogging
	}
	return s.defaultPromptLogging
}

// Cutoff returns the timestamp before which a tenant's rows are expired. An
// empty tenantID yields the default-retention cutoff.
func (s *Settings) Cutoff(tenantID string, now time.Time) time.Time {
	return now.AddDate(0, 0, -s.RetentionDays(tenantID))
}

// TenantIDs returns the tenants with explicit overrides, sorted for stable
// sweep ordering.
func (s *Settings) TenantIDs() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.tenants))
	for id := range s.tenants {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
