package server

// "Needs you" for the Overview (ISSUE-135): what an admin should act on,
// first on the page, before any number. Each item names the problem in plain
// words and links to the one page where it is fixed. Sources are cheap and
// in-memory or a single SQLite read: recent upstream failures, provider keys,
// quick-start progress and device coverage.

import (
	"os"
	"sort"
	"strings"
	"time"
)

// attentionItem is one thing that needs the admin.
type attentionItem struct {
	Level  string // "bad" | "warn"
	Title  string
	Detail string
	Href   string
	Action string
}

// attentionWindow is how far back upstream failures count.
const attentionWindow = time.Hour

func buildAttention(opts DashboardOptions, d *DashboardData, now time.Time) []attentionItem {
	var out []attentionItem

	// A model that keeps failing at its provider: the lab box renamed or
	// unloaded it, the endpoint is down, or the key was rotated.
	if opts.ErrorRing != nil {
		type key struct{ provider, model, code string }
		counts := map[key]int{}
		for _, e := range opts.ErrorRing.Recent(0) {
			if now.Sub(e.At) <= attentionWindow {
				counts[key{e.ProviderID, e.ModelID, e.ErrorCode}]++
			}
		}
		keys := make([]key, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
		for _, k := range keys {
			out = append(out, attentionItem{
				Level:  "bad",
				Title:  k.model + " on " + k.provider + " is failing",
				Detail: plural(counts[k], "failed request") + " in the last hour (" + humanize(k.code) + "). Press Test on the provider to see what it serves now.",
				Href:   "/router/providers",
				Action: "Test provider",
			})
		}
	}

	// A provider whose key is not in the environment cannot answer at all.
	if opts.Engine != nil && opts.Engine.Registry != nil {
		if snap, err := opts.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				ref := strings.TrimSpace(p.AuthSecretRef)
				if ref != "" && strings.TrimSpace(os.Getenv(ref)) == "" && len(snap.ModelsForProvider(p.ID)) > 0 {
					out = append(out, attentionItem{
						Level:  "warn",
						Title:  p.ID + " has no key",
						Detail: ref + " is not set in the environment, so its models cannot be used.",
						Href:   "/router/providers",
						Action: "Open providers",
					})
				}
			}
		}
	}

	if !d.Setup.Complete() {
		out = append(out, attentionItem{
			Level:  "warn",
			Title:  "Setup is not finished",
			Detail: itoa(d.Setup.Done()) + " of 4 steps done: a cloud model, a local model, keep personal data local, try it.",
			Href:   "/router/setup",
			Action: "Quick start",
		})
	}

	if s := activeUpdates.Status(); s.Available {
		out = append(out, attentionItem{
			Level:  "info",
			Title:  "A newer version of Sluss is available",
			Detail: plural(s.Behind, "change") + " since this build. See what is new and how to upgrade the way you deployed it.",
			Href:   "/router/system",
			Action: "What is new",
		})
	}

	if d.Coverage.DirectOpen > 0 {
		out = append(out, attentionItem{
			Level:  "warn",
			Title:  plural(d.Coverage.DirectOpen, "managed device") + " can still reach AI providers directly",
			Detail: "The filter does not apply to them yet. The device list shows which ones.",
			Href:   "/router/rollout",
			Action: "See devices",
		})
	}
	return out
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}
