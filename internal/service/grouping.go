package service

import (
	"fmt"
	"time"
)

const (
	GroupByNone        = "None"
	GroupByDay         = "Daily"
	GroupByWeek        = "Weekly"
	GroupByWeekOfMonth = "WeeklyOfMonth"
)

// Shared helper functions for grouping

func GetWeekOfMonth(t time.Time) int {
	year, month, _ := t.Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
	offset := int(firstOfMonth.Weekday())
	if offset == 0 {
		offset = 7
	}
	firstMonday := firstOfMonth.AddDate(0, 0, -offset+1)

	// Find t's Monday
	tOffset := int(t.Weekday())
	if tOffset == 0 {
		tOffset = 7
	}
	tMonday := t.AddDate(0, 0, -tOffset+1)

	weeks := int(tMonday.Sub(firstMonday).Hours()/24/7) + 1
	return weeks
}

func GetWeekRange(t time.Time) (time.Time, time.Time) {
	// Find t's Monday
	offset := int(t.Weekday())
	if offset == 0 {
		offset = 7
	}
	start := t.AddDate(0, 0, -offset+1)
	end := start.AddDate(0, 0, 6)
	return start, end
}

func GetGroupKey(t time.Time, groupBy string) string {
	if groupBy == GroupByDay {
		return t.Format("2006-01-02")
	} else if groupBy == GroupByWeek {
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	} else if groupBy == GroupByWeekOfMonth {
		year, month, _ := t.Date()
		week := GetWeekOfMonth(t)
		return fmt.Sprintf("%d-%02d-W%d", year, month, week)
	}
	return ""
}

func GetGroupTitle(t time.Time, groupBy string) string {
	if groupBy == GroupByDay {
		return t.Format("Monday, 02 Jan 2006")
	} else if groupBy == GroupByWeek {
		start, end := GetWeekRange(t)
		return fmt.Sprintf("%s - %s", start.Format("Jan 02"), end.Format("Jan 02, 2006"))
	} else if groupBy == GroupByWeekOfMonth {
		start, end := GetWeekRange(t)

		// Clamp to month of t
		year, month, _ := t.Date()
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

		if start.Before(firstOfMonth) {
			start = firstOfMonth
		}
		if end.After(lastOfMonth) {
			end = lastOfMonth
		}

		return fmt.Sprintf("%s - %s", start.Format("Jan 02"), end.Format("Jan 02, 2006"))
	}
	return ""
}
