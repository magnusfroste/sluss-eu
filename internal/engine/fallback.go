package engine

import (
	"fmt"
	"strings"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

const maxFallbacks = 2

// BuildFallbackChain constructs the ordered fallback chain from the scored
// candidate list (primary excluded). For high/critical risk tasks the chain
// only contains models at least as capable as the primary — never downward.
func BuildFallbackChain(
	primary registry.Model,
	scored []ScoredCandidate,
	job *router.JobDescriptor,
	route policy.Route,
) []FallbackEntry {
	var chain []FallbackEntry
	for _, sc := range scored {
		if len(chain) >= maxFallbacks {
			break
		}
		if sc.Model.ID == primary.ID {
			continue
		}
		if !validFallback(primary, sc.Model, job, route) {
			continue
		}
		chain = append(chain, FallbackEntry{
			ModelID:         sc.Model.ID,
			ProviderID:      sc.Model.ProviderID,
			ProviderModelID: sc.Model.ProviderModelID,
		})
	}
	return chain
}

// validFallback returns true if candidate is an acceptable fallback for primary.
func validFallback(primary, candidate registry.Model, job *router.JobDescriptor, route policy.Route) bool {
	// Risky tasks: fallback must not be a lower tier than primary.
	if job.RiskLevel == router.RiskHigh || job.RiskLevel == router.RiskCritical {
		if TierOrdinal(candidate.Tier) < TierOrdinal(primary.Tier) {
			return false
		}
	}
	// Policy fallback profile allowlist.
	if c := route.Constraints; c != nil && len(c.FallbackModelProfiles) > 0 {
		for _, profile := range c.FallbackModelProfiles {
			if string(candidate.Tier) == string(profile) {
				return true
			}
		}
		return false
	}
	return true
}

// FallbackChainReason returns a human-readable summary of the fallback chain.
func FallbackChainReason(primary registry.Model, chain []FallbackEntry) string {
	if len(chain) == 0 {
		return fmt.Sprintf("No fallbacks available beyond primary %s", primary.ID)
	}
	ids := make([]string, len(chain))
	for i, e := range chain {
		ids[i] = e.ModelID
	}
	return fmt.Sprintf("Fallback chain: %s → %s", primary.ID, strings.Join(ids, " → "))
}
