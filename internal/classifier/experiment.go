package classifier

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
)

const (
	experimentEpochs     = 14
	experimentMaxTokens  = 48
	experimentMaxNGrams  = 24
	experimentMarginCeil = 6.0
)

var experimentTokenPattern = regexp.MustCompile(`[a-z0-9_]+`)

// ExperimentExample is a labeled offline-only example used by the lightweight
// classifier experiment. It is not consulted by the production router.
type ExperimentExample struct {
	ID        string
	Prompt    string
	TaskType  string
	RiskLevel string
}

// ExperimentPrediction is the trained model's output for one prompt.
type ExperimentPrediction struct {
	TaskType       string
	TaskConfidence float64
	RiskLevel      string
	RiskConfidence float64
}

// ExperimentCaseResult captures baseline and trained predictions for one case.
type ExperimentCaseResult struct {
	ID                    string
	ExpectedTaskType      string
	ExpectedRiskLevel     string
	BaselineTaskType      string
	BaselineRiskLevel     string
	TrainedTaskType       string
	TrainedTaskConfidence float64
	TrainedRiskLevel      string
	TrainedRiskConfidence float64
}

// ExperimentMetrics summarizes accuracy over a split.
type ExperimentMetrics struct {
	Total        int
	TaskCorrect  int
	RiskCorrect  int
	JointCorrect int
}

func (m ExperimentMetrics) TaskAccuracy() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.TaskCorrect) / float64(m.Total)
}

func (m ExperimentMetrics) RiskAccuracy() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.RiskCorrect) / float64(m.Total)
}

func (m ExperimentMetrics) JointAccuracy() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.JointCorrect) / float64(m.Total)
}

// ExperimentComparisonReport is an offline comparison between the current rule
// baseline and the trained experiment model on the test split.
type ExperimentComparisonReport struct {
	TrainCount      int
	TestCount       int
	BaselineMetrics ExperimentMetrics
	TrainedMetrics  ExperimentMetrics
	Results         []ExperimentCaseResult
}

// Format renders a human-readable summary for tests and docs.
func (r ExperimentComparisonReport) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Offline lightweight classifier experiment\n")
	fmt.Fprintf(&b, "train=%d test=%d\n", r.TrainCount, r.TestCount)
	fmt.Fprintf(&b, "baseline  task=%.1f%% risk=%.1f%% joint=%.1f%%\n",
		r.BaselineMetrics.TaskAccuracy()*100,
		r.BaselineMetrics.RiskAccuracy()*100,
		r.BaselineMetrics.JointAccuracy()*100,
	)
	fmt.Fprintf(&b, "trained   task=%.1f%% risk=%.1f%% joint=%.1f%%\n",
		r.TrainedMetrics.TaskAccuracy()*100,
		r.TrainedMetrics.RiskAccuracy()*100,
		r.TrainedMetrics.JointAccuracy()*100,
	)
	for _, res := range r.Results {
		fmt.Fprintf(&b, "- %s expected=(%s,%s) baseline=(%s,%s) trained=(%s %.2f,%s %.2f)\n",
			res.ID,
			res.ExpectedTaskType,
			res.ExpectedRiskLevel,
			res.BaselineTaskType,
			res.BaselineRiskLevel,
			res.TrainedTaskType,
			res.TrainedTaskConfidence,
			res.TrainedRiskLevel,
			res.TrainedRiskConfidence,
		)
	}
	return b.String()
}

// LightweightExperimentModel trains deterministic linear classifiers for task
// and risk labels. This stays fully offline and unreferenced by production
// routing code until a future ADR approves any rollout path.
type LightweightExperimentModel struct {
	task linearPerceptron
	risk linearPerceptron
}

// TrainLightweightExperiment trains deterministic task and risk models from an
// offline labeled split.
func TrainLightweightExperiment(examples []ExperimentExample) (LightweightExperimentModel, error) {
	if len(examples) == 0 {
		return LightweightExperimentModel{}, fmt.Errorf("classifier experiment: no training examples")
	}
	taskExamples := make([]labeledVector, 0, len(examples))
	riskExamples := make([]labeledVector, 0, len(examples))
	for _, ex := range examples {
		if ex.TaskType == "" || ex.RiskLevel == "" {
			return LightweightExperimentModel{}, fmt.Errorf("classifier experiment: case %q missing label", ex.ID)
		}
		features := experimentFeatureVector(ex.Prompt)
		taskExamples = append(taskExamples, labeledVector{label: ex.TaskType, features: features})
		riskExamples = append(riskExamples, labeledVector{label: ex.RiskLevel, features: features})
	}
	return LightweightExperimentModel{
		task: trainLinearPerceptron(taskExamples),
		risk: trainLinearPerceptron(riskExamples),
	}, nil
}

