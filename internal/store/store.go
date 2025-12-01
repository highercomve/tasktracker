package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-tracker/internal/models"
)

type Storage struct {
	BaseDir string
	mu      sync.Mutex
}

func NewStorage(baseDir string) *Storage {
	// Ensure base directory exists
	os.MkdirAll(filepath.Join(baseDir, "entries"), 0755)
	return &Storage{BaseDir: baseDir}
}

// getEntryFilePath returns the path for a specific date's entry file.
func (s *Storage) getEntryFilePath(date time.Time) string {
	filename := date.Format("2006-01-02") + ".json"
	return filepath.Join(s.BaseDir, "entries", filename)
}

// LoadEntries loads entries for a specific date.
func (s *Storage) LoadEntries(date time.Time) ([]models.TimeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.getEntryFilePath(date)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.TimeEntry{}, nil
		}
		return nil, err
	}

	var entries []models.TimeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// SaveEntry saves or updates an entry.
// For simplicity, we load the day's file, update/append, and save back.
// In a real app, we might optimize this.
func (s *Storage) SaveEntry(entry models.TimeEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine file based on StartTime
	path := s.getEntryFilePath(entry.StartTime)

	// Load existing
	data, err := os.ReadFile(path)
	var entries []models.TimeEntry
	if err == nil {
		json.Unmarshal(data, &entries)
	}

	// Update or Append
	found := false
	for i, e := range entries {
		if e.ID == entry.ID {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}

	// Save
	newData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

// StopActiveTask stops any active task across all known recent files?
// Or just today? Usually today.
// But if a task started yesterday and is still running?
// We should probably track the "Active Task" separately or search for it.
// For MVP, let's assume it's in today's file or we pass it explicitly.
// Or we can have a separate "state.json" for app state including active task ID.
// Let's implement a simple "StopActiveTask" that checks today.
func (s *Storage) StopActiveTask(endTime time.Time) error {
	// Load today's entries
	entries, err := s.LoadEntries(time.Now())
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.EndTime.IsZero() {
			e.EndTime = endTime
			e.Duration = int64(endTime.Sub(e.StartTime).Seconds())
			return s.SaveEntry(e)
		}
	}
	return nil
}

// LoadEntriesForRange loads entries for a date range (inclusive).
func (s *Storage) LoadEntriesForRange(start, end time.Time) ([]models.TimeEntry, error) {
	var allEntries []models.TimeEntry

	// Normalize start and end to beginning/end of day if needed,
	// but here we assume caller handles it or we just iterate days.
	// Let's iterate day by day.
	current := start
	for !current.After(end) {
		entries, err := s.LoadEntries(current)
		if err != nil {
			// Ignore error? Or log? For now, continue.
		} else {
			allEntries = append(allEntries, entries...)
		}
		current = current.AddDate(0, 0, 1)
	}
	return allEntries, nil
}

// DeleteEntry removes an entry from the storage.
func (s *Storage) DeleteEntry(entry models.TimeEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.getEntryFilePath(entry.StartTime)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var entries []models.TimeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	newEntries := []models.TimeEntry{}
	for _, e := range entries {
		if e.ID != entry.ID {
			newEntries = append(newEntries, e)
		}
	}

	newData, err := json.MarshalIndent(newEntries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}
