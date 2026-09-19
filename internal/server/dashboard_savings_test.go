package server

import (
	"math"
	"testing"

	"github.com/magnusfroste/sluss/internal/spend"
)

func TestComputeSavings(t *testing.T) {
	// Premium pricing: $3/Mtok input, $15/Mtok output (micros per million tokens).
	const premIn, premOut = 3_000_000, 15_000_000

	// One cheap row: 1M input + 1M output tokens, actually costing $0.75
	// (gpt-4o-mini: $0.15 in + $0.60 out). All-premium baseline would be
	// 1M*$3 + 1M*$15 = $18.
	rows := []spend.ModelRow{{
		ModelID:      "cheap-general",
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
		CostUSD:      0.75,
	}}

	got := computeSavings(rows, 0.75, premIn, premOut, "openai/gpt-4o")

	if math.Abs(got.PremiumBaselineUSD-18.0) > 1e-9 {
		t.Errorf("premium baseline = %.6f, want 18", got.PremiumBaselineUSD)
	}
	if math.Abs(got.SavedUSD-17.25) > 1e-9 {
		t.Errorf("saved = %.6f, want 17.25", got.SavedUSD)
	}
	if math.Abs(got.SavedPct-95.8333) > 0.01 {
		t.Errorf("saved pct = %.4f, want ~95.83", got.SavedPct)
	}
	if got.BaselineModel != "openai/gpt-4o" {
		t.Errorf("baseline model = %q, want openai/gpt-4o", got.BaselineModel)
	}
}

func TestComputeSavingsNoPricingIsZero(t *testing.T) {
	rows := []spend.ModelRow{{InputTokens: 100, OutputTokens: 100, CostUSD: 0.01}}
	got := computeSavings(rows, 0.01, 0, 0, "")
	if got.PremiumBaselineUSD != 0 || got.SavedUSD != 0 || got.SavedPct != 0 {
		t.Errorf("no pricing should yield zero baseline/savings, got %+v", got)
	}
	if got.ActualUSD != 0.01 {
		t.Errorf("actual should still be reported, got %v", got.ActualUSD)
	}
}
