// Command eval-report runs the routing eval dataset and writes a report
// artifact (report.json + report.txt) for CI to upload (ISSUE-050). It exits
// non-zero if the pass rate falls below -min-pass, so it can double as a gate.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magnusfroste/sluss/internal/autopilot"
	"github.com/magnusfroste/sluss/internal/evals"
)

func main() {
	var (
		dataset = flag.String("dataset", "evals/dataset-v1.yaml", "path to the eval dataset YAML")
		outDir  = flag.String("out", "eval-report", "output directory for the report artifact")
		minPass = flag.Float64("min-pass", 0.0, "fail if pass rate is below this fraction (0 disables)")
		policyA = flag.String("policy-a", "", "path to the primary policy YAML for offline A/B simulation")
		policyB = flag.String("policy-b", "", "path to the secondary policy YAML for offline A/B simulation")

		recommend  = flag.Bool("recommend", false, "emit per-task-class policy floor recommendations from the frontier")
		targetPass = flag.Float64("target-pass", evals.DefaultTargetPassRate, "pass rate a model must reach to be 'good enough' for a recommendation")
		minSamples = flag.Int("min-samples", 1, "minimum eval samples before a model's pass rate is trusted for recommendations")

		autopilotOn = flag.Bool("autopilot", false, "apply guardrails to the recommendations and emit an auto-approved policy (implies -recommend)")
	)
	flag.Parse()

	if (*policyA == "") != (*policyB == "") {
		fmt.Fprintln(os.Stderr, "error: -policy-a and -policy-b must be provided together")
		os.Exit(1)
	}
	if *policyA != "" {
		report, err := generateComparison(*dataset, *policyA, *policyB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		labelA := policyLabel(*policyA)
		labelB := policyLabel(*policyB)
		if err := writeComparisonReport(report, *outDir, labelA, labelB); err != nil {
			fmt.Fprintf(os.Stderr, "error writing comparison report: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(evals.FormatComparisonReport(report, labelA, labelB))
		fmt.Printf("Comparison report written to %s/\n", *outDir)
		return
	}

	report, err := generate(*dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := writeReport(report, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(evals.FormatReport(report))
	fmt.Printf("Report written to %s/\n", *outDir)

	if *recommend || *autopilotOn {
		recs := evals.RecommendPolicy(report.Frontier, *targetPass, *minSamples)
		fmt.Print(evals.FormatPolicyRecommendations(recs))
		if err := writeRecommendations(recs, *outDir); err != nil {
			fmt.Fprintf(os.Stderr, "error writing recommendations: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Policy recommendations written to %s/policy-recommendations.{txt,yaml}\n", *outDir)

		if *autopilotOn {
			result := autopilot.Evaluate(recs, autopilot.DefaultGuardrails())
			fmt.Print(autopilot.FormatDecisions(result))
			if err := writeAutopilot(result, *outDir); err != nil {
				fmt.Fprintf(os.Stderr, "error writing autopilot artifacts: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Autopilot policy (%d approved change(s)) written to %s/autopilot-policy.yaml — review, then adopt with a policy reload.\n", len(result.Approved), *outDir)
		}
	}

	if *minPass > 0 && report.PassRate() < *minPass {
		fmt.Fprintf(os.Stderr, "pass rate %.3f below threshold %.3f\n", report.PassRate(), *minPass)
		os.Exit(1)
	}
}

// generate loads the dataset and runs it through the routing engine.
func generate(datasetPath string) (evals.Report, error) {
	rn, err := evals.NewRunner()
	if err != nil {
		return evals.Report{}, fmt.Errorf("new runner: %w", err)
	}
	ds, err := evals.LoadDataset(datasetPath)
	if err != nil {
		return evals.Report{}, fmt.Errorf("load dataset: %w", err)
	}
	return rn.Run(ds)
}

// generateComparison loads the dataset, compiles both policies, and compares
// the resulting dry-run decisions without calling any provider.
func generateComparison(datasetPath, policyAPath, policyBPath string) (evals.ComparisonReport, error) {
	rn, err := evals.NewRunner()
	if err != nil {
		return evals.ComparisonReport{}, fmt.Errorf("new runner: %w", err)
	}
	ds, err := evals.LoadDataset(datasetPath)
	if err != nil {
		return evals.ComparisonReport{}, fmt.Errorf("load dataset: %w", err)
	}
	policyA, err := evals.LoadCompiledPolicy(policyAPath, rn.Snapshot)
	if err != nil {
		return evals.ComparisonReport{}, err
	}
	policyB, err := evals.LoadCompiledPolicy(policyBPath, rn.Snapshot)
	if err != nil {
		return evals.ComparisonReport{}, err
	}
	return rn.ComparePolicies(ds, policyA, policyB)
}

// writeReport writes report.json and report.txt into dir, creating it if needed.
func writeReport(report evals.Report, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), jsonBytes, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.txt"), []byte(evals.FormatReport(report)), 0o644)
}

func writeComparisonReport(report evals.ComparisonReport, dir, labelA, labelB string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "comparison.json"), jsonBytes, 0o644); err != nil {
		return err
	}
	text := evals.FormatComparisonReport(report, labelA, labelB)
	return os.WriteFile(filepath.Join(dir, "comparison.txt"), []byte(text), 0o644)
}

// writeRecommendations writes the human-readable table and the reviewable policy
// YAML snippet next to the report artifact.
func writeRecommendations(recs []evals.PolicyRecommendation, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	txt := evals.FormatPolicyRecommendations(recs)
	if err := os.WriteFile(filepath.Join(dir, "policy-recommendations.txt"), []byte(txt), 0o644); err != nil {
		return err
	}
	yaml := evals.PolicyRecommendationYAML(recs)
	return os.WriteFile(filepath.Join(dir, "policy-recommendations.yaml"), []byte(yaml), 0o644)
}

// writeAutopilot writes the guardrail decision log and the auto-approved policy
// (only the changes that cleared every guardrail) for operator review.
func writeAutopilot(res autopilot.Result, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	log := autopilot.FormatDecisions(res)
	if err := os.WriteFile(filepath.Join(dir, "autopilot-decisions.txt"), []byte(log), 0o644); err != nil {
		return err
	}
	yaml := evals.PolicyRecommendationYAML(res.Approved)
	return os.WriteFile(filepath.Join(dir, "autopilot-policy.yaml"), []byte(yaml), 0o644)
}

func policyLabel(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}
