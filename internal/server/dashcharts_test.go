package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
)

func TestNiceMax(t *testing.T) {
	for in, want := range map[int]int{0: 4, 3: 4, 7: 8, 12: 20, 29: 30, 31: 40, 55: 60, 99: 100, 410: 600} {
		if got := niceMax(in); got != want {
			t.Errorf("niceMax(%d) = %d, want %d", in, got, want)
		}
	}
}

// ISSUE-123: the 7-day timeline buckets history per UTC day, fills empty
// days, and the charts render with totals, labels and tooltips.
func TestTimelineAndFlowCharts(t *testing.T) {
	h, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	add := func(id string, at time.Time, egress string, blocked bool) {
		h.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, SelectedModel: "m", Egress: egress, Blocked: blocked, DecidedAt: at}})
	}
	add("a", now, "local", false)
	add("b", now, "cloud", false)
	add("c", now.AddDate(0, 0, -2), "", true)
	add("old", now.AddDate(0, 0, -9), "cloud", false) // outside the window

	days := buildTimeline(h, nil, 7, now)
	if len(days) != 7 || days[0].Day != "2026-10-01" || days[6].Day != "2026-10-07" {
		t.Fatalf("window = %v", days)
	}
	if d := days[6]; d.Local != 1 || d.Cloud != 1 || d.Blocked != 0 {
		t.Fatalf("today = %+v", d)
	}
	if days[4].Blocked != 1 || days[3].Total() != 0 {
		t.Fatalf("2 days ago / empty day = %+v / %+v", days[4], days[3])
	}
	svg := string(timelineSVG(days))
	for _, want := range []string{`role="img"`, "Wed 7", `data-tip="Wed 7 · 2 requests · local 1 · cloud 1 · blocked 0"`, colLocal, colBlocked} {
		if !strings.Contains(svg, want) {
			t.Errorf("timeline missing %q", want)
		}
	}
	flow := string(flowSVG(DataFlowView{Local: 2, Cloud: 1, Blocked: 1}))
	for _, want := range []string{"prompts classified", "Stayed in the house", "Blocked fail-closed", "50%"} {
		if !strings.Contains(flow, want) {
			t.Errorf("flow missing %q", want)
		}
	}
	if flowSVG(DataFlowView{}) != "" {
		t.Error("no traffic → no flow chart (the page shows its empty state)")
	}
}
