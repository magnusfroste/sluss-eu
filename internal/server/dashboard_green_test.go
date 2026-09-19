package server

import (
	"math"
	"testing"

	"github.com/magnusfroste/sluss/internal/spend"
)

func TestComputeGreen(t *testing.T) {
	// 1M tokens on cheap (150 Wh/MTok) vs an all-premium baseline (1500 Wh/MTok):
	// actual 150 Wh, baseline 1500 Wh → 1350 Wh saved; at 400 g/kWh → 540 g CO2e.
	rows := []spend.ModelRow{{ModelID: "cheap-general", InputTokens: 600_000, OutputTokens: 400_000}}
	byModel := map[string]float64{"cheap-general": 150}

	g := computeGreen(rows, byModel, 1500, 400)

	if math.Abs(g.ActualWh-150) > 1e-9 || math.Abs(g.BaselineWh-1500) > 1e-9 {
		t.Errorf("actual/baseline = %.2f/%.2f, want 150/1500", g.ActualWh, g.BaselineWh)
	}
	if math.Abs(g.SavedWh-1350) > 1e-9 {
		t.Errorf("saved Wh = %.2f, want 1350", g.SavedWh)
	}
	if math.Abs(g.SavedCO2eGrams-540) > 1e-9 {
		t.Errorf("saved CO2e = %.2f g, want 540", g.SavedCO2eGrams)
	}
}

func TestComputeGreenUnknownModelIsConservative(t *testing.T) {
	// A model without an energy estimate counts as premium → zero saving.
	rows := []spend.ModelRow{{ModelID: "mystery", InputTokens: 1_000_000}}
	g := computeGreen(rows, map[string]float64{}, 1500, 400)
	if g.SavedWh != 0 {
		t.Errorf("unknown model should contribute zero saving, got %.2f Wh", g.SavedWh)
	}
}

func TestComputeGreenDisabledWithoutBaseline(t *testing.T) {
	rows := []spend.ModelRow{{ModelID: "cheap-general", InputTokens: 1000}}
	g := computeGreen(rows, map[string]float64{"cheap-general": 150}, 0, 400)
	if g.SavedWh != 0 || g.BaselineWh != 0 {
		t.Errorf("zero premium baseline should disable, got %+v", g)
	}
}
