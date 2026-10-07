package server

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
)

// Dashboard charts (ISSUE-123): a 7-day timeline of where the data went and a
// flow picture of the totals. Rendered server-side as inline SVG — no chart
// library, no external asset, works without JavaScript; a small script adds
// the hover tooltip. Palette validated for the dark panel (#101c34): local
// #1fb054, cloud #3b82f6, blocked #ef4444 — identity is never color-alone
// (legend, direct labels, tooltip and a table view carry it too).
const (
	colLocal   = "#1fb054"
	colCloud   = "#3b82f6"
	colBlocked = "#ef4444"
	colGrid    = "#22324f"
	colInk     = "#e8eef7"
	colMuted   = "#8fa1bf"
	colSurface = "#101c34"
)

// DayFlow is one day of the timeline.
type DayFlow struct {
	Day     string `json:"day"` // YYYY-MM-DD (UTC)
	Local   int    `json:"local"`
	Cloud   int    `json:"cloud"`
	Blocked int    `json:"blocked"`
	Unknown int    `json:"unknown,omitempty"`
}

// Total is the day's request count.
func (d DayFlow) Total() int { return d.Local + d.Cloud + d.Blocked + d.Unknown }

// Label is the short axis label, e.g. "Mon 5".
func (d DayFlow) Label() string {
	t, err := time.Parse("2006-01-02", d.Day)
	if err != nil {
		return d.Day
	}
	return t.Format("Mon 2")
}

// buildTimeline returns the last `days` UTC days ending today, oldest first,
// with empty days filled in so the axis never skips a date.
func buildTimeline(h *history.Store, eng *engine.Engine, days int, now time.Time) []DayFlow {
	if h == nil || days <= 0 {
		return nil
	}
	today := now.UTC().Truncate(24 * time.Hour)
	start := today.AddDate(0, 0, -(days - 1))
	out := make([]DayFlow, days)
	idx := map[string]int{}
	for i := range out {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out[i] = DayFlow{Day: d}
		idx[d] = i
	}
	legacy := ChatOptions{Engine: eng}
	for _, r := range h.EgressRowsSince(start) {
		i, ok := idx[r.Day]
		if !ok {
			continue
		}
		switch {
		case r.Blocked:
			out[i].Blocked += r.Count
		default:
			eg := r.Egress
			if eg == "" {
				eg = legacy.legacyEgress(r.Model, r.Provider)
			}
			switch eg {
			case "local":
				out[i].Local += r.Count
			case "cloud":
				out[i].Cloud += r.Count
			default:
				out[i].Unknown += r.Count
			}
		}
	}
	return out
}

// niceMax rounds a maximum up to a clean, even axis top (so the midline is a
// whole number) without wasting more than ~a third of the plot.
func niceMax(v int) int {
	if v <= 4 {
		return 4
	}
	p := 1
	for p*10 <= v {
		p *= 10
	}
	for _, m := range []int{1, 2, 3, 4, 6, 8, 10} {
		if m*p >= v {
			return m * p
		}
	}
	return 10 * p
}

