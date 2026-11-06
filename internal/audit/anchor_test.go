package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func chainWith(t *testing.T, n int) (*ChainSink, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit-chain.jsonl")
	s, err := NewChainSink(path, nil)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	for i := 0; i < n; i++ {
		s.Record(context.Background(), Entry{Action: Action("test"), Actor: "t", Outcome: "success", Time: time.Now()})
	}
	return s, path
}

func TestHeadAndAnchorLine(t *testing.T) {
	s, path := chainWith(t, 3)
	a, ok := s.Head()
	if !ok || a.Seq != 3 || len(a.EntryHash) != 64 {
		t.Fatalf("head: %+v ok=%v", a, ok)
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.RunAnchoring(ctx, time.Hour, logger); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	line := buf.String()
	if !strings.Contains(line, `"msg":"audit anchor"`) || !strings.Contains(line, `"seq":3`) || !strings.Contains(line, a.EntryHash) {
		t.Fatalf("anchor line missing or wrong: %s", line)
	}
	_ = path
}

func TestVerifyAnchor(t *testing.T) {
	s, path := chainWith(t, 5)
	a, _ := s.Head()
	data, _ := os.ReadFile(path)

	// Intact export, anchor matches.
	if n, err := VerifyAnchor(bytes.NewReader(data), a.Seq, a.EntryHash); err != nil || n != 5 {
		t.Fatalf("intact: n=%d err=%v", n, err)
	}
	// More records after the anchor are fine: the anchor pins a prefix.
	s.Record(context.Background(), Entry{Action: Action("later"), Actor: "t", Outcome: "success", Time: time.Now()})
	data2, _ := os.ReadFile(path)
	if n, err := VerifyAnchor(bytes.NewReader(data2), a.Seq, a.EntryHash); err != nil || n != 6 {
		t.Fatalf("prefix: n=%d err=%v", n, err)
	}
	// A rewritten chain: change record 2 and recompute every hash after it,
	// which VerifyChain alone cannot see. The anchor can.
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var recs []ChainRecord
	for _, l := range lines {
		var r ChainRecord
		_ = json.Unmarshal([]byte(l), &r)
		recs = append(recs, r)
	}
	recs[1].Actor = "someone-else"
	prev := recs[0].EntryHash
	var rewritten bytes.Buffer
	rewritten.WriteString(lines[0] + "\n")
	for i := 1; i < len(recs); i++ {
		recs[i].PrevHash = prev
		h, _ := hashRecord(recs[i])
		recs[i].EntryHash = h
		prev = h
		b, _ := json.Marshal(recs[i])
		rewritten.Write(append(b, '\n'))
	}
	if _, err := VerifyChain(bytes.NewReader(rewritten.Bytes())); err != nil {
		t.Fatalf("a recomputed chain passes the plain check by design; got %v", err)
	}
	if _, err := VerifyAnchor(bytes.NewReader(rewritten.Bytes()), a.Seq, a.EntryHash); err == nil || !strings.Contains(err.Error(), "rewritten after the anchor") {
		t.Fatalf("the anchor must catch a recomputed chain, got %v", err)
	}
	// A truncated export: records after seq 3 are gone.
	short := strings.Join(lines[:3], "\n") + "\n"
	if _, err := VerifyAnchor(strings.NewReader(short), a.Seq, a.EntryHash); err == nil || !strings.Contains(err.Error(), "records are missing") {
		t.Fatalf("a truncated export must fail the anchor, got %v", err)
	}
}
