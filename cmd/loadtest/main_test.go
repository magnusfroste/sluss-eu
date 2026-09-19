package main

import (
	"math"
	"testing"
)

func TestPercentileNearestRank(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tests := []struct {
		p    float64
		want float64
	}{
		{0.0, 1},   // clamps to min
		{0.5, 5},   // ceil(0.5*10)=5 → sorted[4]
		{0.9, 9},   // ceil(0.9*10)=9 → sorted[8]
		{0.95, 10}, // ceil(0.95*10)=10 → sorted[9]
		{0.99, 10},
		{1.0, 10}, // clamps to max
	}
	for _, tt := range tests {
		if got := percentile(sorted, tt.p); got != tt.want {
			t.Errorf("percentile(p=%.2f) = %.1f, want %.1f", tt.p, got, tt.want)
		}
	}
}

func TestPercentileEmptyAndSingle(t *testing.T) {
	if got := percentile(nil, 0.95); got != 0 {
		t.Errorf("empty slice should be 0, got %.1f", got)
	}
	if got := percentile([]float64{42}, 0.95); got != 42 {
		t.Errorf("single element should be itself, got %.1f", got)
	}
}

func TestPercentileMonotonic(t *testing.T) {
	sorted := make([]float64, 1000)
	for i := range sorted {
		sorted[i] = float64(i)
	}
	p50 := percentile(sorted, 0.50)
	p95 := percentile(sorted, 0.95)
	p99 := percentile(sorted, 0.99)
	if !(p50 <= p95 && p95 <= p99) {
		t.Errorf("percentiles not monotonic: p50=%.0f p95=%.0f p99=%.0f", p50, p95, p99)
	}
	if math.Abs(p95-950) > 1 {
		t.Errorf("p95 of 0..999 should be ~950, got %.0f", p95)
	}
}
