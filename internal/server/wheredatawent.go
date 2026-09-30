package server

import (
	"sort"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/router"
)

// DataFlowRow is one line of "Where your data went": a sensitivity class and
// how many of its prompts stayed local, went to the cloud, or were blocked.
type DataFlowRow struct {
	Sensitivity string `json:"sensitivity"` // "none" for prompts without sensitive data
	Local       int    `json:"local"`
	Cloud       int    `json:"cloud"`
	Blocked     int    `json:"blocked"`
	// Unknown counts old rows whose destination can no longer be derived
	// (model and provider both gone) — shown honestly, never as cloud.
	Unknown int `json:"unknown,omitempty"`
}

// Total is the row's request count.
func (r DataFlowRow) Total() int { return r.Local + r.Cloud + r.Blocked + r.Unknown }

// DataFlowView is the dashboard's lead section (ISSUE-116): the CISO question
// "where did our AI data go?" answered per sensitivity class. Counts and
// classes only — never prompt content.
type DataFlowView struct {
	Local   int           `json:"local"`
	Cloud   int           `json:"cloud"`
	Blocked int           `json:"blocked"`
	Unknown int           `json:"unknown,omitempty"`
	Rows    []DataFlowRow `json:"rows"`
	// PII is the personal-data row (zero-valued when none was seen), lifted
	// out for the "personal data" card.
	PII DataFlowRow `json:"pii"`
}

// Total is the number of classified requests.
func (v DataFlowView) Total() int { return v.Local + v.Cloud + v.Blocked + v.Unknown }

// Pct returns n as a whole percentage of the total (0 when empty).
func (v DataFlowView) Pct(n int) int {
	t := v.Total()
	if t == 0 {
		return 0
	}
	return (n*100 + t/2) / t
}

// sensitivityOrder follows the classifier's ranking (most sensitive first);
// "none" is always last. Unknown classes sort after the known ones.
var sensitivityOrder = map[string]int{
	string(router.SensitivitySecretsPossible):    0,
	string(router.SensitivityPII):                1,
	string(router.SensitivitySecurityClassified): 2,
	string(router.SensitivityHealth):             3,
	string(router.SensitivityFinancial):          4,
	string(router.SensitivityLegal):              5,
	string(router.SensitivitySourceCode):         6,
}

func sensitivityRank(s string) int {
	if s == "none" {
		return 100
	}
	if r, ok := sensitivityOrder[s]; ok {
		return r
	}
	return 50
}

// buildDataFlow aggregates history rows (or, without a data dir, the
// in-memory log) into the data-flow view. Rows recorded before egress was
// stored are classified from the model's current tags — the same rule the
// response header uses.
func buildDataFlow(h *history.Store, recent []eventlog.RequestLogRecord, eng *engine.Engine) DataFlowView {
	var rows []history.EgressRow
	if h != nil {
		rows = h.EgressRows()
	} else {
		for _, r := range recent {
			rows = append(rows, history.EgressRow{Blocked: r.Blocked, Egress: r.Egress, Sensitivity: r.Sensitivity, Model: r.Model, Provider: r.Provider, Count: 1})
		}
	}
	legacy := ChatOptions{Engine: eng}
	by := map[string]*DataFlowRow{}
	var v DataFlowView
	for _, r := range rows {
		sens := r.Sensitivity
		if sens == "" {
			sens = "none"
		}
		row := by[sens]
		if row == nil {
			row = &DataFlowRow{Sensitivity: sens}
			by[sens] = row
		}
		switch {
		case r.Blocked:
			row.Blocked += r.Count
			v.Blocked += r.Count
		default:
			egress := r.Egress
			if egress == "" {
				egress = legacy.legacyEgress(r.Model, r.Provider)
			}
			switch egress {
			case "local":
				row.Local += r.Count
				v.Local += r.Count
			case "cloud":
				row.Cloud += r.Count
				v.Cloud += r.Count
			default:
				row.Unknown += r.Count
				v.Unknown += r.Count
			}
		}
	}
	for _, row := range by {
		v.Rows = append(v.Rows, *row)
	}
	sort.Slice(v.Rows, func(i, j int) bool {
		ri, rj := sensitivityRank(v.Rows[i].Sensitivity), sensitivityRank(v.Rows[j].Sensitivity)
		if ri != rj {
			return ri < rj
		}
		return v.Rows[i].Sensitivity < v.Rows[j].Sensitivity
	})
	if p, ok := by[string(router.SensitivityPII)]; ok {
		v.PII = *p
	}
	return v
}
