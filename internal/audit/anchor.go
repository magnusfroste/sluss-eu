package audit

// Anchoring the chain (ISSUE-160). A hash chain proves that nothing inside
// the file was changed or removed without the links breaking. It cannot, by
// itself, prove that the whole file was not rewritten from a chosen point by
// someone with write access to the disk, because they could recompute every
// hash after that point. The answer is to put the chain's head somewhere that
// person cannot reach: an anchor, which is the sequence number and entry
// hash of the last record, written to the log at a fixed interval. The log
// goes where the operator's logs go (a SIEM, a log service, another machine),
// and an auditor compares an export against any anchor from before: if record
// N in the export does not carry the anchored hash, the file was rewritten
// after that anchor was taken.
//
// No key, no signature, no service to run: the operator already ships logs.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// Anchor is the head of the chain at one moment: enough to pin every record
// up to and including Seq.
type Anchor struct {
	Seq       uint64    `json:"seq"`
	EntryHash string    `json:"entry_hash"`
	Time      time.Time `json:"time"`
}

// Head returns the chain's current anchor, read from the file under the
// append lock so it is never a half-written record. ok is false when the
// chain is empty.
func (s *ChainSink) Head() (Anchor, bool) {
	if s == nil {
		return Anchor{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seq, hash, ok := lastRecord(s.path)
	if !ok {
		return Anchor{}, false
	}
	return Anchor{Seq: seq, EntryHash: hash, Time: time.Now().UTC()}, true
}

// AnchorLog is the message of the anchor line, so a log search finds it.
const AnchorLog = "audit anchor"

// RunAnchoring writes the chain's head to the log now and then every
// interval, until ctx ends. The line carries seq and entry_hash; an operator
// who keeps the log elsewhere keeps the proof. An empty chain logs nothing.
func (s *ChainSink) RunAnchoring(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	if s == nil || interval <= 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	emit := func() {
		if a, ok := s.Head(); ok {
			logger.Info(AnchorLog, "seq", a.Seq, "entry_hash", a.EntryHash, "file", s.path)
		}
	}
	emit()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			emit()
		}
	}
}

// VerifyAnchor checks an export against an anchor: the chain must be intact
// through record seq, and that record's entry_hash must equal the anchored
// hash. It returns the number of valid records in the whole export. A chain
// that is intact but whose record seq carries another hash was rewritten
// after the anchor was taken; a chain shorter than seq lost records.
func VerifyAnchor(r io.Reader, seq uint64, entryHash string) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var (
		count    int
		prevSeq  uint64
		prevHash string
		found    bool
	)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var rec ChainRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return count, fmt.Errorf("seq %d: unparseable record: %w", prevSeq+1, err)
		}
		if rec.Seq != prevSeq+1 || rec.PrevHash != prevHash {
			return count, fmt.Errorf("chain broken at seq %d, before the anchor could be checked", rec.Seq)
		}
		want, err := hashRecord(rec)
		if err != nil || rec.EntryHash != want {
			return count, fmt.Errorf("seq %d: record modified, before the anchor could be checked", rec.Seq)
		}
		if rec.Seq == seq {
			found = true
			if rec.EntryHash != entryHash {
				return count, fmt.Errorf("seq %d carries hash %s, the anchor says %s: the chain was rewritten after the anchor was taken", seq, short(rec.EntryHash), short(entryHash))
			}
		}
		prevSeq, prevHash = rec.Seq, rec.EntryHash
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read: %w", err)
	}
	if !found {
		return count, fmt.Errorf("the export ends at seq %d, before the anchored seq %d: records are missing", prevSeq, seq)
	}
	return count, nil
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12] + "…"
	}
	return h
}
