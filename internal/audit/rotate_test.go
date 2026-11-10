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

func write(t *testing.T, s *ChainSink, n int, action string) {
	t.Helper()
	for i := 0; i < n; i++ {
		s.Record(context.Background(), Entry{Time: time.Now(), Action: Action(action), Outcome: "ok"})
	}
}

func export(t *testing.T, s *ChainSink) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := s.ExportTo(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Rotation moves the file to the archive, keeps every record, and the whole
// chain still verifies from record 1.
func TestRotateKeepsOneContinuousChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-chain.jsonl")
	s, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	write(t, s, 5, "test.before")
	seg, err := s.Rotate("admin", "test")
	if err != nil {
		t.Fatal(err)
	}
	if seg.FirstSeq != 1 || seg.LastSeq != 6 {
		t.Fatalf("segment %+v, want 1..6 (five records and the seal)", seg)
	}
	write(t, s, 3, "test.after")

	// Nothing was deleted: the sealed file is in the archive, read-only.
	sealed := filepath.Join(dir, ArchiveDirName, seg.Name)
	st, err := os.Stat(sealed)
	if err != nil {
		t.Fatalf("sealed file missing: %v", err)
	}
	if st.Mode().Perm()&0o222 != 0 {
		t.Errorf("sealed file is writable: %v", st.Mode())
	}
	// 5 + seal + continued + 3.
	all := export(t, s)
	if n, err := VerifyChain(bytes.NewReader(all)); err != nil || n != 10 {
		t.Fatalf("whole chain: %d records, err %v", n, err)
	}
	// Each file verifies on its own, too.
	sealedBytes, _ := os.ReadFile(sealed)
	if n, err := VerifyChain(bytes.NewReader(sealedBytes)); err != nil || n != 6 {
		t.Fatalf("sealed file alone: %d, %v", n, err)
	}
	current, _ := os.ReadFile(path)
	if n, err := VerifyChain(bytes.NewReader(current)); err != nil || n != 4 {
		t.Fatalf("current file alone: %d, %v", n, err)
	}
	// The record after the seal pins the sealed file.
	var first ChainRecord
	_ = json.Unmarshal(bytes.SplitN(current, []byte("\n"), 2)[0], &first)
	if err := VerifySealed(sealedBytes, first); err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(sealedBytes, []byte("test.before"), []byte("test.BEFORE"), 1)
	if err := VerifySealed(tampered, first); err == nil {
		t.Fatal("a changed sealed file was accepted")
	}
	// Head and segments.
	if h, ok := s.Head(); !ok || h.Seq != 10 {
		t.Fatalf("head %+v", h)
	}
	segs, _ := s.Segments()
	if len(segs) != 1 || segs[0].SHA256 != seg.SHA256 || segs[0].SHA256 != first.Detail["previous_sha256"] {
		t.Fatalf("segments %+v", segs)
	}
}

// A restart after rotation continues the same chain; so does a restart after
// a rotation that was interrupted before the new file got its first record.
func TestRotationSurvivesRestartAndInterruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-chain.jsonl")
	s, _ := NewChainSink(path, nil)
	write(t, s, 4, "a")
	if _, err := s.Rotate("admin", ""); err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Interrupted: the new file never got its record.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s2, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	write(t, s2, 2, "b")
	all := export(t, s2)
	// 4 + seal + continued (written at start) + 2.
	if n, err := VerifyChain(bytes.NewReader(all)); err != nil || n != 8 {
		t.Fatalf("after restart: %d records, err %v\n%s", n, err, all)
	}
	if strings.Count(string(all), string(ActionAuditContinued)) != 1 {
		t.Fatalf("want exactly one continued record:\n%s", all)
	}
}

// Rotation by size happens by itself and can happen many times.
func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewChainSink(filepath.Join(dir, "audit-chain.jsonl"), nil)
	defer s.Close()
	s.SetRotateBytes(600)
	write(t, s, 40, "sized")
	segs, _ := s.Segments()
	if len(segs) < 3 {
		t.Fatalf("want several sealed files, got %d", len(segs))
	}
	all := export(t, s)
	n, err := VerifyChain(bytes.NewReader(all))
	if err != nil {
		t.Fatalf("chain broken after %d: %v", n, err)
	}
	if got := strings.Count(string(all), `"action":"sized"`); got != 40 {
		t.Fatalf("%d of 40 records survived", got)
	}
}

// Two sinks on one file, as during an overlapping deploy: one rotates, the
// other keeps writing, and the chain does not fork.
func TestRotateWithTwoWriters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-chain.jsonl")
	a, _ := NewChainSink(path, nil)
	b, _ := NewChainSink(path, nil)
	defer a.Close()
	defer b.Close()
	write(t, a, 2, "a")
	write(t, b, 2, "b")
	if _, err := a.Rotate("admin", ""); err != nil {
		t.Fatal(err)
	}
	write(t, b, 2, "b") // b still holds the moved file
	write(t, a, 2, "a")
	all := export(t, a)
	if n, err := VerifyChain(bytes.NewReader(all)); err != nil || n != 10 {
		t.Fatalf("%d records, err %v\n%s", n, err, all)
	}
}

func TestRotateEmptyChainIsRefused(t *testing.T) {
	s, _ := NewChainSink(filepath.Join(t.TempDir(), "audit-chain.jsonl"), nil)
	defer s.Close()
	if _, err := s.Rotate("admin", ""); err == nil {
		t.Fatal("an empty chain was sealed")
	}
}
