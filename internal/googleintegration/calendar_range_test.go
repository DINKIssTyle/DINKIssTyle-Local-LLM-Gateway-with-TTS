package googleintegration

import (
	"testing"
	"time"
)

func TestCalendarRelativeRanges(t *testing.T) {
	now := time.Date(2026, 11, 18, 16, 37, 0, 0, time.FixedZone("KST", 9*3600))
	for _, tc := range []struct{ period, start, end string }{
		{"", "2026-11-18T16:37:00+09:00", "2026-11-25T16:37:00+09:00"},
		{"today", "2026-11-18T00:00:00+09:00", "2026-11-19T00:00:00+09:00"},
		{"tomorrow", "2026-11-19T00:00:00+09:00", "2026-11-20T00:00:00+09:00"},
		{"this_week", "2026-11-16T00:00:00+09:00", "2026-11-23T00:00:00+09:00"},
		{"next_week", "2026-11-23T00:00:00+09:00", "2026-11-30T00:00:00+09:00"},
	} {
		start, end, err := calendarRange(Arguments{Period: tc.period, TimeZone: "Asia/Seoul"}, now)
		if err != nil || start != tc.start || end != tc.end {
			t.Fatalf("%s: %s %s %v", tc.period, start, end, err)
		}
	}
}
func TestCalendarRangeUsesCalendarZoneAndDST(t *testing.T) {
	now := time.Date(2026, 3, 8, 17, 0, 0, 0, time.UTC)
	start, end, err := calendarRange(Arguments{Period: "today", TimeZone: "America/New_York"}, now)
	if err != nil || start != "2026-03-08T00:00:00-05:00" || end != "2026-03-09T00:00:00-04:00" {
		t.Fatalf("DST day boundaries: %s %s %v", start, end, err)
	}
	a := Arguments{TimeMin: "2026-12-31T00:00:00+09:00", TimeMax: "2027-01-01T00:00:00+09:00"}
	start, end, err = calendarRange(a, now)
	if err != nil || start != a.TimeMin || end != a.TimeMax {
		t.Fatal("explicit range changed", err)
	}
	for _, invalid := range []Arguments{{TimeZone: "unknown"}, {TimeMin: "bad", TimeMax: a.TimeMax}, {TimeMin: a.TimeMax, TimeMax: a.TimeMin}} {
		if _, _, err = calendarRange(invalid, now); err == nil {
			t.Fatalf("invalid range accepted: %+v", invalid)
		}
	}
}
