//go:build !unix

package audit

import "os"

// lockFile is a no-op where advisory file locks are not available; Sluss runs
// in a Linux container, where lock_unix.go applies.
func lockFile(*os.File) func() { return func() {} }
