// Command audit-verify verifies a tamper-evident audit chain export
// (ISSUE-075): it recomputes every record's hash and checks the chain links and
// sequence numbers. Exit code 0 means the chain is intact; 1 means it is broken
// (with the offending line in the error) — the offline proof an auditor runs on
// a handed-over audit-chain.jsonl.
//
// Usage:
//
//	audit-verify <file>    # or "-" / no arg to read stdin
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/magnusfroste/sluss/internal/audit"
)

func main() {
	var in io.Reader = os.Stdin
	name := "stdin"
	if len(os.Args) > 1 && os.Args[1] != "-" {
		f, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
		name = os.Args[1]
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
	fmt.Printf("audit-verify: OK — %d record(s), chain intact (%s)\n", n, name)
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
