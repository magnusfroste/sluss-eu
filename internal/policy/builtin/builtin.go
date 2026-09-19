// Package builtin embeds ready-to-use policy files into the router binary so an
// operator can select one by name (ROUTER_POLICY_PATH=builtin:pii-local) with no
// YAML files shipped into the container image. The runtime image carries only the
// static binary, so a path-based policy can never resolve there — embedding keeps
// the "single self-contained binary, env-only config" deployment intact.
package builtin

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.yaml
var files embed.FS

// Prefix marks a ROUTER_POLICY_PATH value as naming an embedded policy rather
// than a filesystem path, e.g. "builtin:pii-local".
const Prefix = "builtin:"

// Load returns the embedded policy bytes for a built-in name (without the .yaml
// extension). The bool is false when no such built-in exists.
func Load(name string) ([]byte, bool) {
	b, err := files.ReadFile(name + ".yaml")
	if err != nil {
		return nil, false
	}
	return b, true
}

// Names lists the available built-in policy names (without extension), sorted,
// so an error message can tell the operator what they can pick from.
func Names() []string {
	var out []string
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".yaml") {
			out = append(out, strings.TrimSuffix(p, ".yaml"))
		}
		return nil
	})
	sort.Strings(out)
	return out
}
