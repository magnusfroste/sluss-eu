package evals_test

import (
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

const classifierDatasetPath = "../../docs/fixtures/classifier-experiment-dataset-v1.yaml"

func TestClassifierExperimentDataset_IsValidAndSplit(t *testing.T) {
	ds, err := evals.LoadClassifierExperimentDataset(filepath.Clean(classifierDatasetPath))
	if err != nil {
		t.Fatalf("load classifier experiment dataset: %v", err)
	}
	if len(ds.TrainCases()) < 10 {
		t.Fatalf("train split too small: %d", len(ds.TrainCases()))
	}
	if len(ds.TestCases()) < 5 {
		t.Fatalf("test split too small: %d", len(ds.TestCases()))
	}
}
