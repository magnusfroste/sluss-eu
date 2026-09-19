package history

// Console session persistence (ISSUE-108): admin/console sessions survive a
// redeploy instead of living only in RAM. Without this, every restart wiped
// the in-memory session table and logged everyone out — a poor signal for a
// security product that cannot survive its own restart. Sessions stay
// server-side (revocable: logout, break-glass, expiry) — the opposite of a
// stateless cookie you cannot kill.
//
// Only the opaque random session id is stored (the same value the cookie
// carries); there is no secret material here. The auth request path reads from
// the in-memory cache, never this table — the DB is persistence, loaded at
// boot and written through on create/delete.

import (
	"database/sql"
	"fmt"
	"time"
)

// Session is one persisted console session.
type Session struct {
	ID        string
	Username  string
	Role      string
	ExpiresAt time.Time
}

// InsertSession persists a new session (write-through from the cache).
func (s *Store) InsertSession(sess Session) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sessions (id, username, role, expires_at)
		VALUES (?, ?, ?, ?)`,
		sess.ID, sess.Username, sess.Role, sess.ExpiresAt.UTC().Format(time.RFC3339))
	return err
}

// DeleteSession removes a session by id (logout / expiry eviction).
func (s *Store) DeleteSession(id string) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// ActiveSessions returns all non-expired sessions, so the in-memory cache can
// be rebuilt at boot. Expired rows are dropped opportunistically.
func (s *Store) ActiveSessions() ([]Session, error) {
	if s == nil {
		return nil, fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	nowUTC := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <> '' AND expires_at < ?`, nowUTC); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, username, role, expires_at FROM sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		var exp string
		if err := rows.Scan(&sess.ID, &sess.Username, &sess.Role, &exp); err != nil {
			return nil, err
		}
		sess.ExpiresAt = parseSessionTime(exp)
		out = append(out, sess)
	}
	return out, rows.Err()
}

func parseSessionTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ensure database/sql import is used even if the file is trimmed later.
var _ = sql.ErrNoRows
