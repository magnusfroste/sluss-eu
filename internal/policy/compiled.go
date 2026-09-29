package policy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/registry"
)

var ErrNoCompiledPolicy = errors.New("policy: no compiled policy")

// Scope identifies the tenant/project slot for a compiled policy. A zero
// ProjectID acts as a tenant-wide policy, and the zero scope can hold a default.
type Scope struct {
	TenantID  string
	ProjectID string
}

// Source is one policy document plus the registry snapshot it was validated
// against. Cache reloads compile all sources before swapping the active set.
type Source struct {
	Scope    Scope
	Policy   *Policy
	Registry *registry.Snapshot
}

// EvaluationInput is the fast-path surface consumed by compiled matchers.
// It intentionally contains only derived/trusted descriptor fields.
type EvaluationInput struct {
	TenantID             string
	ProjectID            string
	TaskType             string
	RiskLevel            string
	Sensitivity          string
	PromptTokensEstimate int
	ContainsText         string
	Keywords             []string
	FilesTouched         []string
	RequiresToolUse      bool
	RequiresJSONSchema   bool
	RequiresVision       bool
	RouterMode           string
	// ToolRisk/ToolsDeclared carry the agent-capability classification
	// (ISSUE-111): the highest declared risk class and the tool names.
	ToolRisk      string
	ToolsDeclared []string
}

type Evaluation struct {
	PolicyVersion  string
	MatchedRuleIDs []string
	Explanations   []string
	Route          Route
	Blocked        bool
}

// CompiledPolicy is an immutable, registry-validated policy snapshot. Its rule
// matchers are precomputed so Evaluate performs no parsing, validation or I/O.
type CompiledPolicy struct {
	version         string
	registryVersion string
	settings        Settings
	rules           []compiledRule
	// source is the parsed policy this was compiled from, kept so the policy
	// can be re-validated against a new registry snapshot (ISSUE-115) without
	// re-reading its origin (file, builtin pack or console rules).
	source *Policy
}

type compiledRule struct {
	id          string
	description string
	matcher     compiledMatcher
	route       Route
}

type compiledMatcher struct {
	empty              bool
	taskTypes          stringSet
	riskLevels         stringSet
	tenants            stringSet
	projects           stringSet
	promptTokensGT     *int
	promptTokensLT     *int
	containsAny        stringSet
	filePatterns       []*regexp.Regexp
	requiresToolUse    *bool
	requiresJSONSchema *bool
	requiresVision     *bool
	sensitivities      stringSet
	routerModes        stringSet
	toolRisks          stringSet
	toolPatterns       []*regexp.Regexp
}

type stringSet map[string]struct{}

// Compile validates and compiles a policy against a registry snapshot.
func Compile(p *Policy, snapshot *registry.Snapshot) (*CompiledPolicy, error) {
	if err := Validate(p, snapshot); err != nil {
		return nil, err
	}
	compiled := &CompiledPolicy{
		version:         p.Version,
		registryVersion: snapshot.RegistryVersion(),
		settings:        p.Settings,
		rules:           make([]compiledRule, 0, len(p.Rules)),
		source:          p,
	}
	for _, rule := range p.Rules {
		matcher, err := compileMatcher(rule.When)
		if err != nil {
			return nil, fmt.Errorf("%w: rule %q: %v", ErrInvalidPolicy, rule.ID, err)
		}
		route := cloneRoute(rule.Route)
		normalizeRouteHints(&route)
		compiled.rules = append(compiled.rules, compiledRule{
			id:          rule.ID,
			description: rule.Description,
			matcher:     matcher,
			route:       route,
		})
	}
	return compiled, nil
}

func (p *CompiledPolicy) Version() string {
	if p == nil {
		return ""
	}
	return p.version
}

func (p *CompiledPolicy) RegistryVersion() string {
	if p == nil {
		return ""
	}
	return p.registryVersion
}

func (p *CompiledPolicy) Settings() Settings {
	if p == nil {
		return Settings{}
	}
	return p.settings
}

