package evals

import (
	"fmt"
	"os"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
)

// LoadCompiledPolicy parses and compiles a policy file against the runner's
// registry snapshot so offline simulations use the same model inventory as the
// real routing engine.
func LoadCompiledPolicy(path string, snapshot *registry.Snapshot) (*policy.CompiledPolicy, error) {
	if path == "" {
		return nil, fmt.Errorf("evals: policy path is required")
	}
	if snapshot == nil {
		return nil, fmt.Errorf("evals: registry snapshot is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("evals: read policy %q: %w", path, err)
	}
	parsed, err := policy.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("evals: parse policy %q: %w", path, err)
	}
	compiled, err := policy.Compile(parsed, snapshot)
	if err != nil {
		return nil, fmt.Errorf("evals: compile policy %q: %w", path, err)
	}
	return compiled, nil
}
