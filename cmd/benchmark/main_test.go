package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/secondopinion"
)

func TestScore(t *testing.T) {
	cases := []benchCase{
		{ID: "1", Labels: []string{"pii"}, Kind: "identifier"},
		{ID: "2", Labels: []string{"health"}, Kind: "meaning"},
		{ID: "3", Labels: []string{}, Kind: "lookalike"},
		{ID: "4", Labels: []string{}, Kind: "plain"},
	}
	preds := map[string]prediction{
		"1": {ID: "1", Flagged: true, Classes: []string{"pii"}, Runs: 3},
		"2": {ID: "2"},
		"3": {ID: "3", Flagged: true, Classes: []string{"pii"}},
		"4": {ID: "4"},
	}
	r := score(cases, preds)
	if r.sensitive != 2 || r.caught != 1 {
		t.Fatalf("caught %d of %d, want 1 of 2", r.caught, r.sensitive)
	}
	if r.harmless != 2 || r.overflagged != 1 {
		t.Fatalf("harmless flagged %d of %d, want 1 of 2", r.overflagged, r.harmless)
	}
	if r.exact != 2 {
		t.Fatalf("exact %d, want 2 (cases 1 and 4)", r.exact)
	}
	if p := r.perClass["pii"]; p.tp != 1 || p.fp != 1 || p.fn != 0 {
		t.Fatalf("pii tally %+v", *p)
	}
	if k := r.perKind["meaning"]; k != [2]int{0, 1} {
		t.Fatalf("meaning %v", k)
	}
	if !r.repeated {
		t.Fatal("repeated not detected")
	}
}

func TestLocalRulesAreDeterministicAndFindAnIdentifier(t *testing.T) {
	a, _ := classifyLocal("Mejla anna.svensson@example.se om fakturan.")
	b, _ := classifyLocal("Mejla anna.svensson@example.se om fakturan.")
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("classes %v / %v", a, b)
	}
}

func TestSecondOpinionClasses(t *testing.T) {
	got := secondOpinionClasses([]string{"Policy rule r1 matched", "second opinion (local model m): pii, health — may only make the decision stricter"})
	if len(got) != 2 || got[0] != "pii" || got[1] != "health" {
		t.Fatalf("got %v", got)
	}
}

// ISSUE-170: a recording of the second opinion is replayed on top of the
// rules, raise only, and is refused when it no longer matches.
func TestReplayRecording(t *testing.T) {
	cases := []benchCase{
		{ID: "1", Text: "Plan a team lunch for nine people."},
		{ID: "2", Text: "Mejla anna.svensson@example.se om fakturan."},
	}
	write := func(h recordingHeader, answers ...recordedAnswer) string {
		path := filepath.Join(t.TempDir(), "rec.jsonl")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		_ = enc.Encode(h)
		for _, a := range answers {
			_ = enc.Encode(a)
		}
		return path
	}
	good := recordingHeader{Recording: "second-opinion", Model: "m", PromptHash: secondopinion.PromptHash()}
	a1 := recordedAnswer{ID: "1", Text: textHash(cases[0].Text), Labels: []string{"health"}}
	a2 := recordedAnswer{ID: "2", Text: textHash(cases[1].Text), Labels: []string{}}

	classify := replayClassifier(cases, write(good, a1, a2))
	if got, _ := classify(cases[0].Text); len(got) != 1 || got[0] != "health" {
		t.Fatalf("the model's class was not added: %v", got)
	}
	if got, _ := classify(cases[1].Text); len(got) == 0 {
		t.Fatalf("the rules' class was lost: %v", got)
	}

	stale := good
	stale.PromptHash = "0000000000000000"
	if _, _, err := loadRecording(cases, write(stale, a1, a2)); err == nil {
		t.Fatal("a recording made with another instruction was accepted")
	}
	moved := a1
	moved.Text = textHash("another question")
	if _, _, err := loadRecording(cases, write(good, moved, a2)); err == nil {
		t.Fatal("a recording of other questions was accepted")
	}
	if _, _, err := loadRecording(cases, write(good, a1)); err == nil {
		t.Fatal("a recording with a question missing was accepted")
	}
}
