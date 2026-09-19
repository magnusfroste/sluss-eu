package classifier

import (
	"strings"

	"github.com/magnusfroste/sluss/internal/openai"
)

type TaskClassification struct {
	TaskType   string
	Confidence float64
	Signals    []string
}

const (
	taskSimpleChat          = "simple_chat"
	taskTrivialGit          = "trivial_git"
	taskSimpleShell         = "simple_shell"
	taskSummarization       = "summarization"
	taskSimpleCodeEdit      = "simple_code_edit"
	taskHardCodeDebugging   = "hard_code_debugging"
	taskSecurityReview      = "security_review"
	taskDatabaseMigration   = "database_migration"
	taskLongContextAnalysis = "long_context_analysis"
	taskCreativeCopy        = "creative_copy"
	taskUnknownHighRisk     = "unknown_high_risk"
)

type taskContext struct {
	lower     string
	charCount int
}

func ClassifyTask(features Features, messages []openai.Message) TaskClassification {
	ctx := newTaskContext(messages)
	signals := newSignalSet()
	addFeatureSignals(signals, features)

	// Documentation/summary/explanation intent describes a security topic rather
	// than requesting security work, so it must not escalate to security_review
	// just because security keywords appear (e.g. "document the security review
	// module"). Genuine reviews ("review/audit/scan ... for XSS") carry no doc
	// intent and are unaffected. See ISSUE-066.
	hasSecurityReviewSignal := hasExplicitSecurityReviewSignal(ctx) && !hasDocIntentSignal(ctx)
	hasDatabaseMigrationSignal := hasDatabaseMigrationSignal(ctx, features)
	hasDebugSignal := (features.HasStackTrace || hasAnyKeyword(features.Keywords, "debug")) &&
		(features.RequiresCode || hasAnyKeyword(features.Keywords, "code", "auth", "payment", "security"))
	hasRiskSignal := hasTaskRiskSignal(ctx, features)

	switch {
	case hasSecurityReviewSignal:
		signals.add("security_review_keyword")
		return taskResult(taskSecurityReview, 0.92, signals.values())
	case hasDatabaseMigrationSignal:
		signals.add("database_migration_keyword")
		return taskResult(taskDatabaseMigration, 0.90, signals.values())
	case hasDebugSignal:
		signals.add("debug_keyword")
		return taskResult(taskHardCodeDebugging, 0.88, signals.values())
	case hasTrivialGitSignal(ctx):
		signals.add("git_keyword")
		return taskResult(taskTrivialGit, 0.86, signals.values())
	case hasRiskSignal && !hasStrongTaskIntent(ctx, features):
		signals.add("low_task_confidence")
		return taskResult(taskUnknownHighRisk, 0.34, signals.values())
	case features.RequiresLargeContext:
		signals.add("large_context")
		return taskResult(taskLongContextAnalysis, 0.78, signals.values())
	case hasSummarizationSignal(ctx):
		signals.add("summarize_keyword")
		return taskResult(taskSummarization, 0.84, signals.values())
	case hasSimpleCodeEditSignal(ctx, features):
		signals.add("edit_keyword")
		return taskResult(taskSimpleCodeEdit, 0.82, signals.values())
	case hasCodeGenSignal(ctx):
		signals.add("codegen_keyword")
		return taskResult(taskSimpleCodeEdit, 0.80, signals.values())
	case hasSimpleShellSignal(ctx, features):
		signals.add("shell_keyword")
		return taskResult(taskSimpleShell, 0.76, signals.values())
	case hasCreativeCopySignal(ctx):
		signals.add("creative_keyword")
		return taskResult(taskCreativeCopy, 0.74, signals.values())
	case isShortPrompt(ctx) && !features.RequiresCode && !features.RequiresToolUse && !hasRiskSignal:
		signals.add("short_prompt")
		return taskResult(taskSimpleChat, 0.72, signals.values())
	case hasRiskSignal:
		signals.add("low_task_confidence")
		return taskResult(taskUnknownHighRisk, 0.32, signals.values())
	default:
		signals.add("low_task_confidence")
		return taskResult(taskSimpleChat, 0.45, signals.values())
	}
}

func taskResult(taskType string, confidence float64, signals []string) TaskClassification {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	if len(signals) == 0 {
		signals = []string{"no_signal"}
	}
	return TaskClassification{TaskType: taskType, Confidence: confidence, Signals: signals}
}

