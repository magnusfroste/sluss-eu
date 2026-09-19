package spend

import (
	"bytes"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/cost"
	"github.com/magnusfroste/sluss/internal/registry"
)

func mustModel(t *testing.T, id, provider, inUSD, outUSD string) registry.Model {
	t.Helper()
	meta, err := cost.USDPerMillion(inUSD, outUSD)
	if err != nil {
		t.Fatalf("cost meta: %v", err)
	}
	return registry.Model{ID: id, ProviderID: provider, Cost: meta}
}

func TestSimulatorBaselineSavings(t *testing.T) {
	// Premium: $15/$60 per Mtok. Cheap: $1/$3 per Mtok.
	premium := mustModel(t, "premium-reasoner", "anthropic", "15", "60")
	cheap := mustModel(t, "cheap-fast", "openai", "1", "3")

	sim := Simulator{Baseline: premium}

	// Two requests, both routed to cheap, both low risk so no risk discount.
	reqs := []SimRequest{
		{InputTokens: 1_000_000, OutputTokens: 1_000_000, RiskLevel: "low", RoutedModel: cheap},
		{InputTokens: 1_000_000, OutputTokens: 1_000_000, RiskLevel: "low", RoutedModel: cheap},
	}
	res, err := sim.Run(reqs)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Per request: baseline = 15+60 = $75; actual = 1+3 = $4 → saving $71.
	// micro-USD: baseline 150_000_000, actual 8_000_000 over two requests.
	if res.BaselinePremiumMicroUSD != 150_000_000 {
		t.Errorf("baseline = %d, want 150000000", res.BaselinePremiumMicroUSD)
	}
	if res.ActualMicroUSD != 8_000_000 {
		t.Errorf("actual = %d, want 8000000", res.ActualMicroUSD)
	}
	if res.SavingsMicroUSD != 142_000_000 {
		t.Errorf("savings = %d, want 142000000", res.SavingsMicroUSD)
	}
	// Low risk → weight 1.0, so risk-adjusted equals raw savings.
	if res.RiskAdjustedSavingsMicroUSD != res.SavingsMicroUSD {
		t.Errorf("risk-adjusted = %d, want = savings %d", res.RiskAdjustedSavingsMicroUSD, res.SavingsMicroUSD)
	}
	if res.SavingsPercent < 94.0 || res.SavingsPercent > 95.0 {
		t.Errorf("savings%% = %.2f, want ~94.7", res.SavingsPercent)
	}
}

func TestSimulatorRiskAdjustsSavings(t *testing.T) {
	premium := mustModel(t, "premium", "anthropic", "10", "10")
	cheap := mustModel(t, "cheap", "openai", "1", "1")
	sim := Simulator{Baseline: premium}

	// One high-risk request routed to cheap. Per request:
	// baseline = 10+10 = $20 (20_000_000), actual = 1+1 = $2 (2_000_000),
	// saving = 18_000_000. High risk weight = 0.5 → risk-adjusted = 9_000_000.
	res, err := sim.Run([]SimRequest{
		{InputTokens: 1_000_000, OutputTokens: 1_000_000, RiskLevel: "high", RoutedModel: cheap},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.SavingsMicroUSD != 18_000_000 {
		t.Fatalf("savings = %d, want 18000000", res.SavingsMicroUSD)
	}
	if res.RiskAdjustedSavingsMicroUSD != 9_000_000 {
		t.Errorf("risk-adjusted = %d, want 9000000 (0.5 weight)", res.RiskAdjustedSavingsMicroUSD)
	}
}

func TestSimulatorUnknownRiskUsesDefaultWeight(t *testing.T) {
	premium := mustModel(t, "premium", "anthropic", "10", "10")
	cheap := mustModel(t, "cheap", "openai", "2", "2")
	sim := Simulator{Baseline: premium}

	res, err := sim.Run([]SimRequest{
		{InputTokens: 1_000_000, OutputTokens: 0, RiskLevel: "", RoutedModel: cheap},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// baseline 10_000_000, actual 2_000_000, saving 8_000_000, default weight 0.5.
	if res.RiskAdjustedSavingsMicroUSD != 4_000_000 {
		t.Errorf("risk-adjusted = %d, want 4000000", res.RiskAdjustedSavingsMicroUSD)
	}
}

func TestSimulatorErrorsOnMissingCostMetadata(t *testing.T) {
	premium := mustModel(t, "premium", "anthropic", "10", "10")
	bad := registry.Model{ID: "bad", ProviderID: "x"} // no cost metadata
	sim := Simulator{Baseline: premium}

	if _, err := sim.Run([]SimRequest{{InputTokens: 100, RoutedModel: bad}}); err == nil {
		t.Fatal("expected error for routed model without cost metadata")
	}
}

func TestSimResultSummary(t *testing.T) {
	res := SimResult{
		Requests:                    3,
		BaselinePremiumMicroUSD:     150_000_000,
		ActualMicroUSD:              8_000_000,
		SavingsMicroUSD:             142_000_000,
		SavingsPercent:              94.7,
		RiskAdjustedSavingsMicroUSD: 100_000_000,
	}
	var buf bytes.Buffer
	res.Summary(&buf)
	out := buf.String()
	for _, want := range []string{"Baseline (all premium)", "150.000000", "Risk-adjusted savings", "100.000000", "94.7%"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}
