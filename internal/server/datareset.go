package server

// Audited demo-data reset (ISSUE-096). A demo/pilot instance accumulates stats
// that the next demo shouldn't see. The reset clears OPERATIONAL data only —
// request history, spend/savings, demo chat sessions, in-memory rings/counters —
// and NEVER the audit chain, API keys, users or the roster. The twist that makes
// it credible in an evidence product: the reset itself is recorded in the
// tamper-evident audit chain — you can delete data, but never the fact that you
// deleted it.

import (
	"net/http"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/spend"
)

// actionDataReset audits an operational data reset.
const actionDataReset = audit.Action("data.demo_reset")

// DataResetOptions carries everything the reset clears. All optional.
type DataResetOptions struct {
	History     *history.Store
	Spend       *spend.Tracker
	RequestLog  *eventlog.RequestLogTracker
	Comparisons *eventlog.ComparisonTracker
	Errors      *eventlog.ErrorRing
	Auditor     audit.Sink
}

// DataResetHandler performs the audited reset and bounces back to the policy
// console with a notice.
func DataResetHandler(o DataResetOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rows int64
		if o.History != nil {
			if n, err := o.History.ResetRequests(); err == nil {
				rows = n
			}
			// Demo chat sessions are a KV blob; clearing it resets the demo UI.
			_ = o.History.KVSet(demoSessionsKey, "")
		}
		if o.Spend != nil {
			o.Spend.Reset()
		}
		if o.RequestLog != nil {
			o.RequestLog.Reset()
		}
		if o.Comparisons != nil {
			o.Comparisons.Reset()
		}
		if o.Errors != nil {
			o.Errors.Reset()
		}
		// The reset is itself evidence: who, when, what scope. Never deletable.
		audit.Record(r.Context(), o.Auditor, audit.Entry{
			Action: actionDataReset,
			Actor:  AdminUserFromContext(r.Context()),
			Target: "operational-data",
			Reason: "request history, spend, demo sessions, in-memory counters",
			Detail: map[string]string{"request_rows_deleted": itoa64(rows)},
		})
		http.Redirect(w, r, "/router/policy?ok=Operational%20data%20reset%20(recorded%20in%20the%20audit%20chain).", http.StatusSeeOther)
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
