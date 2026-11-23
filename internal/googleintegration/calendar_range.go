package googleintegration

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // Calendar zones must also resolve on systems without zoneinfo.
)

var singleWeekdaySchedule = regexp.MustCompile(`^(?:(?:다가오는|오는)\s*)?([일월화수목금토])요일(?:에는|에|은|는)?(?:\s*(?:(?:내|제)\s*)?일정(?:을|은|이)?(?:\s*(?:확인(?:해\s*줘|해\s*주세요)?|알려\s*(?:줘|주세요)|뭐야|있어))?)?[\s?!.]*$`)

// RepairWeekdaySearch narrows a calendar call for an unambiguous weekday-only
// question. Explicit dates, ranges and pagination retain their own parameters.
func RepairWeekdaySearch(question, raw string) string {
	match := singleWeekdaySchedule.FindStringSubmatch(strings.TrimSpace(question))
	if match == nil {
		return raw
	}
	var args Arguments
	if json.Unmarshal([]byte(raw), &args) != nil || args.PageToken != "" || args.TimeMin != "" || args.TimeMax != "" {
		return raw
	}
	day := strings.Index("일월화수목금토", match[1]) / len("일")
	period := strings.ToLower(time.Weekday(day).String())
	if args.Period == period {
		return raw
	}
	args.Period = period
	b, _ := json.Marshal(args)
	return string(b)
}

func weekdayPeriod(period string) (time.Weekday, bool) {
	for weekday := time.Sunday; weekday <= time.Saturday; weekday++ {
		if strings.EqualFold(weekday.String(), period) {
			return weekday, true
		}
	}
	return 0, false
}

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
			weekday, ok := weekdayPeriod(a.Period)
			if !ok {
				return "", "", fmt.Errorf("unknown calendar period")
			}
			daysAhead := (int(weekday) - int(midnight.Weekday()) + 7) % 7
			start = midnight.AddDate(0, 0, daysAhead)
			end = start.AddDate(0, 0, 1)
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
