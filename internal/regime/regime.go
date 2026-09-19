// Package regime resolves the active compliance profile (ISSUE-094): the
// market/regime "module" that flavours the control report and public framing.
// A profile changes WORDS ONLY — report headings, named statute/authority,
// landing label. The routing engine stays market-neutral; rules come from the
// policy pack. Explicit ROUTER_PROFILE wins; otherwise the profile is derived
// from the active policy pack's version prefix, so enabling builtin:dora-baseline
// automatically flips the report into DORA framing.
package regime

import "strings"

// Profile is the per-regime wording used by the report and public pages.
type Profile struct {
	Key        string // nis2 | dora | gdpr | eu
	Name       string // human name of the regime
	Statute    string // named statute/regulation (framing, never a legal claim)
	Authority  string // supervisory context, phrased as an example
	ReportName string // control report title
	LandingLbl string // the landing hero label chip
	Framing    string // one-line report framing
}

var profiles = map[string]Profile{
	"nis2": {
		Key: "nis2", Name: "NIS2",
		Statute:    "NIS2 (Directive 2022/2555), transposed nationally (in Sweden: cybersäkerhetslagen, 2026)",
		Authority:  "the national NIS2 supervisory authority (in Sweden e.g. MSB and sector authorities)",
		ReportName: "Control report — LLM routing (NIS2 controls)",
		LandingLbl: "LLM egress control · NIS2",
		Framing:    "The controls below support risk management, data protection and traceability under NIS2. This is a control report, not a legal attestation.",
	},
	"dora": {
		Key: "dora", Name: "DORA (financial sector)",
		Statute:    "DORA (Regulation 2022/2554)",
		Authority:  "the financial supervisory authority (in Sweden e.g. Finansinspektionen)",
		ReportName: "Control report — LLM routing (DORA/ICT third-party controls)",
		LandingLbl: "LLM egress control · DORA",
		Framing:    "The controls below support ICT risk management and third-party control under DORA. This is a control report, not a legal attestation.",
	},
	"gdpr": {
		Key: "gdpr", Name: "GDPR / data sovereignty",
		Statute:    "GDPR (Regulation 2016/679), transfer rules after Schrems II",
		Authority:  "the data protection authority (in Sweden IMY)",
		ReportName: "Control report — LLM routing (GDPR/data residency)",
		LandingLbl: "LLM egress control · GDPR",
		Framing:    "The controls below support data minimisation and residency control for personal data. This is a control report, not a legal attestation.",
	},
	"eu": {
		Key: "eu", Name: "EU data-sovereign baseline",
		Statute:    "EU frameworks (NIS2, DORA, GDPR) depending on sector",
		Authority:  "the relevant national supervisory authority per sector",
		ReportName: "Control report — LLM routing",
		LandingLbl: "LLM egress control · EU",
		Framing:    "The controls below provide data-egress control and provable traceability for LLM usage. This is a control report, not a legal attestation.",
	},
}

// Get returns a profile by key.
func Get(key string) (Profile, bool) {
	p, ok := profiles[strings.ToLower(strings.TrimSpace(key))]
	return p, ok
}

// FromPolicyVersion derives the profile from the active policy pack's version
// prefix (pv_nis2_*, pv_dora_*, pv_gdpr_*). Unknown/empty → the EU baseline.
func FromPolicyVersion(version string) Profile {
	v := strings.ToLower(version)
	switch {
	case strings.HasPrefix(v, "pv_nis2"):
		return profiles["nis2"]
	case strings.HasPrefix(v, "pv_dora"):
		return profiles["dora"]
	case strings.HasPrefix(v, "pv_gdpr"):
		return profiles["gdpr"]
	default:
		return profiles["eu"]
	}
}

// Resolve picks the profile: an explicit key (ROUTER_PROFILE) wins; otherwise
// derive from the active policy version.
func Resolve(explicitKey, policyVersion string) Profile {
	if p, ok := Get(explicitKey); ok {
		return p
	}
	return FromPolicyVersion(policyVersion)
}
