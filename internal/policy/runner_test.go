package policy

import (
	"strings"
	"testing"
)

const policyCasesYAML = `
cases:
  - case: auth file should be premium
    input:
      task_type: hard_code_debugging
      risk_level: critical
      files_touched:
        - src/auth/session.ts
    expected:
      model_profile_name: premium-reasoning
      verifier: true
      require_capabilities: [tool_use, json_schema]
      matched_rules:
        - auth_premium
        - default_balanced
      explanation_contains:
        - Policy rule auth_premium matched
  - case: trivial git should be cheap
    input:
      task_type: trivial_git
      risk_level: low
    expected:
      model_profile: cheap
      matched_rules:
        - trivial_git_cheap
        - default_balanced
`

func TestParsePolicyTestCasesAcceptsSuite(t *testing.T) {
	cases, err := ParsePolicyTestCases([]byte(policyCasesYAML))
	if err != nil {
		t.Fatalf("ParsePolicyTestCases: %v", err)
	}
	if got := len(cases); got != 2 {
		t.Fatalf("cases = %d, want 2", got)
	}
	if cases[0].Name != "auth file should be premium" {
		t.Fatalf("first case name = %q", cases[0].Name)
	}
	if cases[0].Input.FilesTouched[0] != "src/auth/session.ts" {
		t.Fatalf("files_touched not parsed: %+v", cases[0].Input)
	}
	if len(cases[0].Expected.RequireCapabilities) != 2 {
		t.Fatalf("expected capabilities not parsed: %+v", cases[0].Expected)
	}
}

func TestParsePolicyTestCasesAcceptsSingleCase(t *testing.T) {
	src := `
case: simple default
input:
  task_type: simple_chat
expected:
  model_profile: balanced
`
	cases, err := ParsePolicyTestCases([]byte(src))
	if err != nil {
		t.Fatalf("ParsePolicyTestCases: %v", err)
	}
	if got := len(cases); got != 1 {
		t.Fatalf("cases = %d, want 1", got)
	}
}

func TestParsePolicyTestCasesAcceptsTopLevelList(t *testing.T) {
	src := `
- case: simple default
  input:
    task_type: simple_chat
  expected:
    model_profile: balanced
`
	cases, err := ParsePolicyTestCases([]byte(src))
	if err != nil {
		t.Fatalf("ParsePolicyTestCases: %v", err)
	}
	if got := len(cases); got != 1 {
		t.Fatalf("cases = %d, want 1", got)
	}
}

func TestRunPolicyTestsPassesExpectedRoutes(t *testing.T) {
	cases, err := ParsePolicyTestCases([]byte(policyCasesYAML))
	if err != nil {
		t.Fatalf("ParsePolicyTestCases: %v", err)
	}
	report, err := RunPolicyTests(mustParse(t, validPolicy), testSnapshot(t), cases)
	if err != nil {
		t.Fatalf("RunPolicyTests: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("report should pass: %+v", report)
	}
	if err := report.Error(); err != nil {
		t.Fatalf("passing report returned error: %v", err)
	}
}

func TestRunPolicyTestsReportsFailures(t *testing.T) {
	src := `
case: wrong expectation
input:
  task_type: trivial_git
  risk_level: low
expected:
  model_profile: premium
  matched_rules:
    - auth_premium
`
	cases, err := ParsePolicyTestCases([]byte(src))
	if err != nil {
		t.Fatalf("ParsePolicyTestCases: %v", err)
	}
	report, err := RunPolicyTests(mustParse(t, validPolicy), testSnapshot(t), cases)
	if err != nil {
		t.Fatalf("RunPolicyTests: %v", err)
	}
	if report.Passed() {
		t.Fatalf("report should fail")
	}
	err = report.Error()
	if err == nil {
		t.Fatalf("expected report error")
	}
	if !strings.Contains(err.Error(), "matched_rules") || !strings.Contains(err.Error(), "model_profile") {
		t.Fatalf("error should describe route failures: %v", err)
	}
}

func TestParsePolicyTestCasesRejectsInvalidVocabulary(t *testing.T) {
	src := `
case: invalid vocab
input:
  task_type: made_up
expected:
  model_profile: balanced
`
	_, err := ParsePolicyTestCases([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "task_type") {
		t.Fatalf("expected task_type vocab error, got %v", err)
	}
}

func TestParsePolicyTestCasesRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "suite",
			src: `
cases: []
unexpected: true
`,
			want: `unknown field "unexpected"`,
		},
		{
			name: "case",
			src: `
- case: typo
  input:
    task_type: simple_chat
  expected:
    model_profile: balanced
  expect:
    matched_rules: []
`,
			want: `unknown field "expect"`,
		},
		{
			name: "input",
			src: `
case: typo
input:
  task_typ: simple_chat
expected:
  model_profile: balanced
`,
			want: `unknown field "task_typ"`,
		},
		{
			name: "expected",
			src: `
case: typo
input:
  task_type: simple_chat
expected:
  matched_rule:
    - default_balanced
`,
			want: `unknown field "matched_rule"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePolicyTestCases([]byte(tt.src))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %s, got %v", tt.want, err)
			}
		})
	}
}

func TestParsePolicyTestCasesRejectsEmptyExpected(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "missing expected",
			src: `
case: no assertions
input:
  task_type: simple_chat
`,
		},
		{
			name: "empty expected",
			src: `
case: no assertions
input:
  task_type: simple_chat
expected: {}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePolicyTestCases([]byte(tt.src))
			if err == nil || !strings.Contains(err.Error(), "expected must specify at least one assertion") {
				t.Fatalf("expected empty expected error, got %v", err)
			}
		})
	}
}
