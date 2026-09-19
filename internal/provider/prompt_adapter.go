package provider

import (
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
)

// PromptAdapter optionally rewrites existing system-role messages before the
// provider call. It is disabled by default and only mutates a cloned request.
type PromptAdapter struct {
	Enabled       bool
	ModelProfiles map[string]string
	Rules         []PromptAdapterRule
}

type PromptAdapterRule struct {
	Name     string
	Match    PromptAdapterMatch
	Mutation SystemPromptMutation
}

type PromptAdapterMatch struct {
	ModelIDs         []string
	ProviderModelIDs []string
	Profiles         []string
}

type SystemPromptMutation struct {
	Prefix string
	Suffix string
}

type PromptAdapterContext struct {
	ModelID         string
	ProviderModelID string
}

type PromptAdapterResult struct {
	AppliedRules []string
}

func (a *PromptAdapter) Apply(req *NormalizedModelRequest, ctx PromptAdapterContext) (*NormalizedModelRequest, PromptAdapterResult) {
	if a == nil || !a.Enabled || req == nil || len(a.Rules) == 0 {
		return nil, PromptAdapterResult{}
	}

	systemIndexes := systemMessageIndexes(req.Messages)
	if len(systemIndexes) == 0 {
		return nil, PromptAdapterResult{}
	}

	profile := a.profileFor(ctx)
	var (
		candidate *NormalizedModelRequest
		result    PromptAdapterResult
	)
	for _, rule := range a.Rules {
		if !rule.Match.matches(ctx, profile) {
			continue
		}
		if strings.TrimSpace(rule.Mutation.Prefix) == "" && strings.TrimSpace(rule.Mutation.Suffix) == "" {
			continue
		}
		if candidate == nil {
			candidate = req.Clone()
		}
		first := systemIndexes[0]
		last := systemIndexes[len(systemIndexes)-1]
		if rule.Mutation.Prefix != "" {
			candidate.Messages[first].Content = rule.Mutation.Prefix + candidate.Messages[first].Content
		}
		if rule.Mutation.Suffix != "" {
			candidate.Messages[last].Content += rule.Mutation.Suffix
		}
		result.AppliedRules = append(result.AppliedRules, rule.Name)
	}

	if candidate == nil {
		return nil, PromptAdapterResult{}
	}
	return candidate, result
}

func (a *PromptAdapter) profileFor(ctx PromptAdapterContext) string {
	if a == nil || len(a.ModelProfiles) == 0 {
		return ""
	}
	if profile := strings.TrimSpace(a.ModelProfiles[ctx.ModelID]); profile != "" {
		return profile
	}
	return strings.TrimSpace(a.ModelProfiles[ctx.ProviderModelID])
}

func (m PromptAdapterMatch) matches(ctx PromptAdapterContext, profile string) bool {
	if !matchesOneOf(ctx.ModelID, m.ModelIDs) {
		return false
	}
	if !matchesOneOf(ctx.ProviderModelID, m.ProviderModelIDs) {
		return false
	}
	if !matchesOneOf(profile, m.Profiles) {
		return false
	}
	return true
}

func matchesOneOf(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func systemMessageIndexes(messages []openai.Message) []int {
	indexes := make([]int, 0, len(messages))
	for i, message := range messages {
		if message.Role == "system" {
			indexes = append(indexes, i)
		}
	}
	return indexes
}
