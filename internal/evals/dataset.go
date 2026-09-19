// Package evals loads the golden routing dataset and runs each case through the
// real feature extractor and routing engine in dry-run mode (no provider call).
// It produces a pass/fail report used by `make test-eval` and the regression
// suite.
package evals

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Dataset is a parsed eval dataset file.
type Dataset struct {
	Version string `yaml:"version"`
	Cases   []Case `yaml:"cases"`
}

// Case is a single golden routing case.
type Case struct {
	ID            string         `yaml:"id"`
	Name          string         `yaml:"name"`
	TaskType      string         `yaml:"task_type"`
	RiskLevel     string         `yaml:"risk_level"`
	Prompt        string         `yaml:"prompt"`
	TenantID      string         `yaml:"tenant_id,omitempty"`
	ProjectID     string         `yaml:"project_id,omitempty"`
	Metadata      map[string]any `yaml:"metadata,omitempty"`
	RouterMode    string         `yaml:"router_mode,omitempty"`
	ExplicitModel string         `yaml:"explicit_model,omitempty"`
	ExpandTokens  int            `yaml:"expand_tokens,omitempty"`
	ExpectedRoute ExpectedRoute  `yaml:"expected_route"`
}

// ExpectedRoute is the expected routing outcome for a case.
type ExpectedRoute struct {
	Tier  string `yaml:"tier"`
	Model string `yaml:"model,omitempty"`
}

// LoadDataset reads and parses a dataset YAML file.
func LoadDataset(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("evals: read dataset: %w", err)
	}
	var ds Dataset
	if err := yaml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("evals: parse dataset: %w", err)
	}
	if err := ds.Validate(); err != nil {
		return nil, err
	}
	return &ds, nil
}

// Validate checks the dataset for structural issues and obvious secret leaks.
func (ds *Dataset) Validate() error {
	if len(ds.Cases) == 0 {
		return fmt.Errorf("evals: dataset has no cases")
	}
	seen := make(map[string]struct{}, len(ds.Cases))
	for i, c := range ds.Cases {
		if c.ID == "" {
			return fmt.Errorf("evals: case %d missing id", i)
		}
		if _, dup := seen[c.ID]; dup {
			return fmt.Errorf("evals: duplicate case id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		if c.TaskType == "" {
			return fmt.Errorf("evals: case %q missing task_type", c.ID)
		}
		if c.ExpectedRoute.Tier == "" && c.ExpectedRoute.Model == "" {
			return fmt.Errorf("evals: case %q missing expected_route", c.ID)
		}
		if err := checkNoSecrets(c); err != nil {
			return err
		}
	}
	return nil
}

// PromptText returns the effective prompt, expanded with filler when the case
// requests a large-context scenario via expand_tokens.
func (c Case) PromptText() string {
	if c.ExpandTokens <= 0 {
		return c.Prompt
	}
	// Roughly 4 chars per token; pad with neutral filler sentences.
	targetChars := c.ExpandTokens * 4
	var b strings.Builder
	b.WriteString("Analyze the following large document and produce a structured analysis.\n")
	filler := "This is a neutral filler sentence used only to grow the context window for testing. "
	for b.Len() < targetChars {
		b.WriteString(filler)
	}
	return b.String()
}

// checkNoSecrets is a coarse guard against committing real-looking credentials
// in the dataset (the prompts must use fabricated values only).
func checkNoSecrets(c Case) error {
	lower := strings.ToLower(c.Prompt)
	bannedSubstrings := []string{
		"sk-live",        // Stripe live key prefix
		"aws_secret",     // AWS secret access key var
		"-----begin rsa", // PEM private key
		"-----begin openssh",
		"xoxb-", // Slack bot token
	}
	for _, banned := range bannedSubstrings {
		if strings.Contains(lower, banned) {
			return fmt.Errorf("evals: case %q appears to contain a real secret pattern %q", c.ID, banned)
		}
	}
	return nil
}
