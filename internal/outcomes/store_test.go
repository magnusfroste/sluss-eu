package outcomes_test

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/outcomes"
)

func TestOutcome_Validate(t *testing.T) {
	cases := []struct {
		name string
		o    outcomes.Outcome
		want error
	}{
		{"valid accepted", outcomes.Outcome{RequestID: "r1", Verdict: outcomes.VerdictAccepted}, nil},
		{"valid with rating", outcomes.Outcome{RequestID: "r1", Verdict: outcomes.VerdictPartial, Rating: 3}, nil},
		{"missing request id", outcomes.Outcome{Verdict: outcomes.VerdictAccepted}, outcomes.ErrMissingRequestID},
		{"bad verdict", outcomes.Outcome{RequestID: "r1", Verdict: "meh"}, outcomes.ErrInvalidVerdict},
		{"rating too high", outcomes.Outcome{RequestID: "r1", Verdict: outcomes.VerdictAccepted, Rating: 9}, outcomes.ErrInvalidRating},
		{"rating too low", outcomes.Outcome{RequestID: "r1", Verdict: outcomes.VerdictAccepted, Rating: -1}, outcomes.ErrInvalidRating},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.o.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestStore_RecordAndCount(t *testing.T) {
	s := outcomes.NewStore()
	if err := s.Record(outcomes.Outcome{RequestID: "r1", Verdict: outcomes.VerdictAccepted}); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(outcomes.Outcome{RequestID: "r2", Verdict: outcomes.VerdictRejected}); err != nil {
		t.Fatal(err)
	}
	if s.Count() != 2 {
		t.Fatalf("expected 2 outcomes, got %d", s.Count())
	}
}

func TestStore_RejectsInvalid(t *testing.T) {
	s := outcomes.NewStore()
	if err := s.Record(outcomes.Outcome{Verdict: outcomes.VerdictAccepted}); !errors.Is(err, outcomes.ErrMissingRequestID) {
		t.Fatalf("expected ErrMissingRequestID, got %v", err)
	}
	if s.Count() != 0 {
		t.Fatal("invalid outcome should not be stored")
	}
}

func TestStore_AcceptanceRate(t *testing.T) {
	s := outcomes.NewStore()
	// premium-reasoning on security_review: 2 accepted, 1 rejected, 1 partial → (2 + 0.5)/4 = 0.625
	_ = s.Record(outcomes.Outcome{RequestID: "a", Verdict: outcomes.VerdictAccepted, Model: "premium-reasoning", TaskType: "security_review"})
	_ = s.Record(outcomes.Outcome{RequestID: "b", Verdict: outcomes.VerdictAccepted, Model: "premium-reasoning", TaskType: "security_review"})
	_ = s.Record(outcomes.Outcome{RequestID: "c", Verdict: outcomes.VerdictRejected, Model: "premium-reasoning", TaskType: "security_review"})
	_ = s.Record(outcomes.Outcome{RequestID: "d", Verdict: outcomes.VerdictPartial, Model: "premium-reasoning", TaskType: "security_review"})

	rows := s.Acceptance("")
	if len(rows) != 1 {
		t.Fatalf("expected 1 group, got %d", len(rows))
	}
	row := rows[0]
	if row.Total != 4 || row.Accepted != 2 || row.Rejected != 1 || row.Partial != 1 {
		t.Fatalf("unexpected counts: %+v", row)
	}
	if row.AcceptanceRate < 0.62 || row.AcceptanceRate > 0.63 {
		t.Fatalf("expected acceptance ~0.625, got %.3f", row.AcceptanceRate)
	}
}

func TestStore_AcceptanceFilterByTask(t *testing.T) {
	s := outcomes.NewStore()
	_ = s.Record(outcomes.Outcome{RequestID: "a", Verdict: outcomes.VerdictAccepted, Model: "cheap-general", TaskType: "summarization"})
	_ = s.Record(outcomes.Outcome{RequestID: "b", Verdict: outcomes.VerdictRejected, Model: "premium-reasoning", TaskType: "security_review"})

	rows := s.Acceptance("summarization")
	if len(rows) != 1 {
		t.Fatalf("expected 1 filtered group, got %d", len(rows))
	}
	if rows[0].TaskType != "summarization" {
		t.Fatalf("expected summarization, got %s", rows[0].TaskType)
	}
}

func TestStore_TaskTypesSorted(t *testing.T) {
	s := outcomes.NewStore()
	_ = s.Record(outcomes.Outcome{RequestID: "a", Verdict: outcomes.VerdictAccepted, TaskType: "summarization"})
	_ = s.Record(outcomes.Outcome{RequestID: "b", Verdict: outcomes.VerdictAccepted, TaskType: "security_review"})
	_ = s.Record(outcomes.Outcome{RequestID: "c", Verdict: outcomes.VerdictAccepted, TaskType: "summarization"})
	types := s.TaskTypes()
	if len(types) != 2 || types[0] != "security_review" || types[1] != "summarization" {
		t.Fatalf("expected sorted distinct types, got %v", types)
	}
}
