// Package bandit implements a deterministic UCB1 multi-armed bandit over
// (task class, model) pairs. It learns which model actually performs best for
// each task class from realized rewards (attempt success today; richer outcome
// signals can feed the same Record path) and exposes the result as an
// engine.QualitySource so the routing engine balances exploiting the
// best-known model against exploring under-sampled ones.
//
// UCB1 is deterministic — the exploration bonus is a closed-form function of the
// pull counts, no RNG — so the fast path stays reproducible. Rewards are
// recorded off the response path; scoring only reads the in-memory stats.
package bandit

import (
	"math"
	"sort"
	"sync"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/router"
)

// DefaultExploration is the UCB1 exploration weight (the classic constant is
// sqrt(2) ≈ 1.41; a lower value exploits sooner). Applied to the confidence
// bonus.
const DefaultExploration = 1.41

type arm struct {
	pulls     int
	rewardSum float64 // sum of rewards in [0,1]; mean = rewardSum/pulls
}

// Bandit is a concurrent UCB1 bandit keyed by task class then model ID.
type Bandit struct {
	c  float64 // exploration weight
	mu sync.RWMutex
	// byTask[taskClass][modelID] -> arm; taskPulls[taskClass] -> total pulls.
	byTask    map[string]map[string]*arm
	taskPulls map[string]int
}

// compile-time check that Bandit satisfies the engine contract.
var _ engine.QualitySource = (*Bandit)(nil)

// New returns a bandit with the given exploration weight (<=0 → DefaultExploration).
func New(exploration float64) *Bandit {
	if exploration <= 0 {
		exploration = DefaultExploration
	}
	return &Bandit{
		c:         exploration,
		byTask:    make(map[string]map[string]*arm),
		taskPulls: make(map[string]int),
	}
}

// Record adds one realized reward in [0,1] for a (task, model) pull (1 = good,
// 0 = bad). Out-of-range rewards are clamped. Empty task or model is ignored.
func (b *Bandit) Record(task, model string, reward float64) {
	if b == nil || task == "" || model == "" {
		return
	}
	if reward < 0 {
		reward = 0
	} else if reward > 1 {
		reward = 1
	}
	b.mu.Lock()
	b.addLocked(task, model, 1, reward)
	b.mu.Unlock()
}

// Seed injects prior pulls/successes for a (task, model) — e.g. from offline
// eval pass rates — so the bandit starts warm instead of cold. successes is
// counted as reward mass; it is clamped to [0, pulls].
func (b *Bandit) Seed(task, model string, pulls, successes int) {
	if b == nil || task == "" || model == "" || pulls <= 0 {
		return
	}
	if successes < 0 {
		successes = 0
	} else if successes > pulls {
		successes = pulls
	}
	b.mu.Lock()
	b.addLocked(task, model, pulls, float64(successes))
	b.mu.Unlock()
}

// addLocked adds pulls and reward mass; caller holds the write lock.
func (b *Bandit) addLocked(task, model string, pulls int, rewardMass float64) {
	byModel, ok := b.byTask[task]
	if !ok {
		byModel = make(map[string]*arm)
		b.byTask[task] = byModel
	}
	a := byModel[model]
	if a == nil {
		a = &arm{}
		byModel[model] = a
	}
	a.pulls += pulls
	a.rewardSum += rewardMass
	b.taskPulls[task] += pulls
}

// Quality returns the UCB1 score for a (task, model) pair in [0,1], or false
// when the model has no pulls for the task (scoring then falls back to the
// static/measured prior). The score is the observed mean reward plus a
// confidence bonus that shrinks as a model is pulled more, so under-sampled
// models are explored and consistently-good models are exploited.
func (b *Bandit) Quality(task router.TaskType, modelID string) (float64, bool) {
	if b == nil {
		return 0, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	byModel, ok := b.byTask[string(task)]
	if !ok {
		return 0, false
	}
	a := byModel[modelID]
	if a == nil || a.pulls == 0 {
		return 0, false
	}
	mean := a.rewardSum / float64(a.pulls)
	total := b.taskPulls[string(task)]
	score := mean
	if total > 1 {
		score += b.c * math.Sqrt(math.Log(float64(total))/float64(a.pulls))
	}
	if score > 1 {
		score = 1
	}
	return score, true
}

// ArmStat is a diagnostic snapshot of one (task, model) arm.
type ArmStat struct {
	TaskClass  string  `json:"task_class"`
	ModelID    string  `json:"model_id"`
	Pulls      int     `json:"pulls"`
	MeanReward float64 `json:"mean_reward"`
}

// Snapshot returns all arms sorted by task then model — for logging/diagnostics.
func (b *Bandit) Snapshot() []ArmStat {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []ArmStat
	for task, byModel := range b.byTask {
		for model, a := range byModel {
			mean := 0.0
			if a.pulls > 0 {
				mean = a.rewardSum / float64(a.pulls)
			}
			out = append(out, ArmStat{TaskClass: task, ModelID: model, Pulls: a.pulls, MeanReward: mean})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskClass != out[j].TaskClass {
			return out[i].TaskClass < out[j].TaskClass
		}
		return out[i].ModelID < out[j].ModelID
	})
	return out
}

// Arms returns the number of distinct (task, model) arms tracked.
func (b *Bandit) Arms() int {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, byModel := range b.byTask {
		n += len(byModel)
	}
	return n
}
