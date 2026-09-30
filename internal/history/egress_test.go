package history

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// ISSUE-116: egress is stored at decision time and corrected by the attempt
// that actually answered (a fallback can change it); blocked rows have none.
func TestEgressStoredAndCorrectedByAnsweringAttempt(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	dec := func(id, sens, egress string, blocked bool) {
		s.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, SelectedModel: "m-" + egress, Sensitivity: sens, Egress: egress, Blocked: blocked}})
	}
	dec("r1", "pii", "local", false)
	dec("r2", "none", "local", false) // primary local, answered by a cloud fallback
	s.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		RequestID: "r2", ModelID: "cloud-fb", Success: true, Egress: "cloud"}})
	dec("r3", "source_code", "", true)

	got := map[string]string{}
	for _, r := range s.Recent(10) {
		got[r.RequestID] = r.Egress
	}
	if got["r1"] != "local" || got["r2"] != "cloud" || got["r3"] != "" {
		t.Fatalf("egress per request = %v", got)
	}

	var local, cloud, blocked int
	for _, r := range s.EgressRows() {
		switch {
		case r.Blocked:
			blocked += r.Count
		case r.Egress == "local":
			local += r.Count
		case r.Egress == "cloud":
			cloud += r.Count
		}
	}
	if local != 1 || cloud != 1 || blocked != 1 {
		t.Fatalf("EgressRows local=%d cloud=%d blocked=%d", local, cloud, blocked)
	}
}
