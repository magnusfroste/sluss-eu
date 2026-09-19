package server

// No-YAML rule editing for the policy console (ISSUE-093 pt 1): the CISO
// builds an ordered ruleset like firewall rules — condition → action — in the
// admin UI. Rules are stored structured in SQLite, rendered to the same YAML
// dialect a file-based policy uses (so validation and semantics are identical),
// and hot-activated into the running policy cache. The env-configured policy
// (ROUTER_POLICY_PATH) stays the BASELINE a rollback returns to. Every
// activation/rollback is recorded in the tamper-evident audit chain.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
)

const (
	consoleRulesKey   = "console_policy_rules"   // JSON []ConsoleRule
	consoleActiveKey  = "console_policy_active"  // "1" when the console ruleset is live
	consoleSeqKey     = "console_policy_seq"     // version counter
	consoleVersionKey = "console_policy_version" // version of the live console ruleset
)

// consoleSensitivities is the condition vocabulary offered in the UI (order =
// display order). Values must stay in policy's validSensitivities.
var consoleSensitivities = []string{
	"pii", "secrets_possible", "financial", "health", "legal", "security_classified", "source_code",
}

// consoleTaskTypes mirrors policy's validTaskTypes (UI display order).
var consoleTaskTypes = []string{
	"security_review", "database_migration", "hard_code_debugging", "long_context_analysis",
	"summarization", "simple_code_edit", "simple_chat", "creative_copy", "trivial_git", "simple_shell",
	"unknown_high_risk",
}

var consoleRiskLevels = []string{"low", "medium", "high", "critical"}

var tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ConsoleRule is one firewall-style rule authored in the console: at least one
// condition, exactly one action. Stored as JSON in the history KV.
type ConsoleRule struct {
	ID            string   `json:"id"`
	Note          string   `json:"note,omitempty"`
	Sensitivities []string `json:"sensitivities,omitempty"` // when.sensitivity in [...]
	TaskType      string   `json:"task_type,omitempty"`
	RiskLevel     string   `json:"risk_level,omitempty"`
	Action        string   `json:"action"`         // "require_tags" | "block"
	Tags          []string `json:"tags,omitempty"` // for require_tags
}

// Validate checks vocabulary and shape; inputs come from the admin form.
func (r ConsoleRule) Validate() error {
	if len(r.Sensitivities) == 0 && r.TaskType == "" && r.RiskLevel == "" {
		return fmt.Errorf("a rule needs at least one condition")
	}
	for _, s := range r.Sensitivities {
		if !inList(consoleSensitivities, s) {
			return fmt.Errorf("unknown sensitivity %q", s)
		}
	}
	if r.TaskType != "" && !inList(consoleTaskTypes, r.TaskType) {
		return fmt.Errorf("unknown task type %q", r.TaskType)
	}
	if r.RiskLevel != "" && !inList(consoleRiskLevels, r.RiskLevel) {
		return fmt.Errorf("unknown risk level %q", r.RiskLevel)
	}
	switch r.Action {
	case "require_tags":
		if len(r.Tags) == 0 {
			return fmt.Errorf("require-tags needs at least one provider tag")
		}
		for _, t := range r.Tags {
			if !tagPattern.MatchString(t) {
				return fmt.Errorf("invalid tag %q (a-z, 0-9, - and _ only)", t)
			}
		}
	case "block":
		// no extra fields
	default:
		return fmt.Errorf("unknown action %q", r.Action)
	}
	return nil
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// WhenSummary renders the condition half for the UI ("sensitivity ∈ pii, health · risk ≥ …").
func (r ConsoleRule) WhenSummary() string {
	var parts []string
	if len(r.Sensitivities) > 0 {
		parts = append(parts, "sensitivity: "+strings.Join(r.Sensitivities, ", "))
	}
	if r.TaskType != "" {
		parts = append(parts, "task: "+r.TaskType)
	}
	if r.RiskLevel != "" {
		parts = append(parts, "risk: "+r.RiskLevel)
	}
	return strings.Join(parts, " · ")
}

// ActionSummary renders the action half for the UI.
func (r ConsoleRule) ActionSummary() string {
	if r.Action == "block" {
		return "BLOCK (fail-closed)"
	}
	return "require provider tags: " + strings.Join(r.Tags, ", ")
}

// LoadConsoleRules reads the stored ruleset (nil store or empty → none).
func LoadConsoleRules(h *history.Store) []ConsoleRule {
	if h == nil {
		return nil
	}
	v, ok := h.KVGet(consoleRulesKey)
	if !ok || v == "" {
		return nil
	}
	var out []ConsoleRule
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil
	}
	return out
}

