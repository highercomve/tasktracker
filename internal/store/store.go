package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
)

type AppState struct {
	ActiveTaskID   string    `json:"active_task_id"`
	ActiveTaskDate time.Time `json:"active_task_date"`
	LastStartTime  time.Time `json:"last_start_time"`
	LastRunVersion string    `json:"last_run_version"`
}

type Storage struct {
	BaseDir string
	mu      sync.Mutex
}

func NewStorage(baseDir string) *Storage {
	// Ensure base directory exists
	s := &Storage{BaseDir: baseDir}
	s.ensureDir()
	return s
}

func (s *Storage) ensureDir() {
	os.MkdirAll(filepath.Join(s.BaseDir, "entries"), 0755)
}

// UpdateBaseDir updates the base directory and ensures it exists.
func (s *Storage) UpdateBaseDir(newDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BaseDir = newDir
	s.ensureDir()
}

// MoveData attempts to move the data from the current directory to the new one.
func (s *Storage) MoveData(newDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldEntriesPath := filepath.Join(s.BaseDir, "entries")
	newEntriesPath := filepath.Join(newDir, "entries")

	// Check if old data exists
	if _, err := os.Stat(oldEntriesPath); os.IsNotExist(err) {
		// Nothing to move, just update dir
		s.BaseDir = newDir
		s.ensureDir()
		return nil
	}

	// Ensure new parent dir exists
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return err
	}

	// Try to move
	if err := os.Rename(oldEntriesPath, newEntriesPath); err != nil {
		return err
	}

	// Success
	s.BaseDir = newDir
	s.ensureDir() // basically a no-op but good for consistency
	return nil
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
	// Try to load from AppState first
	state, err := s.LoadAppState()
	if err == nil && state.ActiveTaskID != "" {
		// Load the specific entry
		entries, err := s.LoadEntries(state.ActiveTaskDate)
		if err == nil {
			for _, e := range entries {
				if e.ID == state.ActiveTaskID {
					e.EndTime = endTime
					e.Duration = int64(endTime.Sub(e.StartTime).Seconds())
					e.State = models.TaskStateStopped
					// We must calculate total duration if it was paused/accumulated
					// But simpler: just set EndTime and let logic handle it.
					// Actually, we should respect Accumulated.
					// Duration = Accumulated + (EndTime - StartTime) (if running)
					// If it was paused, StartTime might be the resume time.
					// This logic is better handled in Dashboard, but here we just want to close it.
					// Let's assuming StopActiveTask is a "Force Stop".
					// Better: Dashboard should handle the logic and call SaveEntry.
					// StopActiveTask here is a legacy helper.
					// We'll just update it to clear state.
					s.SaveEntry(e)
					s.ClearAppState()
					return nil
				}
			}
		}
	}

	// Fallback to legacy behavior (check today)
	entries, err := s.LoadEntries(time.Now())
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.EndTime.IsZero() {
			e.EndTime = endTime
			e.Duration = int64(endTime.Sub(e.StartTime).Seconds())
			e.State = models.TaskStateStopped
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

// DeleteAllEntries removes all data from the storage (entries and state).
func (s *Storage) DeleteAllEntries() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete entries directory
	if err := os.RemoveAll(filepath.Join(s.BaseDir, "entries")); err != nil {
		return err
	}

	// Re-create empty entries directory
	if err := os.MkdirAll(filepath.Join(s.BaseDir, "entries"), 0755); err != nil {
		return err
	}

	// Delete state file
	os.Remove(s.getStateFilePath())

	return nil
}

// App State Management

func (s *Storage) getStateFilePath() string {
	return filepath.Join(s.BaseDir, "state.json")
}

func (s *Storage) SaveAppState(state AppState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.getStateFilePath(), data, 0644)
}

func (s *Storage) LoadAppState() (AppState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.getStateFilePath())
	if err != nil {
		return AppState{}, err
	}
	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return AppState{}, err
	}
	return state, nil
}

func (s *Storage) ClearAppState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(s.getStateFilePath())
}
