package server

// Admin-managed demo quick prompts (the chips on the /chat demo page). They live
// in the SQLite KV store so a curated demo — the one a CISO clicks through from a
// shared link — survives restarts and can be edited without a redeploy. Seeded
// on first run with a value-showcasing set (cost routing, the PII→local switch,
// premium for hard tasks, fail-closed on a security review).

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/history"
)

// quickPromptsKey is the KV key holding the demo quick prompts JSON array.
const quickPromptsKey = "demo_quick_prompts"

// QuickPrompt is one clickable demo chip. Tier is a short human hint of what the
// router will do; Note explains the value to watch for.
type QuickPrompt struct {
	Label string `json:"label"`
	Tier  string `json:"tier"`
	Text  string `json:"text"`
	Note  string `json:"note,omitempty"`
}

// defaultQuickPrompts is the curated set that demonstrates the platform's value.
// The personnummer is SYNTHETIC (fake, Luhn-valid) — never a real person.
func defaultQuickPrompts() []QuickPrompt {
	return []QuickPrompt{
		{
			Label: "Write a git commit message",
			Tier:  "cheap",
			Text:  "write a concise git commit message for a bugfix",
			Note:  "Simple task → cheap model. Cost is minimised automatically.",
		},
		{
			Label: "Summarise a customer case (contains a personal ID)",
			Tier:  "PII → local",
			Text:  "Summarise the case for customer 811218-9876 who complained about an invoice.",
			Note:  "Personal data detected → routed to a local model. The data never leaves the house.",
		},
		{
			Label: "Debug a broken payment integration",
			Tier:  "hard",
			Text:  "debug this NullPointerException in our Stripe webhook handler that intermittently drops payment confirmations under load",
			Note:  "Hard, business-critical → premium model for quality.",
		},
		{
			Label: "Security-review the login flow",
			Tier:  "blocked",
			Text:  "security review our internal SSO login flow for auth bypass and secret leakage",
			Note:  "Most sensitive class → requires an air-gapped model. None available → blocked (fail-closed).",
		},
		{
			Label: "Translate an invoice reply",
			Tier:  "cheap",
			Text:  "translate to Swedish: 'your invoice is overdue, please pay within 7 days'",
			Note:  "Trivial → cheap model.",
		},
		{
			Label: "Explain what NIS2 requires of management",
			Tier:  "balanced",
			Text:  "explain in plain terms what NIS2 requires of a company's management (governance and accountability)",
			Note:  "Balanced task → mid-tier model. Capability without overspend.",
		},
	}
}

// LoadQuickPrompts returns the stored prompts, or the built-in defaults when the
// store is absent or empty.
func LoadQuickPrompts(h *history.Store) []QuickPrompt {
	if h != nil {
		if v, ok := h.KVGet(quickPromptsKey); ok && strings.TrimSpace(v) != "" {
			var out []QuickPrompt
			if err := json.Unmarshal([]byte(v), &out); err == nil && len(out) > 0 {
				return out
			}
		}
	}
	return defaultQuickPrompts()
}

// SaveQuickPrompts persists the prompts. A nil store is a gentle no-op.
func SaveQuickPrompts(h *history.Store, prompts []QuickPrompt) error {
	if h == nil {
		return nil
	}
	b, err := json.Marshal(prompts)
	if err != nil {
		return err
	}
	return h.KVSet(quickPromptsKey, string(b))
}

// SeedQuickPrompts writes the defaults into the DB on first run (when the KV is
// empty) so the curated set is durable and editable. No-op when already set.
func SeedQuickPrompts(h *history.Store) {
	if h == nil {
		return
	}
	if v, ok := h.KVGet(quickPromptsKey); ok && strings.TrimSpace(v) != "" {
		return
	}
	_ = SaveQuickPrompts(h, defaultQuickPrompts())
}

// QuickPromptsHandler serves the prompts as JSON for the demo chat page.
func QuickPromptsHandler(h *history.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoadQuickPrompts(h))
	}
}
