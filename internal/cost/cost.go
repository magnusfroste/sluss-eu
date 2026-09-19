// Package cost contains pure fixed-point cost estimation helpers.
package cost

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/magnusfroste/sluss/internal/registry"
)

type Mode string

const (
	ModeEstimated Mode = "estimated"
	ModeActual    Mode = "actual"
)

var (
	ErrMissingMetadata = errors.New("cost: missing cost metadata")
	ErrInvalidUsage    = errors.New("cost: invalid token usage")
	ErrInvalidRate     = errors.New("cost: invalid rate")
	ErrOverflow        = errors.New("cost: micro-usd overflow")
)

type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	Mode         Mode
}

type Estimate struct {
	ModelID         string
	ProviderID      string
	Currency        string
	Mode            Mode
	InputTokens     int64
	OutputTokens    int64
	InputMicroUSD   int64
	OutputMicroUSD  int64
	TotalMicroUSD   int64
	InputUSDString  string
	OutputUSDString string
	TotalUSDString  string
}

func EstimateModel(model registry.Model, usage TokenUsage) (Estimate, error) {
	return EstimateCost(model.ID, model.ProviderID, model.Cost, usage)
}

func EstimateCost(modelID, providerID string, meta registry.CostMetadata, usage TokenUsage) (Estimate, error) {
	if err := validateMetadata(meta); err != nil {
		return Estimate{}, err
	}
	if usage.InputTokens < 0 || usage.OutputTokens < 0 {
		return Estimate{}, ErrInvalidUsage
	}
	mode := usage.Mode
	if mode == "" {
		mode = ModeEstimated
	}
	input, err := microsForTokens(usage.InputTokens, meta.InputMicrosPerMillionToken)
	if err != nil {
		return Estimate{}, err
	}
	output, err := microsForTokens(usage.OutputTokens, meta.OutputMicrosPerMillionToken)
	if err != nil {
		return Estimate{}, err
	}
	if input > math.MaxInt64-output {
		return Estimate{}, ErrOverflow
	}
	total := input + output
	return Estimate{
		ModelID:         modelID,
		ProviderID:      providerID,
		Currency:        meta.Currency,
		Mode:            mode,
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		InputMicroUSD:   input,
		OutputMicroUSD:  output,
		TotalMicroUSD:   total,
		InputUSDString:  formatMicroUSD(input),
		OutputUSDString: formatMicroUSD(output),
		TotalUSDString:  formatMicroUSD(total),
	}, nil
}

func USDPerMillion(input, output string) (registry.CostMetadata, error) {
	inputMicros, err := parseUSDMicros(input)
	if err != nil {
		return registry.CostMetadata{}, fmt.Errorf("%w: input: %v", ErrInvalidRate, err)
	}
	outputMicros, err := parseUSDMicros(output)
	if err != nil {
		return registry.CostMetadata{}, fmt.Errorf("%w: output: %v", ErrInvalidRate, err)
	}
	return registry.CostMetadata{
		Currency:                    "USD",
		InputMicrosPerMillionToken:  inputMicros,
		OutputMicrosPerMillionToken: outputMicros,
	}, nil
}

func validateMetadata(meta registry.CostMetadata) error {
	if strings.TrimSpace(meta.Currency) == "" {
		return ErrMissingMetadata
	}
	if meta.InputMicrosPerMillionToken < 0 || meta.OutputMicrosPerMillionToken < 0 {
		return ErrInvalidRate
	}
	if meta.InputMicrosPerMillionToken == 0 && meta.OutputMicrosPerMillionToken == 0 {
		return ErrMissingMetadata
	}
	return nil
}

func microsForTokens(tokens, microsPerMillion int64) (int64, error) {
	if tokens == 0 || microsPerMillion == 0 {
		return 0, nil
	}
	product := new(big.Int).Mul(big.NewInt(tokens), big.NewInt(microsPerMillion))
	product.Quo(product, big.NewInt(1000000))
	if !product.IsInt64() {
		return 0, ErrOverflow
	}
	return product.Int64(), nil
}

func parseUSDMicros(rate string) (int64, error) {
	rate = strings.TrimSpace(rate)
	if rate == "" {
		return 0, errors.New("empty rate")
	}
	value, ok := new(big.Rat).SetString(rate)
	if !ok || value.Sign() < 0 {
		return 0, errors.New("rate must be a non-negative decimal")
	}
	value.Mul(value, big.NewRat(1000000, 1))
	value.Quo(value, big.NewRat(1, 1))
	if !value.IsInt() {
		return 0, errors.New("rate has precision below one micro-usd")
	}
	out := value.Num()
	if !out.IsInt64() {
		return 0, ErrOverflow
	}
	return out.Int64(), nil
}

func formatMicroUSD(micros int64) string {
	whole := micros / 1000000
	fraction := micros % 1000000
	return fmt.Sprintf("%d.%06d", whole, fraction)
}

// FormatMicroUSD renders a micro-USD amount as a fixed-point USD string with six
// decimal places (e.g. 1500000 → "1.500000").
func FormatMicroUSD(micros int64) string {
	return formatMicroUSD(micros)
}