func saveConsoleRules(h *history.Store, rules []ConsoleRule) error {
	b, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	return h.KVSet(consoleRulesKey, string(b))
}

// ConsolePolicyActive reports whether the console ruleset is the live policy.
func ConsolePolicyActive(h *history.Store) bool {
	if h == nil {
		return false
	}
	v, _ := h.KVGet(consoleActiveKey)
	return v == "1"
}

// nextConsoleVersion increments the stored sequence and returns pv_console_N.
func nextConsoleVersion(h *history.Store) string {
	n := 0
	if v, ok := h.KVGet(consoleSeqKey); ok {
		n, _ = strconv.Atoi(v)
	}
	n++
	_ = h.KVSet(consoleSeqKey, strconv.Itoa(n))
	return fmt.Sprintf("pv_console_%03d", n)
}

// GenerateConsolePolicyYAML renders the ruleset in the same YAML dialect a
// file-based policy uses — identical parse/validation path, and a readable
// artifact for evidence export. All values are validated before rendering, so
// nothing user-controlled can break out of its position.
func GenerateConsolePolicyYAML(version string, rules []ConsoleRule) string {
	var b strings.Builder
	b.WriteString("version: " + version + "\n")
	b.WriteString("metadata:\n")
	b.WriteString("  owner: policy-console\n")
	b.WriteString("  description: Ruleset authored in the policy console (no YAML) — ISSUE-093.\n")
	b.WriteString("settings:\n")
	b.WriteString("  default_model_profile: balanced\n")
	b.WriteString("  conservative_unknowns: true\n")
	b.WriteString("  max_router_overhead_ms: 100\n")
	b.WriteString("  default_timeout_ms: 30000\n")
	b.WriteString("  default_retention: standard\n")
	b.WriteString("rules:\n")
	for i, r := range rules {
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("console_rule_%d", i+1)
		}
		b.WriteString("  - id: " + id + "\n")
		b.WriteString("    when:\n")
		if len(r.Sensitivities) > 0 {
			b.WriteString("      sensitivity: { in: [" + strings.Join(r.Sensitivities, ", ") + "] }\n")
		}
		if r.TaskType != "" {
			b.WriteString("      task_type: " + r.TaskType + "\n")
		}
		if r.RiskLevel != "" {
			b.WriteString("      risk_level: " + r.RiskLevel + "\n")
		}
		b.WriteString("    route:\n")
		if r.Action == "block" {
			b.WriteString("      block:\n")
			b.WriteString("        code: console_rule_block\n")
			b.WriteString("        reason: blocked by console rule " + id + "\n")
		} else {
			b.WriteString("      constraints:\n")
			b.WriteString("        require_provider_tags: [" + strings.Join(r.Tags, ", ") + "]\n")
		}
	}
	b.WriteString("  - id: default\n")
	b.WriteString("    when: {}\n")
	b.WriteString("    route:\n")
	b.WriteString("      defaults:\n")
	b.WriteString("        model_profile: balanced\n")
	return b.String()
}

// PolicyRulesOptions wires the console rule handlers.
type PolicyRulesOptions struct {
	Engine  *engine.Engine
	Cache   *policy.Cache
	History *history.Store
	Auditor audit.Sink
	// BaselinePath is the env-configured policy source (ROUTER_POLICY_PATH
	// verbatim; "" = built-in default) — the ruleset a rollback returns to.
	BaselinePath string
}

