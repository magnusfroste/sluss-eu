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
	n, err := audit.VerifyChain(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-verify: CHAIN BROKEN after %d valid record(s): %v\n", n, err)
		os.Exit(1)
	}
	fmt.Printf("audit-verify: OK — %d record(s), chain intact (%s)\n", n, name)
}
