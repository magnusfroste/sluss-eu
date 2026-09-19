package policy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/magnusfroste/sluss/internal/registry"
	"gopkg.in/yaml.v3"
)

type PolicyTestCase struct {
	Name     string
	Input    EvaluationInput
	Expected ExpectedDecision
}

type ExpectedDecision struct {
	ModelProfile        ModelProfile
	ModelProfileName    string
	Verifier            *bool
	RequireCapabilities []Capability
	MatchedRuleIDs      []string
	Blocked             *bool
	ExplanationContains []string
}

type PolicyTestResult struct {
	Name     string
	Passed   bool
	Failures []string
}

type PolicyTestReport struct {
	Results []PolicyTestResult
}

func (r PolicyTestReport) Passed() bool {
	for _, result := range r.Results {
		if !result.Passed {
			return false
		}
	}
	return true
}

func (r PolicyTestReport) Error() error {
	var failures []string
	for _, result := range r.Results {
		if result.Passed {
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %s", result.Name, strings.Join(result.Failures, "; ")))
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidPolicy, strings.Join(failures, " | "))
}

func ParsePolicyTestCases(data []byte) ([]PolicyTestCase, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty policy test cases", ErrInvalidPolicy)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("%w: yaml: %v", ErrInvalidPolicy, err)
	}
	root := policyTestRoot(&node)
	if root.Kind == yaml.SequenceNode {
		if err := validatePolicyTestCasesSequence(root, "policy test cases"); err != nil {
			return nil, err
		}
		var rawCases []rawPolicyTestCase
		if err := root.Decode(&rawCases); err != nil {
			return nil, fmt.Errorf("%w: yaml: %v", ErrInvalidPolicy, err)
		}
		return rawCasesToCases(rawCases)
	}
	if err := validatePolicyTestSuiteNode(root, "policy test suite"); err != nil {
		return nil, err
	}
	var suite rawPolicyTestSuite
	if err := root.Decode(&suite); err != nil {
		return nil, fmt.Errorf("%w: yaml: %v", ErrInvalidPolicy, err)
	}
	rawCases := suite.Cases
	if len(rawCases) == 0 && suite.Case != "" {
		rawCases = []rawPolicyTestCase{suite.rawPolicyTestCase}
	}
	return rawCasesToCases(rawCases)
}