func (o PolicyRulesOptions) ready() error {
	if o.History == nil {
		return fmt.Errorf("rule editing needs a data dir (ROUTER_DATA_DIR)")
	}
	if o.Cache == nil || o.Engine == nil || o.Engine.Registry == nil {
		return fmt.Errorf("routing engine/policy cache not configured")
	}
	return nil
}

// activateConsoleRules generates + parses + hot-reloads the console ruleset.
func (o PolicyRulesOptions) activateConsoleRules(r *http.Request, rules []ConsoleRule) (string, error) {
	version := nextConsoleVersion(o.History)
	src := GenerateConsolePolicyYAML(version, rules)
	parsed, err := policy.Parse([]byte(src))
	if err != nil {
		return "", fmt.Errorf("generated policy failed validation: %w", err)
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return "", err
	}
	if err := o.Cache.Reload([]policy.Source{{Policy: parsed, Registry: snap}}); err != nil {
		return "", err
	}
	if err := o.History.KVSet(consoleActiveKey, "1"); err != nil {
		return "", err
	}
	_ = o.History.KVSet(consoleVersionKey, version)
	audit.Record(r.Context(), o.Auditor, audit.Entry{
		Action: audit.ActionPolicyConsole,
		Actor:  AdminUserFromContext(r.Context()),
		Target: version,
		Reason: fmt.Sprintf("console ruleset activated (%d rules + default)", len(rules)),
	})
	return version, nil
}

// rollbackToBaseline reloads the env-configured policy and clears the flag.
func (o PolicyRulesOptions) rollbackToBaseline(r *http.Request) error {
	data, err := policy.LoadSourceBytes(strings.TrimSpace(o.BaselinePath))
	if err != nil {
		return err
	}
	parsed, err := policy.Parse(data)
	if err != nil {
		return err
	}
	snap, err := o.Engine.Registry.Active()
	if err != nil {
		return err
	}
	if err := o.Cache.Reload([]policy.Source{{Policy: parsed, Registry: snap}}); err != nil {
		return err
	}
	if err := o.History.KVSet(consoleActiveKey, "0"); err != nil {
		return err
	}
	audit.Record(r.Context(), o.Auditor, audit.Entry{
		Action: audit.ActionPolicyConsole,
		Actor:  AdminUserFromContext(r.Context()),
		Target: parsed.Version,
		Reason: "rolled back to baseline policy (" + firstNonEmpty(o.BaselinePath, "built-in default") + ")",
	})
	return nil
}

// reapplyIfActive re-activates after a rule edit so a live ruleset never drifts
// from what the table shows.
func (o PolicyRulesOptions) reapplyIfActive(r *http.Request, rules []ConsoleRule) error {
	if !ConsolePolicyActive(o.History) {
		return nil
	}
	_, err := o.activateConsoleRules(r, rules)
	return err
}

func redirectPolicy(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/policy"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

// PolicyRuleAddHandler appends a rule from the console form.
func PolicyRuleAddHandler(o PolicyRulesOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := o.ready(); err != nil {
			redirectPolicy(w, r, "", err.Error())
			return
		}
		if err := r.ParseForm(); err != nil {
			redirectPolicy(w, r, "", "invalid form")
			return
		}
		rule := ConsoleRule{
			Note:      strings.TrimSpace(r.FormValue("note")),
			TaskType:  strings.TrimSpace(r.FormValue("task_type")),
			RiskLevel: strings.TrimSpace(r.FormValue("risk_level")),
			Action:    strings.TrimSpace(r.FormValue("action")),
		}
		rule.Sensitivities = append(rule.Sensitivities, r.Form["sensitivity"]...)
		sort.Strings(rule.Sensitivities)
		for _, t := range strings.Split(r.FormValue("tags"), ",") {
			if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
				rule.Tags = append(rule.Tags, t)
			}
		}
		rules := LoadConsoleRules(o.History)
		rule.ID = fmt.Sprintf("console_rule_%d", len(rules)+1)
		if err := rule.Validate(); err != nil {
			redirectPolicy(w, r, "", err.Error())
			return
		}
		rules = append(rules, rule)
		if err := saveConsoleRules(o.History, rules); err != nil {
			redirectPolicy(w, r, "", "save: "+err.Error())
			return
		}
		if err := o.reapplyIfActive(r, rules); err != nil {
			redirectPolicy(w, r, "", "rule saved but re-activation failed: "+err.Error())
			return
		}
		msg := "Rule added."
		if ConsolePolicyActive(o.History) {
			msg = "Rule added and live (ruleset re-activated)."
		}
		redirectPolicy(w, r, msg, "")
	}
}

