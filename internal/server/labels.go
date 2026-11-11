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
	agent := false
	for i, w := range words {
		switch {
		case w == "can" && i > 0 && words[i-1] == "agent":
			agent = true
			continue
		case agent && w == "or":
			continue
		case agent:
			if l, ok := agentLabels[w]; ok {
				words[i] = l
				continue
			}
			agent = false
		}
		if l, ok := dataClassLabels[w]; ok && w != "" && w != "none" {
			words[i] = strings.ToLower(l)
		} else if l, ok := taskLabels[w]; ok {
			words[i] = strings.ToLower(l)
		}
	}
	return strings.Join(words, " ")
}

// plainThen is what a rule does, in the words of the three outcomes
// (ISSUE-150). kind and then come from policy.RuleSummary; the full text stays
// in a tooltip.
func plainThen(kind, then string) string {
	switch kind {
	case "block":
		return "Block"
	case "mask":
		return "Mask and send"
	case "require":
		const p = "only models tagged "
		if strings.HasPrefix(then, p) && !strings.Contains(then, " · ") {
			own := true
			for _, t := range strings.Split(strings.TrimPrefix(then, p), " + ") {
				own = own && localTags[strings.TrimSpace(t)]
			}
			if own {
				return "Keep on own hardware"
			}
		}
		return strings.ToUpper(then[:1]) + then[1:]
	case "default", "other":
		return "Any allowed model"
	}
	if then == "" {
		return "Any allowed model"
	}
	return strings.ToUpper(then[:1]) + then[1:]
}

// agentLabels names what an agent's tools can do.
var agentLabels = map[string]string{
	"read":        "read",
	"write":       "change things",
	"external":    "send things out",
	"destructive": "delete things",
}

// plainWhen is the condition of a rule added in the console, in words.
func (r ConsoleRule) PlainWhen() string {
	var parts []string
	if len(r.Sensitivities) > 0 {
		names := make([]string, len(r.Sensitivities))
		for i, s := range r.Sensitivities {
			names[i] = strings.ToLower(dataClassLabel(s))
		}
		parts = append(parts, "data is "+strings.Join(names, " or "))
	}
	if r.TaskType != "" {
		parts = append(parts, "task is "+strings.ToLower(taskLabel(r.TaskType)))
	}
	if r.RiskLevel != "" {
		parts = append(parts, "risk is "+r.RiskLevel)
	}
	if len(r.ToolRisks) > 0 {
		names := make([]string, len(r.ToolRisks))
		for i, t := range r.ToolRisks {
			names[i] = agentLabels[t]
			if names[i] == "" {
				names[i] = t
			}
		}
		parts = append(parts, "agent can "+strings.Join(names, " or "))
	}
	return strings.Join(parts, " and ")
}

// PlainThen is the outcome of a rule added in the console, in words.
func (r ConsoleRule) PlainThen() string {
	switch r.Action {
	case "block":
		return "Block"
	case "mask":
		return "Mask and send"
	}
	own := len(r.Tags) > 0
	for _, t := range r.Tags {
		own = own && localTags[t]
	}
	if own {
		return "Keep on own hardware"
	}
	return "Only models tagged " + strings.Join(r.Tags, " + ")
}

// Kind is the outcome as a short code, for colour.
func (r ConsoleRule) Kind() string {
	switch r.Action {
	case "block":
		return "block"
	case "mask":
		return "mask"
	}
	return "require"
}