// timelineSVG renders the stacked 7-day columns: local at the baseline, then
// cloud, then blocked; a 2px surface gap between segments; a 4px rounded cap
// on the top segment only; the day total as the only direct label.
func timelineSVG(days []DayFlow) template.HTML {
	if len(days) == 0 {
		return ""
	}
	const W, H, left, right, top, bottom = 720.0, 210.0, 34.0, 8.0, 18.0, 26.0
	plotW, plotH := W-left-right, H-top-bottom
	maxV := 0
	for _, d := range days {
		if d.Total() > maxV {
			maxV = d.Total()
		}
	}
	yMax := niceMax(maxV)
	y := func(v float64) float64 { return top + plotH - v/float64(yMax)*plotH }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="tl" viewBox="0 0 %.0f %.0f" role="img" aria-label="Requests per day for the last %d days: local, cloud and blocked">`, W, H, len(days))
	for _, t := range []int{0, yMax / 2, yMax} {
		fmt.Fprintf(&b, `<line x1="%.0f" x2="%.0f" y1="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`, left, W-right, y(float64(t)), y(float64(t)), colGrid)
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="%s" font-size="11" text-anchor="end" dominant-baseline="middle">%d</text>`, left-8, y(float64(t)), colMuted, t)
	}
	slot := plotW / float64(len(days))
	bw := slot * 0.42
	if bw > 24 {
		bw = 24
	}
	for i, d := range days {
		cx := left + slot*(float64(i)+0.5)
		x := cx - bw/2
		tip := fmt.Sprintf("%s · %d requests · local %d · cloud %d · blocked %d", d.Label(), d.Total(), d.Local, d.Cloud, d.Blocked)
		if d.Unknown > 0 {
			tip += fmt.Sprintf(" · unknown %d", d.Unknown)
		}
		// Full-height invisible hit target: easier to hover than the marks.
		fmt.Fprintf(&b, `<g class="col" data-tip="%s"><rect x="%.1f" y="%.0f" width="%.1f" height="%.1f" fill="transparent"/>`, template.HTMLEscapeString(tip), cx-slot/2, top, slot, plotH)
		segs := []struct {
			v   int
			col string
		}{{d.Local, colLocal}, {d.Unknown, colMuted}, {d.Cloud, colCloud}, {d.Blocked, colBlocked}}
		last := -1
		for j, s := range segs {
			if s.v > 0 {
				last = j
			}
		}
		base := 0.0
		for j, s := range segs {
			if s.v == 0 {
				continue
			}
			y1 := y(base + float64(s.v))
			y0 := y(base)
			h := y0 - y1
			if base > 0 {
				h -= 2 // 2px surface gap above the segment below
			}
			if h < 1 {
				h = 1
			}
			if j == last && h > 4 {
				// rounded data-end (top), square at the baseline side
				r := 4.0
				fmt.Fprintf(&b, `<path d="M%.1f %.1f V%.1f Q%.1f %.1f %.1f %.1f H%.1f Q%.1f %.1f %.1f %.1f V%.1f Z" fill="%s"/>`,
					x, y1+h, y1+r, x, y1, x+r, y1, x+bw-r, x+bw, y1, x+bw, y1+r, y1+h, s.col)
			} else {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, x, y1, bw, h, s.col)
			}
			base += float64(s.v)
		}
		if d.Total() > 0 {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-size="11" text-anchor="middle">%d</text>`, cx, y(float64(d.Total()))-6, colInk, d.Total())
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" fill="%s" font-size="11" text-anchor="middle">%s</text></g>`, cx, H-8, colMuted, template.HTMLEscapeString(d.Label()))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// flowSVG renders the totals as a flow: all prompts on the left, split into
// ribbons to local / cloud / blocked on the right, ribbon width ∝ count.
func flowSVG(v DataFlowView) template.HTML {
	total := v.Total()
	if total == 0 {
		return ""
	}
	const W, H, nodeW, gap = 720.0, 170.0, 14.0, 12.0
	type out struct {
		label string
		n     int
		col   string
	}
	outs := []out{{"Stayed in the house", v.Local, colLocal}, {"Went to the cloud", v.Cloud, colCloud}, {"Blocked fail-closed", v.Blocked, colBlocked}}
	if v.Unknown > 0 {
		outs = append(outs, out{"Unknown (older rows)", v.Unknown, colMuted})
	}
	var shown []out
	for _, o := range outs {
		if o.n > 0 {
			shown = append(shown, o)
		}
	}
	usable := H - gap*float64(len(shown)-1)
	scale := usable / float64(total)
	leftX, rightX := 150.0, 470.0
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="flow" viewBox="0 0 %.0f %.0f" role="img" aria-label="Where %d prompts went: %d local, %d cloud, %d blocked">`, W, H+4, total, v.Local, v.Cloud, v.Blocked)
	// left node
	fmt.Fprintf(&b, `<rect x="%.0f" y="2" width="%.0f" height="%.0f" rx="3" fill="%s"/>`, leftX-nodeW, nodeW, usable, colMuted)
	fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="%s" font-size="13" text-anchor="end" dominant-baseline="middle"><tspan font-weight="700" font-size="18">%d</tspan></text>`, leftX-nodeW-10, 2+usable/2-9, colInk, total)
	fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="%s" font-size="12" text-anchor="end" dominant-baseline="middle">prompts classified</text>`, leftX-nodeW-10, 2+usable/2+11, colMuted)
	ly, ry := 2.0, 2.0
	for _, o := range shown {
		h := float64(o.n) * scale
		if h < 2 {
			h = 2
		}
		pct := (o.n*100 + total/2) / total
		tip := fmt.Sprintf("%s: %d of %d prompts (%d%%)", o.label, o.n, total, pct)
		mid := (leftX + rightX) / 2
		fmt.Fprintf(&b, `<g class="col" data-tip="%s">`, template.HTMLEscapeString(tip))
		fmt.Fprintf(&b, `<path d="M%.1f %.1f C%.1f %.1f %.1f %.1f %.1f %.1f V%.1f C%.1f %.1f %.1f %.1f %.1f %.1f Z" fill="%s" fill-opacity="0.28"/>`,
			leftX, ly, mid, ly, mid, ry, rightX, ry, ry+h, mid, ry+h, mid, ly+h, leftX, ly+h, o.col)
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.1f" width="%.0f" height="%.1f" rx="3" fill="%s"/>`, rightX, ry, nodeW, h, o.col)
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="%s" font-size="13" dominant-baseline="middle"><tspan font-weight="700">%d</tspan> <tspan fill="%s">· %d%% · %s</tspan></text></g>`,
			rightX+nodeW+10, ry+h/2, colInk, o.n, colMuted, pct, o.label)
		ly += float64(o.n) * scale
		ry += h + gap
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
