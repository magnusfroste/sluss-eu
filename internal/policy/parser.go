package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/magnusfroste/sluss/internal/registry"
	"gopkg.in/yaml.v3"
)

// ErrInvalidPolicy is the sentinel returned by Parse for all schema/vocabulary
// errors. Callers may use errors.Is(err, ErrInvalidPolicy) to detect parse
// failures without depending on the specific message.
var ErrInvalidPolicy = errors.New("policy: invalid policy")

// Parse decodes a YAML (or JSON, since YAML is a superset) v1 policy document
// and validates its schema, vocabulary, rule structure and uniqueness. It does
// not validate provider/model references; call Validate for that.
func Parse(data []byte) (*Policy, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrInvalidPolicy)
	}
	var raw rawPolicy
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: yaml: %v", ErrInvalidPolicy, err)
	}
	return raw.toPolicy()
}

// Validate cross-checks references against the registry snapshot. It is safe to
// call multiple times. Errors are joined and reported in a single error chain.
func Validate(p *Policy, snapshot *registry.Snapshot) error {
	if p == nil {
		return fmt.Errorf("%w: nil policy", ErrInvalidPolicy)
	}
	if snapshot == nil {
		return fmt.Errorf("%w: nil registry snapshot", ErrInvalidPolicy)
	}
	var errs []string
	checkProvider := func(ctx, id string) {
		if id == "" {
			return
		}
		if _, ok := snapshot.Provider(id); !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown provider %q", ctx, id))
		}
	}
	checkModel := func(ctx, id string) {
		if id == "" {
			return
		}
		if _, ok := snapshot.Model(id); !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown model %q", ctx, id))
		}
	}
	for _, rule := range p.Rules {
		ctx := fmt.Sprintf("rule %q", rule.ID)
		if rule.Route.Force != nil {
			checkProvider(ctx+".force", rule.Route.Force.Provider)
			checkModel(ctx+".force", rule.Route.Force.Model)
			checkModel(ctx+".force.model_profile_name", rule.Route.Force.ModelProfileName)
		}
		if rule.Route.Defaults != nil {
			checkProvider(ctx+".defaults", rule.Route.Defaults.Provider)
			checkModel(ctx+".defaults", rule.Route.Defaults.Model)
			checkModel(ctx+".defaults.model_profile_name", rule.Route.Defaults.ModelProfileName)
		}
		if rule.Route.Constraints != nil {
			for _, id := range rule.Route.Constraints.AllowedProviders {
				checkProvider(ctx+".constraints.allowed_providers", id)
			}
			for _, id := range rule.Route.Constraints.DeniedProviders {
				checkProvider(ctx+".constraints.denied_providers", id)
			}
			for _, id := range rule.Route.Constraints.AllowedModels {
				checkModel(ctx+".constraints.allowed_models", id)
			}
			for _, id := range rule.Route.Constraints.DeniedModels {
				checkModel(ctx+".constraints.denied_models", id)
			}
		}
		if rule.Route.Hints.Provider != "" {
			checkProvider(ctx+".route.provider", rule.Route.Hints.Provider)
		}
		if rule.Route.Hints.Model != "" {
			checkModel(ctx+".route.model", rule.Route.Hints.Model)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidPolicy, strings.Join(errs, "; "))
	}
	return nil
}

// rawPolicy mirrors the YAML structure and converts to the typed Policy. We
// intentionally do not use struct tags on the Policy type itself so the public
// API stays pure Go.
type rawPolicy struct {
	Version  string         `yaml:"version"`
	Metadata *rawMetadata   `yaml:"metadata"`
	Settings *rawSettings   `yaml:"settings"`
	Rules    []rawRule      `yaml:"rules"`
	Extra    map[string]any `yaml:",inline"`
}

type rawMetadata struct {
	Owner       string `yaml:"owner"`
	Description string `yaml:"description"`
}

