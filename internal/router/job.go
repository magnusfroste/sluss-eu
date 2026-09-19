// Package router contains internal routing contracts shared between the
// classifier, policy engine, route decision engine, and request processors.
package router

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/pii"
)

type TaskType string

const (
	TaskSimpleChat          TaskType = "simple_chat"
	TaskTrivialGit          TaskType = "trivial_git"
	TaskSimpleShell         TaskType = "simple_shell"
	TaskSummarization       TaskType = "summarization"
	TaskSimpleCodeEdit      TaskType = "simple_code_edit"
	TaskHardCodeDebugging   TaskType = "hard_code_debugging"
	TaskSecurityReview      TaskType = "security_review"
	TaskDatabaseMigration   TaskType = "database_migration"
	TaskLongContextAnalysis TaskType = "long_context_analysis"
	TaskCreativeCopy        TaskType = "creative_copy"
	TaskUnknownHighRisk     TaskType = "unknown_high_risk"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Sensitivity string

const (
	SensitivityNone            Sensitivity = "none"
	SensitivitySourceCode      Sensitivity = "source_code"
	SensitivityPII             Sensitivity = "pii"
	SensitivitySecretsPossible Sensitivity = "secrets_possible"
	// Richer sensitivity classes (ISSUE-093), rule-based like the rest.
	SensitivityFinancial          Sensitivity = "financial"
	SensitivityHealth             Sensitivity = "health"
	SensitivityLegal              Sensitivity = "legal"
	SensitivitySecurityClassified Sensitivity = "security_classified"
)

type LatencyPreference string

const (
	LatencyFast     LatencyPreference = "fast"
	LatencyBalanced LatencyPreference = "balanced"
	LatencyQuality  LatencyPreference = "quality"
)

type QualityPreference string

const (
	QualityCheap    QualityPreference = "cheap"
	QualityBalanced QualityPreference = "balanced"
	QualityHigh     QualityPreference = "high"
)

type BudgetPreference string

const (
	BudgetCheap  BudgetPreference = "cheap"
	BudgetNormal BudgetPreference = "normal"
	BudgetHigh   BudgetPreference = "high"
)

type RouterMode string

const (
	RouterModeAuto     RouterMode = "auto"
	RouterModeCheap    RouterMode = "cheap"
	RouterModeBalanced RouterMode = "balanced"
	RouterModePremium  RouterMode = "premium"
	RouterModeDisabled RouterMode = "disabled"
)

type AuthTenantContext struct {
	TenantID  string
	ProjectID string
}

type JobDescriptorInput struct {
	RequestID string
	Auth      AuthTenantContext
	Headers   http.Header
	Request   *openai.ChatRequest
}

type JobDescriptor struct {
	RequestID     string
	TenantID      string
	ProjectID     string
	TenantIDHint  string
	ProjectIDHint string
	TaskType      TaskType
	TaskTypeHint  TaskType
	// TaskConfidence is the classifier's confidence in TaskType, in [0,1].
	// Used by the engine's conservative mode to treat low-confidence
	// classifications more cautiously.
	TaskConfidence float64
	// Conservative marks this request for cautious routing (set by the engine
	// when global conservative mode is on and the classification is uncertain).
	Conservative            bool
	RiskLevel               RiskLevel
	RiskLevelHint           RiskLevel
	Sensitivity             Sensitivity
	SensitivityHint         Sensitivity
	PromptTokensEstimate    int
	MaxOutputTokensEstimate int
	RequiresReasoning       bool
	RequiresCode            bool
	RequiresToolUse         bool
	// ToolsDeclared/ToolRisk carry the agent-capability classification
	// (ISSUE-111) so policy can gate on what an agent may invoke.
	ToolsDeclared        []string
	ToolRisk             string
	RequiresJSONSchema   bool
	RequiresLargeContext bool
	RequiresVision       bool
	LatencyPreference    LatencyPreference
	QualityPreference    QualityPreference
	BudgetPreference     BudgetPreference
	FilesTouched         []string
	Keywords             []string
	// PIITypes lists the PII categories detected in the prompt content (e.g.
	// "personnummer", "email") — types only, never values (ISSUE-077).
	PIITypes      []string
	RouterMode    RouterMode
	ExplicitModel *string
	Metadata      map[string]any
}

func NewJobDescriptor(input JobDescriptorInput) *JobDescriptor {
	req := input.Request
	job := &JobDescriptor{
		RequestID:               input.RequestID,
		TaskType:                TaskUnknownHighRisk,
		RiskLevel:               RiskHigh,
		Sensitivity:             SensitivityNone,
		MaxOutputTokensEstimate: classifier.DefaultMaxOutputTokensEstimate,
		LatencyPreference:       LatencyBalanced,
		QualityPreference:       QualityBalanced,
		BudgetPreference:        BudgetNormal,
		RouterMode:              RouterModeAuto,
	}
	if req == nil {
		return job
	}

	hints := descriptorHints{metadata: req.Metadata, headers: input.Headers}
	job.TenantID = input.Auth.TenantID
	job.ProjectID = input.Auth.ProjectID
	job.Metadata = cloneMetadata(req.Metadata)
	tokens := classifier.EstimateChatRequestTokens(req)
	job.PromptTokensEstimate = tokens.PromptTokensEstimate
	job.MaxOutputTokensEstimate = tokens.MaxOutputTokensEstimate
	features := classifier.ExtractFromRequest(*req)
	job.RequiresCode = features.RequiresCode
	job.RequiresToolUse = features.RequiresToolUse
	job.ToolsDeclared = features.ToolsDeclared
	job.ToolRisk = features.ToolRisk
	job.RequiresJSONSchema = features.RequiresJSONSchema || hasTruthyHint(req.Metadata, "requires_json_schema")
	job.RequiresLargeContext = features.RequiresLargeContext
	job.RequiresVision = features.RequiresVision
	job.FilesTouched = append([]string(nil), features.FilesTouched...)
	job.Keywords = append([]string(nil), features.Keywords...)
	job.PIITypes = pii.Types(features.PIIFindings)
	job.SensitivityHint = sensitivityFromFeatureHints(features.SensitivityHints)
	task := classifier.ClassifyTask(features, req.Messages)
	job.TaskConfidence = task.Confidence
	if taskType, ok := parseTaskType(task.TaskType); ok {
		job.TaskType = taskType
	}
	if taskHint, ok := parseTaskType(hints.firstString("task_type", "task", "x-router-task-type")); ok {
		job.TaskTypeHint = taskHint
	}
	if riskHint, ok := parseRiskLevel(hints.firstString("risk_level", "risk", "x-router-risk-level")); ok {
		job.RiskLevelHint = riskHint
	}
	if sensitivityHint, ok := parseSensitivity(hints.firstString("sensitivity", "x-router-sensitivity")); ok {
		job.SensitivityHint = sensitivityHint
	}
	risk := classifier.ClassifyRisk(features, string(job.TaskType), string(job.RiskLevelHint))
	if riskLevel, ok := parseRiskLevel(risk.RiskLevel); ok {
		job.RiskLevel = riskLevel
	}
	if sensitivity, ok := parseSensitivity(risk.Sensitivity); ok {
		job.Sensitivity = sensitivity
	}

	if job.TenantID == "" {
		job.TenantIDHint = hints.firstString("tenant_id", "tenant", "x-router-tenant-id", "x-tenant-id")
	}
	if job.ProjectID == "" {
		job.ProjectIDHint = hints.firstString("project_id", "project", "x-router-project-id", "x-project-id")
	}
	if latency, ok := parseLatencyPreference(hints.firstString("latency_preference", "latency", "x-router-latency-preference")); ok {
		job.LatencyPreference = latency
	}
	if quality, ok := parseQualityPreference(hints.firstString("quality_preference", "quality", "x-router-quality-preference")); ok {
		job.QualityPreference = quality
	}
	if budget, ok := parseBudgetPreference(hints.firstString("budget_preference", "budget", "x-router-budget-preference")); ok {
		job.BudgetPreference = budget
	}
	if mode, ok := parseRouterMode(hints.firstString("router_mode", "x-router-mode")); ok {
		job.RouterMode = mode
	}
	explicitModel := ""
	if req.Model != "" && req.Model != string(RouterModeAuto) {
		explicitModel = req.Model
	}
	if explicitModel != "" {
		job.ExplicitModel = stringPtr(explicitModel)
	}

	return job
}

func (j *JobDescriptor) SafeLogFields() map[string]any {
	if j == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"request_id":                 j.RequestID,
		"tenant_id":                  j.TenantID,
		"project_id":                 j.ProjectID,
		"tenant_id_hint_present":     j.TenantIDHint != "",
		"project_id_hint_present":    j.ProjectIDHint != "",
		"task_type":                  j.TaskType,
		"task_type_hint":             j.TaskTypeHint,
		"risk_level":                 j.RiskLevel,
		"risk_level_hint":            j.RiskLevelHint,
		"sensitivity":                j.Sensitivity,
		"sensitivity_hint":           j.SensitivityHint,
		"prompt_tokens_estimate":     j.PromptTokensEstimate,
		"max_output_tokens_estimate": j.MaxOutputTokensEstimate,
		"requires_reasoning":         j.RequiresReasoning,
		"requires_code":              j.RequiresCode,
		"requires_tool_use":          j.RequiresToolUse,
		"tool_risk":                  j.ToolRisk,
		"tools_declared":             j.ToolsDeclared,
		"requires_json_schema":       j.RequiresJSONSchema,
		"requires_large_context":     j.RequiresLargeContext,
		"requires_vision":            j.RequiresVision,
		"latency_preference":         j.LatencyPreference,
		"quality_preference":         j.QualityPreference,
		"budget_preference":          j.BudgetPreference,
		"files_touched":              append([]string(nil), j.FilesTouched...),
		"keywords":                   append([]string(nil), j.Keywords...),
		"router_mode":                j.RouterMode,
		"selected_model_present":     j.ExplicitModel != nil,
		"metadata_present":           len(j.Metadata) > 0,
		"metadata_key_count":         len(j.Metadata),
	}
	return out
}

