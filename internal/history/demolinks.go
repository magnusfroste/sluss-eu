package history

// Named demo links (ISSUE-089): each prospect gets their own link with a label,
// so "who has tried it" is attributable per recipient — the create+measure half
// of outreach (the CRM loop lives in an external CRM). Tokens are capabilities and
// individually revocable; opens/last_open are the confirmation signal.

import (
	"fmt"
	"time"
)

// DemoLink is one named, shareable demo link.
type DemoLink struct {
	Token      string    `json:"-"` // capability — listed only for the admin page URL
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"created_at"`
	DisabledAt time.Time `json:"disabled_at,omitempty"` // zero = active
	Opens      int64     `json:"opens"`
	LastOpenAt time.Time `json:"last_open_at,omitempty"` // zero = never
}

// Active reports whether the link is usable.
func (l DemoLink) Active() bool { return l.DisabledAt.IsZero() }

// CreateDemoLink stores a new named link.
func (s *Store) CreateDemoLink(token, label string, at time.Time) error {
	if s == nil {
		return fmt.Errorf("history: no store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO demo_links (token, label, created_at) VALUES (?, ?, ?)`,
		token, label, at.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("history: create demo link: %w", err)
	}
	return nil
}

// ListDemoLinks returns all links, newest first.
func (s *Store) ListDemoLinks() ([]DemoLink, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT token, label, created_at, disabled_at, opens, last_open_at
		FROM demo_links ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("history: list demo links: %w", err)
	}
	defer rows.Close()
	var out []DemoLink
	for rows.Next() {
		var l DemoLink
		var created, disabled, lastOpen string
		if err := rows.Scan(&l.Token, &l.Label, &created, &disabled, &l.Opens, &lastOpen); err != nil {
			return nil, fmt.Errorf("history: scan demo link: %w", err)
		}
		l.CreatedAt = parseTime(created)
		l.DisabledAt = parseTime(disabled)
		l.LastOpenAt = parseTime(lastOpen)
		out = append(out, l)
	}
	return out, rows.Err()
}

// DisableDemoLink revokes a link by token. Returns false when unknown.
func (s *Store) DisableDemoLink(token string, at time.Time) (bool, error) {
	if s == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE demo_links SET disabled_at = ? WHERE token = ? AND disabled_at = ''`,
		at.UTC().Format(time.RFC3339), token)
	if err != nil {
		return false, fmt.Errorf("history: disable demo link: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// TouchDemoLink validates an ACTIVE link by token, increments its open counter
// and stamps last_open. Returns the label and whether the token was valid.
func (s *Store) TouchDemoLink(token string, at time.Time) (string, bool) {
	if s == nil || token == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var label string
	err := s.db.QueryRow(`SELECT label FROM demo_links WHERE token = ? AND disabled_at = ''`, token).Scan(&label)
	if err != nil {
		return "", false
	}
	_, _ = s.db.Exec(`UPDATE demo_links SET opens = opens + 1, last_open_at = ? WHERE token = ?`,
		at.UTC().Format(time.RFC3339), token)
	return label, true
}
