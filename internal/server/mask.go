package server

import (
	"log/slog"
	"sync/atomic"

	"github.com/magnusfroste/sluss/internal/secrets"
)

// maskLogger holds the logger used to emit secret-masking events. It is set once
// from ChatOptions.Logger at handler construction and read on the error path;
// atomic access keeps the race detector happy if handlers are built concurrently
// in tests. When unset, the slog default is used.
var maskLogger atomic.Pointer[slog.Logger]

// setMaskLogger records the logger to use for masking events. nil is ignored so
// callers without a configured logger fall back to slog.Default().
func setMaskLogger(l *slog.Logger) {
	if l != nil {
		maskLogger.Store(l)
	}
}

func activeMaskLogger() *slog.Logger {
	if l := maskLogger.Load(); l != nil {
		return l
	}
	return slog.Default()
}

// maskOutbound redacts any secret material in a client-bound message and, when
// something was masked, emits a structured "secret_masked" event. It returns the
// safe-to-send text. The masking event records only the count and types — never
// the secret value or the surrounding message.
func maskOutbound(location, code, msg string) string {
	res := secrets.Mask(msg)
	if res.Masked() {
		activeMaskLogger().Warn("secret_masked",
			"location", location,
			"error_code", code,
			"masked_count", res.Count(),
			"masked_types", res.Types(),
		)
	}
	return res.Text
}
