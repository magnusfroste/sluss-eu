// Package decisioncache caches routing decisions for low-risk requests to skip
// re-scoring identical prompts (ISSUE-052). The cache key is versioned by both
// policy and registry version, so a policy or registry change can never serve a
// stale decision. Only low-risk, non-sensitive requests are cacheable.
package decisioncache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

// Cacheable reports whether a request's decision may be cached. Per the caching
// design only low-risk, non-sensitive requests qualify; high-risk or sensitive
// requests are always re-evaluated.
func Cacheable(job *router.JobDescriptor) bool {
	return job != nil &&
		job.RiskLevel == router.RiskLow &&
		job.Sensitivity == router.SensitivityNone
}

// Key builds a versioned cache key from the request, its classification and the
// active policy/registry versions. Identical prompts under a different policy or
// registry version produce a different key, so old entries are never served.
func Key(req *openai.ChatRequest, job *router.JobDescriptor, policyVersion, registryVersion string) string {
	var b strings.Builder
	if job != nil {
		fmt.Fprintf(&b, "tenant=%s\x1fproject=%s\x1ftask=%s\x1fmode=%s\x1f",
			job.TenantID, job.ProjectID, job.TaskType, job.RouterMode)
	}
	fmt.Fprintf(&b, "policy=%s\x1fregistry=%s\x1f", policyVersion, registryVersion)
	if req != nil {
		b.WriteString("model=")
		b.WriteString(req.Model)
		b.WriteString("\x1fprompt=")
		b.WriteString(promptFingerprint(req.Messages))
		b.WriteString("\x1fmeta=")
		b.WriteString(metadataFingerprint(req.Metadata))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func promptFingerprint(messages []openai.Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(m.Role)
		b.WriteByte('\x1e')
		b.WriteString(m.Content)
		b.WriteByte('\x1d')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func metadataFingerprint(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%v\x1e", k, metadata[k])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

type entry struct {
	decision engine.RouteDecision
	expires  time.Time
}

// Cache is a TTL-bounded in-memory decision cache, safe for concurrent use.
type Cache struct {
	ttl time.Duration
	max int
	mu  sync.Mutex
	m   map[string]entry
	now func() time.Time
}

const defaultMaxEntries = 10_000

// New creates a cache with the given TTL and max entry count (0 → default max).
// A non-positive TTL disables caching (Get always misses, Put is a no-op).
func New(ttl time.Duration, max int) *Cache {
	if max <= 0 {
		max = defaultMaxEntries
	}
	return &Cache{ttl: ttl, max: max, m: make(map[string]entry), now: time.Now}
}

// Get returns the cached decision for key if present and unexpired.
func (c *Cache) Get(key string) (engine.RouteDecision, bool) {
	if c == nil || c.ttl <= 0 {
		return engine.RouteDecision{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return engine.RouteDecision{}, false
	}
	if c.now().After(e.expires) {
		delete(c.m, key)
		return engine.RouteDecision{}, false
	}
	return e.decision, true
}

// Put stores a decision under key with the cache TTL. A no-op for a nil/disabled
// cache. When the cache is full it is cleared before inserting (simple bound).
func (c *Cache) Put(key string, dec engine.RouteDecision) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		if _, exists := c.m[key]; !exists {
			c.m = make(map[string]entry, c.max)
		}
	}
	c.m[key] = entry{decision: dec, expires: c.now().Add(c.ttl)}
}

// Len returns the current number of cached entries (including any expired ones
// not yet evicted). Intended for tests and metrics.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}
