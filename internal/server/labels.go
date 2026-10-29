package server

// Human labels for the codes the engine works with (ISSUE-135). The console
// shows "Personal data", not "pii"; the code stays in a tooltip and in every
// export, audit entry and API response, so nothing an auditor relies on
// changes — only what a person reads on screen.

import "strings"

var dataClassLabels = map[string]string{
	"secrets_possible":    "Secrets and keys",
	"pii":                 "Personal data",
	"security_classified": "Security classified",
	"health":              "Health",
	"financial":           "Financial",
	"legal":               "Legal",
	"source_code":         "Source code",
	"none":                "Nothing sensitive",
	"":                    "Nothing sensitive",
}

var taskLabels = map[string]string{
	"trivial_git":           "Git commit",
	"simple_shell":          "Shell command",
	"simple_chat":           "Chat",
	"summarization":         "Summary",
	"simple_code_edit":      "Code edit",
	"hard_code_debugging":   "Debugging",
	"security_review":       "Security review",
	"database_migration":    "Database migration",
	"long_context_analysis": "Long document",
	"creative_copy":         "Writing",
	"unknown_high_risk":     "Unclassified, high risk",
}

var blockCodeLabels = map[string]string{
	"residency_no_compliant_provider": "No allowed model for this data",
}

// dataClassLabel is the on-screen name for a sensitivity class.
func dataClassLabel(code string) string {
	if l, ok := dataClassLabels[code]; ok {
		return l
	}
	return humanize(code)
}

// taskLabel is the on-screen name for a task type.
func taskLabel(code string) string {
	if l, ok := taskLabels[code]; ok {
		return l
	}
	return humanize(code)
}

// blockLabel is the on-screen reason for a block code.
func blockLabel(code string) string {
	if l, ok := blockCodeLabels[code]; ok {
		return l
	}
	return humanize(code)
}

// humanize turns an unknown snake_case code into "Snake case", so a new code
// never shows up raw.
func humanize(code string) string {
	s := strings.TrimSpace(strings.ReplaceAll(code, "_", " "))
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// plainRule rewrites a policy rule summary ("data is pii or secrets_possible",
// "task is security_review") for the Policy page: codes become lower-case
// labels mid-sentence. Reports and MCP keep the code form.
func plainRule(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if l, ok := dataClassLabels[w]; ok && w != "" && w != "none" {
			words[i] = strings.ToLower(l)
		} else if l, ok := taskLabels[w]; ok {
			words[i] = strings.ToLower(l)
		}
	}
	return strings.Join(words, " ")
}
