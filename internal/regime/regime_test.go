package regime

import "testing"

func TestFromPolicyVersion(t *testing.T) {
	cases := map[string]string{
		"pv_nis2_baseline_2026_01":  "nis2",
		"pv_dora_baseline_2026_01":  "dora",
		"pv_gdpr_sovereign_2026_01": "gdpr",
		"pv_pii_local_2026_07":      "eu", // grundskyddet är regim-neutralt
		"pv_runtime_2026_06_12":     "eu",
		"":                          "eu",
	}
	for version, want := range cases {
		if got := FromPolicyVersion(version).Key; got != want {
			t.Errorf("FromPolicyVersion(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestResolveExplicitWins(t *testing.T) {
	// Explicit ROUTER_PROFILE beats the derived pack profile.
	if got := Resolve("dora", "pv_nis2_baseline_2026_01").Key; got != "dora" {
		t.Fatalf("explicit profile should win, got %q", got)
	}
	// Unknown explicit falls back to derivation.
	if got := Resolve("banana", "pv_gdpr_sovereign_2026_01").Key; got != "gdpr" {
		t.Fatalf("unknown explicit should derive, got %q", got)
	}
	// Every profile carries the non-negotiable wording pieces.
	for key := range profiles {
		p, _ := Get(key)
		if p.ReportName == "" || p.LandingLbl == "" || p.Framing == "" {
			t.Errorf("profile %q missing wording fields: %+v", key, p)
		}
	}
}
