// Package council implements a Fusion-inspired deliberation panel for
// high-risk task classes. Instead of routing a request to a single model, the
// council fans it out to the top-N routed candidates concurrently and a judge
// selects the consensus answer. This trades cost/latency for reliability and is
// only ever used on the slow path (non-streaming, high-risk), never on the
// fast path.
//
// The judge is deterministic and never calls an LLM: it clusters the panel's
// answers by textual agreement and returns the answer the largest set of models
// agreed on, breaking ties by routing preference (panel order). That keeps the
// project's "no model in the meta-decision" ethos intact while still capturing
// the reliability win of cross-model agreement.
package council

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
)

// Candidate is one panel member: a resolved provider adapter plus the model
// identity used for routing metadata and the provider wire call.
type Candidate struct {
	ProviderID      string
	ModelID         string // registry model ID (routing identity)
	ProviderModelID string // ID sent on the wire to the provider
	Adapter         provider.Adapter
}

// Result is the outcome of one panel member's call. Index is the member's
// position in the panel (0 = most preferred by routing).
type Result struct {
	Index      int
	Candidate  Candidate
	Response   *openai.ChatResponse
	Err        error
	DurationMs int64
}

// answer returns the member's normalized first-choice content, and whether it is
// a usable (successful, non-empty) answer.
func (r Result) answer() (string, bool) {
	if r.Err != nil || r.Response == nil || len(r.Response.Choices) == 0 {
		return "", false
	}
	text := normalize(r.Response.Choices[0].Message.Content)
	if text == "" {
		return "", false
	}
	return text, true
}

// Verdict is the council's judgment over a set of results.
type Verdict struct {
	WinnerIndex  int     // index into the results slice
	Reason       string  // human-readable explanation
	Agreement    float64 // fraction of successful members that agreed with the winner
	PanelSize    int     // number of panel members that ran
	SuccessCount int     // members that returned a usable answer
}

// Judge selects a winning result. It returns false when no member produced a
// usable answer.
type Judge interface {
	Judge(results []Result) (Verdict, bool)
}

// ConsensusJudge picks the answer the largest set of models agreed on, using
// token-set (Jaccard) similarity with Threshold. Ties break toward the
// lower-indexed (more preferred) member. When every answer is distinct it
// returns the top-ranked successful member and flags the lack of consensus.
type ConsensusJudge struct {
	// Threshold is the minimum Jaccard similarity (0..1) for two answers to be
	// considered agreeing. Zero uses DefaultThreshold.
	Threshold float64
}

// DefaultThreshold is the Jaccard word-set similarity above which two free-text
// answers are treated as agreeing.
const DefaultThreshold = 0.6

// Judge implements Judge.
func (j ConsensusJudge) Judge(results []Result) (Verdict, bool) {
	threshold := j.Threshold
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	type ok struct {
		idx    int
		tokens map[string]struct{}
	}
	var usable []ok
	for i, r := range results {
		if text, good := r.answer(); good {
			usable = append(usable, ok{idx: i, tokens: tokenSet(text)})
		}
	}
	if len(usable) == 0 {
		return Verdict{PanelSize: len(results)}, false
	}

	// For each usable member, count how many usable members (including itself)
	// agree with it. The anchor with the largest agreement set wins; ties break
	// toward the lower panel index (more preferred by routing).
	bestAnchor := 0
	bestCount := 0
	for a := range usable {
		count := 0
		for b := range usable {
			if a == b || jaccard(usable[a].tokens, usable[b].tokens) >= threshold {
				count++
			}
		}
		if count > bestCount || (count == bestCount && usable[a].idx < usable[bestAnchor].idx) {
			bestAnchor, bestCount = a, count
		}
	}

	winner := usable[bestAnchor].idx
	agreement := float64(bestCount) / float64(len(usable))
	reason := consensusReason(bestCount, len(usable))
	return Verdict{
		WinnerIndex:  winner,
		Reason:       reason,
		Agreement:    agreement,
		PanelSize:    len(results),
		SuccessCount: len(usable),
	}, true
}

func consensusReason(agreeing, successful int) string {
	if successful == 1 {
		return "single model responded; no cross-check available"
	}
	if agreeing <= 1 {
		return "no consensus; returned the top-ranked model's answer"
	}
	if agreeing == successful {
		return "unanimous: all responding models agreed"
	}
	return "majority consensus among responding models"
}

// Run fans the request out to every panel member concurrently and applies the
// judge. onResult, when non-nil, is called once per member (in panel order,
// after all complete) so callers can record attempts for health and spend. It
// returns the ordered results, the verdict, and whether any member succeeded.
func Run(ctx context.Context, panel []Candidate, req *provider.NormalizedModelRequest, judge Judge, onResult func(Result)) ([]Result, Verdict, bool) {
	results := make([]Result, len(panel))
	var wg sync.WaitGroup
	for i, c := range panel {
		wg.Add(1)
		go func(i int, c Candidate) {
			defer wg.Done()
			memberReq := req.Clone()
			memberReq.Model = c.ProviderModelID
			memberReq.Stream = false
			start := time.Now()
			resp, err := c.Adapter.Complete(ctx, memberReq)
			results[i] = Result{
				Index:      i,
				Candidate:  c,
				Response:   resp,
				Err:        err,
				DurationMs: time.Since(start).Milliseconds(),
			}
		}(i, c)
	}
	wg.Wait()

	if onResult != nil {
		for _, r := range results {
			onResult(r)
		}
	}

	if judge == nil {
		judge = ConsensusJudge{}
	}
	verdict, ok := judge.Judge(results)
	return results, verdict, ok
}

// normalize lowercases and collapses whitespace so trivial formatting
// differences don't defeat agreement detection.
func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// tokenSet returns the set of whitespace-delimited words in a normalized string.
func tokenSet(normalized string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range strings.Fields(normalized) {
		set[w] = struct{}{}
	}
	return set
}

// jaccard returns the Jaccard similarity of two token sets (|A∩B| / |A∪B|).
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for w := range a {
		if _, ok := b[w]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// ParseTasks parses a comma-separated task-class list (e.g. from an env var)
// into a set. Blank entries are ignored. Whitespace is trimmed.
func ParseTasks(csv string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range strings.Split(csv, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out[t] = true
		}
	}
	return out
}

// SortedTasks returns the task classes in a set, sorted — for stable logging.
func SortedTasks(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
