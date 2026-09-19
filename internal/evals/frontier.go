package evals

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/cost"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

const bestValueQualityTolerance = 0.05

// FrontierReport is an offline cost/quality summary derived from the eval
// dataset, registry priors, and routed model pass rates.
type FrontierReport struct {
	TaskClasses []TaskClassFrontier `json:"task_classes"`
}

// TaskClassFrontier is the cost/quality comparison for one task class.
type TaskClassFrontier struct {
	TaskClass       string                   `json:"task_class"`
	CaseCount       int                      `json:"case_count"`
	Models          []FrontierModel          `json:"models"`
	Recommendations []FrontierRecommendation `json:"recommendations,omitempty"`
}

// FrontierModel captures one model's average estimated cost and blended quality
// for a task class. Outcome-based signals stay separate so v1 does not fold
// acceptance data into the frontier score.
type FrontierModel struct {
	ModelID                      string   `json:"model_id"`
	ProviderID                   string   `json:"provider_id"`
	Tier                         string   `json:"tier"`
	EvalSamples                  int      `json:"eval_samples"`
	EvalPassed                   int      `json:"eval_passed"`
	EvalPassRate                 float64  `json:"eval_pass_rate"`
	RegistryQualityPrior         float64  `json:"registry_quality_prior"`
	FrontierQuality              float64  `json:"frontier_quality"`
	AverageEstimatedCostMicroUSD int64    `json:"average_estimated_cost_microusd"`
	AverageEstimatedCostUSD      float64  `json:"average_estimated_cost_usd"`
	OutcomeSamples               int      `json:"outcome_samples,omitempty"`
	OutcomeAcceptanceRate        *float64 `json:"outcome_acceptance_rate,omitempty"`
	OnFrontier                   bool     `json:"on_frontier"`
}

// FrontierRecommendation is a deterministic, report-only recommendation for a
// task class frontier.
type FrontierRecommendation struct {
	Kind         string   `json:"kind"`
	ModelID      string   `json:"model_id"`
	Reason       string   `json:"reason"`
	TiedModelIDs []string `json:"tied_model_ids,omitempty"`
}

type evalStat struct {
	samples int
	passed  int
}

func buildFrontierReport(ds *Dataset, results []CaseResult, snap *registry.Snapshot) FrontierReport {
	if ds == nil || snap == nil {
		return FrontierReport{}
	}
	models := sortedChatModels(snap)
	groupedCases := make(map[string][]Case)
	for _, c := range ds.Cases {
		groupedCases[c.TaskType] = append(groupedCases[c.TaskType], c)
	}
	stats := collectEvalStats(results)

	taskClasses := make([]string, 0, len(groupedCases))
	for taskClass := range groupedCases {
		taskClasses = append(taskClasses, taskClass)
	}
	sort.Strings(taskClasses)

	report := FrontierReport{TaskClasses: make([]TaskClassFrontier, 0, len(taskClasses))}
	for _, taskClass := range taskClasses {
		taskCases := groupedCases[taskClass]
		taskFrontier := TaskClassFrontier{
			TaskClass: taskClass,
			CaseCount: len(taskCases),
		}
		avgCosts := averageEstimatedCostByModel(taskCases, models)
		for _, model := range models {
			stat := stats[taskClass][model.ID]
			evalSamples := 0
			evalPassed := 0
			evalPassRate := 0.0
			if stat != nil {
				evalSamples = stat.samples
				evalPassed = stat.passed
				if stat.samples > 0 {
					evalPassRate = roundTo(float64(stat.passed)/float64(stat.samples), 4)
				}
			}
			prior := roundTo(registryQualityPrior(model, taskClass), 4)
			frontierQuality := blendFrontierQuality(prior, evalPassed, evalSamples)
			avgCostMicros := avgCosts[model.ID]
			taskFrontier.Models = append(taskFrontier.Models, FrontierModel{
				ModelID:                      model.ID,
				ProviderID:                   model.ProviderID,
				Tier:                         string(model.Tier),
				EvalSamples:                  evalSamples,
				EvalPassed:                   evalPassed,
				EvalPassRate:                 evalPassRate,
				RegistryQualityPrior:         prior,
				FrontierQuality:              frontierQuality,
				AverageEstimatedCostMicroUSD: avgCostMicros,
				AverageEstimatedCostUSD:      float64(avgCostMicros) / 1_000_000,
			})
		}
		sortFrontierModels(taskFrontier.Models)
		markFrontier(taskFrontier.Models)
		taskFrontier.Recommendations = recommendTaskClass(taskFrontier)
		report.TaskClasses = append(report.TaskClasses, taskFrontier)
	}
	return report
}

func collectEvalStats(results []CaseResult) map[string]map[string]*evalStat {
	stats := make(map[string]map[string]*evalStat)
	for _, res := range results {
		taskClass := res.Case.TaskType
		if taskClass == "" || res.SelectedModel == "" || res.Blocked {
			continue
		}
		if _, ok := stats[taskClass]; !ok {
			stats[taskClass] = make(map[string]*evalStat)
		}
		stat := stats[taskClass][res.SelectedModel]
		if stat == nil {
			stat = &evalStat{}
			stats[taskClass][res.SelectedModel] = stat
		}
		stat.samples++
		if res.Pass {
			stat.passed++
		}
	}
	return stats
}

