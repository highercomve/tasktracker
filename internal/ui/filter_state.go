package ui

import (
	"encoding/json"
	"sync"

	"fyne.io/fyne/v2"
)

// FilterState represents the current state of report filters
type FilterState struct {
	SearchQuery      string `json:"search_query"`
	SelectedCategory string `json:"selected_category"`
	SelectedProject  string `json:"selected_project"`
	GroupBy          string `json:"group_by"`
	PanelExpanded    bool   `json:"panel_expanded"`
}

// FilterStateManager manages filter state persistence
type FilterStateManager struct {
	app      fyne.App
	prefKey  string
	state    FilterState
	mu       sync.RWMutex
	onChange []func(FilterState)
}

// NewFilterStateManager creates a new filter state manager
func NewFilterStateManager(app fyne.App, tabName string) *FilterStateManager {
	fsm := &FilterStateManager{
		app:      app,
		prefKey:  "filter_state_" + tabName,
		onChange: make([]func(FilterState), 0),
	}
	fsm.load()
	return fsm
}

// load loads the state from preferences
func (fsm *FilterStateManager) load() {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	prefs := fsm.app.Preferences()
	data := prefs.String(fsm.prefKey)
	if data != "" {
		json.Unmarshal([]byte(data), &fsm.state)
	}
}

// save saves the state to preferences
func (fsm *FilterStateManager) save() {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	data, err := json.Marshal(fsm.state)
	if err != nil {
		return
	}
	prefs := fsm.app.Preferences()
	prefs.SetString(fsm.prefKey, string(data))
}

// GetState returns a copy of the current state
func (fsm *FilterStateManager) GetState() FilterState {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.state
}

// SetSearchQuery sets the search query
func (fsm *FilterStateManager) SetSearchQuery(query string) {
	fsm.mu.Lock()
	fsm.state.SearchQuery = query
	fsm.mu.Unlock()
	fsm.save()
	fsm.notifyChange()
}

// SetSelectedCategory sets the selected category
func (fsm *FilterStateManager) SetSelectedCategory(category string) {
	fsm.mu.Lock()
	fsm.state.SelectedCategory = category
	fsm.mu.Unlock()
	fsm.save()
	fsm.notifyChange()
}

// SetSelectedProject sets the selected project
func (fsm *FilterStateManager) SetSelectedProject(project string) {
	fsm.mu.Lock()
	fsm.state.SelectedProject = project
	fsm.mu.Unlock()
	fsm.save()
	fsm.notifyChange()
}

// SetGroupBy sets the group by option
func (fsm *FilterStateManager) SetGroupBy(groupBy string) {
	fsm.mu.Lock()
	fsm.state.GroupBy = groupBy
	fsm.mu.Unlock()
	fsm.save()
	fsm.notifyChange()
}

// SetPanelExpanded sets the panel expanded state
func (fsm *FilterStateManager) SetPanelExpanded(expanded bool) {
	fsm.mu.Lock()
	fsm.state.PanelExpanded = expanded
	fsm.mu.Unlock()
	fsm.save()
	// Don't notify on panel state change to avoid refresh loops
}

// ClearFilters resets all filters to default values
func (fsm *FilterStateManager) ClearFilters(defaultCategory string) {
	fsm.mu.Lock()
	fsm.state.SearchQuery = ""
	fsm.state.SelectedCategory = defaultCategory
	fsm.state.SelectedProject = ""
	fsm.state.GroupBy = ""
	fsm.mu.Unlock()
	fsm.save()
	fsm.notifyChange()
}

// OnChange registers a callback for state changes
func (fsm *FilterStateManager) OnChange(callback func(FilterState)) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.onChange = append(fsm.onChange, callback)
}

// notifyChange notifies all registered callbacks
func (fsm *FilterStateManager) notifyChange() {
	fsm.mu.RLock()
	state := fsm.state
	callbacks := fsm.onChange
	fsm.mu.RUnlock()

	for _, cb := range callbacks {
		cb(state)
	}
}

// HasActiveFilters returns true if any filters are active
func (fsm *FilterStateManager) HasActiveFilters(defaultCategory string, defaultProject string) bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	return fsm.state.SearchQuery != "" ||
		(fsm.state.SelectedCategory != "" && fsm.state.SelectedCategory != defaultCategory) ||
		(fsm.state.SelectedProject != "" && fsm.state.SelectedProject != defaultProject)
}

// GetActiveFilterCount returns the number of active filters
func (fsm *FilterStateManager) GetActiveFilterCount(defaultCategory string, defaultProject string) int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	count := 0
	if fsm.state.SearchQuery != "" {
		count++
	}
	if fsm.state.SelectedCategory != "" && fsm.state.SelectedCategory != defaultCategory {
		count++
	}
	if fsm.state.SelectedProject != "" && fsm.state.SelectedProject != defaultProject {
		count++
	}
	return count
}
