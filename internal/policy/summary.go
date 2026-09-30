package policy

import (
	"fmt"
	"strings"
)

// RuleSummary is a human-readable one-liner of a compiled rule for the
// policy console (ISSUE-116): what it matches and what it does — so a CISO
// can read the active firewall without opening YAML.
type RuleSummary struct {
	ID          string
	Description string
	When        string
	Then        string
	Kind        string // "block" | "require" | "force" | "default" | "other"
}

// Summaries returns one summary per rule, in evaluation order. Nil-safe.
func (p *CompiledPolicy) Summaries() []RuleSummary {
	if p == nil || p.source == nil {
		return nil
	}
	out := make([]RuleSummary, 0, len(p.source.Rules))
	for _, r := range p.source.Rules {
		then, kind := summarizeRoute(r.Route)
		out = append(out, RuleSummary{
			ID:          r.ID,
			Description: strings.TrimSpace(r.Description),
			When:        summarizeWhen(r.When),
			Then:        then,
			Kind:        kind,
		})
	}
	return out
}

func enumText(label string, m *EnumMatch) string {
	if m == nil || len(m.Values) == 0 {
		return ""
	}
	return label + " " + strings.Join(m.Values, " or ")
}

func summarizeWhen(w When) string {
	var parts []string
	add := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	add(enumText("data is", w.Sensitivity))
	add(enumText("task is", w.TaskType))
	add(enumText("risk is", w.RiskLevel))
	add(enumText("agent can", w.ToolRisk))
	add(enumText("tenant is", w.Tenant))
	add(enumText("project is", w.Project))
	add(enumText("mode is", w.RouterMode))
	if len(w.AnyToolMatches) > 0 {
		add("a tool matches " + strings.Join(w.AnyToolMatches, ", "))
	}
	if len(w.ContainsAny) > 0 {
		add(fmt.Sprintf("prompt mentions %d watched term(s)", len(w.ContainsAny)))
	}
	if len(w.AnyFileMatches) > 0 {
		add("a file matches " + strings.Join(w.AnyFileMatches, ", "))
	}
	if w.PromptTokensGT != nil {
		add(fmt.Sprintf("prompt > %d tokens", *w.PromptTokensGT))
	}
	if w.PromptTokensLT != nil {
		add(fmt.Sprintf("prompt < %d tokens", *w.PromptTokensLT))
	}
	if w.RequiresToolUse != nil && *w.RequiresToolUse {
		add("tools are used")
	}
	if w.RequiresVision != nil && *w.RequiresVision {
		add("attachments need vision")
	}
	if w.RequiresJSONSchema != nil && *w.RequiresJSONSchema {
		add("structured output is requested")
	}
	if len(parts) == 0 {
		return "every request"
	}
	return strings.Join(parts, " · ")
}

func summarizeRoute(r Route) (string, string) {
	if r.Block != nil {
		msg := "BLOCK (fail-closed)"
		if r.Block.Code != "" {
			msg += " — " + r.Block.Code
		}
		return msg, "block"
	}
	var parts []string
	kind := "other"
	if c := r.Constraints; c != nil {
		if len(c.RequireProviderTags) > 0 {
			parts = append(parts, "only models tagged "+strings.Join(c.RequireProviderTags, " + "))
			kind = "require"
		}
		if len(c.DenyProviderTags) > 0 {
			parts = append(parts, "never models tagged "+strings.Join(c.DenyProviderTags, ", "))
			if kind == "other" {
				kind = "require"
			}
		}
		if len(c.AllowedProviders) > 0 {
			parts = append(parts, "providers "+strings.Join(c.AllowedProviders, ", ")+" only")
		}
		if len(c.DeniedProviders) > 0 {
			parts = append(parts, "not "+strings.Join(c.DeniedProviders, ", "))
		}
		if len(c.AllowedModels) > 0 {
			parts = append(parts, "models "+strings.Join(c.AllowedModels, ", ")+" only")
		}
		if len(c.DeniedModels) > 0 {
			parts = append(parts, "not models "+strings.Join(c.DeniedModels, ", "))
		}
		if c.MaxCostUSD != nil {
			parts = append(parts, fmt.Sprintf("max $%.2f", *c.MaxCostUSD))
		}
	}
	if f := r.Force; f != nil {
		switch {
		case f.Model != "":
			parts = append(parts, "force model "+f.Model)
		case f.ModelProfileName != "":
			parts = append(parts, "force "+f.ModelProfileName)
		case f.ModelProfile != "":
			parts = append(parts, "force "+string(f.ModelProfile)+" tier")
		}
		if f.Verifier != nil && *f.Verifier {
			parts = append(parts, "with verifier")
		}
		if kind == "other" {
			kind = "force"
		}
	}
	if d := r.Defaults; d != nil {
		switch {
		case d.ModelProfileName != "":
			parts = append(parts, "default "+d.ModelProfileName)
		case d.ModelProfile != "":
			parts = append(parts, "default "+string(d.ModelProfile)+" tier")
		}
		if kind == "other" {
			kind = "default"
		}
	}
	if h := r.Hints; h.Tier != "" {
		parts = append(parts, "prefer "+h.Tier+" tier")
	}
	if len(parts) == 0 {
		return "route by scoring", kind
	}
	return strings.Join(parts, " · "), kind
}
