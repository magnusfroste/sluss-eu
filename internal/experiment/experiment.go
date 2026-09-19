// Package experiment implements deterministic traffic bucketing for live policy
// A/B tests. A configured percentage of assignment keys (typically tenant +
// project) are placed in the "treatment" arm and served an experiment policy
// variant; the rest stay in "control" on the primary policy. Assignment is a
// pure hash of the key, so the same tenant consistently gets the same arm across
// requests and process restarts — a stable experience, and outcomes that can be
// sliced by arm.
//
// The bucketing is on the fast path, so it does no I/O and never calls an LLM.
package experiment

import (
	"hash/fnv"
)

// Arm is the experiment bucket a request is assigned to.
type Arm string

const (
	// ArmControl is served the primary (existing) policy.
	ArmControl Arm = "control"
	// ArmTreatment is served the experiment policy variant.
	ArmTreatment Arm = "treatment"
)

// Config controls live experiment assignment. The zero value is disabled.
type Config struct {
	// Percentage is the share of assignment keys placed in the treatment arm,
	// 0..100. Values <= 0 disable the experiment (everything is control); values
	// >= 100 put all traffic in treatment.
	Percentage int
	// Salt isolates and lets you rotate an assignment: changing it reshuffles
	// which keys land in treatment without changing the percentage.
	Salt string
}

// Enabled reports whether the experiment assigns any traffic to treatment.
func (c Config) Enabled() bool {
	return c.Percentage > 0
}

// Assign returns the arm for an assignment key. It is deterministic: the same
// (config, key) always yields the same arm. An empty key is always control so a
// missing tenant/project can never be silently bucketed into treatment.
func (c Config) Assign(key string) Arm {
	if c.Percentage <= 0 || key == "" {
		return ArmControl
	}
	if c.Percentage >= 100 {
		return ArmTreatment
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(c.Salt))
	_, _ = h.Write([]byte{0x1f})
	_, _ = h.Write([]byte(key))
	if int(h.Sum32()%100) < c.Percentage {
		return ArmTreatment
	}
	return ArmControl
}
