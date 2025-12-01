package models

import "time"

// TimeEntry represents a single unit of work.
type TimeEntry struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	ProjectID   string    `json:"project_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`     // Zero if running
	Duration    int64     `json:"duration_sec"` // Calculated on stop
	Tags        []string  `json:"tags"`
}

// Project represents a client or category.
type Project struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Client   string `json:"client"`
	ColorHex string `json:"color_hex"`
}