func newTaskContext(messages []openai.Message) taskContext {
	var b strings.Builder
	charCount := 0
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(msg.Content)
		charCount += len(msg.Content)
	}
	return taskContext{lower: strings.ToLower(b.String()), charCount: charCount}
}

func addFeatureSignals(signals *signalSet, features Features) {
	if features.HasStackTrace {
		signals.add("stack_trace")
	}
	if features.HasCodeBlock {
		signals.add("code_block")
	}
	if features.HasInlineCode {
		signals.add("inline_code")
	}
	if len(features.FilesTouched) > 0 {
		signals.add("file_path")
	}
	if features.RequiresCode {
		signals.add("requires_code")
	}
	if features.RequiresToolUse {
		signals.add("tool_use")
	}
	if features.RequiresJSONSchema {
		signals.add("json_schema")
	}
	if features.RequiresVision {
		signals.add("vision")
	}
	if features.RequiresLargeContext {
		signals.add("large_context")
	}
	for _, keyword := range features.Keywords {
		switch keyword {
		case "auth":
			signals.add("auth_keyword")
		case "payment":
			signals.add("payment_keyword")
		case "security":
			signals.add("security_keyword")
		case "secret":
			signals.add("secret_keyword")
		case "pii":
			signals.add("pii_keyword")
		case "migration":
			signals.add("migration_keyword")
		case "sql":
			signals.add("sql_keyword")
		case "source_code":
			signals.add("source_code_keyword")
		case "debug":
			signals.add("debug_keyword")
		case "code":
			signals.add("code_keyword")
		}
	}
	for _, hint := range features.SensitivityHints {
		switch hint {
		case "source_code":
			signals.add("source_code_sensitivity")
		case "secrets_possible":
			signals.add("secret_sensitivity")
		case "pii":
			signals.add("pii_sensitivity")
		case "auth":
			signals.add("auth_sensitivity")
		case "payment":
			signals.add("payment_sensitivity")
		case "security":
			signals.add("security_sensitivity")
		}
	}
}

func hasExplicitSecurityReviewSignal(ctx taskContext) bool {
	if containsAnyTerm(ctx.lower, []string{"xss", "csrf", "ssrf", "vulnerability", "security review", "security audit", "threat model", "exploit", "cve"}) {
		return true
	}
	return containsAnyTerm(ctx.lower, []string{"review", "audit", "scan"}) && containsAnyTerm(ctx.lower, []string{"secret", "secrets", "injection", "auth bypass", "permission bypass"})
}

func hasDatabaseMigrationSignal(ctx taskContext, features Features) bool {
	if hasAnyKeyword(features.Keywords, "migration") && hasAnyKeyword(features.Keywords, "sql") {
		return true
	}
	if containsAnyTerm(ctx.lower, []string{".sql", "db/migrations", "migration", "migrations"}) && containsAnyTerm(ctx.lower, []string{"alter table", "create table", "create index", "drop table", "schema", "rollback"}) {
		return true
	}
	return containsAnyTerm(ctx.lower, []string{"alter table", "create table", "create index", "drop table"})
}

func hasTrivialGitSignal(ctx taskContext) bool {
	hasCommitMessage := containsAnyTerm(ctx.lower, []string{"commit message", "commit title", "commit body"})
	hasDiff := containsAnyTerm(ctx.lower, []string{"diff --git", "git diff", "+++ b/", "--- a/", "@@"})
	hasGitAction := containsAnyTerm(ctx.lower, []string{"git commit", "staged changes", "changelog", "release notes"})
	return hasCommitMessage && (hasDiff || hasGitAction)
}

func hasSummarizationSignal(ctx taskContext) bool {
	return containsAnyTerm(ctx.lower, []string{"summarize", "summarise", "summary", "tl;dr", "recap", "sammanfatta"}) ||
		hasDocIntentSignal(ctx)
}

// hasDocIntentSignal detects documentation/explanation intent — writing about or
// describing code rather than acting on it. It both routes such requests as
// summarization and (via hasSecurityReviewSignal) prevents a security topic from
// over-escalating a doc task to security_review. Deliberately uses specific doc
// nouns/verbs, not broad words like "update"/"add", to avoid false positives.
func hasDocIntentSignal(ctx taskContext) bool {
	// Use "documentation"/verb-phrases, not bare "document": the noun ("HTML
	// document", "document.getElementById") is not a documentation intent and
	// must not suppress code-generation tasks. See classifier-codegen-gap.
	return containsAnyTerm(ctx.lower, []string{
		"documentation", "docs", "docstring", "doc comment",
		"document this", "document the", "documenting",
		"readme", "changelog", "release notes", "explain", "describe",
	})
}