func (j *JobDescriptor) SafeJSON() ([]byte, error) {
	return json.Marshal(j.SafeLogFields())
}

type descriptorHints struct {
	metadata map[string]any
	headers  http.Header
}

func (h descriptorHints) firstString(keys ...string) string {
	for _, key := range keys {
		if value := metadataString(h.metadata, key); value != "" {
			return value
		}
		if value := headerString(h.headers, key); value != "" {
			return value
		}
	}
	return ""
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, candidate := range []string{key, strings.ReplaceAll(key, "-", "_")} {
		if value, ok := metadata[candidate]; ok {
			return anyString(value)
		}
	}
	return ""
}

func headerString(headers http.Header, key string) string {
	if len(headers) == 0 {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(key), "x-") {
		return ""
	}
	return strings.TrimSpace(headers.Get(key))
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func hasTruthyHint(metadata map[string]any, key string) bool {
	value, ok := metadata[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func parseTaskType(value string) (TaskType, bool) {
	switch TaskType(value) {
	case TaskSimpleChat, TaskTrivialGit, TaskSimpleShell, TaskSummarization, TaskSimpleCodeEdit, TaskHardCodeDebugging, TaskSecurityReview, TaskDatabaseMigration, TaskLongContextAnalysis, TaskCreativeCopy, TaskUnknownHighRisk:
		return TaskType(value), true
	default:
		return "", false
	}
}

func parseRiskLevel(value string) (RiskLevel, bool) {
	switch RiskLevel(value) {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return RiskLevel(value), true
	default:
		return "", false
	}
}

func parseSensitivity(value string) (Sensitivity, bool) {
	switch Sensitivity(value) {
	case SensitivityNone, SensitivitySourceCode, SensitivityPII, SensitivitySecretsPossible,
		SensitivityFinancial, SensitivityHealth, SensitivityLegal, SensitivitySecurityClassified:
		return Sensitivity(value), true
	default:
		return "", false
	}
}

// sensitivityHintRank mirrors the classifier's ranking: secrets and pii stay on
// top so a new class never displaces a value existing deployed policies match.
func sensitivityHintRank(s Sensitivity) int {
	switch s {
	case SensitivitySecretsPossible:
		return 7
	case SensitivityPII:
		return 6
	case SensitivitySecurityClassified:
		return 5
	case SensitivityHealth:
		return 4
	case SensitivityFinancial:
		return 3
	case SensitivityLegal:
		return 2
	case SensitivitySourceCode:
		return 1
	default:
		return 0
	}
}

func sensitivityFromFeatureHints(hints []string) Sensitivity {
	best := SensitivityNone
	for _, hint := range hints {
		var s Sensitivity
		switch hint {
		case string(SensitivitySecretsPossible), "secret":
			s = SensitivitySecretsPossible
		case string(SensitivityPII):
			s = SensitivityPII
		case string(SensitivitySecurityClassified):
			s = SensitivitySecurityClassified
		case string(SensitivityHealth):
			s = SensitivityHealth
		case string(SensitivityFinancial):
			s = SensitivityFinancial
		case string(SensitivityLegal):
			s = SensitivityLegal
		case string(SensitivitySourceCode), "auth", "payment", "security":
			s = SensitivitySourceCode
		default:
			continue
		}
		if sensitivityHintRank(s) > sensitivityHintRank(best) {
			best = s
		}
	}
	return best
}

func parseLatencyPreference(value string) (LatencyPreference, bool) {
	switch LatencyPreference(value) {
	case LatencyFast, LatencyBalanced, LatencyQuality:
		return LatencyPreference(value), true
	default:
		return "", false
	}
}

func parseQualityPreference(value string) (QualityPreference, bool) {
	switch QualityPreference(value) {
	case QualityCheap, QualityBalanced, QualityHigh:
		return QualityPreference(value), true
	default:
		return "", false
	}
}

func parseBudgetPreference(value string) (BudgetPreference, bool) {
	switch BudgetPreference(value) {
	case BudgetCheap, BudgetNormal, BudgetHigh:
		return BudgetPreference(value), true
	default:
		return "", false
	}
}

func parseRouterMode(value string) (RouterMode, bool) {
	switch RouterMode(value) {
	case RouterModeAuto, RouterModeCheap, RouterModeBalanced, RouterModePremium, RouterModeDisabled:
		return RouterMode(value), true
	default:
		return "", false
	}
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneMetadata(typed)
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = cloneAny(value)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(typed))
		for k, value := range typed {
			out[k] = value
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, value := range typed {
			out[i] = cloneMetadata(value)
		}
		return out
	default:
		return v
	}
}

func stringPtr(value string) *string {
	return &value
}