func (p *CompiledPolicy) RuleCount() int {
	if p == nil {
		return 0
	}
	return len(p.rules)
}

// RequiredProviderTags returns the union of provider tags any rule in this
// policy can require, sorted. The provider risk register (ISSUE-093) uses it to
// show which tags the active policy depends on — a required tag no provider
// carries means those prompts fail closed.
func (p *CompiledPolicy) RequiredProviderTags() []string {
	if p == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, r := range p.rules {
		if r.route.Constraints == nil {
			continue
		}
		for _, t := range r.route.Constraints.RequireProviderTags {
			seen[t] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func (p *CompiledPolicy) Evaluate(input EvaluationInput) Evaluation {
	out := Evaluation{}
	if p == nil {
		return out
	}
	out.PolicyVersion = p.version
	for _, rule := range p.rules {
		if !rule.matcher.matches(input) {
			continue
		}
		out.MatchedRuleIDs = append(out.MatchedRuleIDs, rule.id)
		out.Explanations = append(out.Explanations, rule.explanations()...)
		out.Explanations = append(out.Explanations, mergeRoute(&out.Route, rule.route, rule.id)...)
		if rule.route.Block != nil {
			out.Blocked = true
			break
		}
	}
	return out
}

func (e Evaluation) LogFields() []any {
	return []any{
		"policy_version", e.PolicyVersion,
		"policy_matched_rules", append([]string(nil), e.MatchedRuleIDs...),
		"policy_explanations", append([]string(nil), e.Explanations...),
		"policy_blocked", e.Blocked,
	}
}

func ExplainEnabled(headers http.Header) bool {
	switch strings.ToLower(strings.TrimSpace(headers.Get("X-Router-Explain"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (r compiledRule) explanations() []string {
	out := []string{fmt.Sprintf("Policy rule %s matched", r.id)}
	if r.description != "" {
		out = append(out, fmt.Sprintf("Rule %s description: %s", r.id, r.description))
	}
	if r.route.Reason != "" {
		out = append(out, fmt.Sprintf("Rule %s reason: %s", r.id, r.route.Reason))
	}
	return out
}

// Cache is an atomic tenant/project map of compiled policy snapshots.
type Cache struct {
	active  atomic.Value // map[Scope]*CompiledPolicy
	auditor audit.Sink   // optional; audits (re)loads
}

// SetAuditor attaches an audit sink so subsequent reloads are recorded. Passing
// nil disables auditing. Safe to call once at startup before serving traffic.
func (c *Cache) SetAuditor(sink audit.Sink) {
	if c == nil {
		return
	}
	c.auditor = sink
}

func NewCache(sources []Source) (*Cache, error) {
	compiled, err := compileSources(sources)
	if err != nil {
		return nil, err
	}
	c := &Cache{}
	c.active.Store(compiled)
	return c, nil
}

func (c *Cache) Active(scope Scope) (*CompiledPolicy, bool) {
	if c == nil {
		return nil, false
	}
	policies, ok := c.active.Load().(map[Scope]*CompiledPolicy)
	if !ok {
		return nil, false
	}
	if p, ok := policies[scope]; ok {
		return p, true
	}
	if scope.ProjectID != "" {
		if p, ok := policies[Scope{TenantID: scope.TenantID}]; ok {
			return p, true
		}
	}
	p, ok := policies[Scope{}]
	return p, ok
}

func (c *Cache) Reload(sources []Source) error {
	if c == nil {
		return ErrNoCompiledPolicy
	}
	compiled, err := compileSources(sources)
	if err != nil {
		// A rejected reload is itself a security-relevant change attempt.
		audit.Record(context.Background(), c.auditor, audit.Entry{
			Action:  audit.ActionPolicyReload,
			Actor:   "system",
			Outcome: audit.OutcomeFailure,
			Reason:  err.Error(),
		})
		return err
	}
	c.active.Store(compiled)
	for scope, p := range compiled {
		audit.Record(context.Background(), c.auditor, audit.Entry{
			Action:    audit.ActionPolicyReload,
			Actor:     "system",
			TenantID:  scope.TenantID,
			ProjectID: scope.ProjectID,
			Target:    p.Version(),
			Detail:    map[string]string{"registry_version": p.RegistryVersion()},
		})
	}
	return nil
}

// Rebind re-validates and recompiles every active policy against a new
// registry snapshot (ISSUE-115: roster hot-reload). It is two-phase: nothing
// changes until the returned commit is called, so a caller can prepare several
// caches and the registry swap and commit them together — or abandon all of
// them when any one fails (e.g. the new roster drops a model a policy forces).
// A nil or empty cache yields a no-op commit.
func (c *Cache) Rebind(snapshot *registry.Snapshot) (commit func(), err error) {
	if c == nil {
		return func() {}, nil
	}
	current, ok := c.active.Load().(map[Scope]*CompiledPolicy)
	if !ok || len(current) == 0 {
		return func() {}, nil
	}
	next := make(map[Scope]*CompiledPolicy, len(current))
	for scope, p := range current {
		if p == nil || p.source == nil {
			return nil, fmt.Errorf("%w: policy %q has no source to rebind", ErrInvalidPolicy, p.Version())
		}
		compiled, err := Compile(p.source, snapshot)
		if err != nil {
			return nil, fmt.Errorf("active policy %s does not validate against the new roster: %w", p.Version(), err)
		}
		next[scope] = compiled
	}
	return func() { c.active.Store(next) }, nil
}

func compileSources(sources []Source) (map[Scope]*CompiledPolicy, error) {
	if len(sources) == 0 {
		return nil, ErrNoCompiledPolicy
	}
	compiled := make(map[Scope]*CompiledPolicy, len(sources))
	for _, source := range sources {
		if source.Scope.ProjectID != "" && source.Scope.TenantID == "" {
			return nil, fmt.Errorf("%w: project-scoped policy requires tenant id", ErrInvalidPolicy)
		}
		if _, exists := compiled[source.Scope]; exists {
			return nil, fmt.Errorf("%w: duplicate policy scope tenant=%q project=%q", ErrInvalidPolicy, source.Scope.TenantID, source.Scope.ProjectID)
		}
		policy, err := Compile(source.Policy, source.Registry)
		if err != nil {
			return nil, err
		}
		compiled[source.Scope] = policy
	}
	return compiled, nil
}

func compileMatcher(when When) (compiledMatcher, error) {
	m := compiledMatcher{
		empty:              when.Empty,
		taskTypes:          setFromMatch(when.TaskType),
		riskLevels:         setFromMatch(when.RiskLevel),
		tenants:            setFromMatch(when.Tenant),
		projects:           setFromMatch(when.Project),
		promptTokensGT:     cloneIntPtr(when.PromptTokensGT),
		promptTokensLT:     cloneIntPtr(when.PromptTokensLT),
		containsAny:        setFromStrings(when.ContainsAny, true),
		requiresToolUse:    cloneBoolPtr(when.RequiresToolUse),
		requiresJSONSchema: cloneBoolPtr(when.RequiresJSONSchema),
		requiresVision:     cloneBoolPtr(when.RequiresVision),
		sensitivities:      setFromMatch(when.Sensitivity),
		routerModes:        setFromMatch(when.RouterMode),
		toolRisks:          setFromMatch(when.ToolRisk),
	}
	for _, pattern := range when.AnyToolMatches {
		re, err := compileGlob(pattern)
		if err != nil {
			return m, err
		}
		m.toolPatterns = append(m.toolPatterns, re)
	}
	for _, pattern := range when.AnyFileMatches {
		re, err := compileGlob(pattern)
		if err != nil {
			return m, err
		}
		m.filePatterns = append(m.filePatterns, re)
	}
	return m, nil
}

func (m compiledMatcher) matches(input EvaluationInput) bool {
	if m.empty {
		return true
	}
	if !m.taskTypes.matches(input.TaskType) ||
		!m.riskLevels.matches(input.RiskLevel) ||
		!m.tenants.matches(input.TenantID) ||
		!m.projects.matches(input.ProjectID) ||
		!m.sensitivities.matches(input.Sensitivity) ||
		!m.toolRisks.matches(input.ToolRisk) ||
		!m.routerModes.matches(input.RouterMode) {
		return false
	}
	if m.promptTokensGT != nil && input.PromptTokensEstimate <= *m.promptTokensGT {
		return false
	}
	if m.promptTokensLT != nil && input.PromptTokensEstimate >= *m.promptTokensLT {
		return false
	}
	if m.requiresToolUse != nil && input.RequiresToolUse != *m.requiresToolUse {
		return false
	}
	if m.requiresJSONSchema != nil && input.RequiresJSONSchema != *m.requiresJSONSchema {
		return false
	}
	if m.requiresVision != nil && input.RequiresVision != *m.requiresVision {
		return false
	}
	if len(m.containsAny) > 0 && !m.containsAny.matchesTextOrAny(input.ContainsText, input.Keywords) {
		return false
	}
	if len(m.filePatterns) > 0 && !matchesAnyFile(m.filePatterns, input.FilesTouched) {
		return false
	}
	if len(m.toolPatterns) > 0 && !matchesAnyFile(m.toolPatterns, input.ToolsDeclared) {
		return false
	}
	return true
}

func (s stringSet) matches(value string) bool {
	if len(s) == 0 {
		return true
	}
	_, ok := s[value]
	return ok
}

func (s stringSet) matchesTextOrAny(text string, values []string) bool {
	lowerText := strings.ToLower(text)
	for value := range s {
		if lowerText != "" && strings.Contains(lowerText, value) {
			return true
		}
	}
	for _, value := range values {
		if _, ok := s[strings.ToLower(value)]; ok {
			return true
		}
	}
	return false
}

func setFromMatch(match *EnumMatch) stringSet {
	if match == nil {
		return nil
	}
	return setFromStrings(match.Values, false)
}

func setFromStrings(values []string, lower bool) stringSet {
	if len(values) == 0 {
		return nil
	}
	out := make(stringSet, len(values))
	for _, value := range values {
		if lower {
			value = strings.ToLower(value)
		}
		out[value] = struct{}{}
	}
	return out
}

func matchesAnyFile(patterns []*regexp.Regexp, files []string) bool {
	for _, file := range files {
		normalized := strings.ReplaceAll(file, "\\", "/")
		for _, pattern := range patterns {
			if pattern.MatchString(normalized) {
				return true
			}
		}
	}
	return false
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("empty file glob")
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if ch == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
					continue
				}
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
			continue
		}
		if ch == '?' {
			b.WriteString("[^/]")
			continue
		}
		b.WriteString(regexp.QuoteMeta(string(ch)))
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func mergeRoute(dst *Route, src Route, ruleID string) []string {
	var explanations []string
	if src.Block != nil {
		dst.Block = cloneBlock(src.Block)
	}
	if src.Force != nil {
		var forceExplanations []string
		dst.Force, forceExplanations = mergeForce(dst.Force, src.Force, ruleID)
		explanations = append(explanations, forceExplanations...)
	}
	if src.Constraints != nil {
		dst.Constraints = mergeConstraints(dst.Constraints, src.Constraints)
	}
	if src.Defaults != nil {
		dst.Defaults = mergeDefaults(dst.Defaults, src.Defaults)
	}
	if src.Reason != "" {
		dst.Reason = src.Reason
	}
	dst.Hints = mergeHints(dst.Hints, src.Hints)
	return explanations
}

func mergeForce(existing, next *Force, ruleID string) (*Force, []string) {
	if existing == nil {
		return cloneForce(next), nil
	}
	out := cloneForce(existing)
	var explanations []string
	keepFirst := func(field string) {
		explanations = append(explanations, fmt.Sprintf("Rule %s force.%s ignored because an earlier matched rule already set it", ruleID, field))
	}
	if next.ModelProfile != "" {
		if out.ModelProfile == "" {
			out.ModelProfile = next.ModelProfile
		} else {
			keepFirst("model_profile")
		}
	}
	if next.ModelProfileName != "" {
		if out.ModelProfileName == "" {
			out.ModelProfileName = next.ModelProfileName
		} else {
			keepFirst("model_profile_name")
		}
	}
	if next.Provider != "" {
		if out.Provider == "" {
			out.Provider = next.Provider
		} else {
			keepFirst("provider")
		}
	}
	if next.Model != "" {
		if out.Model == "" {
			out.Model = next.Model
		} else {
			keepFirst("model")
		}
	}
	if next.Verifier != nil {
		if out.Verifier == nil {
			out.Verifier = cloneBoolPtr(next.Verifier)
		} else {
			keepFirst("verifier")
		}
	}
	if next.ContextPipeline != nil {
		if out.ContextPipeline == nil {
			out.ContextPipeline = cloneBoolPtr(next.ContextPipeline)
		} else {
			keepFirst("context_pipeline")
		}
	}
	if next.TimeoutMS != nil {
		if out.TimeoutMS == nil {
			out.TimeoutMS = cloneIntPtr(next.TimeoutMS)
		} else {
			keepFirst("timeout_ms")
		}
	}
	if next.Retention != "" {
		if out.Retention == "" {
			out.Retention = next.Retention
		} else {
			keepFirst("retention")
		}
	}
	return out, explanations
}

func mergeConstraints(existing, next *Constraints) *Constraints {
	if existing == nil {
		return cloneConstraints(next)
	}
	out := cloneConstraints(existing)
	out.AllowedProviders = intersectIfBoth(out.AllowedProviders, next.AllowedProviders)
	out.DeniedProviders = unionValues(out.DeniedProviders, next.DeniedProviders)
	out.AllowedModels = intersectIfBoth(out.AllowedModels, next.AllowedModels)
	out.DeniedModels = unionValues(out.DeniedModels, next.DeniedModels)
	out.RequireCapabilities = unionValues(out.RequireCapabilities, next.RequireCapabilities)
	out.DenyCapabilities = unionValues(out.DenyCapabilities, next.DenyCapabilities)
	// Residency/compliance tags accumulate: a stricter requirement in any matched
	// rule must survive (fail-closed), never be dropped (ISSUE-078/082).
	out.RequireProviderTags = unionValues(out.RequireProviderTags, next.RequireProviderTags)
	out.DenyProviderTags = unionValues(out.DenyProviderTags, next.DenyProviderTags)
	if next.MaxCostUSD != nil && (out.MaxCostUSD == nil || *next.MaxCostUSD < *out.MaxCostUSD) {
		out.MaxCostUSD = cloneFloatPtr(next.MaxCostUSD)
	}
	if next.MaxLatencyMS != nil && (out.MaxLatencyMS == nil || *next.MaxLatencyMS < *out.MaxLatencyMS) {
		out.MaxLatencyMS = cloneIntPtr(next.MaxLatencyMS)
	}
	if stricterRetention(next.Retention, out.Retention) {
		out.Retention = next.Retention
	}
	out.FallbackModelProfiles = unionValues(out.FallbackModelProfiles, next.FallbackModelProfiles)
	return out
}

func normalizeRouteHints(route *Route) {
	if route == nil {
		return
	}
	h := route.Hints
	if h.Tier != "" {
		defaults := route.ensureDefaults()
		if defaults.ModelProfile == "" {
			defaults.ModelProfile = ModelProfile(h.Tier)
		}
	}
	if h.Model != "" || h.Provider != "" || h.Verifier != nil {
		force := route.ensureForce()
		if h.Model != "" && force.Model == "" {
			force.Model = h.Model
		}
		if h.Provider != "" && force.Provider == "" {
			force.Provider = h.Provider
		}
		if h.Verifier != nil && force.Verifier == nil {
			force.Verifier = cloneBoolPtr(h.Verifier)
		}
	}
	if h.TimeoutMS != nil {
		if route.Force != nil {
			if route.Force.TimeoutMS == nil {
				route.Force.TimeoutMS = cloneIntPtr(h.TimeoutMS)
			}
		} else {
			defaults := route.ensureDefaults()
			if defaults.TimeoutMS == nil {
				defaults.TimeoutMS = cloneIntPtr(h.TimeoutMS)
			}
		}
	}
	if h.FallbackTier != "" || len(h.FallbackModels) > 0 || h.MaxCostUSD != nil || h.Retention != "" || h.RequireCapability != "" {
		constraints := route.ensureConstraints()
		if h.FallbackTier != "" {
			constraints.FallbackModelProfiles = unionValues(constraints.FallbackModelProfiles, []ModelProfile{ModelProfile(h.FallbackTier)})
		}
		for _, fallback := range h.FallbackModels {
			constraints.FallbackModelProfiles = unionValues(constraints.FallbackModelProfiles, []ModelProfile{ModelProfile(fallback)})
		}
		if h.MaxCostUSD != nil && constraints.MaxCostUSD == nil {
			constraints.MaxCostUSD = cloneFloatPtr(h.MaxCostUSD)
		}
		if h.Retention != "" && constraints.Retention == "" {
			constraints.Retention = Retention(h.Retention)
		}
		if h.RequireCapability != "" {
			constraints.RequireCapabilities = unionValues(constraints.RequireCapabilities, []Capability{Capability(h.RequireCapability)})
		}
	}
}

func (r *Route) ensureForce() *Force {
	if r.Force == nil {
		r.Force = &Force{}
	}
	return r.Force
}

func (r *Route) ensureConstraints() *Constraints {
	if r.Constraints == nil {
		r.Constraints = &Constraints{}
	}
	return r.Constraints
}

func (r *Route) ensureDefaults() *Defaults {
	if r.Defaults == nil {
		r.Defaults = &Defaults{}
	}
	return r.Defaults
}

func intersectIfBoth(left, right []string) []string {
	if len(left) == 0 {
		return append([]string(nil), right...)
	}
	if len(right) == 0 {
		return append([]string(nil), left...)
	}
	allowed := make(map[string]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	out := make([]string, 0, min(len(left), len(right)))
	for _, value := range left {
		if _, ok := allowed[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

func stricterRetention(next, current Retention) bool {
	if next == "" {
		return false
	}
	return current == "" || next == RetentionNone
}

func unionValues[T comparable](left, right []T) []T {
	seen := make(map[T]struct{}, len(left)+len(right))
	out := make([]T, 0, len(left)+len(right))
	for _, values := range [][]T{left, right} {
		for _, value := range values {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func mergeDefaults(existing, next *Defaults) *Defaults {
	out := &Defaults{}
	if existing != nil {
		*out = *cloneDefaults(existing)
	}
	if next.ModelProfile != "" && out.ModelProfile == "" {
		out.ModelProfile = next.ModelProfile
	}
	if next.ModelProfileName != "" && out.ModelProfileName == "" {
		out.ModelProfileName = next.ModelProfileName
	}
	if next.Provider != "" && out.Provider == "" {
		out.Provider = next.Provider
	}
	if next.Model != "" && out.Model == "" {
		out.Model = next.Model
	}
	if next.Verifier != nil && out.Verifier == nil {
		out.Verifier = cloneBoolPtr(next.Verifier)
	}
	if next.TimeoutMS != nil && out.TimeoutMS == nil {
		out.TimeoutMS = cloneIntPtr(next.TimeoutMS)
	}
	if next.Retention != "" && out.Retention == "" {
		out.Retention = next.Retention
	}
	if next.MaxCostUSD != nil && out.MaxCostUSD == nil {
		out.MaxCostUSD = cloneFloatPtr(next.MaxCostUSD)
	}
	if next.MaxLatencyMS != nil && out.MaxLatencyMS == nil {
		out.MaxLatencyMS = cloneIntPtr(next.MaxLatencyMS)
	}
	return out
}

func mergeHints(existing, next RouteHints) RouteHints {
	out := cloneHints(existing)
	if next.Tier != "" {
		out.Tier = next.Tier
	}
	if next.Model != "" {
		out.Model = next.Model
	}
	if next.Provider != "" {
		out.Provider = next.Provider
	}
	if next.FallbackTier != "" {
		out.FallbackTier = next.FallbackTier
	}
	if len(next.FallbackModels) > 0 {
		out.FallbackModels = append([]string(nil), next.FallbackModels...)
	}
	if next.Verifier != nil {
		out.Verifier = cloneBoolPtr(next.Verifier)
	}
	if next.MaxCostUSD != nil {
		out.MaxCostUSD = cloneFloatPtr(next.MaxCostUSD)
	}
	if next.TimeoutMS != nil {
		out.TimeoutMS = cloneIntPtr(next.TimeoutMS)
	}
	if next.Retention != "" {
		out.Retention = next.Retention
	}
	if next.RequireCapability != "" {
		out.RequireCapability = next.RequireCapability
	}
	return out
}

func cloneRoute(route Route) Route {
	return Route{
		Block:       cloneBlock(route.Block),
		Force:       cloneForce(route.Force),
		Constraints: cloneConstraints(route.Constraints),
		Defaults:    cloneDefaults(route.Defaults),
		Reason:      route.Reason,
		Hints:       cloneHints(route.Hints),
	}
}

func cloneBlock(in *Block) *Block {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneForce(in *Force) *Force {
	if in == nil {
		return nil
	}
	out := *in
	out.Verifier = cloneBoolPtr(in.Verifier)
	out.ContextPipeline = cloneBoolPtr(in.ContextPipeline)
	out.TimeoutMS = cloneIntPtr(in.TimeoutMS)
	return &out
}

func cloneConstraints(in *Constraints) *Constraints {
	if in == nil {
		return nil
	}
	out := *in
	out.AllowedProviders = append([]string(nil), in.AllowedProviders...)
	out.DeniedProviders = append([]string(nil), in.DeniedProviders...)
	out.AllowedModels = append([]string(nil), in.AllowedModels...)
	out.DeniedModels = append([]string(nil), in.DeniedModels...)
	out.RequireCapabilities = append([]Capability(nil), in.RequireCapabilities...)
	out.DenyCapabilities = append([]Capability(nil), in.DenyCapabilities...)
	out.RequireProviderTags = append([]string(nil), in.RequireProviderTags...)
	out.DenyProviderTags = append([]string(nil), in.DenyProviderTags...)
	out.MaxCostUSD = cloneFloatPtr(in.MaxCostUSD)
	out.MaxLatencyMS = cloneIntPtr(in.MaxLatencyMS)
	out.FallbackModelProfiles = append([]ModelProfile(nil), in.FallbackModelProfiles...)
	return &out
}

func cloneDefaults(in *Defaults) *Defaults {
	if in == nil {
		return nil
	}
	out := *in
	out.Verifier = cloneBoolPtr(in.Verifier)
	out.TimeoutMS = cloneIntPtr(in.TimeoutMS)
	out.MaxCostUSD = cloneFloatPtr(in.MaxCostUSD)
	out.MaxLatencyMS = cloneIntPtr(in.MaxLatencyMS)
	return &out
}

func cloneHints(in RouteHints) RouteHints {
	out := in
	out.FallbackModels = append([]string(nil), in.FallbackModels...)
	out.Verifier = cloneBoolPtr(in.Verifier)
	out.MaxCostUSD = cloneFloatPtr(in.MaxCostUSD)
	out.TimeoutMS = cloneIntPtr(in.TimeoutMS)
	return out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneFloatPtr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
