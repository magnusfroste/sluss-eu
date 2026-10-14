package history

// Device coverage (ISSUE-128a): the rollout kit's coverage script runs on
// every managed desktop (via Intune, Jamf or any MDM that runs scripts) and
// reports whether the device can still reach generative-AI providers directly,
// and whether it reaches Sluss. One row per device, latest report wins. Only
// hostname, OS and two yes/no answers — never a user, never content.

import (
	"fmt"
	"time"
)

// CoverageReport is one device's latest answer.
type CoverageReport struct {
	Device     string // hostname as the script saw it
	OS         string
	TenantID   string // the department whose key the script used
	DirectAI   string // "open" | "closed": can a provider API be reached directly?
	Sluss      string // "ok" | "unreachable"
	ReportedAt time.Time
}

// CoverageSummary is the dashboard's number.
type CoverageSummary struct {
	Devices    int       // devices that have reported at all
	DirectOpen int       // devices where a direct provider path is still open
	SlussDown  int       // devices that could not reach Sluss
	Latest     time.Time // most recent report; zero when none
}

// UpsertCoverage records a device's latest report, replacing the previous one.
func (s *Store) UpsertCoverage(c CoverageReport) error {
	if s == nil {
		return fmt.Errorf("history: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO coverage_devices
		(device, os, tenant_id, direct_ai, sluss, reported_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.Device, c.OS, c.TenantID, c.DirectAI, c.Sluss, c.ReportedAt.UTC().Format(time.RFC3339))
	return err
}

// ListCoverage returns every device, most recent report first.
func (s *Store) ListCoverage() ([]CoverageReport, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT device, os, tenant_id, direct_ai, sluss, reported_at
		FROM coverage_devices ORDER BY reported_at DESC, device`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageReport
	for rows.Next() {
		var c CoverageReport
		var at string
		if err := rows.Scan(&c.Device, &c.OS, &c.TenantID, &c.DirectAI, &c.Sluss, &at); err != nil {
			return nil, err
		}
		c.ReportedAt = parseSessionTime(at)
		out = append(out, c)
	}
	return out, rows.Err()
}

// SummarizeCoverage counts devices, open direct paths and unreachable Sluss.
func (s *Store) SummarizeCoverage() CoverageSummary {
	var sum CoverageSummary
	rows, err := s.ListCoverage()
	if err != nil {
		return sum
	}
	for _, c := range rows {
		sum.Devices++
		if c.DirectAI == "open" {
			sum.DirectOpen++
		}
		if c.Sluss != "ok" {
			sum.SlussDown++
		}
		if c.ReportedAt.After(sum.Latest) {
			sum.Latest = c.ReportedAt
		}
	}
	return sum
}
