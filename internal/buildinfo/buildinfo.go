// Package buildinfo identifies the running build, so an operator can tell
// which commit a deployed instance actually runs (a redeploy that silently
// kept the old container is otherwise invisible).
//
// Commit and Built are set at link time by the image build:
//
//	go build -ldflags "-X github.com/magnusfroste/sluss/internal/buildinfo.Commit=<sha>
//	                   -X github.com/magnusfroste/sluss/internal/buildinfo.Built=<rfc3339>"
//
// A plain `go build` inside a git checkout falls back to the VCS stamp Go
// embeds; anything else reports "dev".
package buildinfo

import (
	"runtime/debug"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Set via -ldflags -X; empty means "not stamped".
var (
	Commit string
	Built  string
)

var resolveOnce sync.Once

func resolve() {
	if Commit != "" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev, at string
		dirty := false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				at = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" {
			Commit = rev
			if dirty {
				Commit += "-dirty"
			}
			if Built == "" {
				Built = at
			}
		}
	}
	if Commit == "" {
		Commit = "dev"
	}
}

// FullCommit returns the full commit id ("dev" when unknown).
func FullCommit() string {
	resolveOnce.Do(resolve)
	return Commit
}

// Short returns the 7-character commit id, keeping any "-dirty" suffix.
func Short() string {
	c := FullCommit()
	suffix := ""
	if n := len(c) - len("-dirty"); n > 0 && c[n:] == "-dirty" {
		c, suffix = c[:n], "-dirty"
	}
	if len(c) > 7 {
		c = c[:7]
	}
	return c + suffix
}

// BuiltAt returns the build timestamp ("" when unknown).
func BuiltAt() string {
	resolveOnce.Do(resolve)
	return Built
}

var info = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "sluss_build_info",
	Help: "Build of the running router: always 1, labelled with the commit and build time.",
}, []string{"commit", "built"})

// Register publishes sluss_build_info on /metrics. Call once at startup.
func Register() {
	info.WithLabelValues(FullCommit(), BuiltAt()).Set(1)
}