// PolicyRuleDeleteHandler removes a rule by index.
func PolicyRuleDeleteHandler(o PolicyRulesOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := o.ready(); err != nil {
			redirectPolicy(w, r, "", err.Error())
			return
		}
		_ = r.ParseForm()
		idx, err := strconv.Atoi(r.FormValue("index"))
		rules := LoadConsoleRules(o.History)
		if err != nil || idx < 0 || idx >= len(rules) {
			redirectPolicy(w, r, "", "unknown rule")
			return
		}
		rules = append(rules[:idx], rules[idx+1:]...)
		if err := saveConsoleRules(o.History, rules); err != nil {
			redirectPolicy(w, r, "", "save: "+err.Error())
			return
		}
		if err := o.reapplyIfActive(r, rules); err != nil {
			redirectPolicy(w, r, "", "rule removed but re-activation failed: "+err.Error())
			return
		}
		redirectPolicy(w, r, "Rule removed.", "")
	}
}

// PolicyActivateHandler makes the console ruleset the live policy.
func PolicyActivateHandler(o PolicyRulesOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := o.ready(); err != nil {
			redirectPolicy(w, r, "", err.Error())
			return
		}
		rules := LoadConsoleRules(o.History)
		if len(rules) == 0 {
			redirectPolicy(w, r, "", "no console rules to activate — add a rule first")
			return
		}
		version, err := o.activateConsoleRules(r, rules)
		if err != nil {
			redirectPolicy(w, r, "", "activate: "+err.Error())
			return
		}
		redirectPolicy(w, r, "Console ruleset live as "+version+" — try the dry-run below.", "")
	}
}

// PolicyRollbackHandler returns to the env-configured baseline policy.
func PolicyRollbackHandler(o PolicyRulesOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := o.ready(); err != nil {
			redirectPolicy(w, r, "", err.Error())
			return
		}
		if err := o.rollbackToBaseline(r); err != nil {
			redirectPolicy(w, r, "", "rollback: "+err.Error())
			return
		}
		redirectPolicy(w, r, "Rolled back to the baseline policy.", "")
	}
}

// ApplyStoredConsolePolicy re-activates a stored, previously-active console
// ruleset at boot, so the CISO's rules survive a restart/redeploy. It keeps the
// version the ruleset was activated under (no bump on restart). Returns the
// applied version ("" when nothing was applied). On failure the baseline policy
// stays in place — falling back to the OPERATOR-configured baseline, never to
// an unvalidated ruleset.
func ApplyStoredConsolePolicy(cache *policy.Cache, snap *registry.Snapshot, h *history.Store) (string, error) {
	if h == nil || cache == nil || snap == nil || !ConsolePolicyActive(h) {
		return "", nil
	}
	rules := LoadConsoleRules(h)
	if len(rules) == 0 {
		return "", nil
	}
	version, _ := h.KVGet(consoleVersionKey)
	if version == "" {
		version = nextConsoleVersion(h)
	}
	src := GenerateConsolePolicyYAML(version, rules)
	parsed, err := policy.Parse([]byte(src))
	if err != nil {
		return "", err
	}
	if err := cache.Reload([]policy.Source{{Policy: parsed, Registry: snap}}); err != nil {
		return "", err
	}
	return version, nil
}
