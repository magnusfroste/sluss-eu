// Command audit-verify verifies a tamper-evident audit chain export
// (ISSUE-075): it recomputes every record's hash and checks the chain links and
// sequence numbers. Exit code 0 means the chain is intact; 1 means it is broken
// (with the offending line in the error) — the offline proof an auditor runs on
// a handed-over audit-chain.jsonl.
//
// With -anchor it also checks the export against an anchor taken earlier
// (ISSUE-160): the "audit anchor" line Sluss writes to its log every hour, or
// the answer of GET /router/audit/anchor. A chain that was rewritten from some
// point on, with every later hash recomputed, passes the plain check; it does
// not pass the anchor.
//
// Usage:
//
//	audit-verify <file>                         # or "-" / no arg to read stdin
//	audit-verify -anchor <seq>:<entry_hash> <file>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/audit"
)

func main() {
	anchor := flag.String("anchor", "", "an anchor to check the export against, as <seq>:<entry_hash> (from the \"audit anchor\" log line or /router/audit/anchor)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: audit-verify [-anchor <seq>:<entry_hash>] <file>")
		flag.PrintDefaults()
	}
	flag.Parse()

	var in io.Reader = os.Stdin
	name := "stdin"
	if arg := flag.Arg(0); arg != "" && arg != "-" {
		f, err := os.Open(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
		name = arg
	}
	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-verify: %v\n", err)
		os.Exit(1)
	}
	n, err := audit.VerifyChain(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-verify: CHAIN BROKEN after %d valid record(s): %v\n", n, err)
		explain(data)
		os.Exit(1)
	}
	if *anchor != "" {
		seq, hash, perr := parseAnchor(*anchor)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: %v\n", perr)
			os.Exit(2)
		}
		if _, aerr := audit.VerifyAnchor(bytes.NewReader(data), seq, hash); aerr != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: ANCHOR MISMATCH — %v\n", aerr)
			os.Exit(1)
		}
		fmt.Printf("audit-verify: OK — %d record(s), chain intact and record %d matches the anchor (%s)\n", n, seq, name)
		return
	}
	fmt.Printf("audit-verify: OK — %d record(s), chain intact (%s)\n", n, name)
}

func parseAnchor(s string) (uint64, string, error) {
	seqS, hash, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, "", fmt.Errorf("-anchor wants <seq>:<entry_hash>, got %q", s)
	}
	seq, err := strconv.ParseUint(strings.TrimSpace(seqS), 10, 64)
	if err != nil || seq == 0 {
		return 0, "", fmt.Errorf("-anchor: seq must be a positive number, got %q", seqS)
	}
	hash = strings.TrimSpace(hash)
	if len(hash) != 64 {
		return 0, "", fmt.Errorf("-anchor: entry_hash must be 64 hex characters, got %d", len(hash))
	}
	return seq, hash, nil
}

// explain reads the whole file and says what every break is, so an auditor
// can tell a changed or removed record from two processes writing at once.
func explain(data []byte) {
	d, err := audit.DiagnoseChain(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-verify: cannot read the whole file: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "\nWhole file: %d record(s).\n", d.Records)
	for _, group := range []struct {
		title string
		lines []string
	}{{"Changed records", d.Modified}, {"Removed records", d.Unknown}, {"Two writers at once", d.Forks}} {
		if len(group.lines) == 0 {
			continue
		}
		fmt.Fprintf(os.Stderr, "\n%s:\n", group.title)
		for _, l := range group.lines {
			fmt.Fprintf(os.Stderr, "  - %s\n", l)
		}
	}
	if len(d.Missing) > 0 {
		fmt.Fprintf(os.Stderr, "\nMissing sequence numbers: %v\n", d.Missing)
	}
	if d.Explained() {
		fmt.Fprintln(os.Stderr, "\nEvery record's own hash is intact and no sequence number is missing. The breaks are forks from two processes writing at the same time, not changes or removals.")
	}
}
