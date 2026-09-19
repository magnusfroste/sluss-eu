// Package auth implements the API-key bearer authentication used by the
// proxy. Keys are stored as their SHA-256 hash; the plaintext is never
// retained server-side once the store has been populated.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/tenant"
)

type KeyStore interface {
	Lookup(hashedKey string) (*tenant.Tenant, bool)
}

type InMemoryKeyStore struct {
	mu      sync.RWMutex
	keys    map[string]*tenant.Tenant
	auditor audit.Sink // optional; audits key mutations
}

func NewInMemoryKeyStore() *InMemoryKeyStore {
	return &InMemoryKeyStore{keys: make(map[string]*tenant.Tenant)}
}

// SetAuditor attaches an audit sink so subsequent key mutations are recorded.
// Passing nil disables auditing. Safe to call once at startup.
func (s *InMemoryKeyStore) SetAuditor(sink audit.Sink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditor = sink
}

// Add stores the SHA-256 of the plaintext key. The plaintext is discarded.
func (s *InMemoryKeyStore) Add(plaintext string, t *tenant.Tenant) {
	s.mu.Lock()
	s.keys[hashKey(plaintext)] = t
	auditor := s.auditor
	s.mu.Unlock()

	if t != nil {
		audit.Record(context.Background(), auditor, audit.Entry{
			Action:    audit.ActionAPIKeyAdd,
			Actor:     "system",
			TenantID:  t.ID,
			ProjectID: t.Project,
			Target:    t.KeyID,
		})
	}
}

// Disable removes the key matching plaintext and records an audit entry. It
// returns true if a key was present and removed.
func (s *InMemoryKeyStore) Disable(plaintext string) bool {
	hashed := hashKey(plaintext)
	s.mu.Lock()
	t, ok := s.keys[hashed]
	if ok {
		delete(s.keys, hashed)
	}
	auditor := s.auditor
	s.mu.Unlock()

	if !ok {
		return false
	}
	entry := audit.Entry{
		Action: audit.ActionAPIKeyDisable,
		Actor:  "system",
	}
	if t != nil {
		entry.TenantID = t.ID
		entry.ProjectID = t.Project
		entry.Target = t.KeyID
	}
	audit.Record(context.Background(), auditor, entry)
	return true
}

// AddHashed registers a key by its precomputed SHA-256 hash (hex). Used to load
// DB-backed keys at startup, where the plaintext is never known — only the hash
// is stored at rest (ISSUE-079). No audit entry is emitted here; provisioning is
// audited at mint/revoke time.
func (s *InMemoryKeyStore) AddHashed(hashed string, t *tenant.Tenant) {
	s.mu.Lock()
	s.keys[hashed] = t
	s.mu.Unlock()
}

// RemoveHashed deletes a key by its hash. Returns true if it was present. Used
// to revoke a DB-backed key (its plaintext is not retained). No audit entry —
// the provisioning layer records the revoke with full context.
func (s *InMemoryKeyStore) RemoveHashed(hashed string) bool {
	s.mu.Lock()
	_, ok := s.keys[hashed]
	if ok {
		delete(s.keys, hashed)
	}
	s.mu.Unlock()
	return ok
}

func (s *InMemoryKeyStore) Lookup(hashed string) (*tenant.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.keys[hashed]
	return t, ok
}

// HashKey returns the hex SHA-256 of a plaintext key — the at-rest form. API
// keys are high-entropy random tokens (see apikeys.Generate), so an unsalted
// hash is safe (rainbow tables don't apply to 256-bit secrets) and a per-key
// salt would break the constant-time hash-map lookup the auth path depends on.
func HashKey(plaintext string) string { return hashKey(plaintext) }

func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Middleware enforces a Bearer token. On success it attaches the tenant to
// the request context. On failure it returns 401 with an OpenAI-style error
// envelope so SDKs can parse the response uniformly.
func Middleware(store KeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeUnauthorized(w, "missing bearer token")
				return
			}
			plaintext := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if plaintext == "" {
				writeUnauthorized(w, "empty bearer token")
				return
			}
			t, ok := store.Lookup(hashKey(plaintext))
			if !ok {
				writeUnauthorized(w, "invalid api key")
				return
			}
			next.ServeHTTP(w, r.WithContext(tenant.WithTenant(r.Context(), t)))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
		Error: openai.ErrorBody{Message: msg, Type: "unauthorized"},
	})
}
