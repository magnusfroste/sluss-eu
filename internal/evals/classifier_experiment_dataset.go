package evals

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClassifierExperimentDataset is a deterministic offline dataset for
// lightweight classifier experiments. It is not used in the production request
// path.
type ClassifierExperimentDataset struct {
	Version string                     `yaml:"version"`
	Cases   []ClassifierExperimentCase `yaml:"cases"`
}

// ClassifierExperimentCase is a labeled prompt for the offline experiment.
type ClassifierExperimentCase struct {
	ID           string `yaml:"id"`
	Split        string `yaml:"split"`
	TaskType     string `yaml:"task_type"`
	RiskLevel    string `yaml:"risk_level"`
	Prompt       string `yaml:"prompt"`
	ExpandTokens int    `yaml:"expand_tokens,omitempty"`
}

// PromptText returns the effective prompt, expanding large-context fixtures the
// same way as the routing eval dataset.
func (c ClassifierExperimentCase) PromptText() string {
	if c.ExpandTokens <= 0 {
		return c.Prompt
	}
	targetChars := c.ExpandTokens * 4
	var b strings.Builder
	b.WriteString(c.Prompt)
	if !strings.HasSuffix(c.Prompt, "\n") {
		b.WriteByte('\n')
	}
	filler := "This offline classifier experiment repeats neutral context to exercise long-context labeling only. "
	for b.Len() < targetChars {
		b.WriteString(filler)
	}
	return b.String()
}

// LoadClassifierExperimentDataset reads and validates the experiment dataset.
func LoadClassifierExperimentDataset(path string) (*ClassifierExperimentDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("evals: read classifier experiment dataset: %w", err)
	}
	var ds ClassifierExperimentDataset
	if err := yaml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("evals: parse classifier experiment dataset: %w", err)
	}
	if err := ds.Validate(); err != nil {
		return nil, err
	}
	return &ds, nil
}

// Validate checks structure and the fabricated-secret guard.
func (ds *ClassifierExperimentDataset) Validate() error {
	if len(ds.Cases) == 0 {
		return fmt.Errorf("evals: classifier experiment dataset has no cases")
	}
	seen := make(map[string]struct{}, len(ds.Cases))
	trainCount := 0
	testCount := 0
	for i, c := range ds.Cases {
		if c.ID == "" {
			return fmt.Errorf("evals: classifier experiment case %d missing id", i)
		}
		if _, dup := seen[c.ID]; dup {
			return fmt.Errorf("evals: classifier experiment duplicate case id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		switch c.Split {
		case "train":
			trainCount++
		case "test":
			testCount++
		default:
			return fmt.Errorf("evals: classifier experiment case %q has invalid split %q", c.ID, c.Split)
		}
		if c.TaskType == "" {
			return fmt.Errorf("evals: classifier experiment case %q missing task_type", c.ID)
		}
		if c.RiskLevel == "" {
			return fmt.Errorf("evals: classifier experiment case %q missing risk_level", c.ID)
		}
		if strings.TrimSpace(c.Prompt) == "" {
			return fmt.Errorf("evals: classifier experiment case %q missing prompt", c.ID)
		}
		if err := checkNoSecrets(Case{ID: c.ID, Prompt: c.Prompt}); err != nil {
			return err
		}
	}
	if trainCount == 0 || testCount == 0 {
		return fmt.Errorf("evals: classifier experiment dataset requires both train and test cases")
	}
	return nil
}

// TrainCases returns the ordered training split.
func (ds *ClassifierExperimentDataset) TrainCases() []ClassifierExperimentCase {
	return ds.filterSplit("train")
}

// TestCases returns the ordered test split.
func (ds *ClassifierExperimentDataset) TestCases() []ClassifierExperimentCase {
	return ds.filterSplit("test")
}

func (ds *ClassifierExperimentDataset) filterSplit(split string) []ClassifierExperimentCase {
	out := make([]ClassifierExperimentCase, 0, len(ds.Cases))
	for _, c := range ds.Cases {
		if c.Split == split {
			out = append(out, c)
		}
	}
	return out
}