// Predict returns task and risk predictions for one prompt.
func (m LightweightExperimentModel) Predict(prompt string) ExperimentPrediction {
	features := experimentFeatureVector(prompt)
	taskLabel, taskConfidence := m.task.predict(features)
	riskLabel, riskConfidence := m.risk.predict(features)
	baselineTask, baselineRisk := classifyRuleBaseline(prompt)
	if shouldApplyExperimentRiskFloor(baselineTask, baselineRisk) && riskRank(riskLabel) < riskRank(baselineRisk) {
		riskLabel = baselineRisk
		if riskConfidence < 0.55 {
			riskConfidence = 0.55
		}
	}
	return ExperimentPrediction{
		TaskType:       taskLabel,
		TaskConfidence: taskConfidence,
		RiskLevel:      riskLabel,
		RiskConfidence: riskConfidence,
	}
}

// CompareExperimentWithRules evaluates the current rules against the trained
// experiment model on the provided test split.
func CompareExperimentWithRules(train, test []ExperimentExample) (ExperimentComparisonReport, error) {
	model, err := TrainLightweightExperiment(train)
	if err != nil {
		return ExperimentComparisonReport{}, err
	}
	report := ExperimentComparisonReport{
		TrainCount: len(train),
		TestCount:  len(test),
		Results:    make([]ExperimentCaseResult, 0, len(test)),
	}
	for _, ex := range test {
		baselineTask, baselineRisk := classifyRuleBaseline(ex.Prompt)
		trained := model.Predict(ex.Prompt)
		report.Results = append(report.Results, ExperimentCaseResult{
			ID:                    ex.ID,
			ExpectedTaskType:      ex.TaskType,
			ExpectedRiskLevel:     ex.RiskLevel,
			BaselineTaskType:      baselineTask,
			BaselineRiskLevel:     baselineRisk,
			TrainedTaskType:       trained.TaskType,
			TrainedTaskConfidence: trained.TaskConfidence,
			TrainedRiskLevel:      trained.RiskLevel,
			TrainedRiskConfidence: trained.RiskConfidence,
		})
		report.BaselineMetrics.add(ex.TaskType == baselineTask, ex.RiskLevel == baselineRisk)
		report.TrainedMetrics.add(ex.TaskType == trained.TaskType, ex.RiskLevel == trained.RiskLevel)
	}
	return report, nil
}

func (m *ExperimentMetrics) add(taskCorrect, riskCorrect bool) {
	m.Total++
	if taskCorrect {
		m.TaskCorrect++
	}
	if riskCorrect {
		m.RiskCorrect++
	}
	if taskCorrect && riskCorrect {
		m.JointCorrect++
	}
}

func classifyRuleBaseline(prompt string) (taskType string, riskLevel string) {
	messages := []openai.Message{{Role: "user", Content: prompt}}
	features := ExtractFromMessages(messages, RequestHints{})
	task := ClassifyTask(features, messages)
	risk := ClassifyRisk(features, task.TaskType, "")
	return task.TaskType, risk.RiskLevel
}

func shouldApplyExperimentRiskFloor(taskType, riskLevel string) bool {
	if riskLevel == RiskCritical {
		return true
	}
	switch taskType {
	case taskSecurityReview, taskDatabaseMigration, taskUnknownHighRisk:
		return true
	default:
		return false
	}
}

type labeledVector struct {
	label    string
	features map[string]float64
}

type linearPerceptron struct {
	labels  []string
	bias    map[string]float64
	weights map[string]map[string]float64
}

func trainLinearPerceptron(examples []labeledVector) linearPerceptron {
	labels := sortedUniqueLabels(examples)
	model := linearPerceptron{
		labels:  labels,
		bias:    make(map[string]float64, len(labels)),
		weights: make(map[string]map[string]float64, len(labels)),
	}
	for _, label := range labels {
		model.weights[label] = make(map[string]float64)
	}

	for epoch := 0; epoch < experimentEpochs; epoch++ {
		mistakes := 0
		for _, ex := range examples {
			predicted, _ := model.predict(ex.features)
			if predicted == ex.label {
				continue
			}
			mistakes++
			model.bias[ex.label]++
			model.bias[predicted]--
			for feature, value := range ex.features {
				model.weights[ex.label][feature] += value
				model.weights[predicted][feature] -= value
			}
		}
		if mistakes == 0 {
			break
		}
	}

	return model
}

