// Command mcp is a stdio Model Context Protocol server exposing tokenizer's
// read-only introspection tools (ISSUE-074) to a local dev agent such as Claude
// Code. It shares the exact tool set and logic with the HTTP surface mounted at
// /mcp on the running router (see internal/server.NewMCPServer) — this binary
// only wires the dependencies and speaks stdio.
//
// It is dry-run only: route_explain never calls a provider, and no tool mutates
// state. With ROUTER_DATA_DIR set it reads the same SQLite history/roster the
// router uses; otherwise it introspects the default in-memory registry so a
// dev agent can still ask "why would this route to premium?".
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/mcp"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/server"
)

func main() {
	// Logs go to stderr so they never corrupt the JSON-RPC stream on stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	var historyStore *history.Store
	var rosterStore providercfg.RosterStore
	if dataDir := strings.TrimSpace(os.Getenv("ROUTER_DATA_DIR")); dataDir != "" {
		path := filepath.Join(dataDir, "history.db")
		if hs, err := history.Open(path); err != nil {
			logger.Warn("could not open history db; introspecting default registry", "err", err, "file", path)
		} else {
			historyStore = hs
			rosterStore = hs
			defer func() { _ = hs.Close() }()
		}
	}

	snap, err := buildSnapshot(historyStore, logger)
	if err != nil {
		logger.Error("failed to build registry", "err", err)
		os.Exit(1)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		logger.Error("failed to create registry store", "err", err)
		os.Exit(1)
	}
	policyCache, err := policy.NewRuntimeCache(snap, strings.TrimSpace(os.Getenv("ROUTER_POLICY_PATH")))
	if err != nil {
		logger.Error("failed to build policy cache", "err", err)
		os.Exit(1)
	}
	eng := engine.New(store)
	premIn, premOut := premiumPricing(snap)

	srv := server.NewMCPServer(server.MCPOptions{
		Engine:      eng,
		PolicyCache: policyCache,
		Dashboard: server.DashboardOptions{
			History:                    historyStore,
			Version:                    snap.RegistryVersion(),
			PremiumInputMicrosPerMTok:  premIn,
			PremiumOutputMicrosPerMTok: premOut,
		},
		History: historyStore,
		// A fresh process has no live provider-health window; provider_health is
		// populated by the running router's HTTP surface. Kept non-nil so the tool
		// returns an empty set rather than an error.
		Health:  health.New(),
		Roster:  rosterStore,
		Version: snap.RegistryVersion(),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("tokenizer mcp (stdio) ready", "registry_version", snap.RegistryVersion())
	if err := mcp.ServeStdio(ctx, srv, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		logger.Error("stdio serve failed", "err", err)
		os.Exit(1)
	}
}

// buildSnapshot mirrors the router's roster→registry construction in miniature:
// prefer the SQLite roster (the source of truth), else fall back to the built-in
// default snapshot so the tool works with zero configuration.
func buildSnapshot(hs *history.Store, logger *slog.Logger) (*registry.Snapshot, error) {
	if hs != nil {
		provs, perr := hs.LoadRosterProviders()
		models, merr := hs.LoadRosterModels()
		if perr == nil && merr == nil && len(models) > 0 {
			regProvs, regModels := providercfg.RegistryEntries(provs, models)
			def := registry.Definition{
				RegistryVersion: "registry-roster-mcp",
				CreatedAt:       time.Now().UTC(),
				Providers:       regProvs,
				Models:          regModels,
			}
			return registry.NewSnapshot(def)
		}
		if perr != nil || merr != nil {
			logger.Warn("could not load roster; using default registry", "providers_err", perr, "models_err", merr)
		}
	}
	return registry.DefaultSnapshot()
}

// premiumPricing returns the premium-tier model's input/output price (micros per
// million tokens) for savings_report's all-premium baseline, or 0,0 when there
// is no premium model.
func premiumPricing(snap *registry.Snapshot) (inMicros, outMicros int64) {
	for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}) {
		if m.Tier == registry.TierPremium {
			return m.Cost.InputMicrosPerMillionToken, m.Cost.OutputMicrosPerMillionToken
		}
	}
	return 0, 0
}
