package service

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/highercomve/tasktracker/internal/models"
)

// CreateProject creates a new project with the given name, description, and optional color.
// It generates a unique ID and sets creation/update timestamps.
func CreateProject(name, description, colorHex string) models.Project {
	now := time.Now()
	return models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		ColorHex:    colorHex,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// UpdateProject updates an existing project's mutable fields.
// It preserves the original ID and CreatedAt, but updates the UpdatedAt timestamp.
func UpdateProject(project *models.Project, name, description, colorHex string) {
	project.Name = name
	project.Description = description
	project.ColorHex = colorHex
	project.UpdatedAt = time.Now()
}

// FindProjectByID returns a project by its ID, or nil if not found.
func FindProjectByID(projects []models.Project, id string) *models.Project {
	for i := range projects {
		if projects[i].ID == id {
			return &projects[i]
		}
	}
	return nil
}

// FindProjectByName returns a project by its name, or nil if not found.
// This search is case-sensitive.
func FindProjectByName(projects []models.Project, name string) *models.Project {
	for i := range projects {
		if projects[i].Name == name {
			return &projects[i]
		}
	}
	return nil
}

// DeleteProject removes a project from the slice by ID.
// Returns the updated slice and a boolean indicating if the project was found and deleted.
func DeleteProject(projects []models.Project, id string) ([]models.Project, bool) {
	for i, p := range projects {
		if p.ID == id {
			return append(projects[:i], projects[i+1:]...), true
		}
	}
	return projects, false
}

// SortProjectsByName returns projects sorted alphabetically by name.
func SortProjectsByName(projects []models.Project) []models.Project {
	sorted := make([]models.Project, len(projects))
	copy(sorted, projects)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// SortProjectsByCreatedAt returns projects sorted by creation date (newest first).
func SortProjectsByCreatedAt(projects []models.Project) []models.Project {
	sorted := make([]models.Project, len(projects))
	copy(sorted, projects)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

// GetProjectTotals calculates total time spent on each project from entries.
// Returns a map of project ID to total duration.
func GetProjectTotals(entries []models.TimeEntry) map[string]time.Duration {
	totals := make(map[string]time.Duration)

	for _, e := range entries {
		var dur time.Duration
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		} else {
			dur = time.Duration(e.Duration) * time.Second
		}

		projectID := e.ProjectID
		if projectID == "" {
			projectID = "unassigned"
		}

		totals[projectID] += dur
	}

	return totals
}

// FilterByProject returns entries matching the specified project ID.
// If projectID is empty or "All", returns all entries.
// If projectID is "unassigned", returns entries without a project.
func FilterByProject(entries []models.TimeEntry, projectID string) []models.TimeEntry {
	if projectID == "" || projectID == "All" {
		return entries
	}

	var filtered []models.TimeEntry
	for _, e := range entries {
		if projectID == "unassigned" {
			// Filter for unassigned (no project) entries
			if e.ProjectID == "" {
				filtered = append(filtered, e)
			}
		} else if e.ProjectID == projectID {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// ExtractProjectIDs returns unique project IDs from entries, sorted.
// Excludes empty project IDs.
func ExtractProjectIDs(entries []models.TimeEntry) []string {
	projectMap := make(map[string]bool)
	var projectIDs []string

	for _, e := range entries {
		if e.ProjectID != "" && !projectMap[e.ProjectID] {
			projectMap[e.ProjectID] = true
			projectIDs = append(projectIDs, e.ProjectID)
		}
	}

	sort.Strings(projectIDs)
	return projectIDs
}

// GetProjectsWithStats returns a map of project IDs to their stats (count of entries and total duration).
// Useful for display in project management UI.
type ProjectStats struct {
	ProjectID   string
	Name        string
	EntryCount  int
	TotalTime   time.Duration
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func GetProjectsWithStats(projects []models.Project, entries []models.TimeEntry) []ProjectStats {
	totals := GetProjectTotals(entries)
	entryCounts := make(map[string]int)

	for _, e := range entries {
		projectID := e.ProjectID
		if projectID == "" {
			projectID = "unassigned"
		}
		entryCounts[projectID]++
	}

	var stats []ProjectStats
	for _, p := range projects {
		stats = append(stats, ProjectStats{
			ProjectID:   p.ID,
			Name:        p.Name,
			EntryCount:  entryCounts[p.ID],
			TotalTime:   totals[p.ID],
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	// Sort by name for consistency
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Name < stats[j].Name
	})

	return stats
}

// ValidateProject checks if a project has valid required fields.
// Returns an error string if invalid, or empty string if valid.
func ValidateProject(project *models.Project) string {
	if project.Name == "" {
		return "project name is required"
	}
	if project.ID == "" {
		return "project ID is required"
	}
	return ""
}