type rawSettings struct {
	DefaultModelProfile  string `yaml:"default_model_profile"`
	ConservativeUnknowns *bool  `yaml:"conservative_unknowns"`
	MaxRouterOverheadMS  *int   `yaml:"max_router_overhead_ms"`
	DefaultTimeoutMS     *int   `yaml:"default_timeout_ms"`
	DefaultRetention     string `yaml:"default_retention"`
}

type rawRule struct {
	ID          string    `yaml:"id"`
	Description string    `yaml:"description"`
	When        yaml.Node `yaml:"when"`
	Route       yaml.Node `yaml:"route"`
}

func (r *rawPolicy) toPolicy() (*Policy, error) {
	if strings.TrimSpace(r.Version) == "" {
		return nil, fmt.Errorf("%w: missing version", ErrInvalidPolicy)
	}
	if r.Settings == nil {
		return nil, fmt.Errorf("%w: missing settings", ErrInvalidPolicy)
	}
	settings, err := r.Settings.toSettings()
	if err != nil {
		return nil, err
	}
	if len(r.Rules) == 0 {
		return nil, fmt.Errorf("%w: at least one rule is required", ErrInvalidPolicy)
	}
	policy := &Policy{
		Version:  r.Version,
		Settings: settings,
	}
	if r.Metadata != nil {
		policy.Metadata = Metadata{Owner: r.Metadata.Owner, Description: r.Metadata.Description}
	}
	seen := make(map[string]struct{}, len(r.Rules))
	for i, raw := range r.Rules {
		rule, err := raw.toRule(i)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[rule.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate rule id %q", ErrInvalidPolicy, rule.ID)
		}
		seen[rule.ID] = struct{}{}
		policy.Rules = append(policy.Rules, rule)
	}
	return policy, nil
}

func (s *rawSettings) toSettings() (Settings, error) {
	out := Settings{}
	switch s.DefaultModelProfile {
	case string(ProfileCheap), string(ProfileBalanced), string(ProfilePremium):
		out.DefaultModelProfile = ModelProfile(s.DefaultModelProfile)
	case "":
		return out, fmt.Errorf("%w: settings.default_model_profile is required", ErrInvalidPolicy)
	default:
		return out, fmt.Errorf("%w: settings.default_model_profile %q not in [cheap balanced premium]", ErrInvalidPolicy, s.DefaultModelProfile)
	}
	if s.ConservativeUnknowns == nil {
		return out, fmt.Errorf("%w: settings.conservative_unknowns is required", ErrInvalidPolicy)
	}
	out.ConservativeUnknowns = *s.ConservativeUnknowns
	if s.MaxRouterOverheadMS == nil || *s.MaxRouterOverheadMS <= 0 {
		return out, fmt.Errorf("%w: settings.max_router_overhead_ms must be > 0", ErrInvalidPolicy)
	}
	out.MaxRouterOverheadMS = *s.MaxRouterOverheadMS
	if s.DefaultTimeoutMS == nil || *s.DefaultTimeoutMS <= 0 {
		return out, fmt.Errorf("%w: settings.default_timeout_ms must be > 0", ErrInvalidPolicy)
	}
	out.DefaultTimeoutMS = *s.DefaultTimeoutMS
	switch s.DefaultRetention {
	case string(RetentionStandard), string(RetentionNone):
		out.DefaultRetention = Retention(s.DefaultRetention)
	case "":
		return out, fmt.Errorf("%w: settings.default_retention is required", ErrInvalidPolicy)
	default:
		return out, fmt.Errorf("%w: settings.default_retention %q not in [standard none]", ErrInvalidPolicy, s.DefaultRetention)
	}
	return out, nil
}

