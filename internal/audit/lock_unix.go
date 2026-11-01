//go:build unix

package audit

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on f across processes and returns
// the unlock. If locking fails the append still happens — recording must
// never block the request path — and the chain is no worse than before.
func lockFile(f *os.File) func() {
	if f == nil {
		return func() {}
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return func() {}
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
}
