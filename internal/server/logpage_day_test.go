package server

import (
	"testing"
	"time"
)

func TestDayLabel(t *testing.T) {
	now := time.Date(2026, 10, 10, 15, 0, 0, 0, time.Local)
	cases := map[time.Time]string{
		now.Add(-2 * time.Hour): "Today · 2026-10-10",
		now.AddDate(0, 0, -1):   "Yesterday · 2026-10-09",
		now.AddDate(0, 0, -5):   "Mon 5 Oct 2026",
	}
	for in, want := range cases {
		if got := dayLabel(in, now); got != want {
			t.Errorf("dayLabel(%v) = %q, want %q", in, got, want)
		}
	}
}
