package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
)

const (
	GroupByNone        = "None"
	GroupByDay         = "Daily"
	GroupByWeek        = "Weekly"
	GroupByWeekOfMonth = "WeeklyOfMonth"
	GroupByCategory    = "Category"
	GroupByProject     = "Project"
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

// ExtractCategories returns unique primary categories (first tag) from entries
func ExtractCategories(entries []models.TimeEntry) []string {
	categoryMap := make(map[string]bool)
	var categories []string

	for _, e := range entries {
		if len(e.Tags) > 0 && e.Tags[0] != "" {
			if !categoryMap[e.Tags[0]] {
				categoryMap[e.Tags[0]] = true
				categories = append(categories, e.Tags[0])
			}
		}
	}

	// Sort for consistent ordering
	sort.Strings(categories)
	return categories
}

// FilterByCategory returns entries matching the specified category
func FilterByCategory(entries []models.TimeEntry, category string) []models.TimeEntry {
	if category == "" || category == "All" {
		return entries
	}

	var filtered []models.TimeEntry
	for _, e := range entries {
		var entryCategory string
		if len(e.Tags) > 0 && e.Tags[0] != "" {
			entryCategory = e.Tags[0]
		} else {
			entryCategory = "Untagged"
		}

		if entryCategory == category {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// GetCategoryTotals returns time duration per category
func GetCategoryTotals(entries []models.TimeEntry) map[string]time.Duration {
	totals := make(map[string]time.Duration)

	for _, e := range entries {
		var category string
		if len(e.Tags) > 0 && e.Tags[0] != "" {
			category = e.Tags[0]
		} else {
			category = "Untagged"
		}

		var dur time.Duration
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		} else {
			dur = time.Duration(e.Duration) * time.Second
		}

		totals[category] += dur
	}

	return totals
}

// GroupByProjectID groups entries by their project ID
// Returns a map where key is project ID and value is the list of entries
func GroupByProjectID(entries []models.TimeEntry) map[string][]models.TimeEntry {
	groups := make(map[string][]models.TimeEntry)

	for _, e := range entries {
		projectID := e.ProjectID
		if projectID == "" {
			projectID = "unassigned"
		}
		groups[projectID] = append(groups[projectID], e)
	}

	return groups
}

// GroupEntriesByProject groups entries by project ID with project details
// Returns a slice of maps containing project information and their associated entries
// Useful for displaying entries organized by project in the UI
func GroupEntriesByProject(entries []models.TimeEntry, projects []models.Project) map[string]map[string]interface{} {
	groups := make(map[string]map[string]interface{})
	projectMap := make(map[string]models.Project)

	// Build project lookup map for quick access
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	// Group entries by project ID
	for _, e := range entries {
		projectID := e.ProjectID
		if projectID == "" {
			projectID = "unassigned"
		}

		if _, exists := groups[projectID]; !exists {
			groups[projectID] = make(map[string]interface{})
			// Add project details if it exists
			if project, found := projectMap[projectID]; found {
				groups[projectID]["name"] = project.Name
				groups[projectID]["description"] = project.Description
				groups[projectID]["color_hex"] = project.ColorHex
				groups[projectID]["id"] = project.ID
			} else if projectID == "unassigned" {
				groups[projectID]["name"] = "Unassigned"
				groups[projectID]["description"] = "Tasks without a project"
				groups[projectID]["color_hex"] = ""
				groups[projectID]["id"] = "unassigned"
			}
		}

		// Append entry to the project's entries list
		if entries, ok := groups[projectID]["entries"].([]models.TimeEntry); ok {
			groups[projectID]["entries"] = append(entries, e)
		} else {
			groups[projectID]["entries"] = []models.TimeEntry{e}
		}
	}

	return groups
}
