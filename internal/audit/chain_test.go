package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestChain(t *testing.T) (*ChainSink, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit-chain.jsonl")
	s, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatalf("NewChainSink: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func record(s *ChainSink, action Action, target string) {
	Record(context.Background(), s, Entry{
		Time:   time.Unix(1_700_000_000, 0),
		Action: action,
		Actor:  "tn_test",
		Target: target,
		Detail: map[string]string{"b": "2", "a": "1"},
	})
}

func exportBytes(t *testing.T, s *ChainSink) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := s.ExportTo(&buf); err != nil {
		t.Fatalf("export: %v", err)
	}
	return buf.Bytes()
}

func TestChainAppendsAndVerifies(t *testing.T) {
	s, _ := newTestChain(t)
	record(s, ActionPolicyReload, "pv1")
	record(s, ActionRequestBlocked, "model-x")
	record(s, ActionAPIKeyAdd, "key_1")

	data := exportBytes(t, s)
	n, err := VerifyChain(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n != 3 {
		t.Fatalf("verified %d records, want 3", n)
	}
	// Chain structure: seq 1..3, first prev_hash empty, links intact.
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var first, second ChainRecord
	_ = json.Unmarshal([]byte(lines[0]), &first)
	_ = json.Unmarshal([]byte(lines[1]), &second)
	if first.Seq != 1 || first.PrevHash != "" {
		t.Fatalf("genesis record wrong: %+v", first)
	}
	if second.PrevHash != first.EntryHash {
		t.Fatal("second record does not link to first")
	}
}

func TestVerifyDetectsModifiedRecord(t *testing.T) {
	s, _ := newTestChain(t)
	record(s, ActionPolicyReload, "pv1")
	record(s, ActionAPIKeyAdd, "key_1")
	data := string(exportBytes(t, s))

	tampered := strings.Replace(data, "key_1", "key_EVIL", 1)
	if _, err := VerifyChain(strings.NewReader(tampered)); err == nil {
		t.Fatal("modified record must break verification")
	} else if !strings.Contains(err.Error(), "modified") {
		t.Fatalf("want 'modified' error, got: %v", err)
	}
}

func TestVerifyDetectsRemovedRecord(t *testing.T) {
	s, _ := newTestChain(t)
	record(s, ActionPolicyReload, "pv1")
	record(s, ActionAPIKeyAdd, "key_1")
	record(s, ActionAPIKeyDisable, "key_1")
	lines := strings.Split(strings.TrimSpace(string(exportBytes(t, s))), "\n")

	// Drop the middle record.
	tampered := lines[0] + "\n" + lines[2] + "\n"
	if _, err := VerifyChain(strings.NewReader(tampered)); err == nil {
		t.Fatal("removed record must break verification")
	}
}

func TestVerifyDetectsInsertedRecord(t *testing.T) {
	s, _ := newTestChain(t)
	record(s, ActionPolicyReload, "pv1")
	record(s, ActionAPIKeyAdd, "key_1")
	lines := strings.Split(strings.TrimSpace(string(exportBytes(t, s))), "\n")

	// Forge a plausible record (valid self-hash but wrong link) and insert it.
	forged := ChainRecord{Seq: 2, Time: time.Unix(1_700_000_000, 0).UTC(),
		Action: string(ActionAPIKeyAdd), Actor: "attacker", Target: "key_forged",
		Outcome: OutcomeSuccess, PrevHash: "0000"}
	forged.EntryHash, _ = hashRecord(forged)
	fb, _ := json.Marshal(forged)
	tampered := lines[0] + "\n" + string(fb) + "\n" + lines[1] + "\n"
	if _, err := VerifyChain(strings.NewReader(tampered)); err == nil {
		t.Fatal("inserted record must break verification")
	}
}

func TestChainResumesAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-chain.jsonl")
	s1, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	record(s1, ActionPolicyReload, "pv1")
	s1.Close()

	s2, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatalf("open 2 (resume): %v", err)
	}
	defer s2.Close()
	record(s2, ActionAPIKeyAdd, "key_1")

	data := exportBytes(t, s2)
	n, err := VerifyChain(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("resumed chain must verify: %v", err)
	}
	if n != 2 {
		t.Fatalf("verified %d, want 2 (one per process)", n)
	}
}

func TestNewChainSinkRefusesCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-chain.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewChainSink(path, nil); err == nil {
		t.Fatal("corrupt existing chain must refuse to open (never extend a broken chain)")
	}
}

func TestVerifyEmptyChainIsValid(t *testing.T) {
	n, err := VerifyChain(strings.NewReader(""))
	if err != nil || n != 0 {
		t.Fatalf("empty chain: n=%d err=%v, want 0,nil", n, err)
	}
}