func (m linearPerceptron) predict(features map[string]float64) (label string, confidence float64) {
	if len(m.labels) == 0 {
		return "", 0
	}
	bestLabel := m.labels[0]
	bestScore := m.score(bestLabel, features)
	secondBest := math.Inf(-1)
	for _, candidate := range m.labels[1:] {
		score := m.score(candidate, features)
		if score > bestScore {
			secondBest = bestScore
			bestScore = score
			bestLabel = candidate
			continue
		}
		if score > secondBest {
			secondBest = score
		}
	}
	if math.IsInf(secondBest, -1) {
		secondBest = 0
	}
	margin := bestScore - secondBest
	if margin < 0 {
		margin = 0
	}
	if margin > experimentMarginCeil {
		margin = experimentMarginCeil
	}
	confidence = 0.5 + 0.5*(margin/experimentMarginCeil)
	return bestLabel, confidence
}

func (m linearPerceptron) score(label string, features map[string]float64) float64 {
	score := m.bias[label]
	weights := m.weights[label]
	for feature, value := range features {
		score += weights[feature] * value
	}
	return score
}

func sortedUniqueLabels(examples []labeledVector) []string {
	seen := make(map[string]struct{}, len(examples))
	labels := make([]string, 0, len(examples))
	for _, ex := range examples {
		if _, ok := seen[ex.label]; ok {
			continue
		}
		seen[ex.label] = struct{}{}
		labels = append(labels, ex.label)
	}
	sort.Strings(labels)
	return labels
}

func experimentFeatureVector(prompt string) map[string]float64 {
	features := make(map[string]float64)
	messages := []openai.Message{{Role: "user", Content: prompt}}
	extracted := ExtractFromMessages(messages, RequestHints{})
	lower := strings.ToLower(prompt)
	taskBaseline := ClassifyTask(extracted, messages)
	riskBaseline := ClassifyRisk(extracted, taskBaseline.TaskType, "")

	addExperimentTokens(features, lower)
	addFeature(features, "char_bucket:"+experimentCharBucket(len(prompt)), 1)
	addFeature(features, "code_block", boolValue(extracted.HasCodeBlock))
	addFeature(features, "inline_code", boolValue(extracted.HasInlineCode))
	addFeature(features, "stack_trace", boolValue(extracted.HasStackTrace))
	addFeature(features, "requires_code", boolValue(extracted.RequiresCode))
	addFeature(features, "requires_tool_use", boolValue(extracted.RequiresToolUse))
	addFeature(features, "requires_json_schema", boolValue(extracted.RequiresJSONSchema))
	addFeature(features, "requires_large_context", boolValue(extracted.RequiresLargeContext))
	addFeature(features, "requires_vision", boolValue(extracted.RequiresVision))
	if len(extracted.FilesTouched) > 0 {
		addFeature(features, "has_file_path", 1)
	}
	if len(extracted.FilesTouched) > 1 {
		addFeature(features, "multi_file_path", 1)
	}
	for _, keyword := range extracted.Keywords {
		addFeature(features, "kw:"+keyword, 1)
	}
	for _, hint := range extracted.SensitivityHints {
		addFeature(features, "sens:"+hint, 1)
	}
	addFeature(features, "baseline_task:"+taskBaseline.TaskType, 1)
	addFeature(features, "baseline_risk:"+riskBaseline.RiskLevel, 1)
	addFeature(features, "baseline_task_conf:"+experimentConfidenceBucket(taskBaseline.Confidence), 1)
	return features
}

func addExperimentTokens(features map[string]float64, lower string) {
	tokens := experimentTokenPattern.FindAllString(lower, -1)
	unique := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if len(token) <= 1 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		unique = append(unique, token)
		if len(unique) >= experimentMaxTokens {
			break
		}
	}
	for _, token := range unique {
		addFeature(features, "tok:"+token, 1)
	}
	for i := 0; i < len(unique)-1 && i < experimentMaxNGrams; i++ {
		addFeature(features, "bigram:"+unique[i]+"_"+unique[i+1], 1)
	}
}

func experimentCharBucket(n int) string {
	switch {
	case n <= 120:
		return "short"
	case n <= 600:
		return "medium"
	case n <= 4000:
		return "long"
	default:
		return "xl"
	}
}

func experimentConfidenceBucket(confidence float64) string {
	switch {
	case confidence >= 0.85:
		return "high"
	case confidence >= 0.60:
		return "medium"
	default:
		return "low"
	}
}

func addFeature(features map[string]float64, key string, value float64) {
	if key == "" || value == 0 {
		return
	}
	features[key] = value
}

func boolValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
