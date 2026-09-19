// Package sdk provides client-side helpers for attaching routing hints to a
// request without having to know the router's magic metadata keys and header
// names (ISSUE-048).
//
// A Builder produces either a metadata map (to embed in a chat request body) or
// X-Router-* headers — both forms the router already understands. The hint
// types are aliases of the router's own enums, so there is a single source of
// truth for valid values.
//
// All hints are optional and additive: an empty Builder changes nothing, and
// Apply never overwrites metadata the caller already set, so adopting the
// helper is backward compatible.
package sdk

import (
	"net/http"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

// Hint value types, aliased from the router so callers use one vocabulary.
type (
	Task        = router.TaskType
	Risk        = router.RiskLevel
	Sensitivity = router.Sensitivity
	Latency     = router.LatencyPreference
	Quality     = router.QualityPreference
	Budget      = router.BudgetPreference
	Mode        = router.RouterMode
)

// Re-exported hint values. These mirror the router's wire vocabulary; the
// round-trip test guards against drift if the router renames any of them.
const (
	TaskSimpleChat          = router.TaskSimpleChat
	TaskTrivialGit          = router.TaskTrivialGit
	TaskSimpleShell         = router.TaskSimpleShell
	TaskSummarization       = router.TaskSummarization
	TaskSimpleCodeEdit      = router.TaskSimpleCodeEdit
	TaskHardCodeDebugging   = router.TaskHardCodeDebugging
	TaskSecurityReview      = router.TaskSecurityReview
	TaskDatabaseMigration   = router.TaskDatabaseMigration
	TaskLongContextAnalysis = router.TaskLongContextAnalysis
	TaskCreativeCopy        = router.TaskCreativeCopy
	TaskUnknownHighRisk     = router.TaskUnknownHighRisk

	RiskLow      = router.RiskLow
	RiskMedium   = router.RiskMedium
	RiskHigh     = router.RiskHigh
	RiskCritical = router.RiskCritical

	SensitivityNone            = router.SensitivityNone
	SensitivitySourceCode      = router.SensitivitySourceCode
	SensitivityPII             = router.SensitivityPII
	SensitivitySecretsPossible = router.SensitivitySecretsPossible

	LatencyFast     = router.LatencyFast
	LatencyBalanced = router.LatencyBalanced
	LatencyQuality  = router.LatencyQuality

	QualityCheap    = router.QualityCheap
	QualityBalanced = router.QualityBalanced
	QualityHigh     = router.QualityHigh

	BudgetCheap  = router.BudgetCheap
	BudgetNormal = router.BudgetNormal
	BudgetHigh   = router.BudgetHigh

	ModeAuto     = router.RouterModeAuto
	ModeCheap    = router.RouterModeCheap
	ModeBalanced = router.RouterModeBalanced
	ModePremium  = router.RouterModePremium
	ModeDisabled = router.RouterModeDisabled
)

// Canonical metadata keys the router reads. Kept private; callers go through the
// typed setters.
const (
	keyProject     = "project_id"
	keyTenant      = "tenant_id"
	keyTask        = "task_type"
	keyRisk        = "risk_level"
	keySensitivity = "sensitivity"
	keyLatency     = "latency_preference"
	keyQuality     = "quality_preference"
	keyBudget      = "budget_preference"
	keyMode        = "router_mode"
	keyJSONSchema  = "requires_json_schema"
)

// headerForKey maps a canonical metadata key to the X-Router-* header the router
// also accepts. Keys absent here (e.g. requires_json_schema) are metadata-only.
var headerForKey = map[string]string{
	keyProject:     "X-Router-Project-Id",
	keyTenant:      "X-Router-Tenant-Id",
	keyTask:        "X-Router-Task-Type",
	keyRisk:        "X-Router-Risk-Level",
	keySensitivity: "X-Router-Sensitivity",
	keyLatency:     "X-Router-Latency-Preference",
	keyQuality:     "X-Router-Quality-Preference",
	keyBudget:      "X-Router-Budget-Preference",
	keyMode:        "X-Router-Mode",
}

// Builder accumulates routing hints. The zero value is not usable; call New.
type Builder struct {
	vals  map[string]string
	flags map[string]bool
	extra map[string]any
}

// New returns an empty Builder.
func New() *Builder {
	return &Builder{
		vals:  map[string]string{},
		flags: map[string]bool{},
		extra: map[string]any{},
	}
}

func (b *Builder) set(key, val string) *Builder {
	if val != "" {
		b.vals[key] = val
	}
	return b
}

// Project sets the project hint. Used when no authenticated project is present.
func (b *Builder) Project(id string) *Builder { return b.set(keyProject, id) }

// Tenant sets the tenant hint. Used when no authenticated tenant is present.
func (b *Builder) Tenant(id string) *Builder { return b.set(keyTenant, id) }

// Task hints the task type.
func (b *Builder) Task(t Task) *Builder { return b.set(keyTask, string(t)) }

// Risk hints the risk level.
func (b *Builder) Risk(r Risk) *Builder { return b.set(keyRisk, string(r)) }

// Sensitivity hints the data sensitivity.
func (b *Builder) Sensitivity(s Sensitivity) *Builder { return b.set(keySensitivity, string(s)) }

// Latency hints the latency preference.
func (b *Builder) Latency(l Latency) *Builder { return b.set(keyLatency, string(l)) }

// Quality hints the quality preference.
func (b *Builder) Quality(q Quality) *Builder { return b.set(keyQuality, string(q)) }

// Budget hints the budget preference.
func (b *Builder) Budget(bg Budget) *Builder { return b.set(keyBudget, string(bg)) }

// Mode forces a router mode (auto/cheap/balanced/premium/disabled).
func (b *Builder) Mode(m Mode) *Builder { return b.set(keyMode, string(m)) }

// RequiresJSONSchema marks the request as needing structured JSON output.
func (b *Builder) RequiresJSONSchema(v bool) *Builder {
	b.flags[keyJSONSchema] = v
	return b
}

// Set is an escape hatch for hints not yet covered by a typed setter. The value
// is carried in metadata only.
func (b *Builder) Set(key string, val any) *Builder {
	if key != "" {
		b.extra[key] = val
	}
	return b
}

// Build returns the hints as a metadata map suitable for ChatRequest.Metadata.
// The returned map is freshly allocated and owned by the caller.
func (b *Builder) Build() map[string]any {
	out := make(map[string]any, len(b.vals)+len(b.flags)+len(b.extra))
	for k, v := range b.vals {
		out[k] = v
	}
	for k, v := range b.flags {
		out[k] = v
	}
	for k, v := range b.extra {
		out[k] = v
	}
	return out
}

// Headers returns the hints as X-Router-* headers. Hints with no header form
// (requires_json_schema and Set escape-hatch values) are omitted — use Build
// for those.
func (b *Builder) Headers() http.Header {
	h := http.Header{}
	for k, v := range b.vals {
		if hk, ok := headerForKey[k]; ok {
			h.Set(hk, v)
		}
	}
	return h
}

// Apply merges the hints into req.Metadata without overwriting keys the caller
// already set. A nil req or empty Builder is a no-op, so it is safe to call
// unconditionally and backward compatible.
func (b *Builder) Apply(req *openai.ChatRequest) {
	if req == nil {
		return
	}
	hints := b.Build()
	if len(hints) == 0 {
		return
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]any, len(hints))
	}
	for k, v := range hints {
		if _, exists := req.Metadata[k]; !exists {
			req.Metadata[k] = v
		}
	}
}
