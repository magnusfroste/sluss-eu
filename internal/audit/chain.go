package audit

// Tamper-evident audit chain (ISSUE-075): every audit entry is appended to a
// JSONL file where each record carries the SHA-256 of its own canonical JSON
// and the hash of the previous record. The chain makes the exported trail
// verifiable offline: any edited, removed, or inserted line breaks the chain at
// a precise sequence number. This is tamper-EVIDENCE, not tamper-prevention —
// the guarantee an auditor needs from a handed-over artifact.
//
// Like every audit sink, the chain must never affect the request path: append
// failures are logged and dropped, never propagated.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChainRecord is one line in the audit chain file. Field order is part of the
// canonical form: EntryHash is computed over the record serialized WITHOUT the
// entry_hash field (see canonicalBytes). encoding/json sorts map keys, so
// Detail serializes deterministically.
type ChainRecord struct {
	Seq       uint64            `json:"seq"`
	Time      time.Time         `json:"time"`
	Action    string            `json:"action"`
	Actor     string            `json:"actor,omitempty"`
	TenantID  string            `json:"tenant_id,omitempty"`
	ProjectID string            `json:"project_id,omitempty"`
	Target    string            `json:"target,omitempty"`
	Outcome   string            `json:"outcome"`
	RequestID string            `json:"request_id,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Detail    map[string]string `json:"detail,omitempty"`
	PrevHash  string            `json:"prev_hash"`
	EntryHash string            `json:"entry_hash"`
}

// canonicalBytes serializes the record with EntryHash blanked — the byte form
// the hash commits to.
func canonicalBytes(r ChainRecord) ([]byte, error) {
	r.EntryHash = ""
	return json.Marshal(r)
}

func hashRecord(r ChainRecord) (string, error) {
	b, err := canonicalBytes(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// ChainSink appends hash-chained audit records to a JSONL file. Safe for
// concurrent use. Create with NewChainSink.
type ChainSink struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	seq      uint64
	prevHash string
	logger   *slog.Logger
}

// NewChainSink opens (creating if needed) the chain file and resumes the chain
// from the last valid record, so restarts extend one continuous chain.
func NewChainSink(path string, logger *slog.Logger) (*ChainSink, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("audit chain: create dir: %w", err)
		}
	}
	s := &ChainSink{path: path, logger: logger}
	// Resume: scan the existing file for the last record.
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var rec ChainRecord
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				f.Close()
				return nil, fmt.Errorf("audit chain: corrupt record after seq %d — refusing to extend a broken chain (move the file aside to start fresh): %w", s.seq, err)
			}
			s.seq = rec.Seq
			s.prevHash = rec.EntryHash
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("audit chain: scan existing file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("audit chain: open existing file: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audit chain: open for append: %w", err)
	}
	s.file = f
	return s, nil
}

// Record appends the entry to the chain. Errors are logged, never propagated —
// the audit contract is that recording can never affect the request path.
func (s *ChainSink) Record(_ context.Context, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := ChainRecord{
		Seq:       s.seq + 1,
		Time:      e.Time.UTC(),
		Action:    string(e.Action),
		Actor:     e.Actor,
		TenantID:  e.TenantID,
		ProjectID: e.ProjectID,
		Target:    e.Target,
		Outcome:   e.Outcome,
		RequestID: e.RequestID,
		Reason:    e.Reason,
		Detail:    e.Detail,
		PrevHash:  s.prevHash,
	}
	hash, err := hashRecord(rec)
	if err != nil {
		s.logf("audit chain: hash failed", err)
		return
	}
	rec.EntryHash = hash
	line, err := json.Marshal(rec)
	if err != nil {
		s.logf("audit chain: marshal failed", err)
		return
	}
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		s.logf("audit chain: append failed", err)
		return
	}
	s.seq = rec.Seq
	s.prevHash = rec.EntryHash
}

func (s *ChainSink) logf(msg string, err error) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(msg, "err", err, "path", s.path)
}

// Path returns the chain file path.
func (s *ChainSink) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// ExportTo streams the current chain file to w while holding the append lock,
// so the export is a consistent prefix of the chain.
func (s *ChainSink) ExportTo(w io.Writer) error {
	if s == nil {
		return fmt.Errorf("audit chain: not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("audit chain: sync before export: %w", err)
	}
	f, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("audit chain: open for export: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// Close releases the underlying file.
func (s *ChainSink) Close() error {
	if s == nil || s.file == nil {
		return nil
	}
	return s.file.Close()
}

// VerifyChain reads a chain (JSONL) and verifies every record: its own hash,
// the link to the previous record, and monotonically increasing sequence
// numbers. It returns the number of valid records, or an error naming the
// first broken sequence number.
func VerifyChain(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var (
		count    int
		prevSeq  uint64
		prevHash string
	)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var rec ChainRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return count, fmt.Errorf("line %d: unparseable record: %w", line, err)
		}
		if rec.Seq != prevSeq+1 {
			return count, fmt.Errorf("line %d: sequence %d does not follow %d (record removed or inserted)", line, rec.Seq, prevSeq)
		}
		if rec.PrevHash != prevHash {
			return count, fmt.Errorf("line %d (seq %d): prev_hash mismatch (chain broken)", line, rec.Seq)
		}
		want, err := hashRecord(rec)
		if err != nil {
			return count, fmt.Errorf("line %d (seq %d): hash: %w", line, rec.Seq, err)
		}
		if rec.EntryHash != want {
			return count, fmt.Errorf("line %d (seq %d): entry_hash mismatch (record modified)", line, rec.Seq)
		}
		prevSeq = rec.Seq
		prevHash = rec.EntryHash
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read: %w", err)
	}
	return count, nil
}
