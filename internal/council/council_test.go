package council

import (
	"context"
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
)

// stubAdapter returns a fixed answer (or error) for every Complete call.
type stubAdapter struct {
	name   string
	answer string
	err    error
}

func (s stubAdapter) Name() string { return s.name }

func (s stubAdapter) Complete(_ context.Context, _ *provider.NormalizedModelRequest) (*openai.ChatResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &openai.ChatResponse{
		Model:   s.name,
		Choices: []openai.Choice{{Message: openai.Message{Role: "assistant", Content: s.answer}}},
		Usage:   openai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func member(id, answer string, err error) Candidate {
	return Candidate{
		ProviderID:      "p",
		ModelID:         id,
		ProviderModelID: id,
		Adapter:         stubAdapter{name: id, answer: answer, err: err},
	}
}

func runPanel(t *testing.T, panel []Candidate) ([]Result, Verdict, bool) {
	t.Helper()
	req := &provider.NormalizedModelRequest{Model: "x", Messages: []openai.Message{{Role: "user", Content: "q"}}}
	return Run(context.Background(), panel, req, ConsensusJudge{}, nil)
}

func TestConsensusMajorityWins(t *testing.T) {
	// Two models say "the migration is safe to apply", one dissents.
	panel := []Candidate{
		member("m0", "The migration is safe to apply", nil),
		member("m1", "The migration is safe to apply", nil),
		member("m2", "Do not apply; it drops a column", nil),
	}
	results, v, ok := runPanel(t, panel)
	if !ok {
		t.Fatal("expected a verdict")
	}
	if v.WinnerIndex != 0 {
		t.Fatalf("winner index = %d, want 0 (majority cluster, most preferred)", v.WinnerIndex)
	}
	if v.SuccessCount != 3 {
		t.Fatalf("success count = %d, want 3", v.SuccessCount)
	}
	if v.Agreement < 0.66 || v.Agreement > 0.67 {
		t.Fatalf("agreement = %.3f, want ~0.667", v.Agreement)
	}
	if got := results[v.WinnerIndex].Response.Choices[0].Message.Content; got != "The migration is safe to apply" {
		t.Fatalf("winner content = %q", got)
	}
}

func TestUnanimousReported(t *testing.T) {
	panel := []Candidate{
		member("m0", "Looks good, ship it", nil),
		member("m1", "Looks good, ship it", nil),
	}
	_, v, ok := runPanel(t, panel)
	if !ok {
		t.Fatal("expected a verdict")
	}
	if v.Agreement != 1.0 {
		t.Fatalf("agreement = %v, want 1.0", v.Agreement)
	}
	if v.Reason != "unanimous: all responding models agreed" {
		t.Fatalf("reason = %q", v.Reason)
	}
}

func TestNoConsensusFallsBackToTopRanked(t *testing.T) {
	// Three completely different answers → no cluster larger than one → the
	// most-preferred (index 0) member wins and low consensus is flagged.
	panel := []Candidate{
		member("m0", "alpha beta gamma", nil),
		member("m1", "delta epsilon zeta", nil),
		member("m2", "eta theta iota", nil),
	}
	_, v, ok := runPanel(t, panel)
	if !ok {
		t.Fatal("expected a verdict")
	}
	if v.WinnerIndex != 0 {
		t.Fatalf("winner = %d, want 0 (top-ranked fallback)", v.WinnerIndex)
	}
	if v.Reason != "no consensus; returned the top-ranked model's answer" {
		t.Fatalf("reason = %q", v.Reason)
	}
}

func TestFailedMembersExcluded(t *testing.T) {
	// The preferred model errors; the two healthy ones agree and one wins.
	panel := []Candidate{
		member("m0", "", errors.New("boom")),
		member("m1", "same answer here", nil),
		member("m2", "same answer here", nil),
	}
	_, v, ok := runPanel(t, panel)
	if !ok {
		t.Fatal("expected a verdict despite one failure")
	}
	if v.SuccessCount != 2 {
		t.Fatalf("success count = %d, want 2", v.SuccessCount)
	}
	if v.WinnerIndex != 1 {
		t.Fatalf("winner = %d, want 1 (first successful in agreeing cluster)", v.WinnerIndex)
	}
}

func TestAllFailNoVerdict(t *testing.T) {
	panel := []Candidate{
		member("m0", "", errors.New("boom")),
		member("m1", "", errors.New("boom")),
	}
	_, v, ok := runPanel(t, panel)
	if ok {
		t.Fatal("expected no verdict when every member fails")
	}
	if v.PanelSize != 2 {
		t.Fatalf("panel size = %d, want 2", v.PanelSize)
	}
}

func TestRunRecordsEveryMember(t *testing.T) {
	panel := []Candidate{
		member("m0", "a", nil),
		member("m1", "b", errors.New("x")),
	}
	var recorded []Result
	req := &provider.NormalizedModelRequest{Model: "x"}
	Run(context.Background(), panel, req, ConsensusJudge{}, func(r Result) {
		recorded = append(recorded, r)
	})
	if len(recorded) != 2 {
		t.Fatalf("recorded %d members, want 2 (health/spend must see all calls)", len(recorded))
	}
}

func TestParseTasks(t *testing.T) {
	set := ParseTasks(" security_review , database_migration ,, ")
	if len(set) != 2 || !set["security_review"] || !set["database_migration"] {
		t.Fatalf("parsed = %v", set)
	}
	if len(ParseTasks("")) != 0 {
		t.Fatal("empty string should parse to empty set")
	}
}
