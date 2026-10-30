package server

// The model panel (ISSUE-142): clicking a row on Models opens a side panel
// with everything about that one model — status, a live test, what its
// provider actually serves, its settings, and its recent traffic — without
// leaving the list. Saving goes through the existing model handlers; the two
// endpoints here only read: recent traffic and failures for one model, and
// the provider's own model list for the "Use this" choice.

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// panelWindow is how far back the panel's traffic and failures reach.
const panelWindow = 24 * time.Hour

// panelRequest is one recent request in the panel. Classification only,
// never content.
type panelRequest struct {
	Time    time.Time `json:"time"`
	Task    string    `json:"task"`
	Data    string    `json:"data"`
	Tokens  int       `json:"tokens"`
	CostUSD float64   `json:"cost_usd"`
}

type panelFailure struct {
	Time   time.Time `json:"time"`
	Reason string    `json:"reason"`
	Ms     int64     `json:"ms"`
}

type modelActivity struct {
	ModelID  string         `json:"model_id"`
	Count    int            `json:"count"` // requests in the window
	CostUSD  float64        `json:"cost_usd"`
	Recent   []panelRequest `json:"recent"`
	Failures []panelFailure `json:"failures"`
	Failed   int            `json:"failed"` // failures in the window
	Durable  bool           `json:"durable"`
}

// recentRows reads the request log the same way the log page does: durable
// history when there is a data dir, the in-memory ring otherwise.
func (o ModelsOptions) recentRows(limit int) ([]eventlog.RequestLogRecord, bool) {
	switch {
	case o.History != nil:
		return o.History.Recent(limit), true
	case o.RequestLog != nil:
		return o.RequestLog.Recent(limit), false
	}
	return nil, false
}

func (o ModelsOptions) activity(id string, now time.Time) modelActivity {
	a := modelActivity{ModelID: id, Recent: []panelRequest{}, Failures: []panelFailure{}}
	rows, durable := o.recentRows(2000)
	a.Durable = durable
	for _, r := range rows { // newest first
		if r.Model != id || r.Blocked || now.Sub(r.Time) > panelWindow {
			continue
		}
		a.Count++
		a.CostUSD += r.CostUSD
		if len(a.Recent) < 8 {
			a.Recent = append(a.Recent, panelRequest{Time: r.Time, Task: taskLabel(r.TaskType),
				Data: dataClassLabel(r.Sensitivity), Tokens: r.InputTokens + r.OutputTokens, CostUSD: r.CostUSD})
		}
	}
	if o.ErrorRing != nil {
		for _, e := range o.ErrorRing.Recent(0) {
			if e.ModelID != id || now.Sub(e.At) > panelWindow {
				continue
			}
			a.Failed++
			if len(a.Failures) < 5 {
				a.Failures = append(a.Failures, panelFailure{Time: e.At, Reason: humanize(e.ErrorCode), Ms: e.DurationMs})
			}
		}
	}
	return a
}

// ModelActivityHandler serves GET /router/models/activity?id=.
func ModelActivityHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing model id"})
			return
		}
		_ = json.NewEncoder(w).Encode(opts.activity(id, time.Now()))
	}
}

// ModelServedHandler serves POST /router/models/served: the models the row's
// provider lists on its own /models, so the panel can offer "Use this" for a
// renamed or swapped model. Only configured connections are asked (no
// arbitrary URLs, same stance as the Test button).
func ModelServedHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := strings.TrimSpace(r.FormValue("id"))
		w.Header().Set("Content-Type", "application/json")
		out := struct {
			ModelID  string   `json:"model_id"`
			Provider string   `json:"provider_id"`
			Slug     string   `json:"slug"`
			Served   []string `json:"served"`
			Error    string   `json:"error,omitempty"`
		}{ModelID: id, Served: []string{}}
		providerID, slug, _, found := opts.modelConn(id)
		if id == "" || !found {
			w.WriteHeader(http.StatusNotFound)
			out.Error = "unknown model id"
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		out.Provider, out.Slug = providerID, slug
		baseURL, keyEnv, ok := opts.providerConnByID(providerID)
		if !ok || baseURL == "" {
			out.Error = "no provider connection (base_url) for " + providerID
		} else if served, serr := opts.servedModels(r, baseURL, keyEnv); serr != "" {
			out.Error = serr
		} else if served != nil {
			out.Served = served
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}
