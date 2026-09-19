package history

// API-key persistence (ISSUE-079): DB-backed "virtual keys" so different
// departments get their own keys (tenant/project/scopes) without a redeploy.
// Only the SHA-256 hash is stored at rest — the plaintext is shown once at
// mint time and never persisted. Auth stays cache-fast: keys load into the
// in-memory keystore at boot and on each mutation; the DB is never read on the
// request path.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// APIKey is one persisted key record. Scopes is stored comma-separated.
type APIKey struct {
	KeyID      string
	KeyHash    string
	TenantID   string
	ProjectID  string
	Role       string
	Scopes     []string
	CreatedAt  time.Time
	RevokedAt  time.Time // zero = active
	LastUsedAt time.Time // zero = never
}

// Revoked reports whether the key has been revoked.
func (k APIKey) Revoked() bool { return !k.RevokedAt.IsZero() }

// InsertAPIKey stores a new key. The caller supplies the hash (never the
// plaintext). Fails if the key_id already exists.
func (s *Store) InsertAPIKey(k APIKey) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO api_keys
			(key_id, key_hash, tenant_id, project_id, role, scopes, created_at, revoked_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '', '')`,
		k.KeyID, k.KeyHash, k.TenantID, k.ProjectID, k.Role,
		strings.Join(k.Scopes, ","), k.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("history: insert api key %q: %w", k.KeyID, err)
	}
	return nil
}

// LoadActiveAPIKeys returns all non-revoked keys (for boot into the keystore).
func (s *Store) LoadActiveAPIKeys() ([]APIKey, error) {
	return s.loadAPIKeys(false)
}

// LoadAllAPIKeys returns every key including revoked ones (for the admin page).
func (s *Store) LoadAllAPIKeys() ([]APIKey, error) {
	return s.loadAPIKeys(true)
}

func (s *Store) loadAPIKeys(includeRevoked bool) ([]APIKey, error) {
	if s == nil {
		return nil, nil
	}
	q := `SELECT key_id, key_hash, tenant_id, project_id, role, scopes, created_at, revoked_at, last_used_at FROM api_keys`
	if !includeRevoked {
		q += ` WHERE revoked_at = ''`
	}
	q += ` ORDER BY created_at, key_id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("history: load api keys: %w", err)
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var scopes, created, revoked, lastUsed string
		if err := rows.Scan(&k.KeyID, &k.KeyHash, &k.TenantID, &k.ProjectID, &k.Role,
			&scopes, &created, &revoked, &lastUsed); err != nil {
			return nil, fmt.Errorf("history: scan api key: %w", err)
		}
		k.Scopes = splitTags(scopes) // reuse: trims + drops empties
		k.CreatedAt = parseTime(created)
		k.RevokedAt = parseTime(revoked)
		k.LastUsedAt = parseTime(lastUsed)
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey marks a key revoked (soft delete). Returns the revoked key's
// hash so the caller can evict it from the in-memory keystore, and false if no
// active key with that id exists.
func (s *Store) RevokeAPIKey(keyID string, at time.Time) (string, bool, error) {
	if s == nil {
		return "", false, fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var hash string
	err := s.db.QueryRow(`SELECT key_hash FROM api_keys WHERE key_id = ? AND revoked_at = ''`, keyID).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("history: revoke lookup %q: %w", keyID, err)
	}
	if _, err := s.db.Exec(`UPDATE api_keys SET revoked_at = ? WHERE key_id = ?`,
		at.UTC().Format(time.RFC3339), keyID); err != nil {
		return "", false, fmt.Errorf("history: revoke api key %q: %w", keyID, err)
	}
	return hash, true, nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
