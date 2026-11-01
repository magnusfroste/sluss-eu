package engine_test

import (
	"errors"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/router"
)

// A rule that blocks classified material fires even when the prompt's top
// class is "secrets" — the case found on the demo (ISSUE-158).
func TestBlockRuleOnClassifiedFiresWhenSecretsRankHigher(t *testing.T) {
	store := complianceStore(t)
	p := &policy.Policy{
		Version: "test-all-classes",
		Rules: []policy.Rule{{
			ID:    "block-classified",
			When:  policy.When{Sensitivity: &policy.EnumMatch{Values: []string{"security_classified"}}},
			Route: policy.Route{Block: &policy.Block{Reason: "classified material stays out"}},
		}},
	}
	pol := mustCompilePolicy(t, p, store)
	req := openai.ChatRequest{Model: "auto", Messages: []openai.Message{{Role: "user",
		Content: "HEMLIG enligt säkerhetsskyddslagen: the key is AKIAQ3EGTRWKX5VL7P2N, summarise the plan"}}}
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: &req})
	if job.Sensitivity != router.SensitivitySecretsPossible {
		t.Fatalf("setup: primary class = %q, want secrets_possible", job.Sensitivity)
	}
	dec, err := engine.New(store).Decide(job, pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrBlocked) || !dec.Blocked {
		t.Fatalf("the classified-material rule did not fire: %+v %v", dec, err)
	}

	// A prompt without classified material is not blocked by that rule.
	req.Messages[0].Content = "the key is AKIAQ3EGTRWKX5VL7P2N, why does the deploy fail"
	job = router.NewJobDescriptor(router.JobDescriptorInput{Request: &req})
	if dec, err := engine.New(store).Decide(job, pol, engine.FullyHealthy, false); dec.Blocked || errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("a plain secrets prompt must not hit the classified rule: %+v %v", dec, err)
	}
}
