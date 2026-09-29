package server

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
)

// RosterBuild is what a roster rebuild produces: the registry definition and
// one adapter per routable provider connection. main supplies the builder so
// a reload runs the exact code path startup runs (one way to build a roster).
type RosterBuild struct {
	Definition registry.Definition
	Adapters   map[string]provider.Adapter
}

// RosterReloader applies roster edits (models, providers, compliance tags)
// without a restart (ISSUE-115). A reload is all-or-nothing: the new registry
// snapshot is built and validated, every active policy is recompiled against
// it, and only when all of that succeeds are the registry, the adapter set and
// the policy caches swapped. Any failure leaves routing exactly as it was —
// fail-closed also for configuration changes. Every attempt is audited.
//
// Environment variables are only read at process start, so a provider whose
// key env var is not set in the running process stays unroutable until the
// next restart; the reload reports that instead of pretending it is live.
type RosterReloader struct {
	Build    func(ctx context.Context) (RosterBuild, error)
	Registry *registry.Store
	Caches   []*policy.Cache
	Auditor  audit.Sink
	Logger   *slog.Logger

	mu       sync.Mutex // serialises reloads; request paths never take it
	adapters atomic.Pointer[map[string]provider.Adapter]
}

// NewRosterReloader seeds the adapter set with the startup adapters.
func NewRosterReloader(initial map[string]provider.Adapter) *RosterReloader {
	r := &RosterReloader{}
	m := cloneAdapters(initial)
	r.adapters.Store(&m)
	return r
}

// Adapters returns the current provider-ID → adapter set. The map is never
// mutated after it is published, so callers may read it without locking.
func (r *RosterReloader) Adapters() map[string]provider.Adapter {
	if r == nil {
		return nil
	}
	if m := r.adapters.Load(); m != nil {
		return *m
	}
	return nil
}

// RosterReloadResult describes what a successful reload made live.
type RosterReloadResult struct {
	RegistryVersion string
	Models          int
	// Unroutable lists provider connections in the roster that have no
	// adapter (their key env var is not set in this process) — they need a
	// restart after the key is added.
	Unroutable []string
}

// Summary is a one-line, user-facing description of the result.
func (res RosterReloadResult) Summary() string {
	msg := fmt.Sprintf("live now (%d models, %s)", res.Models, res.RegistryVersion)
	if len(res.Unroutable) > 0 {
		msg += "; no key in the running process for " + strings.Join(res.Unroutable, ", ") + " — set it and restart"
	}
	return msg
}

// Reload rebuilds the roster and swaps it in atomically, or changes nothing.
func (r *RosterReloader) Reload(ctx context.Context, actor string) (RosterReloadResult, error) {
	if r == nil || r.Build == nil || r.Registry == nil {
		return RosterReloadResult{}, fmt.Errorf("live roster reload is not available in this mode — restart to apply")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	res, err := r.reload(ctx)
	entry := audit.Entry{Action: audit.ActionRosterReload, Actor: actor}
	if err != nil {
		entry.Outcome = audit.OutcomeFailure
		entry.Reason = err.Error()
		audit.Record(ctx, r.Auditor, entry)
		if r.Logger != nil {
			r.Logger.Warn("roster reload rejected; routing unchanged", "err", err)
		}
		return RosterReloadResult{}, err
	}
	entry.Target = res.RegistryVersion
	entry.Detail = map[string]string{"models": fmt.Sprint(res.Models)}
	if len(res.Unroutable) > 0 {
		entry.Detail["unroutable"] = strings.Join(res.Unroutable, ",")
	}
	audit.Record(ctx, r.Auditor, entry)
	if r.Logger != nil {
		r.Logger.Info("roster reloaded live", "registry_version", res.RegistryVersion, "models", res.Models, "unroutable", res.Unroutable)
	}
	return res, nil
}

func (r *RosterReloader) reload(ctx context.Context) (RosterReloadResult, error) {
	built, err := r.Build(ctx)
	if err != nil {
		return RosterReloadResult{}, fmt.Errorf("could not build roster: %w", err)
	}
	snap, err := registry.NewSnapshot(built.Definition)
	if err != nil {
		return RosterReloadResult{}, fmt.Errorf("new roster is invalid: %w", err)
	}
	// Phase 1: prepare every policy cache against the new snapshot.
	commits := make([]func(), 0, len(r.Caches))
	for _, c := range r.Caches {
		commit, err := c.Rebind(snap)
		if err != nil {
			return RosterReloadResult{}, err
		}
		commits = append(commits, commit)
	}
	// Phase 2: swap. Registry first so a request that already sees the new
	// adapters also sees the models that reference them.
	if _, err := r.Registry.Reload(built.Definition); err != nil {
		return RosterReloadResult{}, fmt.Errorf("registry swap failed: %w", err)
	}
	adapters := cloneAdapters(built.Adapters)
	r.adapters.Store(&adapters)
	for _, commit := range commits {
		commit()
	}

	res := RosterReloadResult{
		RegistryVersion: snap.RegistryVersion(),
		Models:          len(built.Definition.Models),
	}
	seen := map[string]bool{}
	for _, p := range built.Definition.Providers {
		if _, ok := adapters[p.ID]; !ok && !seen[p.ID] {
			seen[p.ID] = true
			res.Unroutable = append(res.Unroutable, p.ID)
		}
	}
	sort.Strings(res.Unroutable)
	return res, nil
}

func cloneAdapters(in map[string]provider.Adapter) map[string]provider.Adapter {
	out := make(map[string]provider.Adapter, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// applyRosterEdit runs after a roster edit was persisted and returns the
// (ok, err) flash pair for the page. With no reloader it keeps the old
// "restart to apply" behaviour; a rejected reload says the edit is saved but
// routing is unchanged, and why.
func applyRosterEdit(ctx context.Context, rl *RosterReloader, actor, saved string) (ok, errMsg string) {
	if rl == nil {
		return saved + " — restart (redeploy) to apply.", ""
	}
	res, err := rl.Reload(ctx, actor)
	if err != nil {
		return "", saved + " but NOT applied — routing is unchanged: " + err.Error()
	}
	return saved + " — " + res.Summary() + ".", ""
}
