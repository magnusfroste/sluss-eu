package main

// The boot banner (ISSUE-141): a few plain lines on stderr when the router is
// ready, so whoever reads `docker logs` sees at a glance which build runs,
// where its data lives and where the console is — the way LiteLLM and
// AnythingLLM greet their operators. The same facts go out as one structured
// "sluss ready" log line for log pipelines; ROUTER_LOG_BANNER=off drops the
// plain-text lines for setups that want JSON only. Never prints a secret.

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

const bannerArt = `  ____  _
 / ___|| |_   _ ___ ___
 \___ \| | | | / __/ __|
  ___) | | |_| \__ \__ \
 |____/|_|\__,_|___/___/   EU LLM gateway
`

// bootInfo is what the banner and the "sluss ready" line report.
type bootInfo struct {
	Version, Commit, Built string
	Addr, PublicURL        string
	DataDir                string
	Providers              string // "openrouter" or "mock"
	Deployment             string
	UpdateCheck            bool
}

func currentBootInfo(addr, providers string) bootInfo {
	return bootInfo{
		Version:     buildinfo.Release(),
		Commit:      buildinfo.Short(),
		Built:       buildinfo.BuiltAt(),
		Addr:        addr,
		PublicURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("ROUTER_PUBLIC_URL")), "/"),
		DataDir:     strings.TrimSpace(os.Getenv("ROUTER_DATA_DIR")),
		Providers:   providers,
		Deployment:  strings.ToLower(strings.TrimSpace(os.Getenv("ROUTER_DEPLOYMENT"))),
		UpdateCheck: !strings.EqualFold(strings.TrimSpace(os.Getenv("ROUTER_UPDATE_CHECK")), "off"),
	}
}

// versionLine reads "v1.0.0 (commit 4f5a346, built …)" for a release and
// "main build 4f5a346 (built …)" otherwise.
func (b bootInfo) versionLine() string {
	built := ""
	if b.Built != "" {
		built = ", built " + b.Built
	}
	if b.Version != "" {
		return b.Version + " (commit " + b.Commit + built + ")"
	}
	if built != "" {
		built = " (" + strings.TrimPrefix(built, ", ") + ")"
	}
	return "main build " + b.Commit + built
}

// consoleURL is where the admin console answers: the public URL when one is
// set, otherwise localhost on the listening port.
func (b bootInfo) consoleURL() string {
	base := b.PublicURL
	if base == "" {
		port := strings.TrimPrefix(b.Addr, ":")
		if _, p, err := net.SplitHostPort(b.Addr); err == nil {
			port = p
		}
		base = "http://localhost:" + port
	}
	return base + "/router/dashboard"
}

func writeBanner(w io.Writer, b bootInfo) {
	data := b.DataDir
	if data == "" {
		data = "none (ROUTER_DATA_DIR unset: history, keys and audit chain live in memory only)"
	}
	upd := "off (ROUTER_UPDATE_CHECK=off)"
	if b.UpdateCheck {
		upd = "on, every 6 h against github.com/magnusfroste/sluss-eu"
	}
	deploy := b.Deployment
	if deploy == "" {
		deploy = "not set (ROUTER_DEPLOYMENT)"
	}
	fmt.Fprint(w, "\n"+bannerArt+"\n")
	for _, row := range [][2]string{
		{"Version", b.versionLine()},
		{"Runtime", runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH},
		{"Listening", b.Addr},
		{"Console", b.consoleURL()},
		{"Providers", b.Providers},
		{"Data", data},
		{"Deployment", deploy},
		{"Update check", upd},
		{"Manual", strings.TrimSuffix(b.consoleURL(), "/router/dashboard") + "/manual"},
	} {
		fmt.Fprintf(w, " %-13s%s\n", row[0], row[1])
	}
	fmt.Fprintln(w)
}
