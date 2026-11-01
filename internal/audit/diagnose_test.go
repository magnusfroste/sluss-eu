package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A fork from two writers is told apart from a changed or removed record.
func TestDiagnoseChain(t *testing.T) {
	// Build a forked file the way the old writer did: two sinks that each
	// remember their own tail (no locking), simulated by writing lines by hand.
	path := filepath.Join(t.TempDir(), "c.jsonl")
	a, _ := NewChainSink(path, nil)
	record(a, "one", "")
	record(a, "two", "")
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	// fork: a third record that continues from record 1 instead of record 2
	recs := []ChainRecord{}
	for _, l := range lines {
		var r ChainRecord
		_ = jsonUnmarshal([]byte(l), &r)
		recs = append(recs, r)
	}
	fork := ChainRecord{Seq: 3, Action: "three", PrevHash: recs[0].EntryHash, Time: recs[1].Time}
	fork.EntryHash, _ = hashRecord(fork)
	forked := append(append([]byte{}, data...), mustLine(t, fork)...)
	d, err := DiagnoseChain(bytes.NewReader(forked))
	if err != nil || len(d.Forks) != 1 || !d.Explained() {
		t.Fatalf("fork not explained: %+v %v", d, err)
	}

	// changed: edit the action of record 1 without fixing its hash
	changed := strings.Replace(string(data), `"action":"one"`, `"action":"ONE"`, 1)
	d, _ = DiagnoseChain(strings.NewReader(changed))
	if len(d.Modified) != 1 || d.Explained() {
		t.Fatalf("change not reported: %+v", d)
	}

	// removed: drop record 1, so record 2 links to a hash not in the file
	d, _ = DiagnoseChain(strings.NewReader(lines[1] + "\n"))
	if len(d.Unknown) != 1 || len(d.Missing) != 1 || d.Explained() {
		t.Fatalf("removal not reported: %+v", d)
	}
}

func mustLine(t *testing.T, r ChainRecord) []byte {
	t.Helper()
	b, err := jsonMarshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

var (
	jsonMarshal   = json.Marshal
	jsonUnmarshal = json.Unmarshal
)
