package main

import "testing"

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
