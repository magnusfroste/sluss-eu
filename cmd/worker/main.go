// Worker runs background jobs that must not block the request path. As of
// Sprint 8 it runs the retention cleanup sweeper (ISSUE-045): on a fixed
// interval it purges expired log rows according to per-tenant retention.
//
// No database driver is wired in this build, so the worker uses a dry-run
// purger that logs the deletes it would issue. Swapping in a retention.SQLPurger
// backed by *sql.DB turns it into a live cleaner with no other changes.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/magnusfroste/sluss/internal/retention"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	settings := retention.NewSettings(
		parseIntEnv(os.Getenv("ROUTER_RETENTION_DAYS"), retention.DefaultRetentionDays),
		parseBoolEnv(os.Getenv("ROUTER_PROMPT_LOGGING")),
	)
	cleaner := retention.NewCleaner(settings, &retention.DryRunPurger{Logger: logger}, logger)

	interval := time.Duration(parseIntEnv(os.Getenv("ROUTER_RETENTION_INTERVAL_HOURS"), 24)) * time.Hour
	logger.Info("worker starting: retention cleanup sweeper",
		"retention_days", settings.RetentionDays(""),
		"interval", interval.String(),
		"mode", "dry-run",
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cleaner.Run(ctx, interval, nil)

	logger.Info("worker shutting down")
}

func parseBoolEnv(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseIntEnv(s string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
		return v
	}
	return fallback
}
