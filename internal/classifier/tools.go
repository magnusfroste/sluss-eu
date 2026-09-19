package classifier

// Tool-risk classification (ISSUE-111). An agent request declares the tools the
// model may call; the market calls governing that layer an "agent gateway".
// Sluss classifies declared tools the same way it classifies content:
// deterministic keyword rules, no LLM, ranked so policy can gate on the highest
// risk present.
//
// This governs the tools an agent DECLARES, not the tool call a model returns.
// Declaration is what the policy engine sees before the provider call, on the
// fast path, which keeps the fail-closed contract: a prompt that can reach a
// destructive tool is classified as such BEFORE any model sees it.

import "strings"

// ToolRisk ranks the capability an agent hands to the model. Ranking is fixed:
// deployed policies match on these names, so values must never be reordered or
// renamed (same rule as sensitivity classes).
const (
	ToolRiskNone        = ""            // no tools declared
	ToolRiskRead        = "read"        // retrieval only: search, get, list, fetch
	ToolRiskWrite       = "write"       // mutates internal state: create, update, upload
	ToolRiskExternal    = "external"    // leaves the organisation: email, sms, publish, pay
	ToolRiskDestructive = "destructive" // delete, execute, admin: hardest to undo
)

// toolRiskRank orders the classes; the highest-ranked class present wins.
var toolRiskRank = map[string]int{
	ToolRiskNone:        0,
	ToolRiskRead:        1,
	ToolRiskWrite:       2,
	ToolRiskExternal:    3,
	ToolRiskDestructive: 4,
}

// Substring rules per class, matched against the tool name and description
// (lowercased). Ordered most-severe first so the first hit wins per tool.
var toolRiskRules = []struct {
	class string
	terms []string
}{
	{ToolRiskDestructive, []string{
		"delete", "destroy", "drop_", "drop table", "purge", "wipe", "truncate",
		"remove", "revoke", "terminate", "shutdown", "kill", "exec", "execute",
		"shell", "bash", "eval", "run_command", "sudo", "chmod", "format",
		"radera", "ta_bort", "avsluta",
	}},
	{ToolRiskExternal, []string{
		"send_email", "send_mail", "sendmail", "email", "sms", "slack", "teams",
		"webhook", "publish", "tweet", "post_message", "notify", "call_api",
		"http_request", "fetch_url", "browse", "payment", "charge", "transfer",
		"invoice", "refund", "skicka", "betal",
	}},
	{ToolRiskWrite, []string{
		"create", "insert", "update", "write", "upload", "save", "store",
		"modify", "edit", "patch", "put_", "set_", "add_", "commit", "merge",
		"deploy", "provision", "grant", "skapa", "spara", "uppdatera",
	}},
	{ToolRiskRead, []string{
		"search", "query", "get_", "list", "read", "fetch", "lookup", "find",
		"retrieve", "describe", "show", "view", "select", "count", "summar",
		"sok", "hamta", "lista",
	}},
}

// classifyTool returns the risk class for one tool name+description.
// Unrecognised tools fall back to write: an unknown capability is assumed to
// change something. Assuming read-only would be the fail-open choice.
func classifyTool(name, description string) string {
	hay := strings.ToLower(name + " " + description)
	if strings.TrimSpace(hay) == "" {
		return ToolRiskWrite
	}
	for _, rule := range toolRiskRules {
		for _, term := range rule.terms {
			if strings.Contains(hay, term) {
				return rule.class
			}
		}
	}
	return ToolRiskWrite
}

// toolSpec is one declared tool, flattened from either wire format.
type toolSpec struct {
	Name        string
	Description string
}

// extractTools flattens declared tools from the raw request payload. It accepts
// both the OpenAI shape ({"type":"function","function":{"name","description"}})
// and the Anthropic shape ({"name","description"}), because Sluss speaks both.
func extractTools(tools []any) []toolSpec {
	var out []toolSpec
	for _, raw := range tools {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		// OpenAI: the useful fields sit under "function".
		if fn, ok := m["function"].(map[string]any); ok {
			out = append(out, toolSpec{Name: stringField(fn, "name"), Description: stringField(fn, "description")})
			continue
		}
		// Anthropic / flat: name and description at the top level.
		name := stringField(m, "name")
		if name == "" && stringField(m, "description") == "" {
			continue
		}
		out = append(out, toolSpec{Name: name, Description: stringField(m, "description")})
	}
	return out
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// ClassifyTools returns the declared tool names and the highest risk class
// among them. Names are surfaced so policy can match specific tools and so the
// audit trail records which capabilities were on the table — never the
// arguments, which may carry customer data.
func ClassifyTools(tools []any) (names []string, risk string) {
	specs := extractTools(tools)
	if len(specs) == 0 {
		return nil, ToolRiskNone
	}
	risk = ToolRiskNone
	for _, s := range specs {
		if s.Name != "" {
			names = addCappedNames(names, s.Name)
		}
		c := classifyTool(s.Name, s.Description)
		if toolRiskRank[c] > toolRiskRank[risk] {
			risk = c
		}
	}
	return names, risk
}

// MaxToolNames caps how many tool names ride along in the descriptor; agent
// harnesses can declare dozens and the descriptor stays on the fast path.
const MaxToolNames = 32

func addCappedNames(list []string, name string) []string {
	if len(list) >= MaxToolNames {
		return list
	}
	name = strings.TrimSpace(name)
	for _, existing := range list {
		if existing == name {
			return list
		}
	}
	return append(list, name)
}
