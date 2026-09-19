package evals

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/router"
)

// MeasuredQuality is a per-(task, model) predicted-quality signal derived from
// the eval frontier report's measured pass rates. It implements
// engine.QualitySource so the routing engine can prefer the cheapest model
// measured "good enough" per task class instead of relying on static registry
// priors. Only models with at least MinSamples eval samples contribute, so a
// single lucky/unlucky run does not swing routing.
type MeasuredQuality struct {
	byTaskModel map[string]map[string]float64
}

// compile-time check that MeasuredQuality satisfies the engine contract.
var _ engine.QualitySource = (*MeasuredQuality)(nil)

// NewMeasuredQuality builds a quality source from a frontier report. minSamples
// (< 1 → 1) is the minimum eval sample count required for a model's measured
// pass rate to be used for a task class.
func NewMeasuredQuality(report FrontierReport, minSamples int) *MeasuredQuality {
	if minSamples < 1 {
		minSamples = 1
	}
	m := &MeasuredQuality{byTaskModel: make(map[string]map[string]float64)}
	for _, tc := range report.TaskClasses {
		for _, fm := range tc.Models {
			if fm.EvalSamples < minSamples {
				continue
			}
			byModel, ok := m.byTaskModel[tc.TaskClass]
			if !ok {
				byModel = make(map[string]float64)
				m.byTaskModel[tc.TaskClass] = byModel
			}
			byModel[fm.ModelID] = fm.EvalPassRate
		}
	}
	return m
}

// Quality returns the measured pass rate for a (task, model) pair, or false when
// no sufficiently-sampled measurement exists (scoring then falls back to the
// static registry prior).
func (m *MeasuredQuality) Quality(task router.TaskType, modelID string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	byModel, ok := m.byTaskModel[string(task)]
	if !ok {
		return 0, false
	}
	s, ok := byModel[modelID]
	return s, ok
}

// TaskClasses returns the number of task classes with at least one measured
// model. Intended for logging/diagnostics.
func (m *MeasuredQuality) TaskClasses() int {
	if m == nil {
		return 0
	}
	return len(m.byTaskModel)
}

// LoadReport reads an eval report (as written by cmd/eval-report) from a JSON
// file. Its Frontier field feeds NewMeasuredQuality.
func LoadReport(path string) (Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read eval report: %w", err)
	}
	var report Report
	if err := json.Unmarshal(raw, &report); err != nil {
		return Report{}, fmt.Errorf("parse eval report: %w", err)
	}
	return report, nil
}
