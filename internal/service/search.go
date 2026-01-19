package service

import (
	"strings"

	"github.com/highercomve/tasktracker/internal/models"
)

// FilterTasks filters entries based on a search query.
// It searches in Description, Tags, and ProjectID.
func FilterTasks(entries []models.TimeEntry, query string) []models.TimeEntry {
	if query == "" {
		return entries
	}

	query = strings.ToLower(query)
	var filtered []models.TimeEntry

	for _, e := range entries {
		// Check Description
		if strings.Contains(strings.ToLower(e.Description), query) {
			filtered = append(filtered, e)
			continue
		}

		// Check ProjectID
		if strings.Contains(strings.ToLower(e.ProjectID), query) {
			filtered = append(filtered, e)
			continue
		}

		// Check Tags
		foundInTags := false
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				foundInTags = true
				break
			}
		}
		if foundInTags {
			filtered = append(filtered, e)
			continue
		}
	}

	return filtered
}
