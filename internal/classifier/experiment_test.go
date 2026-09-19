package classifier_test

import (
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/evals"
)

const classifierExperimentDatasetPath = "../../docs/fixtures/classifier-experiment-dataset-v1.yaml"

func TestLightweightClassifierExperiment_IsDeterministicAndOfflineOnly(t *testing.T) {
	ds, err := evals.LoadClassifierExperimentDataset(filepath.Clean(classifierExperimentDatasetPath))
	if err != nil {
		t.Fatalf("load classifier experiment dataset: %v", err)
	}

	train := experimentExamplesFromCases(ds.TrainCases())
	test := experimentExamplesFromCases(ds.TestCases())

	reportA, err := classifier.CompareExperimentWithRules(train, test)
	if err != nil {
		t.Fatalf("compare experiment with rules: %v", err)
	}
	reportB, err := classifier.CompareExperimentWithRules(train, test)
	if err != nil {
		t.Fatalf("compare experiment with rules (repeat): %v", err)
	}

	if reportA.Format() != reportB.Format() {
		t.Fatalf("expected deterministic report\nfirst:\n%s\nsecond:\n%s", reportA.Format(), reportB.Format())
	}

	t.Log("\n" + reportA.Format())

	if reportA.TrainCount == 0 || reportA.TestCount == 0 {
		t.Fatalf("expected train/test splits, got %+v", reportA)
	}
	if reportA.BaselineMetrics.Total != reportA.TestCount {
		t.Fatalf("baseline total = %d, want %d", reportA.BaselineMetrics.Total, reportA.TestCount)
	}
	if reportA.TrainedMetrics.Total != reportA.TestCount {
		t.Fatalf("trained total = %d, want %d", reportA.TrainedMetrics.Total, reportA.TestCount)
	}
	if reportA.TrainedMetrics.TaskAccuracy() < reportA.BaselineMetrics.TaskAccuracy() {
		t.Fatalf("trained task accuracy %.1f%% below baseline %.1f%%",
			reportA.TrainedMetrics.TaskAccuracy()*100,
			reportA.BaselineMetrics.TaskAccuracy()*100)
	}
	if reportA.TrainedMetrics.RiskAccuracy() < 0.80 {
		t.Fatalf("trained risk accuracy %.1f%% below floor 80%%", reportA.TrainedMetrics.RiskAccuracy()*100)
	}
}

func TestLightweightClassifierExperiment_PredictsKnownHeldOutCases(t *testing.T) {
	ds, err := evals.LoadClassifierExperimentDataset(filepath.Clean(classifierExperimentDatasetPath))
	if err != nil {
		t.Fatalf("load classifier experiment dataset: %v", err)
	}
	model, err := classifier.TrainLightweightExperiment(experimentExamplesFromCases(ds.TrainCases()))
	if err != nil {
		t.Fatalf("train model: %v", err)
	}

	expectations := map[string]struct {
		task string
		risk string
	}{
		"clf_test_shell_grep":      {task: "simple_shell", risk: "low"},
		"clf_test_unknown_secret":  {task: "unknown_high_risk", risk: "critical"},
		"clf_test_long_context":    {task: "long_context_analysis", risk: "low"},
		"clf_test_security_review": {task: "security_review", risk: "high"},
	}

	for _, c := range ds.TestCases() {
		want, ok := expectations[c.ID]
		if !ok {
			continue
		}
		got := model.Predict(c.PromptText())
		if got.TaskType != want.task || got.RiskLevel != want.risk {
			t.Fatalf("%s = (%s,%s), want (%s,%s)", c.ID, got.TaskType, got.RiskLevel, want.task, want.risk)
		}
	}
}

func experimentExamplesFromCases(cases []evals.ClassifierExperimentCase) []classifier.ExperimentExample {
	examples := make([]classifier.ExperimentExample, 0, len(cases))
	for _, c := range cases {
		examples = append(examples, classifier.ExperimentExample{
			ID:        c.ID,
			Prompt:    c.PromptText(),
			TaskType:  c.TaskType,
			RiskLevel: c.RiskLevel,
		})
	}
	return examples
}