func policyTestRoot(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func validatePolicyTestSuiteNode(node *yaml.Node, ctx string) error {
	if err := validateMappingKeys(node, ctx, policyTestSuiteFields); err != nil {
		return err
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]
		switch key {
		case "input":
			if err := validateMappingKeys(value, ctx+".input", policyTestInputFields); err != nil {
				return err
			}
		case "expected":
			if err := validateMappingKeys(value, ctx+".expected", policyTestExpectedFields); err != nil {
				return err
			}
		case "cases":
			if err := validatePolicyTestCasesSequence(value, ctx+".cases"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePolicyTestCasesSequence(node *yaml.Node, ctx string) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%w: %s must be a sequence", ErrInvalidPolicy, ctx)
	}
	for i, item := range node.Content {
		if err := validatePolicyTestCaseNode(item, fmt.Sprintf("%s[%d]", ctx, i)); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicyTestCaseNode(node *yaml.Node, ctx string) error {
	if err := validateMappingKeys(node, ctx, policyTestCaseFields); err != nil {
		return err
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]
		switch key {
		case "input":
			if err := validateMappingKeys(value, ctx+".input", policyTestInputFields); err != nil {
				return err
			}
		case "expected":
			if err := validateMappingKeys(value, ctx+".expected", policyTestExpectedFields); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMappingKeys(node *yaml.Node, ctx string, allowed map[string]struct{}) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%w: %s must be a mapping", ErrInvalidPolicy, ctx)
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: %s has unknown field %q", ErrInvalidPolicy, ctx, key)
		}
	}
	return nil
}

func rawCasesToCases(rawCases []rawPolicyTestCase) ([]PolicyTestCase, error) {
	if len(rawCases) == 0 {
		return nil, fmt.Errorf("%w: at least one policy test case is required", ErrInvalidPolicy)
	}
	cases := make([]PolicyTestCase, 0, len(rawCases))
	for i, raw := range rawCases {
		testCase, err := raw.toCase(i)
		if err != nil {
			return nil, err
		}
		cases = append(cases, testCase)
	}
	return cases, nil
}

func RunPolicyTests(p *Policy, snapshot *registry.Snapshot, cases []PolicyTestCase) (PolicyTestReport, error) {
	compiled, err := Compile(p, snapshot)
	if err != nil {
		return PolicyTestReport{}, err
	}
	report := PolicyTestReport{Results: make([]PolicyTestResult, 0, len(cases))}
	for _, testCase := range cases {
		result := evaluatePolicyTestCase(compiled, testCase)
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func evaluatePolicyTestCase(compiled *CompiledPolicy, testCase PolicyTestCase) PolicyTestResult {
	result := PolicyTestResult{Name: testCase.Name}
	decision := compiled.Evaluate(testCase.Input)
	result.Failures = append(result.Failures, testCase.Expected.compare(decision)...)
	result.Passed = len(result.Failures) == 0
	return result
}

func (e ExpectedDecision) compare(decision Evaluation) []string {
	var failures []string
	if e.Blocked != nil && decision.Blocked != *e.Blocked {
		failures = append(failures, fmt.Sprintf("blocked = %t, want %t", decision.Blocked, *e.Blocked))
	}
	if len(e.MatchedRuleIDs) > 0 && !slices.Equal(decision.MatchedRuleIDs, e.MatchedRuleIDs) {
		failures = append(failures, fmt.Sprintf("matched_rules = %v, want %v", decision.MatchedRuleIDs, e.MatchedRuleIDs))
	}
	if e.ModelProfile != "" {
		if got := effectiveModelProfile(decision.Route); got != e.ModelProfile {
			failures = append(failures, fmt.Sprintf("model_profile = %q, want %q", got, e.ModelProfile))
		}
	}
	if e.ModelProfileName != "" {
		if got := effectiveModelProfileName(decision.Route); got != e.ModelProfileName {
			failures = append(failures, fmt.Sprintf("model_profile_name = %q, want %q", got, e.ModelProfileName))
		}
	}
	if e.Verifier != nil {
		got, ok := effectiveVerifier(decision.Route)
		if !ok || got != *e.Verifier {
			failures = append(failures, fmt.Sprintf("verifier = %t/%t, want %t", got, ok, *e.Verifier))
		}
	}
	for _, capability := range e.RequireCapabilities {
		if decision.Route.Constraints == nil || !slices.Contains(decision.Route.Constraints.RequireCapabilities, capability) {
			failures = append(failures, fmt.Sprintf("require_capabilities missing %q", capability))
		}
	}
	for _, fragment := range e.ExplanationContains {
		if !containsFragment(decision.Explanations, fragment) {
			failures = append(failures, fmt.Sprintf("explanations missing %q", fragment))
		}
	}
	return failures
}

func effectiveModelProfile(route Route) ModelProfile {
	if route.Force != nil && route.Force.ModelProfile != "" {
		return route.Force.ModelProfile
	}
	if route.Defaults != nil {
		return route.Defaults.ModelProfile
	}
	return ""
}

func effectiveModelProfileName(route Route) string {
	if route.Force != nil && route.Force.ModelProfileName != "" {
		return route.Force.ModelProfileName
	}
	if route.Defaults != nil {
		return route.Defaults.ModelProfileName
	}
	return ""
}

func effectiveVerifier(route Route) (bool, bool) {
	if route.Force != nil && route.Force.Verifier != nil {
		return *route.Force.Verifier, true
	}
	if route.Defaults != nil && route.Defaults.Verifier != nil {
		return *route.Defaults.Verifier, true
	}
	return false, false
}

func containsFragment(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

type rawPolicyTestSuite struct {
	rawPolicyTestCase `yaml:",inline"`
	Cases             []rawPolicyTestCase `yaml:"cases"`
}

type rawPolicyTestCase struct {
	Case     string              `yaml:"case"`
	Input    rawPolicyTestInput  `yaml:"input"`
	Expected rawExpectedDecision `yaml:"expected"`
}

type rawPolicyTestInput struct {
	TenantID             string   `yaml:"tenant"`
	ProjectID            string   `yaml:"project"`
	TaskType             string   `yaml:"task_type"`
	RiskLevel            string   `yaml:"risk_level"`
	Sensitivity          string   `yaml:"sensitivity"`
	PromptTokensEstimate int      `yaml:"prompt_tokens_estimate"`
	ContainsText         string   `yaml:"contains_text"`
	Keywords             []string `yaml:"keywords"`
	FilesTouched         []string `yaml:"files_touched"`
	RequiresToolUse      *bool    `yaml:"requires_tool_use"`
	RequiresJSONSchema   *bool    `yaml:"requires_json_schema"`
	RequiresVision       *bool    `yaml:"requires_vision"`
	RouterMode           string   `yaml:"router_mode"`
}

type rawExpectedDecision struct {
	ModelProfile        string   `yaml:"model_profile"`
	ModelProfileName    string   `yaml:"model_profile_name"`
	Verifier            *bool    `yaml:"verifier"`
	RequireCapabilities []string `yaml:"require_capabilities"`
	MatchedRuleIDs      []string `yaml:"matched_rules"`
	Blocked             *bool    `yaml:"blocked"`
	ExplanationContains []string `yaml:"explanation_contains"`
	Explanations        []string `yaml:"explanations"`
}

var policyTestSuiteFields = map[string]struct{}{
	"case":     {},
	"input":    {},
	"expected": {},
	"cases":    {},
}

var policyTestCaseFields = map[string]struct{}{
	"case":     {},
	"input":    {},
	"expected": {},
}

var policyTestInputFields = map[string]struct{}{
	"tenant":                 {},
	"project":                {},
	"task_type":              {},
	"risk_level":             {},
	"sensitivity":            {},
	"prompt_tokens_estimate": {},
	"contains_text":          {},
	"keywords":               {},
	"files_touched":          {},
	"requires_tool_use":      {},
	"requires_json_schema":   {},
	"requires_vision":        {},
	"router_mode":            {},
}

var policyTestExpectedFields = map[string]struct{}{
	"model_profile":        {},
	"model_profile_name":   {},
	"verifier":             {},
	"require_capabilities": {},
	"matched_rules":        {},
	"blocked":              {},
	"explanation_contains": {},
	"explanations":         {},
}

func (r rawPolicyTestCase) toCase(index int) (PolicyTestCase, error) {
	if strings.TrimSpace(r.Case) == "" {
		return PolicyTestCase{}, fmt.Errorf("%w: case[%d] missing case name", ErrInvalidPolicy, index)
	}
	input, err := r.Input.toInput(r.Case)
	if err != nil {
		return PolicyTestCase{}, err
	}
	expected, err := r.Expected.toExpected(r.Case)
	if err != nil {
		return PolicyTestCase{}, err
	}
	return PolicyTestCase{Name: r.Case, Input: input, Expected: expected}, nil
}

func (r rawPolicyTestInput) toInput(name string) (EvaluationInput, error) {
	if r.TaskType != "" {
		if err := checkVocab(validTaskTypes, "case "+name+".input.task_type", r.TaskType); err != nil {
			return EvaluationInput{}, err
		}
	}
	if r.RiskLevel != "" {
		if err := checkVocab(validRiskLevels, "case "+name+".input.risk_level", r.RiskLevel); err != nil {
			return EvaluationInput{}, err
		}
	}
	if r.Sensitivity != "" {
		if err := checkVocab(validSensitivities, "case "+name+".input.sensitivity", r.Sensitivity); err != nil {
			return EvaluationInput{}, err
		}
	}
	if r.RouterMode != "" {
		vocab := map[string]struct{}{
			string(RouterModeAuto):     {},
			string(RouterModeCheap):    {},
			string(RouterModeBalanced): {},
			string(RouterModePremium):  {},
			string(RouterModeDisabled): {},
		}
		if err := checkVocab(vocab, "case "+name+".input.router_mode", r.RouterMode); err != nil {
			return EvaluationInput{}, err
		}
	}
	out := EvaluationInput{
		TenantID:             r.TenantID,
		ProjectID:            r.ProjectID,
		TaskType:             r.TaskType,
		RiskLevel:            r.RiskLevel,
		Sensitivity:          r.Sensitivity,
		PromptTokensEstimate: r.PromptTokensEstimate,
		ContainsText:         r.ContainsText,
		Keywords:             append([]string(nil), r.Keywords...),
		FilesTouched:         append([]string(nil), r.FilesTouched...),
		RouterMode:           r.RouterMode,
	}
	if r.RequiresToolUse != nil {
		out.RequiresToolUse = *r.RequiresToolUse
	}
	if r.RequiresJSONSchema != nil {
		out.RequiresJSONSchema = *r.RequiresJSONSchema
	}
	if r.RequiresVision != nil {
		out.RequiresVision = *r.RequiresVision
	}
	return out, nil
}

func (r rawExpectedDecision) toExpected(name string) (ExpectedDecision, error) {
	out := ExpectedDecision{
		ModelProfileName:    r.ModelProfileName,
		Verifier:            cloneBoolPtr(r.Verifier),
		MatchedRuleIDs:      append([]string(nil), r.MatchedRuleIDs...),
		Blocked:             cloneBoolPtr(r.Blocked),
		ExplanationContains: append([]string(nil), r.ExplanationContains...),
	}
	out.ExplanationContains = append(out.ExplanationContains, r.Explanations...)
	if r.ModelProfile != "" {
		if !isValidProfileOrFallback(r.ModelProfile) {
			return out, fmt.Errorf("%w: case %s.expected.model_profile %q must be cheap|balanced|premium", ErrInvalidPolicy, name, r.ModelProfile)
		}
		out.ModelProfile = ModelProfile(r.ModelProfile)
	}
	for _, capability := range r.RequireCapabilities {
		if !isValidCapability(capability) {
			return out, fmt.Errorf("%w: case %s.expected.require_capabilities %q is not a recognised capability", ErrInvalidPolicy, name, capability)
		}
		out.RequireCapabilities = append(out.RequireCapabilities, Capability(capability))
	}
	if out.empty() {
		return out, fmt.Errorf("%w: case %s.expected must specify at least one assertion", ErrInvalidPolicy, name)
	}
	return out, nil
}

func (e ExpectedDecision) empty() bool {
	return e.ModelProfile == "" &&
		e.ModelProfileName == "" &&
		e.Verifier == nil &&
		len(e.RequireCapabilities) == 0 &&
		len(e.MatchedRuleIDs) == 0 &&
		e.Blocked == nil &&
		len(e.ExplanationContains) == 0
}
