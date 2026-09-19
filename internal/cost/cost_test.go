package cost

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

func TestEstimateCost(t *testing.T) {
	standard := registry.CostMetadata{
		Currency:                    "USD",
		InputMicrosPerMillionToken:  1000000,
		OutputMicrosPerMillionToken: 3000000,
	}
	decimal, err := USDPerMillion("0.125", "2.50")
	if err != nil {
		t.Fatalf("USDPerMillion: %v", err)
	}
	tests := []struct {
		name       string
		meta       registry.CostMetadata
		usage      TokenUsage
		wantInput  int64
		wantOutput int64
		wantTotal  int64
		wantMode   Mode
		wantErr    error
	}{
		{
			name:       "standard estimated",
			meta:       standard,
			usage:      TokenUsage{InputTokens: 1000000, OutputTokens: 500000, Mode: ModeEstimated},
			wantInput:  1000000,
			wantOutput: 1500000,
			wantTotal:  2500000,
			wantMode:   ModeEstimated,
		},
		{
			name:      "zero tokens",
			meta:      standard,
			usage:     TokenUsage{},
			wantMode:  ModeEstimated,
			wantTotal: 0,
		},
		{
			name:      "input only",
			meta:      standard,
			usage:     TokenUsage{InputTokens: 250000, Mode: ModeActual},
			wantInput: 250000,
			wantTotal: 250000,
			wantMode:  ModeActual,
		},
		{
			name:       "output only",
			meta:       standard,
			usage:      TokenUsage{OutputTokens: 250000},
			wantOutput: 750000,
			wantTotal:  750000,
			wantMode:   ModeEstimated,
		},
		{
			name:       "decimal rates",
			meta:       decimal,
			usage:      TokenUsage{InputTokens: 1000000, OutputTokens: 200000},
			wantInput:  125000,
			wantOutput: 500000,
			wantTotal:  625000,
			wantMode:   ModeEstimated,
		},
		{
			name:       "large counts",
			meta:       standard,
			usage:      TokenUsage{InputTokens: 1000000000, OutputTokens: 1000000000},
			wantInput:  1000000000,
			wantOutput: 3000000000,
			wantTotal:  4000000000,
			wantMode:   ModeEstimated,
		},
		{
			name:    "missing metadata",
			meta:    registry.CostMetadata{},
			usage:   TokenUsage{InputTokens: 1},
			wantErr: ErrMissingMetadata,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EstimateCost("model-a", "provider-a", tt.meta, tt.usage)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("EstimateCost: %v", err)
			}
			if got.InputMicroUSD != tt.wantInput || got.OutputMicroUSD != tt.wantOutput || got.TotalMicroUSD != tt.wantTotal {
				t.Fatalf("got costs %+v", got)
			}
			if got.Mode != tt.wantMode {
				t.Fatalf("got mode %q, want %q", got.Mode, tt.wantMode)
			}
		})
	}
}

func TestEstimateModelCarriesIdentityAndStrings(t *testing.T) {
	got, err := EstimateModel(registry.Model{
		ID:         "balanced-coder",
		ProviderID: "openai",
		Cost: registry.CostMetadata{
			Currency:                    "USD",
			InputMicrosPerMillionToken:  1000000,
			OutputMicrosPerMillionToken: 3000000,
		},
	}, TokenUsage{InputTokens: 1000000, OutputTokens: 1000000, Mode: ModeActual})
	if err != nil {
		t.Fatalf("EstimateModel: %v", err)
	}
	if got.ModelID != "balanced-coder" || got.ProviderID != "openai" || got.Mode != ModeActual {
		t.Fatalf("identity not carried: %+v", got)
	}
	if got.InputUSDString != "1.000000" || got.OutputUSDString != "3.000000" || got.TotalUSDString != "4.000000" {
		t.Fatalf("unexpected decimal strings: %+v", got)
	}
}

func TestUSDPerMillionRejectsInvalidRates(t *testing.T) {
	if _, err := USDPerMillion("-1", "0"); !errors.Is(err, ErrInvalidRate) {
		t.Fatalf("got %v", err)
	}
	if _, err := USDPerMillion("0.0000001", "0"); !errors.Is(err, ErrInvalidRate) {
		t.Fatalf("got %v", err)
	}
}