func (r *rawRule) toRule(index int) (Rule, error) {
	if strings.TrimSpace(r.ID) == "" {
		return Rule{}, fmt.Errorf("%w: rule[%d] missing id", ErrInvalidPolicy, index)
	}
	ctx := fmt.Sprintf("rule %q", r.ID)
	if r.When.Kind == 0 {
		return Rule{}, fmt.Errorf("%w: %s missing when", ErrInvalidPolicy, ctx)
	}
	if r.Route.Kind == 0 {
		return Rule{}, fmt.Errorf("%w: %s missing route", ErrInvalidPolicy, ctx)
	}
	when, err := parseWhen(&r.When, ctx)
	if err != nil {
		return Rule{}, err
	}
	route, err := parseRoute(&r.Route, ctx)
	if err != nil {
		return Rule{}, err
	}
	if !route.HasAction() {
		return Rule{}, fmt.Errorf("%w: %s route must specify block, force, constraints, defaults or a hint", ErrInvalidPolicy, ctx)
	}
	return Rule{ID: r.ID, Description: r.Description, When: when, Route: route}, nil
}

func parseWhen(node *yaml.Node, ctx string) (When, error) {
	out := When{}
	if node.Kind != yaml.MappingNode {
		return out, fmt.Errorf("%w: %s.when must be a mapping", ErrInvalidPolicy, ctx)
	}
	if len(node.Content) == 0 {
		out.Empty = true
		return out, nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "task_type":
			m, err := decodeEnumMatch(val, validTaskTypes, ctx+".when.task_type")
			if err != nil {
				return out, err
			}
			out.TaskType = m
		case "risk_level":
			m, err := decodeEnumMatch(val, validRiskLevels, ctx+".when.risk_level")
			if err != nil {
				return out, err
			}
			out.RiskLevel = m
		case "tenant":
			m, err := decodeEnumMatch(val, nil, ctx+".when.tenant")
			if err != nil {
				return out, err
			}
			out.Tenant = m
		case "project":
			m, err := decodeEnumMatch(val, nil, ctx+".when.project")
			if err != nil {
				return out, err
			}
			out.Project = m
		case "prompt_tokens_gt":
			n, err := decodeInt(val, ctx+".when.prompt_tokens_gt")
			if err != nil {
				return out, err
			}
			out.PromptTokensGT = &n
		case "prompt_tokens_lt":
			n, err := decodeInt(val, ctx+".when.prompt_tokens_lt")
			if err != nil {
				return out, err
			}
			out.PromptTokensLT = &n
		case "contains_any":
			ss, err := decodeStringList(val, ctx+".when.contains_any")
			if err != nil {
				return out, err
			}
			out.ContainsAny = ss
		case "any_file_matches":
			ss, err := decodeStringList(val, ctx+".when.any_file_matches")
			if err != nil {
				return out, err
			}
			out.AnyFileMatches = ss
		case "requires_tool_use":
			b, err := decodeBool(val, ctx+".when.requires_tool_use")
			if err != nil {
				return out, err
			}
			out.RequiresToolUse = &b
		case "requires_json_schema":
			b, err := decodeBool(val, ctx+".when.requires_json_schema")
			if err != nil {
				return out, err
			}
			out.RequiresJSONSchema = &b
		case "requires_vision":
			b, err := decodeBool(val, ctx+".when.requires_vision")
			if err != nil {
				return out, err
			}
			out.RequiresVision = &b
		case "tool_risk":
			m, err := decodeEnumMatch(val, validToolRisks, ctx+".when.tool_risk")
			if err != nil {
				return out, err
			}
			out.ToolRisk = m
		case "any_tool_matches":
			ss, err := decodeStringList(val, ctx+".when.any_tool_matches")
			if err != nil {
				return out, err
			}
			out.AnyToolMatches = ss
		case "sensitivity":
			m, err := decodeEnumMatch(val, validSensitivities, ctx+".when.sensitivity")
			if err != nil {
				return out, err
			}
			out.Sensitivity = m
		case "router_mode":
			vocab := map[string]struct{}{
				string(RouterModeAuto):     {},
				string(RouterModeCheap):    {},
				string(RouterModeBalanced): {},
				string(RouterModePremium):  {},
				string(RouterModeDisabled): {},
			}
			m, err := decodeEnumMatch(val, vocab, ctx+".when.router_mode")
			if err != nil {
				return out, err
			}
			out.RouterMode = m
		default:
			return out, fmt.Errorf("%w: %s.when has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return out, nil
}

func parseRoute(node *yaml.Node, ctx string) (Route, error) {
	out := Route{}
	if node.Kind != yaml.MappingNode {
		return out, fmt.Errorf("%w: %s.route must be a mapping", ErrInvalidPolicy, ctx)
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "block":
			b, err := decodeBlock(val, ctx+".route.block")
			if err != nil {
				return out, err
			}
			out.Block = b
		case "force":
			f, err := decodeForce(val, ctx+".route.force")
			if err != nil {
				return out, err
			}
			out.Force = f
		case "constraints":
			c, err := decodeConstraints(val, ctx+".route.constraints")
			if err != nil {
				return out, err
			}
			out.Constraints = c
		case "defaults":
			d, err := decodeDefaults(val, ctx+".route.defaults")
			if err != nil {
				return out, err
			}
			out.Defaults = d
		case "reason":
			s, err := decodeString(val, ctx+".route.reason")
			if err != nil {
				return out, err
			}
			out.Reason = s
		case "tier":
			s, err := decodeString(val, ctx+".route.tier")
			if err != nil {
				return out, err
			}
			if !isValidProfileOrFallback(s) {
				return out, fmt.Errorf("%w: %s.route.tier %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
			}
			out.Hints.Tier = s
		case "model":
			s, err := decodeString(val, ctx+".route.model")
			if err != nil {
				return out, err
			}
			out.Hints.Model = s
		case "provider":
			s, err := decodeString(val, ctx+".route.provider")
			if err != nil {
				return out, err
			}
			out.Hints.Provider = s
		case "fallback_tier":
			s, err := decodeString(val, ctx+".route.fallback_tier")
			if err != nil {
				return out, err
			}
			if !isValidProfileOrFallback(s) {
				return out, fmt.Errorf("%w: %s.route.fallback_tier %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
			}
			out.Hints.FallbackTier = s
		case "fallback_models":
			ss, err := decodeStringList(val, ctx+".route.fallback_models")
			if err != nil {
				return out, err
			}
			for _, s := range ss {
				if !isValidProfileOrFallback(s) {
					return out, fmt.Errorf("%w: %s.route.fallback_models %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
				}
			}
			out.Hints.FallbackModels = ss
		case "verifier":
			b, err := decodeBool(val, ctx+".route.verifier")
			if err != nil {
				return out, err
			}
			out.Hints.Verifier = &b
		case "max_cost_usd":
			f, err := decodeFloat(val, ctx+".route.max_cost_usd")
			if err != nil {
				return out, err
			}
			out.Hints.MaxCostUSD = &f
		case "timeout_ms":
			n, err := decodeInt(val, ctx+".route.timeout_ms")
			if err != nil {
				return out, err
			}
			out.Hints.TimeoutMS = &n
		case "retention":
			s, err := decodeString(val, ctx+".route.retention")
			if err != nil {
				return out, err
			}
			if s != string(RetentionStandard) && s != string(RetentionNone) {
				return out, fmt.Errorf("%w: %s.route.retention %q must be standard|none", ErrInvalidPolicy, ctx, s)
			}
			out.Hints.Retention = s
		case "require_capability":
			s, err := decodeString(val, ctx+".route.require_capability")
			if err != nil {
				return out, err
			}
			if !isValidCapability(s) {
				return out, fmt.Errorf("%w: %s.route.require_capability %q is not a recognised capability", ErrInvalidPolicy, ctx, s)
			}
			out.Hints.RequireCapability = s
		default:
			return out, fmt.Errorf("%w: %s.route has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return out, nil
}

func decodeBlock(node *yaml.Node, ctx string) (*Block, error) {
	if node.Kind == yaml.ScalarNode {
		if node.Tag == "!!bool" && node.Value == "true" {
			return &Block{}, nil
		}
		if node.Tag == "!!bool" && node.Value == "false" {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %s must be true or a mapping", ErrInvalidPolicy, ctx)
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: %s must be a mapping", ErrInvalidPolicy, ctx)
	}
	block := &Block{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "code":
			s, err := decodeString(val, ctx+".code")
			if err != nil {
				return nil, err
			}
			block.Code = s
		case "reason":
			s, err := decodeString(val, ctx+".reason")
			if err != nil {
				return nil, err
			}
			block.Reason = s
		case "status":
			n, err := decodeInt(val, ctx+".status")
			if err != nil {
				return nil, err
			}
			block.Status = n
		default:
			return nil, fmt.Errorf("%w: %s has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return block, nil
}

func decodeForce(node *yaml.Node, ctx string) (*Force, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: %s must be a mapping", ErrInvalidPolicy, ctx)
	}
	f := &Force{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "model_profile":
			s, err := decodeString(val, ctx+".model_profile")
			if err != nil {
				return nil, err
			}
			if !isValidProfileOrFallback(s) {
				return nil, fmt.Errorf("%w: %s.model_profile %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
			}
			f.ModelProfile = ModelProfile(s)
		case "model_profile_name":
			s, err := decodeString(val, ctx+".model_profile_name")
			if err != nil {
				return nil, err
			}
			f.ModelProfileName = s
		case "provider":
			s, err := decodeString(val, ctx+".provider")
			if err != nil {
				return nil, err
			}
			f.Provider = s
		case "model":
			s, err := decodeString(val, ctx+".model")
			if err != nil {
				return nil, err
			}
			f.Model = s
		case "verifier":
			b, err := decodeBool(val, ctx+".verifier")
			if err != nil {
				return nil, err
			}
			f.Verifier = &b
		case "context_pipeline":
			b, err := decodeBool(val, ctx+".context_pipeline")
			if err != nil {
				return nil, err
			}
			f.ContextPipeline = &b
		case "timeout_ms":
			n, err := decodeInt(val, ctx+".timeout_ms")
			if err != nil {
				return nil, err
			}
			f.TimeoutMS = &n
		case "retention":
			s, err := decodeString(val, ctx+".retention")
			if err != nil {
				return nil, err
			}
			if s != string(RetentionStandard) && s != string(RetentionNone) {
				return nil, fmt.Errorf("%w: %s.retention %q must be standard|none", ErrInvalidPolicy, ctx, s)
			}
			f.Retention = Retention(s)
		default:
			return nil, fmt.Errorf("%w: %s has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return f, nil
}

func decodeConstraints(node *yaml.Node, ctx string) (*Constraints, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: %s must be a mapping", ErrInvalidPolicy, ctx)
	}
	c := &Constraints{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "allowed_providers":
			ss, err := decodeStringList(val, ctx+".allowed_providers")
			if err != nil {
				return nil, err
			}
			c.AllowedProviders = ss
		case "denied_providers":
			ss, err := decodeStringList(val, ctx+".denied_providers")
			if err != nil {
				return nil, err
			}
			c.DeniedProviders = ss
		case "allowed_models":
			ss, err := decodeStringList(val, ctx+".allowed_models")
			if err != nil {
				return nil, err
			}
			c.AllowedModels = ss
		case "denied_models":
			ss, err := decodeStringList(val, ctx+".denied_models")
			if err != nil {
				return nil, err
			}
			c.DeniedModels = ss
		case "require_provider_tags":
			ss, err := decodeStringList(val, ctx+".require_provider_tags")
			if err != nil {
				return nil, err
			}
			c.RequireProviderTags = normalizeTags(ss)
		case "deny_provider_tags":
			ss, err := decodeStringList(val, ctx+".deny_provider_tags")
			if err != nil {
				return nil, err
			}
			c.DenyProviderTags = normalizeTags(ss)
		case "require_capabilities":
			caps, err := decodeCapabilityList(val, ctx+".require_capabilities")
			if err != nil {
				return nil, err
			}
			c.RequireCapabilities = caps
		case "deny_capabilities":
			caps, err := decodeCapabilityList(val, ctx+".deny_capabilities")
			if err != nil {
				return nil, err
			}
			c.DenyCapabilities = caps
		case "max_cost_usd":
			f, err := decodeFloat(val, ctx+".max_cost_usd")
			if err != nil {
				return nil, err
			}
			c.MaxCostUSD = &f
		case "max_latency_ms":
			n, err := decodeInt(val, ctx+".max_latency_ms")
			if err != nil {
				return nil, err
			}
			c.MaxLatencyMS = &n
		case "retention":
			s, err := decodeString(val, ctx+".retention")
			if err != nil {
				return nil, err
			}
			if s != string(RetentionStandard) && s != string(RetentionNone) {
				return nil, fmt.Errorf("%w: %s.retention %q must be standard|none", ErrInvalidPolicy, ctx, s)
			}
			c.Retention = Retention(s)
		case "fallback_model_profiles":
			ss, err := decodeStringList(val, ctx+".fallback_model_profiles")
			if err != nil {
				return nil, err
			}
			for _, s := range ss {
				if !isValidProfileOrFallback(s) {
					return nil, fmt.Errorf("%w: %s.fallback_model_profiles %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
				}
				c.FallbackModelProfiles = append(c.FallbackModelProfiles, ModelProfile(s))
			}
		default:
			return nil, fmt.Errorf("%w: %s has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return c, nil
}

func decodeDefaults(node *yaml.Node, ctx string) (*Defaults, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: %s must be a mapping", ErrInvalidPolicy, ctx)
	}
	d := &Defaults{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "model_profile":
			s, err := decodeString(val, ctx+".model_profile")
			if err != nil {
				return nil, err
			}
			if !isValidProfileOrFallback(s) {
				return nil, fmt.Errorf("%w: %s.model_profile %q must be cheap|balanced|premium", ErrInvalidPolicy, ctx, s)
			}
			d.ModelProfile = ModelProfile(s)
		case "model_profile_name":
			s, err := decodeString(val, ctx+".model_profile_name")
			if err != nil {
				return nil, err
			}
			d.ModelProfileName = s
		case "provider":
			s, err := decodeString(val, ctx+".provider")
			if err != nil {
				return nil, err
			}
			d.Provider = s
		case "model":
			s, err := decodeString(val, ctx+".model")
			if err != nil {
				return nil, err
			}
			d.Model = s
		case "verifier":
			b, err := decodeBool(val, ctx+".verifier")
			if err != nil {
				return nil, err
			}
			d.Verifier = &b
		case "timeout_ms":
			n, err := decodeInt(val, ctx+".timeout_ms")
			if err != nil {
				return nil, err
			}
			d.TimeoutMS = &n
		case "retention":
			s, err := decodeString(val, ctx+".retention")
			if err != nil {
				return nil, err
			}
			if s != string(RetentionStandard) && s != string(RetentionNone) {
				return nil, fmt.Errorf("%w: %s.retention %q must be standard|none", ErrInvalidPolicy, ctx, s)
			}
			d.Retention = Retention(s)
		case "max_cost_usd":
			f, err := decodeFloat(val, ctx+".max_cost_usd")
			if err != nil {
				return nil, err
			}
			d.MaxCostUSD = &f
		case "max_latency_ms":
			n, err := decodeInt(val, ctx+".max_latency_ms")
			if err != nil {
				return nil, err
			}
			d.MaxLatencyMS = &n
		default:
			return nil, fmt.Errorf("%w: %s has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return d, nil
}

func decodeEnumMatch(node *yaml.Node, vocab map[string]struct{}, ctx string) (*EnumMatch, error) {
	if node.Kind == yaml.ScalarNode {
		v := node.Value
		if err := checkVocab(vocab, ctx, v); err != nil {
			return nil, err
		}
		return &EnumMatch{Values: []string{v}}, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: %s must be a string or {in: [...]}", ErrInvalidPolicy, ctx)
	}
	if len(node.Content) != 2 || node.Content[0].Value != "in" {
		return nil, fmt.Errorf("%w: %s mapping must have a single key 'in'", ErrInvalidPolicy, ctx)
	}
	list := node.Content[1]
	if list.Kind != yaml.SequenceNode || len(list.Content) == 0 {
		return nil, fmt.Errorf("%w: %s.in must be a non-empty list", ErrInvalidPolicy, ctx)
	}
	out := &EnumMatch{Values: make([]string, 0, len(list.Content))}
	for _, item := range list.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%w: %s.in entries must be strings", ErrInvalidPolicy, ctx)
		}
		if err := checkVocab(vocab, ctx, item.Value); err != nil {
			return nil, err
		}
		out.Values = append(out.Values, item.Value)
	}
	return out, nil
}

func checkVocab(vocab map[string]struct{}, ctx, value string) error {
	if vocab == nil {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s must be a non-empty string", ErrInvalidPolicy, ctx)
		}
		return nil
	}
	if _, ok := vocab[value]; !ok {
		return fmt.Errorf("%w: %s %q not in allowed vocabulary", ErrInvalidPolicy, ctx, value)
	}
	return nil
}

func decodeString(node *yaml.Node, ctx string) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%w: %s must be a string", ErrInvalidPolicy, ctx)
	}
	return node.Value, nil
}

func decodeInt(node *yaml.Node, ctx string) (int, error) {
	if node.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrInvalidPolicy, ctx)
	}
	var n int
	if err := node.Decode(&n); err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer (%v)", ErrInvalidPolicy, ctx, err)
	}
	return n, nil
}

func decodeFloat(node *yaml.Node, ctx string) (float64, error) {
	if node.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("%w: %s must be a number", ErrInvalidPolicy, ctx)
	}
	var f float64
	if err := node.Decode(&f); err != nil {
		return 0, fmt.Errorf("%w: %s must be a number (%v)", ErrInvalidPolicy, ctx, err)
	}
	return f, nil
}

func decodeBool(node *yaml.Node, ctx string) (bool, error) {
	if node.Kind != yaml.ScalarNode {
		return false, fmt.Errorf("%w: %s must be a boolean", ErrInvalidPolicy, ctx)
	}
	var b bool
	if err := node.Decode(&b); err != nil {
		return false, fmt.Errorf("%w: %s must be a boolean (%v)", ErrInvalidPolicy, ctx, err)
	}
	return b, nil
}

func decodeStringList(node *yaml.Node, ctx string) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%w: %s must be a list of strings", ErrInvalidPolicy, ctx)
	}
	out := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%w: %s entries must be strings", ErrInvalidPolicy, ctx)
		}
		out = append(out, item.Value)
	}
	return out, nil
}

// normalizeTags lowercases and trims compliance tags, dropping empties, so
// policy-side tags compare cleanly against the registry's normalized tags.
func normalizeTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, tag := range in {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			out = append(out, tag)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func decodeCapabilityList(node *yaml.Node, ctx string) ([]Capability, error) {
	ss, err := decodeStringList(node, ctx)
	if err != nil {
		return nil, err
	}
	caps := make([]Capability, 0, len(ss))
	for _, s := range ss {
		if !isValidCapability(s) {
			return nil, fmt.Errorf("%w: %s %q is not a recognised capability", ErrInvalidPolicy, ctx, s)
		}
		caps = append(caps, Capability(s))
	}
	return caps, nil
}

func isValidCapability(s string) bool {
	switch Capability(s) {
	case CapStreaming, CapToolUse, CapJSONSchema, CapVision, CapLongContext:
		return true
	default:
		return false
	}
}

func isValidProfileOrFallback(s string) bool {
	switch ModelProfile(s) {
	case ProfileCheap, ProfileBalanced, ProfilePremium:
		return true
	default:
		return false
	}
}
