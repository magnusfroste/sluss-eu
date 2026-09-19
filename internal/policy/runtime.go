package policy

import (
	"fmt"
	"os"
	"strings"

	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/registry"
)

const defaultRuntimePolicy = `
version: pv_runtime_2026_06_12
metadata:
  owner: platform
  description: Built-in runtime policy
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: block_disabled
    when:
      router_mode: disabled
    route:
      block:
        code: router_disabled
        reason: disabled
`

// NewDefaultRuntimeCache builds the built-in compiled policy cache used by the
// router when no external policy loader exists yet.
func NewDefaultRuntimeCache(snapshot *registry.Snapshot) (*Cache, error) {
	return NewRuntimeCache(snapshot, "")
}

// LoadSourceBytes resolves a policy path the way the runtime does: "" → the
// built-in default policy, "builtin:<name>" → an embedded pack, anything else →
// a file on disk. Exported so the policy console can rebuild the baseline
// (rollback) from the same source the boot used (ISSUE-093).
func LoadSourceBytes(path string) ([]byte, error) {
	switch {
	case path == "":
		return []byte(defaultRuntimePolicy), nil
	case strings.HasPrefix(path, builtin.Prefix):
		// ROUTER_POLICY_PATH=builtin:<name> loads a policy embedded in the binary,
		// so no YAML file has to ship into the (binary-only) runtime image.
		name := strings.TrimPrefix(path, builtin.Prefix)
		b, ok := builtin.Load(name)
		if !ok {
			return nil, fmt.Errorf("policy: unknown built-in policy %q (available: %s)", name, strings.Join(builtin.Names(), ", "))
		}
		return b, nil
	default:
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("policy: read runtime policy %q: %w", path, err)
		}
		return data, nil
	}
}

// NewRuntimeCache builds the runtime compiled policy cache. When path is empty
// it uses the built-in default policy; otherwise it parses the provided policy
// file as the default runtime scope. Tenant/project matching remains expressed
// inside policy rules, so request-path lookup stays in-memory.
func NewRuntimeCache(snapshot *registry.Snapshot, path string) (*Cache, error) {
	data, err := LoadSourceBytes(path)
	if err != nil {
		return nil, err
	}
	parsed, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("policy: parse runtime policy: %w", err)
	}
	cache, err := NewCache([]Source{{Scope: Scope{}, Policy: parsed, Registry: snapshot}})
	if err != nil {
		return nil, fmt.Errorf("policy: compile runtime policy: %w", err)
	}
	return cache, nil
}