func sortedChatModels(snap *registry.Snapshot) []registry.Model {
	models := snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true})
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func averageEstimatedCostByModel(cases []Case, models []registry.Model) map[string]int64 {
	out := make(map[string]int64, len(models))
	if len(cases) == 0 {
		return out
	}
	sums := make(map[string]int64, len(models))
	for _, c := range cases {
		job := jobForCase(c)
		usage := cost.TokenUsage{
			InputTokens:  int64(job.PromptTokensEstimate),
			OutputTokens: int64(job.MaxOutputTokensEstimate),
			Mode:         cost.ModeEstimated,
		}
		for _, model := range models {
			estimate, err := cost.EstimateModel(model, usage)
			if err != nil {
				continue
			}
			sums[model.ID] += estimate.TotalMicroUSD
		}
	}
	for _, model := range models {
		out[model.ID] = int64(math.Round(float64(sums[model.ID]) / float64(len(cases))))
	}
	return out
}

func jobForCase(c Case) *router.JobDescriptor {
	req := &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: c.PromptText()}},
		Metadata: caseMetadata(c),
	}
	if c.ExplicitModel != "" {
		req.Model = c.ExplicitModel
	}
	return router.NewJobDescriptor(router.JobDescriptorInput{
		RequestID: "frontier_" + c.ID,
		Auth: router.AuthTenantContext{
			TenantID:  c.TenantID,
			ProjectID: c.ProjectID,
		},
		Request: req,
	})
}

func registryQualityPrior(model registry.Model, taskClass string) float64 {
	if score, ok := model.QualityScores[taskClass]; ok {
		return score
	}
	switch model.Tier {
	case registry.TierCheap:
		return 0.55
	case registry.TierBalanced:
		return 0.70
	case registry.TierPremium:
		return 0.85
	default:
		return 0.60
	}
}

func blendFrontierQuality(prior float64, evalPassed, evalSamples int) float64 {
	if evalSamples <= 0 {
		return roundTo(prior, 4)
	}
	blended := (float64(evalPassed) + prior) / float64(evalSamples+1)
	return roundTo(blended, 4)
}

func sortFrontierModels(models []FrontierModel) {
	sort.Slice(models, func(i, j int) bool {
		if models[i].AverageEstimatedCostMicroUSD != models[j].AverageEstimatedCostMicroUSD {
			return models[i].AverageEstimatedCostMicroUSD < models[j].AverageEstimatedCostMicroUSD
		}
		if models[i].FrontierQuality != models[j].FrontierQuality {
			return models[i].FrontierQuality > models[j].FrontierQuality
		}
		return models[i].ModelID < models[j].ModelID
	})
}

func markFrontier(models []FrontierModel) {
	for i := range models {
		models[i].OnFrontier = true
		for j := range models {
			if i == j {
				continue
			}
			if dominates(models[j], models[i]) {
				models[i].OnFrontier = false
				break
			}
		}
	}
}

func dominates(a, b FrontierModel) bool {
	if a.AverageEstimatedCostMicroUSD > b.AverageEstimatedCostMicroUSD {
		return false
	}
	if a.FrontierQuality < b.FrontierQuality {
		return false
	}
	return a.AverageEstimatedCostMicroUSD < b.AverageEstimatedCostMicroUSD ||
		a.FrontierQuality > b.FrontierQuality
}

func recommendTaskClass(task TaskClassFrontier) []FrontierRecommendation {
	frontier := make([]FrontierModel, 0, len(task.Models))
	for _, model := range task.Models {
		if model.OnFrontier {
			frontier = append(frontier, model)
		}
	}
	switch len(frontier) {
	case 0:
		return nil
	case 1:
		return []FrontierRecommendation{{
			Kind:    "only_option",
			ModelID: frontier[0].ModelID,
			Reason:  "only frontier model after cost/quality dominance filtering",
		}}
	}

	recommendations := make([]FrontierRecommendation, 0, 3)
	appendUnique := func(rec FrontierRecommendation) {
		for _, existing := range recommendations {
			if existing.ModelID == rec.ModelID {
				return
			}
		}
		recommendations = append(recommendations, rec)
	}

	bestValue, bestValueTies := selectBestValue(frontier)
	appendUnique(FrontierRecommendation{
		Kind:         "best_value",
		ModelID:      bestValue.ModelID,
		Reason:       tieAwareReason("cheapest frontier model within 0.05 quality of the best-quality frontier point", bestValueTies),
		TiedModelIDs: bestValueTies,
	})

	lowestCost, lowestCostTies := selectLowestCost(frontier)
	appendUnique(FrontierRecommendation{
		Kind:         "lowest_cost",
		ModelID:      lowestCost.ModelID,
		Reason:       tieAwareReason("lowest estimated cost on the frontier", lowestCostTies),
		TiedModelIDs: lowestCostTies,
	})

	highestQuality, highestQualityTies := selectHighestQuality(frontier)
	appendUnique(FrontierRecommendation{
		Kind:         "highest_quality",
		ModelID:      highestQuality.ModelID,
		Reason:       tieAwareReason("highest blended frontier quality on the frontier", highestQualityTies),
		TiedModelIDs: highestQualityTies,
	})

	return recommendations
}

