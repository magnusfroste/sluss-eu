package audit

// Sealed rotation (ISSUE-171). The chain file grows for ever; rotation closes
// it without breaking the chain and without deleting anything.
//
//   - The last record of the file being closed is "audit.sealed".
//   - The file is moved to the archive directory beside it, named after the
//     sequence numbers it holds, and made read-only. It is never deleted.
//   - The first record of the new file is "audit.continued". Its sequence
//     number follows the seal's, its prev_hash is the seal's entry_hash, and
//     it carries the SHA-256 of the whole archived file.
//
// So the files read one after another are one continuous chain, exactly as
// before rotation; and one archived file on its own is pinned by the hash in
// the record that follows it, which is inside the chain.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ActionAuditSealed    Action = "audit.sealed"
	ActionAuditContinued Action = "audit.continued"

	// ArchiveDirName is the directory, beside the chain file, that holds the
	// sealed files.
	ArchiveDirName = "audit-archive"
)

// Segment describes one sealed file.
type Segment struct {
	Name     string `json:"name"`
	FirstSeq uint64 `json:"first_seq"`
	LastSeq  uint64 `json:"last_seq"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}

func (s *ChainSink) archiveDir() string {
	return filepath.Join(filepath.Dir(s.path), ArchiveDirName)
}

// SetRotateBytes makes the sink seal and archive the file once it is at least
// n bytes. Zero turns automatic rotation off.
func (s *ChainSink) SetRotateBytes(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.rotateBytes = n
	s.mu.Unlock()
}

// Rotate seals the current file, moves it to the archive and continues the
// chain in a new file. An empty chain is left alone.
func (s *ChainSink) Rotate(actor, reason string) (Segment, error) {
	if s == nil {
		return Segment{}, fmt.Errorf("audit chain: not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock := lockFile(s.file)
	seg, err := s.rotateLocked(actor, reason)
	unlock()
	return seg, err
}

// rotateLocked does the work. The caller holds s.mu and the file lock.
func (s *ChainSink) rotateLocked(actor, reason string) (Segment, error) {
	s.resyncLocked()
	if s.seq == 0 {
		return Segment{}, fmt.Errorf("audit chain: nothing to seal")
	}
	first := firstSeq(s.path)
	if first == 0 {
		first = 1
	}
	if err := os.MkdirAll(s.archiveDir(), 0o755); err != nil {
		return Segment{}, fmt.Errorf("audit chain: create archive dir: %w", err)
	}
	// 1. Seal.
	sealSeq := s.seq + 1
	name := fmt.Sprintf("audit-chain-%09d-%09d.jsonl", first, sealSeq)
	dest := filepath.Join(s.archiveDir(), name)
	if _, err := os.Stat(dest); err == nil {
		return Segment{}, fmt.Errorf("audit chain: %s already exists in the archive; nothing was moved", name)
	}
	if err := s.appendLocked(Entry{
		Time: time.Now(), Action: ActionAuditSealed, Actor: actor, Target: name, Outcome: "ok", Reason: reason,
		Detail: map[string]string{"first_seq": strconv.FormatUint(first, 10), "last_seq": strconv.FormatUint(sealSeq, 10)},
	}); err != nil {
		return Segment{}, err
	}
	if err := s.file.Sync(); err != nil {
		return Segment{}, fmt.Errorf("audit chain: sync before sealing: %w", err)
	}
	sum, size, err := fileSHA256(s.path)
	if err != nil {
		return Segment{}, fmt.Errorf("audit chain: hash the sealed file: %w", err)
	}
	// 2. Move. A rename keeps the bytes; nothing is deleted.
	if err := os.Rename(s.path, dest); err != nil {
		return Segment{}, fmt.Errorf("audit chain: move to archive: %w", err)
	}
	_ = os.Chmod(dest, 0o444)
	seg := Segment{Name: name, FirstSeq: first, LastSeq: sealSeq, Bytes: size, SHA256: sum}
	// 3. Continue in a new file.
	old := s.file
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// The sealed file is safe in the archive; keep appending to it
		// through the open handle would be wrong, so report and stop.
		return seg, fmt.Errorf("audit chain: open the new file: %w", err)
	}
	// Hold the new file before letting go of the old one, so another
	// process cannot write between the seal and the record that follows it.
	relock := lockFile(f)
	defer relock()
	s.file = f
	_ = old.Close()
	if err := s.appendLocked(continuedEntry(seg, actor)); err != nil {
		return seg, err
	}
	return seg, nil
}

func continuedEntry(seg Segment, actor string) Entry {
	return Entry{
		Time: time.Now(), Action: ActionAuditContinued, Actor: actor, Target: seg.Name, Outcome: "ok",
		Detail: map[string]string{
			"previous_file":     seg.Name,
			"previous_last_seq": strconv.FormatUint(seg.LastSeq, 10),
			"previous_sha256":   seg.SHA256,
			"previous_bytes":    strconv.FormatInt(seg.Bytes, 10),
		},
	}
}

// appendLocked writes one record after the current head. The caller holds
// s.mu and the file lock.
func (s *ChainSink) appendLocked(e Entry) error {
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
		return fmt.Errorf("audit chain: hash: %w", err)
	}
	rec.EntryHash = hash
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("audit chain: marshal: %w", err)
	}
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("audit chain: append: %w", err)
	}
	s.seq = rec.Seq
	s.prevHash = rec.EntryHash
	return nil
}

// resyncLocked makes the sink agree with what is on disk: another process
// may have appended, or rotated the file away under this one. The caller
// holds s.mu.
func (s *ChainSink) resyncLocked() {
	// Rotated by another process: the handle points at the archived file.
	if cur, err := os.Stat(s.path); err != nil || !sameFile(s.file, cur) {
		if f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			_ = s.file.Close()
			s.file = f
		}
	}
	if seq, prev, ok := lastRecord(s.path); ok {
		s.seq, s.prevHash = seq, prev
		return
	}
	// The current file is empty: continue from the newest sealed file.
	if segs, err := s.segmentsLocked(false); err == nil && len(segs) > 0 {
		last := segs[len(segs)-1]
		if seq, prev, ok := lastRecord(filepath.Join(s.archiveDir(), last.Name)); ok {
			s.seq, s.prevHash = seq, prev
		}
	}
}

func sameFile(f *os.File, st os.FileInfo) bool {
	if f == nil || st == nil {
		return false
	}
	fst, err := f.Stat()
	return err == nil && os.SameFile(fst, st)
}

// Segments lists the sealed files, oldest first, with their hashes.
func (s *ChainSink) Segments() ([]Segment, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.segmentsLocked(true)
}

func (s *ChainSink) segmentsLocked(withHash bool) ([]Segment, error) {
	entries, err := os.ReadDir(s.archiveDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Segment
	for _, e := range entries {
		first, last, ok := parseSegmentName(e.Name())
		if !ok || e.IsDir() {
			continue
		}
		seg := Segment{Name: e.Name(), FirstSeq: first, LastSeq: last}
		if info, err := e.Info(); err == nil {
			seg.Bytes = info.Size()
		}
		if withHash {
			if sum, _, err := fileSHA256(filepath.Join(s.archiveDir(), e.Name())); err == nil {
				seg.SHA256 = sum
			}
		}
		out = append(out, seg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FirstSeq < out[j].FirstSeq })
	return out, nil
}

func parseSegmentName(name string) (first, last uint64, ok bool) {
	if !strings.HasPrefix(name, "audit-chain-") || !strings.HasSuffix(name, ".jsonl") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "audit-chain-"), ".jsonl"), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	first, err1 := strconv.ParseUint(parts[0], 10, 64)
	last, err2 := strconv.ParseUint(parts[1], 10, 64)
	return first, last, err1 == nil && err2 == nil && first > 0 && last >= first
}

// firstSeq reads the sequence number of the first record in a chain file.
func firstSeq(path string) uint64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var rec ChainRecord
		if json.Unmarshal(sc.Bytes(), &rec) != nil {
			return 0
		}
		return rec.Seq
	}
	return 0
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// exportAllLocked streams the sealed files, oldest first, and then the
// current file: the whole chain from record 1. The caller holds s.mu.
func (s *ChainSink) exportAllLocked(w io.Writer) error {
	segs, err := s.segmentsLocked(false)
	if err != nil {
		return fmt.Errorf("audit chain: list archive: %w", err)
	}
	paths := make([]string, 0, len(segs)+1)
	for _, seg := range segs {
		paths = append(paths, filepath.Join(s.archiveDir(), seg.Name))
	}
	paths = append(paths, s.path)
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("audit chain: open for export: %w", err)
		}
		_, err = io.Copy(w, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// maybeRotateLocked seals the file when it has reached the configured size.
// The caller holds s.mu and the file lock.
func (s *ChainSink) maybeRotateLocked(ctx context.Context) {
	if s.rotateBytes <= 0 {
		return
	}
	st, err := s.file.Stat()
	if err != nil || st.Size() < s.rotateBytes {
		return
	}
	if _, err := s.rotateLocked("system", "size limit reached"); err != nil {
		s.logf("audit chain: rotation failed; the chain continues in the same file", err)
	}
}

// VerifySealed checks that data, the bytes of a sealed file, is the file the
// record next in the chain says it is. next is the first record of the file
// that follows it.
func VerifySealed(data []byte, next ChainRecord) error {
	if next.Action != string(ActionAuditContinued) {
		return fmt.Errorf("the next file does not start with an %s record", ActionAuditContinued)
	}
	want := next.Detail["previous_sha256"]
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("the sealed file's SHA-256 is %s; the chain says %s", short(got), short(want))
	}
	return nil
}

// Current describes the file being written: the sequence numbers it holds
// and its size. first is 0 when the chain is empty.
func (s *ChainSink) Current() (first, last uint64, bytes int64) {
	if s == nil {
		return 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, err := os.Stat(s.path); err == nil {
		bytes = st.Size()
	}
	if seq, _, ok := lastRecord(s.path); ok {
		last = seq
		first = firstSeq(s.path)
	}
	return first, last, bytes
}
