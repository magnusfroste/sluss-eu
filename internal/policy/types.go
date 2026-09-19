// Package policy implements the Policy DSL v1 parser, validator and
// (in later issues) compiled-policy cache used by the routing engine.
//
// The schema, vocabulary and merge semantics live in
// docs/06-engineering/01-routing-policy-reference.md. This package owns the in-memory
// representation of a parsed policy plus validation against the model registry.
//
// ISSUE-021 covered parsing and validation. ISSUE-022 adds compiled fast-path
// matchers and an atomic in-memory cache. Explanations and the test runner come
// in ISSUE-023–024.
package policy

// ModelProfile is the broad v1 profile vocabulary.
type ModelProfile string

const (
	ProfileCheap    ModelProfile = "cheap"
	ProfileBalanced ModelProfile = "balanced"
	ProfilePremium  ModelProfile = "premium"
)

// Retention enum.
type Retention string

const (
	RetentionStandard Retention = "standard"
	RetentionNone     Retention = "none"
)

// Capability enum used by require/deny_capabilities.
type Capability string

const (
	CapStreaming   Capability = "streaming"
	CapToolUse     Capability = "tool_use"
	CapJSONSchema  Capability = "json_schema"
	CapVision      Capability = "vision"
	CapLongContext Capability = "long_context"
)

// RouterMode enum used by `when.router_mode` and `route.block` semantics.
type RouterMode string

const (
	RouterModeAuto     RouterMode = "auto"
	RouterModeCheap    RouterMode = "cheap"
	RouterModeBalanced RouterMode = "balanced"
	RouterModePremium  RouterMode = "premium"
	RouterModeDisabled RouterMode = "disabled"
)

// Task type vocabulary mirrored from internal/router. Kept as strings so the
// policy package does not need a hard dep on router.
var validTaskTypes = map[string]struct{}{
	"simple_chat":           {},
	"trivial_git":           {},
	"simple_shell":          {},
	"summarization":         {},
	"simple_code_edit":      {},
	"hard_code_debugging":   {},
	"security_review":       {},
	"database_migration":    {},
	"long_context_analysis": {},
	"creative_copy":         {},
	"unknown_high_risk":     {},
}

var validRiskLevels = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"critical": {},
}

var validSensitivities = map[string]struct{}{
	"none":             {},
	"source_code":      {},
	"pii":              {},
	"secrets_possible": {},
	// Richer sensitivity classes (ISSUE-093).
	"financial":           {},
	"health":              {},
	"legal":               {},
	"security_classified": {},
}

// validToolRisks are the agent-capability classes (ISSUE-111). Like
// sensitivity, deployed policies match on these names — never rename or
// reorder them.
var validToolRisks = map[string]struct{}{
	"read":        {},
	"write":       {},
	"external":    {},
	"destructive": {},
}

// Policy is the parsed v1 policy document.
type Policy struct {
	Version  string
	Metadata Metadata
	Settings Settings
	Rules    []Rule
}

type Metadata struct {
	Owner       string
	Description string
}

type Settings struct {
	DefaultModelProfile  ModelProfile
	ConservativeUnknowns bool
	MaxRouterOverheadMS  int
	DefaultTimeoutMS     int
	DefaultRetention     Retention
}

// Rule represents a single ordered rule in the policy.
type Rule struct {
	ID          string
	Description string
	When        When
	Route       Route
}

// EnumMatch represents the `string | {in: [...]}` form. After parsing it is
// normalised to the set of accepted values (always non-empty when present).
type EnumMatch struct {
	Values []string
}

// When holds the v1 match conditions. Nil pointers mean "field not specified".
type When struct {
	Empty              bool // true when the rule used `when: {}`
	TaskType           *EnumMatch
	RiskLevel          *EnumMatch
	Tenant             *EnumMatch
	Project            *EnumMatch
	PromptTokensGT     *int
	PromptTokensLT     *int
	ContainsAny        []string
	AnyFileMatches     []string
	RequiresToolUse    *bool
	RequiresJSONSchema *bool
	RequiresVision     *bool
	Sensitivity        *EnumMatch
	RouterMode         *EnumMatch
	// ToolRisk gates on the capability an agent declares (ISSUE-111);
	// AnyToolMatches globs against declared tool names.
	ToolRisk       *EnumMatch
	AnyToolMatches []string
}

// Route holds v1 route semantics. Hints from the bakåtkompat surface are
// preserved as raw values so the compiler in ISSUE-022 can map them.
type Route struct {
	Block       *Block
	Force       *Force
	Constraints *Constraints
	Defaults    *Defaults
	Reason      string
	Hints       RouteHints
}

// RouteHints mirrors the deprecated single-level hints listed in §6.5 of the
// reference. They are captured verbatim during parse; mapping happens later.
type RouteHints struct {
	Tier              string
	Model             string
	Provider          string
	FallbackTier      string
	FallbackModels    []string
	Verifier          *bool
	MaxCostUSD        *float64
	TimeoutMS         *int
	Retention         string
	RequireCapability string
}

type Block struct {
	Code   string
	Reason string
	Status int
}

// Force, Constraints and Defaults are kept as separate structs so precedence
// is unambiguous when the compiler walks the matched rules.
type Force struct {
	ModelProfile     ModelProfile
	ModelProfileName string
	Provider         string
	Model            string
	Verifier         *bool
	ContextPipeline  *bool
	TimeoutMS        *int
	Retention        Retention
}

type Constraints struct {
	AllowedProviders      []string
	DeniedProviders       []string
	AllowedModels         []string
	DeniedModels          []string
	RequireCapabilities   []Capability
	DenyCapabilities      []Capability
	MaxCostUSD            *float64
	MaxLatencyMS          *int
	Retention             Retention
	FallbackModelProfiles []ModelProfile
	// RequireProviderTags/DenyProviderTags match against the model's effective
	// compliance tags (model ∪ provider, from the roster) — e.g. require
	// [eu-resident, dpa-signed] for PII traffic (ISSUE-078). Enforced in the
	// same single constraint check as allow/deny lists, so pinned/forced models
	// cannot bypass them.
	RequireProviderTags []string
	DenyProviderTags    []string
}

type Defaults struct {
	ModelProfile     ModelProfile
	ModelProfileName string
	Provider         string
	Model            string
	Verifier         *bool
	TimeoutMS        *int
	Retention        Retention
	MaxCostUSD       *float64
	MaxLatencyMS     *int
}

// HasAction reports whether the route specifies at least one action.
func (r Route) HasAction() bool {
	if r.Block != nil || r.Force != nil || r.Constraints != nil || r.Defaults != nil {
		return true
	}
	h := r.Hints
	return h.Tier != "" || h.Model != "" || h.Provider != "" || h.FallbackTier != "" ||
		len(h.FallbackModels) > 0 || h.Verifier != nil || h.MaxCostUSD != nil ||
		h.TimeoutMS != nil || h.Retention != "" || h.RequireCapability != ""
}

// ContextPipelineEnabled reports whether policy force explicitly enables the
// context-processor pipeline. Absence defaults to false.
func (r Route) ContextPipelineEnabled() bool {
	return r.Force != nil && r.Force.ContextPipeline != nil && *r.Force.ContextPipeline
}
