package policy

import "testing"

// require_provider_tags / deny_provider_tags parse into constraints, normalized
// (lowercased, trimmed) so they compare cleanly against registry tags
// (ISSUE-078).
func TestParseProviderTagConstraints(t *testing.T) {
	src := `
version: pv_tags
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: pii_stays_in_eu
    when:
      sensitivity: pii
    route:
      constraints:
        require_provider_tags: [" EU-Resident ", dpa-signed]
        deny_provider_tags: [US-ONLY]
  - id: default
    when: {}
    route:
      defaults:
        model_profile: balanced
`
	p := mustParse(t, src)
	c := p.Rules[0].Route.Constraints
	if c == nil {
		t.Fatal("constraints not parsed")
	}
	if len(c.RequireProviderTags) != 2 || c.RequireProviderTags[0] != "eu-resident" || c.RequireProviderTags[1] != "dpa-signed" {
		t.Fatalf("require tags = %v, want normalized [eu-resident dpa-signed]", c.RequireProviderTags)
	}
	if len(c.DenyProviderTags) != 1 || c.DenyProviderTags[0] != "us-only" {
		t.Fatalf("deny tags = %v, want [us-only]", c.DenyProviderTags)
	}
}

func TestParseProviderTagsRejectNonList(t *testing.T) {
	src := `
version: pv_tags_bad
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: bad
    when: {}
    route:
      constraints:
        require_provider_tags: eu-resident
`
	if _, err := Parse([]byte(src)); err == nil {
		t.Fatal("scalar require_provider_tags should be rejected (must be a list)")
	}
}