// hasCodeGenSignal detects a request to generate new code from scratch (build /
// create / write a program), as opposed to editing or debugging existing code.
// Such requests otherwise fall through to simple_chat → cheap; routing them to a
// coding-capable tier matches intent. Requires both an action verb and a
// code-specific object to avoid catching prose ("write a blog post").
func hasCodeGenSignal(ctx taskContext) bool {
	verb := containsAnyTerm(ctx.lower, []string{
		"create", "build", "write", "implement", "generate", "develop", "scaffold", "code up", "make a", "make me",
	})
	object := containsAnyTerm(ctx.lower, []string{
		"app", "application", "script", "program", "function", "class", "component",
		"api", "endpoint", "rest api", "website", "web page", "webpage", "web app",
		"game", "cli", "command-line tool", "module", "library", "server", "parser",
		"algorithm", "bot", "crawler", "scraper", "microservice", "html", "css",
		"javascript", "typescript", "react", "vue", "svelte", "python script",
		"go program", "sql query", "regex", "unit tests", "data structure",
	})
	return verb && object
}

func hasSimpleCodeEditSignal(ctx taskContext, features Features) bool {
	if features.HasStackTrace {
		return false
	}
	if !features.RequiresCode {
		return false
	}
	// These verbs only classify as a code edit when code was already detected
	// (RequiresCode, checked above), so broadening them adds no false positives —
	// it just keeps code-with-clear-intent from falling through to
	// unknown_high_risk. See classifier-codegen-gap.
	return containsAnyTerm(ctx.lower, []string{
		"fix", "change", "edit", "update", "refactor", "rename", "add", "remove", "modify",
		"convert", "optimize", "port", "rewrite", "simplify", "clean up", "improve", "review",
	})
}

func hasSimpleShellSignal(ctx taskContext, features Features) bool {
	if features.RequiresCode || features.HasStackTrace {
		return false
	}
	return containsAnyTerm(ctx.lower, []string{"shell command", "terminal command", "bash", "zsh", "cli command"})
}

func hasCreativeCopySignal(ctx taskContext) bool {
	return containsAnyTerm(ctx.lower, []string{"write copy", "rewrite copy", "headline", "tagline", "landing page copy", "ad copy"})
}

func hasStrongTaskIntent(ctx taskContext, features Features) bool {
	if features.HasStackTrace || features.RequiresLargeContext || features.RequiresToolUse || features.RequiresJSONSchema {
		return true
	}
	if features.RequiresCode || hasAnyKeyword(features.Keywords, "debug", "code") {
		return true
	}
	if hasCodeGenSignal(ctx) {
		return true
	}
	return hasSummarizationSignal(ctx) ||
		hasSimpleCodeEditSignal(ctx, features) ||
		hasTrivialGitSignal(ctx) ||
		hasDatabaseMigrationSignal(ctx, features) ||
		hasExplicitSecurityReviewSignal(ctx) ||
		hasSimpleShellSignal(ctx, features) ||
		hasCreativeCopySignal(ctx)
}

func hasTaskRiskSignal(ctx taskContext, features Features) bool {
	if len(features.SensitivityHints) > 0 {
		return true
	}
	if hasAnyKeyword(features.Keywords, "auth", "payment", "security", "secret", "pii") {
		return true
	}
	return containsAnyTerm(ctx.lower, []string{"production", "prod", "customer data", "pii", "personnummer", "ssn", "secret", "secrets", "api key", "private key"})
}

func isShortPrompt(ctx taskContext) bool {
	return ctx.charCount > 0 && ctx.charCount <= 240
}

type signalSet struct {
	seen       map[string]struct{}
	valuesList []string
}

func newSignalSet() *signalSet {
	return &signalSet{seen: make(map[string]struct{})}
}

func (s *signalSet) add(signal string) {
	if signal == "" {
		return
	}
	if _, ok := s.seen[signal]; ok {
		return
	}
	s.seen[signal] = struct{}{}
	s.valuesList = append(s.valuesList, signal)
}

func (s *signalSet) values() []string {
	return append([]string(nil), s.valuesList...)
}
