// Package apikeys provisions DB-backed API keys (ISSUE-079): mint, list and
// revoke "virtual keys" bound to a tenant/project/role/scopes. The plaintext
// secret is generated from crypto/rand, shown exactly once, and only its
// SHA-256 hash is persisted. Auth stays cache-fast — the manager keeps the
// in-memory keystore in sync so the request path never touches the DB.
package apikeys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// keyBytes is the entropy of a minted key (256 bits) — high enough that an
// unsalted SHA-256 at rest is safe.
const keyBytes = 32

// Generate returns a new random key with the "tk_" prefix and its hash. The
// plaintext is the only time the secret exists; callers must not persist it.
func Generate() (plaintext, hash string, err error) {
	buf := make([]byte, keyBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("apikeys: entropy: %w", err)
	}
	plaintext = "tk_" + base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, auth.HashKey(plaintext), nil
}

// Manager mints/lists/revokes keys against the durable store and keeps the
// in-memory keystore in sync. A nil Store makes it a no-op (read-only).
type Manager struct {
	store   *history.Store
	keys    *auth.InMemoryKeyStore
	auditor audit.Sink
	now     func() time.Time
}

// NewManager wires the provisioning manager. now defaults to time.Now.
func NewManager(store *history.Store, keys *auth.InMemoryKeyStore, auditor audit.Sink) *Manager {
	return &Manager{store: store, keys: keys, auditor: auditor, now: time.Now}
}

// Enabled reports whether persistence is configured (a data dir / DB is set).
func (m *Manager) Enabled() bool { return m != nil && m.store != nil }

// MintRequest describes a key to create.
type MintRequest struct {
	TenantID  string
	ProjectID string
	Role      string
	Scopes    []string
	// Actor is the console user performing the mint, recorded in the audit
	// trail. Empty defaults to "admin".
	Actor string
}

// actorOr returns actor or "admin" when blank.
func actorOr(actor string) string {
	if strings.TrimSpace(actor) == "" {
		return "admin"
	}
	return actor
}

// Mint creates a key, persists its hash, activates it in the keystore, audits
// the event, and returns the plaintext (shown once) plus the stored record.
func (m *Manager) Mint(req MintRequest) (plaintext string, rec history.APIKey, err error) {
	if !m.Enabled() {
		return "", history.APIKey{}, fmt.Errorf("apikeys: provisioning disabled (no data dir)")
	}
	if strings.TrimSpace(req.TenantID) == "" {
		return "", history.APIKey{}, fmt.Errorf("apikeys: tenant_id is required")
	}
	plaintext, hash, err := Generate()
	if err != nil {
		return "", history.APIKey{}, err
	}
	rec = history.APIKey{
		KeyID:     newKeyID(hash),
		KeyHash:   hash,
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Role:      req.Role,
		Scopes:    normalizeScopes(req.Scopes),
		CreatedAt: m.now().UTC(),
	}
	if err := m.store.InsertAPIKey(rec); err != nil {
		return "", history.APIKey{}, err
	}
	m.keys.AddHashed(hash, tenantFor(rec))
	audit.Record(context.Background(), m.auditor, audit.Entry{
		Action:    audit.ActionAPIKeyAdd,
		Actor:     actorOr(req.Actor),
		TenantID:  rec.TenantID,
		ProjectID: rec.ProjectID,
		Target:    rec.KeyID,
	})
	return plaintext, rec, nil
}

// Revoke soft-deletes a key and evicts it from the keystore. Returns false when
// no active key with that id exists. actor is recorded in the audit trail.
func (m *Manager) Revoke(keyID, actor string) (bool, error) {
	if !m.Enabled() {
		return false, fmt.Errorf("apikeys: provisioning disabled (no data dir)")
	}
	hash, ok, err := m.store.RevokeAPIKey(keyID, m.now())
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	m.keys.RemoveHashed(hash)
	audit.Record(context.Background(), m.auditor, audit.Entry{
		Action: audit.ActionAPIKeyDisable,
		Actor:  actorOr(actor),
		Target: keyID,
	})
	return true, nil
}

// List returns all keys (including revoked) for the admin page. Never includes
// secrets — only the key id and metadata.
func (m *Manager) List() ([]history.APIKey, error) {
	if !m.Enabled() {
		return nil, nil
	}
	return m.store.LoadAllAPIKeys()
}

// LoadIntoKeystore activates every non-revoked DB key in the in-memory keystore.
// Called once at startup, after the env bootstrap key is seeded.
func (m *Manager) LoadIntoKeystore() (int, error) {
	if !m.Enabled() {
		return 0, nil
	}
	active, err := m.store.LoadActiveAPIKeys()
	if err != nil {
		return 0, err
	}
	for _, rec := range active {
		m.keys.AddHashed(rec.KeyHash, tenantFor(rec))
	}
	return len(active), nil
}

func tenantFor(rec history.APIKey) *tenant.Tenant {
	scopes := rec.Scopes
	if len(scopes) == 0 {
		scopes = auth.AllScopes() // no explicit scopes → full access (legacy semantics)
	}
	role := rec.Role
	if role == "" {
		role = auth.RoleUser
	}
	return &tenant.Tenant{
		ID:      rec.TenantID,
		Project: rec.ProjectID,
		KeyID:   rec.KeyID,
		Role:    role,
		Scopes:  scopes,
	}
}

// newKeyID derives a short, stable, non-secret id from the key hash — safe to
// show and reference (revoke) without exposing the secret.
func newKeyID(hash string) string {
	short := hash
	if len(short) > 12 {
		short = short[:12]
	}
	return "key_" + short
}

func normalizeScopes(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
