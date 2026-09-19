package classifier

import "testing"

// ISSUE-111: an agent's declared tools are classified deterministically so
// policy can gate on capability, not just content.
func TestClassifyToolsRanksHighestRisk(t *testing.T) {
	openAI := func(name, desc string) any {
		return map[string]any{"type": "function", "function": map[string]any{
			"name": name, "description": desc}}
	}
	cases := []struct {
		name  string
		tools []any
		want  string
	}{
		{"no tools", nil, ToolRiskNone},
		{"read only", []any{openAI("search_docs", "Search the knowledge base")}, ToolRiskRead},
		{"write", []any{openAI("create_ticket", "Open a support ticket")}, ToolRiskWrite},
		{"external", []any{openAI("send_email", "Email the customer")}, ToolRiskExternal},
		{"destructive", []any{openAI("delete_account", "Remove a customer account")}, ToolRiskDestructive},
		{"highest wins", []any{
			openAI("search_docs", "read"),
			openAI("create_ticket", "write"),
			openAI("delete_account", "danger"),
		}, ToolRiskDestructive},
		{"external beats write", []any{
			openAI("update_record", ""), openAI("send_sms", ""),
		}, ToolRiskExternal},
		// Unknown capability must not be assumed harmless — fail-closed-ish.
		{"unknown defaults to write", []any{openAI("frobnicate", "does a thing")}, ToolRiskWrite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got := ClassifyTools(tc.tools)
			if got != tc.want {
				t.Fatalf("risk = %q, want %q", got, tc.want)
			}
		})
	}
}

// Sluss speaks both wire formats, so tool extraction must too.
func TestClassifyToolsAnthropicShape(t *testing.T) {
	tools := []any{
		map[string]any{"name": "get_weather", "description": "Look up the forecast"},
		map[string]any{"name": "execute_query", "description": "Run SQL"},
	}
	names, risk := ClassifyTools(tools)
	if risk != ToolRiskDestructive {
		t.Fatalf("risk = %q, want destructive (execute)", risk)
	}
	if len(names) != 2 || names[0] != "get_weather" {
		t.Fatalf("names = %v", names)
	}
}

// Names are evidence; arguments are not. We record what could be called.
func TestClassifyToolsCapsAndDedupesNames(t *testing.T) {
	var tools []any
	for i := 0; i < MaxToolNames+10; i++ {
		tools = append(tools, map[string]any{"name": "search_a", "description": "read"})
		tools = append(tools, map[string]any{"name": "search_b", "description": "read"})
	}
	names, _ := ClassifyTools(tools)
	if len(names) != 2 {
		t.Fatalf("expected dedupe to 2 names, got %d: %v", len(names), names)
	}
}
