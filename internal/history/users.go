package history

// Admin-user persistence (named console logins): individual accounts so the
// audit trail can attribute console actions — policy changes, key mints, audit
// exports — to a PERSON, not a shared "admin". This is the accountability the
// NIS2 pitch needs. Only a password KDF hash is stored at rest (never the
// plaintext). The env dashboard password remains a break-glass fallback.

import (
	"database/sql"
	"fmt"
	"time"
)

// AdminUser is one console account. PasswordHash is a self-describing PBKDF2
// string (see server.HashPassword); the plaintext is never stored.
type AdminUser struct {
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	DisabledAt   time.Time // zero = active
	LastLoginAt  time.Time // zero = never
}

// Disabled reports whether the account is disabled.
func (u AdminUser) Disabled() bool { return !u.DisabledAt.IsZero() }

// UpsertAdminUser inserts or updates an account by username (create or reset).
func (s *Store) UpsertAdminUser(u AdminUser) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO admin_users (username, password_hash, role, created_at, disabled_at, last_login_at)
			VALUES (?, ?, ?, ?, '', '')
		ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash, role=excluded.role, disabled_at=''`,
		u.Username, u.PasswordHash, u.Role, u.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("history: upsert admin user %q: %w", u.Username, err)
	}
	return nil
}

// GetAdminUser returns one account by username, or false if not found.
func (s *Store) GetAdminUser(username string) (AdminUser, bool, error) {
	if s == nil {
		return AdminUser{}, false, nil
	}
	var u AdminUser
	var created, disabled, lastLogin string
	err := s.db.QueryRow(`SELECT username, password_hash, role, created_at, disabled_at, last_login_at
		FROM admin_users WHERE username = ?`, username).
		Scan(&u.Username, &u.PasswordHash, &u.Role, &created, &disabled, &lastLogin)
	if err == sql.ErrNoRows {
		return AdminUser{}, false, nil
	}
	if err != nil {
		return AdminUser{}, false, fmt.Errorf("history: get admin user %q: %w", username, err)
	}
	u.CreatedAt = parseTime(created)
	u.DisabledAt = parseTime(disabled)
	u.LastLoginAt = parseTime(lastLogin)
	return u, true, nil
}

// ListAdminUsers returns all accounts (including disabled), ordered by username.
// PasswordHash is included for internal use; callers must never expose it.
func (s *Store) ListAdminUsers() ([]AdminUser, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT username, password_hash, role, created_at, disabled_at, last_login_at
		FROM admin_users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("history: list admin users: %w", err)
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var u AdminUser
		var created, disabled, lastLogin string
		if err := rows.Scan(&u.Username, &u.PasswordHash, &u.Role, &created, &disabled, &lastLogin); err != nil {
			return nil, fmt.Errorf("history: scan admin user: %w", err)
		}
		u.CreatedAt = parseTime(created)
		u.DisabledAt = parseTime(disabled)
		u.LastLoginAt = parseTime(lastLogin)
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetAdminUserDisabled disables (at != zero) or re-enables (at zero) an account.
func (s *Store) SetAdminUserDisabled(username string, at time.Time) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	val := ""
	if !at.IsZero() {
		val = at.UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(`UPDATE admin_users SET disabled_at = ? WHERE username = ?`, val, username)
	if err != nil {
		return fmt.Errorf("history: disable admin user %q: %w", username, err)
	}
	return nil
}

// TouchAdminUserLogin records a successful login time (best-effort).
func (s *Store) TouchAdminUserLogin(username string, at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`UPDATE admin_users SET last_login_at = ? WHERE username = ?`,
		at.UTC().Format(time.RFC3339), username)
}

// CountAdminUsers returns the number of accounts (for seed-on-empty).
func (s *Store) CountAdminUsers() int {
	if s == nil {
		return 0
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n
}