func selectBestValue(frontier []FrontierModel) (FrontierModel, []string) {
	bestQuality := frontier[0].FrontierQuality
	for _, model := range frontier[1:] {
		if model.FrontierQuality > bestQuality {
			bestQuality = model.FrontierQuality
		}
	}
	candidates := make([]FrontierModel, 0, len(frontier))
	for _, model := range frontier {
		if model.FrontierQuality >= bestQuality-bestValueQualityTolerance {
			candidates = append(candidates, model)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].AverageEstimatedCostMicroUSD != candidates[j].AverageEstimatedCostMicroUSD {
			return candidates[i].AverageEstimatedCostMicroUSD < candidates[j].AverageEstimatedCostMicroUSD
		}
		if candidates[i].FrontierQuality != candidates[j].FrontierQuality {
			return candidates[i].FrontierQuality > candidates[j].FrontierQuality
		}
		return candidates[i].ModelID < candidates[j].ModelID
	})
	chosen := candidates[0]
	return chosen, tiedModels(candidates, func(model FrontierModel) bool {
		return model.AverageEstimatedCostMicroUSD == chosen.AverageEstimatedCostMicroUSD &&
			model.FrontierQuality == chosen.FrontierQuality
	})
}

func selectLowestCost(frontier []FrontierModel) (FrontierModel, []string) {
	candidates := append([]FrontierModel(nil), frontier...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].AverageEstimatedCostMicroUSD != candidates[j].AverageEstimatedCostMicroUSD {
			return candidates[i].AverageEstimatedCostMicroUSD < candidates[j].AverageEstimatedCostMicroUSD
		}
		if candidates[i].FrontierQuality != candidates[j].FrontierQuality {
			return candidates[i].FrontierQuality > candidates[j].FrontierQuality
		}
		return candidates[i].ModelID < candidates[j].ModelID
	})
	chosen := candidates[0]
	return chosen, tiedModels(candidates, func(model FrontierModel) bool {
		return model.AverageEstimatedCostMicroUSD == chosen.AverageEstimatedCostMicroUSD &&
			model.FrontierQuality == chosen.FrontierQuality
	})
}

func selectHighestQuality(frontier []FrontierModel) (FrontierModel, []string) {
	candidates := append([]FrontierModel(nil), frontier...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].FrontierQuality != candidates[j].FrontierQuality {
			return candidates[i].FrontierQuality > candidates[j].FrontierQuality
		}
		if candidates[i].AverageEstimatedCostMicroUSD != candidates[j].AverageEstimatedCostMicroUSD {
			return candidates[i].AverageEstimatedCostMicroUSD < candidates[j].AverageEstimatedCostMicroUSD
		}
		return candidates[i].ModelID < candidates[j].ModelID
	})
	chosen := candidates[0]
	return chosen, tiedModels(candidates, func(model FrontierModel) bool {
		return model.FrontierQuality == chosen.FrontierQuality &&
			model.AverageEstimatedCostMicroUSD == chosen.AverageEstimatedCostMicroUSD
	})
}

func tiedModels(models []FrontierModel, match func(FrontierModel) bool) []string {
	var tied []string
	for _, model := range models {
		if match(model) {
			tied = append(tied, model.ModelID)
		}
	}
	sort.Strings(tied)
	if len(tied) <= 1 {
		return nil
	}
	return tied
}

func tieAwareReason(base string, tied []string) string {
	if len(tied) <= 1 {
		return base
	}
	return fmt.Sprintf("%s; tie broken by model id among [%s]", base, strings.Join(tied, ", "))
}

func roundTo(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func formatFrontierReport(report FrontierReport) string {
	if len(report.TaskClasses) == 0 {
		return "\nCost-quality frontier:\n  No task-class frontier data.\n"
	}
	var b strings.Builder
	b.WriteString("\nCost-quality frontier:\n")
	for _, task := range report.TaskClasses {
		fmt.Fprintf(&b, "  %s (%d case(s))\n", task.TaskClass, task.CaseCount)
		for _, model := range task.Models {
			status := "dominated"
			if model.OnFrontier {
				status = "frontier"
			}
			fmt.Fprintf(&b,
				"    - %s [%s] cost=$%.6f quality=%.4f prior=%.4f eval=%d/%d\n",
				model.ModelID,
				status,
				model.AverageEstimatedCostUSD,
				model.FrontierQuality,
				model.RegistryQualityPrior,
				model.EvalPassed,
				model.EvalSamples,
			)
		}
		if len(task.Recommendations) == 0 {
			b.WriteString("    Recommendations: none\n")
			continue
		}
		b.WriteString("    Recommendations:\n")
		for _, rec := range task.Recommendations {
			fmt.Fprintf(&b, "      - %s: %s (%s)\n", rec.Kind, rec.ModelID, rec.Reason)
		}
	}
	return b.String()
}
