package googleintegration

import (
	"fmt"
	"time"
	_ "time/tzdata" // Calendar zones must also resolve on systems without zoneinfo.
)

func calendarRange(a Arguments, now time.Time) (string, string, error) {
	startText, endText := a.TimeMin, a.TimeMax
	if startText == "" || endText == "" {
		loc, err := time.LoadLocation(a.TimeZone)
		if err != nil || a.TimeZone == "" {
			return "", "", fmt.Errorf("a valid calendar time zone is required for relative dates")
		}
		now = now.In(loc)
		start := now
		end := now.AddDate(0, 0, 7)
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		switch a.Period {
		case "", "upcoming":
		case "today":
			start = midnight
			end = start.AddDate(0, 0, 1)
		case "tomorrow":
			start = midnight.AddDate(0, 0, 1)
			end = start.AddDate(0, 0, 1)
		case "this_week", "next_week":
			daysSinceMonday := (int(midnight.Weekday()) + 6) % 7
			start = midnight.AddDate(0, 0, -daysSinceMonday)
			if a.Period == "next_week" {
				start = start.AddDate(0, 0, 7)
			}
			end = start.AddDate(0, 0, 7)
		default:
			return "", "", fmt.Errorf("unknown calendar period")
		}
		if startText == "" {
			startText = start.Format(time.RFC3339)
		}
		if endText == "" {
			if a.TimeMin != "" {
				parsed, e := time.Parse(time.RFC3339, a.TimeMin)
				if e != nil {
					return "", "", fmt.Errorf("time_min must be RFC3339 with a timezone offset")
				}
				end = parsed.In(loc).AddDate(0, 0, 7)
			}
			endText = end.Format(time.RFC3339)
		}
	}
	start, e1 := time.Parse(time.RFC3339, startText)
	end, e2 := time.Parse(time.RFC3339, endText)
	if e1 != nil || e2 != nil || !end.After(start) {
		return "", "", fmt.Errorf("time_min and time_max must be RFC3339 timestamps with offsets, and time_max must be later; omit both for the next 7 days or use period:today/tomorrow")
	}
	return startText, endText, nil
}
