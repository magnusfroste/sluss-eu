package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// ChainDiagnosis explains a chain that does not verify, record by record, so
// an auditor can tell a change from an accident.
type ChainDiagnosis struct {
	Records  int
	Modified []string // records whose own hash does not match their content
	Forks    []string // records linked to an earlier record than the one before
	Missing  []uint64 // sequence numbers that never appear
	Unknown  []string // links to a hash that is not in the file
}

// Explained reports whether every break is a fork from two writers at once,
// with no record changed and none missing.
func (d ChainDiagnosis) Explained() bool {
	return len(d.Forks) > 0 && len(d.Modified) == 0 && len(d.Missing) == 0 && len(d.Unknown) == 0
}

// DiagnoseChain reads a whole chain export and classifies every break.
func DiagnoseChain(r io.Reader) (ChainDiagnosis, error) {
	var d ChainDiagnosis
	var recs []ChainRecord
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var rec ChainRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return d, fmt.Errorf("line %d: %w", len(recs)+1, err)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return d, err
	}
	d.Records = len(recs)
	byHash := make(map[string]int, len(recs))
	seen := map[uint64]bool{}
	var maxSeq uint64
	for i, rec := range recs {
		if h, err := hashRecord(rec); err != nil || h != rec.EntryHash {
			d.Modified = append(d.Modified, fmt.Sprintf("line %d (seq %d, %s): content does not match its hash — the record was changed", i+1, rec.Seq, rec.Action))
		}
		byHash[rec.EntryHash] = i
		seen[rec.Seq] = true
		if rec.Seq > maxSeq {
			maxSeq = rec.Seq
		}
		if i == 0 {
			if rec.PrevHash != "" {
				d.Unknown = append(d.Unknown, fmt.Sprintf("line 1 (seq %d, %s): continues from a record that is not in the file — the start of the chain was removed", rec.Seq, rec.Action))
			}
			continue
		}
		if rec.PrevHash == recs[i-1].EntryHash {
			continue
		}
		if j, ok := byHash[rec.PrevHash]; ok {
			p := recs[j]
			d.Forks = append(d.Forks, fmt.Sprintf("line %d (seq %d, %s at %s): continues from seq %d (%s) instead of the line before — two Sluss processes wrote at the same time, for example during an overlapping deploy",
				i+1, rec.Seq, rec.Action, rec.Time.Format("2006-01-02 15:04:05"), p.Seq, p.Time.Format("15:04:05")))
		} else {
			d.Unknown = append(d.Unknown, fmt.Sprintf("line %d (seq %d, %s): links to a record that is not in the file — a record was removed", i+1, rec.Seq, rec.Action))
		}
	}
	for s := uint64(1); s <= maxSeq; s++ {
		if !seen[s] {
			d.Missing = append(d.Missing, s)
		}
	}
	return d, nil
}
