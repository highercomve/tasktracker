package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type Dashboard struct {
	storage   *store.Storage
	timerData binding.String
	taskList  []models.TimeEntry
	// todayEntries is today's list before the search filter
	todayEntries []models.TimeEntry

	// State - protected by mu
	mu                  sync.RWMutex
	activeID            string
	activeOriginalStart time.Time
	activeLastStart     time.Time
	accumulated         int64
	activeState         int
	lastActivity        time.Time
	isIdleDialogShowing bool
	stopTicker          chan struct{}

	// UI
	startBtn      *widget.Button
	pauseBtn      *widget.Button
	taskEntry     *widget.Entry
	searchEntry   *widget.Entry
	projectSelect *widget.Select
	categoryEntry *widget.Entry
	refreshList   func()
	projects      []models.Project

	// Status shown above the timer and in the app bar
	statusSwatch *Swatch
	statusLabel  *widget.Label
	badge        *fyne.Container
	badgeSwatch  *Swatch
}

// Colours of the status dot for a running and a paused task.
const (
	stateColorRunning = "#10B981"
	stateColorPaused  = "#F59E0B"
)

func NewDashboard(s *store.Storage) *Dashboard {
	return &Dashboard{
		storage:      s,
		timerData:    binding.NewString(),
		lastActivity: time.Now(),
	}
}

// Safe accessor methods for protected state
func (d *Dashboard) GetActiveState() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeState
}

func (d *Dashboard) SetActiveState(state int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeState = state
}

func (d *Dashboard) GetActiveID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeID
}

func (d *Dashboard) SetActiveID(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeID = id
}

func (d *Dashboard) GetLastActivity() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastActivity
}

func (d *Dashboard) SetLastActivity(t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastActivity = t
}

func (d *Dashboard) IsIdleDialogShowing() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isIdleDialogShowing
}

func (d *Dashboard) SetIsIdleDialogShowing(showing bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isIdleDialogShowing = showing
}

func (d *Dashboard) GetAccumulated() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.accumulated
}

func (d *Dashboard) SetAccumulated(acc int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.accumulated = acc
}

// safeGetMainWindow returns the main window or nil if no windows are available
func safeGetMainWindow() fyne.Window {
	windows := fyne.CurrentApp().Driver().AllWindows()
	if len(windows) == 0 {
		return nil
	}
	return windows[0]
}

func (d *Dashboard) RegisterActivity() {
	d.SetLastActivity(time.Now())
}

func (d *Dashboard) checkIdle() {
	if d.GetActiveState() != models.TaskStateRunning || d.IsIdleDialogShowing() {
		return
	}

	if !viper.GetBool("idle_detection") {
		return
	}

	threshold := viper.GetInt("idle_threshold")
	if threshold <= 0 {
		threshold = 5 // Default 5 minutes
	}

	idleTime := time.Since(d.GetLastActivity())
	if idleTime > time.Duration(threshold)*time.Minute {
		d.SetIsIdleDialogShowing(true)

		parentWindow := safeGetMainWindow()
		if parentWindow == nil {
			return
		}

		// Round idle time to minutes for the message
		idleMinutes := int(idleTime.Minutes())

		msg := fmt.Sprintf(lang.L("idle_detected_msg"), idleMinutes)

		dialog.ShowCustomConfirm(
			lang.L("idle_detected_title"),
			lang.L("keep_idle_time"),
			lang.L("discard_idle_time"),
			widget.NewLabel(msg),
			func(keep bool) {
				d.SetIsIdleDialogShowing(false)
				d.RegisterActivity() // Reset activity after dialog

				if !keep {
					// Discard idle time
					// Subtract idleTime from the current run
					// A simpler way: set activeLastStart to now
					d.mu.Lock()
					d.activeLastStart = time.Now()
					d.mu.Unlock()
					d.refreshList()
				}
			},
			parentWindow,
		)
	}
}

func (d *Dashboard) MakeUI() fyne.CanvasObject {
	d.timerData.Set("00:00:00")

	// Timer: large monospace digits with the state of the active task above.
	timerLabel := widget.NewLabelWithData(d.timerData)
	timerLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	timerLabel.Alignment = fyne.TextAlignCenter
	timerLabel.SizeName = sizeNameTimer

	d.statusSwatch = NewSwatch("", 10)
	d.statusLabel = widget.NewLabel(lang.L("ready_to_track"))
	d.statusLabel.Importance = widget.LowImportance
	statusRow := container.NewCenter(container.NewHBox(container.NewCenter(d.statusSwatch), d.statusLabel))

	// Load projects
	projects, err := d.storage.LoadProjects()
	if err == nil {
		d.projects = projects
	}

	// Input
	d.taskEntry = widget.NewEntry()
	d.taskEntry.PlaceHolder = lang.L("what_working_on")

	// Category Entry
	d.categoryEntry = widget.NewEntry()
	d.categoryEntry.PlaceHolder = lang.L("category_hint")

	// Project Selection with color indicator
	d.projectSelect = widget.NewSelect(d.projectOptions(), nil)
	d.projectSelect.SetSelected(lang.L("none"))
	d.projectSelect.PlaceHolder = lang.L("select_project")

	projectColorIndicator := NewSwatch("", 12)

	// Update color indicator when project changes
	d.projectSelect.OnChanged = func(selected string) {
		projectColorIndicator.SetHex(d.projectColor(d.projectIDByName(selected)))
	}

	// Buttons
	d.startBtn = widget.NewButtonWithIcon(lang.L("start"), theme.MediaPlayIcon(), nil)
	d.pauseBtn = widget.NewButtonWithIcon(lang.L("pause"), theme.MediaPauseIcon(), nil)
	d.pauseBtn.Disable() // Initially disabled

	d.startBtn.OnTapped = func() {
		d.RegisterActivity()
		state := d.GetActiveState()
		if state == models.TaskStateRunning || state == models.TaskStatePaused {
			// Stop
			d.StopTask()
			d.taskEntry.SetText("")
			d.categoryEntry.SetText("")
			d.projectSelect.SetSelected(lang.L("none"))
		} else {
			// Start
			if d.taskEntry.Text == "" {
				return
			}
			projectID := d.getSelectedProjectID()
			tags := d.parseCategoryInput()
			d.StartTask(d.taskEntry.Text, projectID, tags)
			d.taskEntry.SetText("")
			d.categoryEntry.SetText("")
			d.projectSelect.SetSelected(lang.L("none"))
		}
		d.refreshList()
	}

	d.pauseBtn.OnTapped = func() {
		d.RegisterActivity()
		state := d.GetActiveState()
		if state == models.TaskStateRunning {
			d.PauseTask()
		} else if state == models.TaskStatePaused {
			d.ResumeTask()
		}
		d.refreshList()
	}

	d.taskEntry.OnSubmitted = func(text string) {
		d.RegisterActivity()
		if text == "" {
			return
		}
		projectID := d.getSelectedProjectID()
		tags := d.parseCategoryInput()
		d.StartTask(text, projectID, tags)
		d.taskEntry.SetText("")
		d.categoryEntry.SetText("")
		d.projectSelect.SetSelected(lang.L("none"))
		d.refreshList()
	}

	d.taskEntry.OnChanged = func(s string) {
		d.RegisterActivity()
	}

	// Search
	d.searchEntry = widget.NewEntry()
	d.searchEntry.PlaceHolder = lang.L("search_tasks")
	d.searchEntry.ActionItem = widget.NewIcon(theme.SearchIcon())
	d.searchEntry.OnChanged = func(s string) {
		d.refreshList()
	}

	// List
	simpleList := widget.NewList(
		func() int { return len(d.taskList) },
		func() fyne.CanvasObject { return newTaskRow() },
		func(i int, o fyne.CanvasObject) {
			// Safety check
			if i >= len(d.taskList) {
				return
			}
			entry := d.taskList[len(d.taskList)-1-i] // Reverse order
			row := o.(*taskRow)

			row.title.SetText(entry.Description)
			row.swatch.SetHex(d.projectColor(entry.ProjectID))
			row.meta.SetText(entryMeta(entry, d.projectName(entry.ProjectID), "15:04"))

			// Calculate duration for display
			activeID := d.GetActiveID()
			activeState := d.GetActiveState()
			if entry.ID == activeID {
				// Active task - use in-memory state for live update
				accumulated := d.GetAccumulated()
				currentDur := time.Duration(accumulated) * time.Second
				if activeState == models.TaskStateRunning {
					d.mu.RLock()
					lastStart := d.activeLastStart
					d.mu.RUnlock()
					currentDur += time.Since(lastStart)
				}
				row.setDuration(utils.FormatDuration(currentDur), true)
				row.edit.Disable()
			} else {
				// History items
				if entry.State == models.TaskStatePaused {
					row.setDuration(utils.FormatDuration(time.Duration(entry.Accumulated)*time.Second), true)
					row.edit.Disable()
				} else if entry.State == models.TaskStateRunning {
					// Should technically not happen for non-active tasks unless multiple running (bug)
					// or if activeID mismatch.
					row.setDuration(lang.L("running"), true)
					row.edit.Disable()
				} else {
					row.setDuration(utils.FormatDuration(time.Duration(entry.Duration)*time.Second), false)
					row.edit.Enable()
				}
			}

			row.edit.OnTapped = func() {
				d.showEditDialog(entry)
			}
			row.del.OnTapped = func() {
				parentWindow := safeGetMainWindow()
				if parentWindow == nil {
					return
				}
				dialog.ShowConfirm(lang.L("confirm_deletion"), lang.L("confirm_delete_task"), func(confirmed bool) {
					if !confirmed {
						return
					}

					// If deleting the active task, clear the active state
					if entry.ID == d.GetActiveID() {
						d.storage.ClearAppState()
						d.SetActiveID("")
						d.SetActiveState(models.TaskStateStopped)
						d.SetAccumulated(0)
						d.timerData.Set("00:00:00")
						d.updateButtons()
					}

					d.storage.DeleteEntry(entry)
					d.refreshList()
				}, parentWindow)
			}
		},
	)
	// Rows are not selectable; the buttons carry the actions.
	simpleList.OnSelected = func(id widget.ListItemID) { simpleList.UnselectAll() }

	summaryLabel := captionLabel("")
	emptyToday := emptyState(theme.HistoryIcon(), lang.L("no_tasks_today"), lang.L("no_tasks_today_hint"))
	emptySearch := emptyState(theme.SearchIcon(), lang.L("no_matching_tasks"), "")

	d.refreshList = func() {
		// Load today or active date?
		// If active task is from yesterday, we might want to see it.
		// But dashboard usually shows "Today".
		// Let's stick to Today for the list.
		all, _ := d.storage.LoadEntries(time.Now())
		entries := all
		if d.searchEntry.Text != "" {
			entries = service.FilterTasks(entries, d.searchEntry.Text)
		}
		d.taskList = entries
		d.todayEntries = all
		simpleList.Refresh()

		var total time.Duration
		for _, e := range all {
			if e.State == models.TaskStateStopped || !e.EndTime.IsZero() {
				total += time.Duration(e.Duration) * time.Second
			}
		}
		summaryLabel.SetText(fmt.Sprintf(lang.L("today_summary"), len(all), utils.FormatDuration(total)))

		emptyToday.Hide()
		emptySearch.Hide()
		if len(entries) == 0 {
			if len(all) == 0 {
				emptyToday.Show()
			} else {
				emptySearch.Show()
			}
		}
		d.updateButtons()
	}

	// Ticker with lifecycle management
	d.stopTicker = make(chan struct{})
	// Keep a copy: StopTicker clears the field (under the lock) when closing it.
	stop := d.stopTicker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fyne.Do(func() {
					activeState := d.GetActiveState()
					if activeState == models.TaskStateRunning {
						accumulated := d.GetAccumulated()
						d.mu.RLock()
						lastStart := d.activeLastStart
						d.mu.RUnlock()
						dur := time.Duration(accumulated)*time.Second + time.Since(lastStart)
						d.timerData.Set(utils.FormatDuration(dur))
						d.checkIdle()
					} else if activeState == models.TaskStatePaused {
						accumulated := d.GetAccumulated()
						d.timerData.Set(utils.FormatDuration(time.Duration(accumulated) * time.Second))
					} else {
						// Stopped
						d.timerData.Set("00:00:00")
					}
					simpleList.Refresh()
				})
			}
		}
	}()

	// Project selector with color indicator
	projectSelectorWithColor := container.NewBorder(nil, nil,
		container.NewCenter(Inset(4, projectColorIndicator)), nil, d.projectSelect)

	// Category input with icon
	categoryWithIcon := container.NewBorder(nil, nil, widget.NewIcon(theme.ListIcon()), nil, d.categoryEntry)

	// Compact input row for project and category
	inputDetailsRow := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewIcon(theme.FolderIcon()), nil, projectSelectorWithColor),
		categoryWithIcon,
	)

	// Main input area with task entry and buttons
	taskInputRow := container.NewBorder(nil, nil, nil, container.NewHBox(d.startBtn, d.pauseBtn), d.taskEntry)

	timerBox := container.NewVBox(statusRow, timerLabel)
	timerBox.Layout = &tightVBox{gap: -12}
	timerCard := NewSurface(container.NewVBox(
		timerBox,
		widget.NewSeparator(),
		taskInputRow,
		inputDetailsRow,
	))

	todayTitle := container.NewVBox(sectionTitle(lang.L("today")), summaryLabel)
	todayTitle.Layout = &tightVBox{gap: -12}
	listHeader := container.NewBorder(nil, nil, todayTitle, container.NewCenter(minWidth(200, d.searchEntry)))

	listCard := newSurfaceWithInset(container.NewStack(simpleList, emptyToday, emptySearch), 4)

	// Check for active task on load
	d.checkForActiveTask()
	d.refreshList() // Initial load

	return Inset(12, container.NewBorder(
		container.NewVBox(timerCard, Inset(2, listHeader)),
		nil, nil, nil,
		listCard,
	))
}

// StatusBadge is the compact timer shown in the app bar, so a running task is
// visible from every tab. It is hidden while nothing is tracked.
func (d *Dashboard) StatusBadge() fyne.CanvasObject {
	d.badgeSwatch = NewSwatch(stateColorRunning, 10)
	timeLabel := widget.NewLabelWithData(d.timerData)
	timeLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	d.badge = container.NewHBox(container.NewCenter(d.badgeSwatch), timeLabel)
	d.updateButtons()
	// Keep the space reserved while hidden, so showing the badge needs no
	// relayout of the app bar.
	reserve := canvas.NewRectangle(color.Transparent)
	reserve.SetMinSize(fyne.NewSize(120, timeLabel.MinSize().Height))
	return container.NewStack(reserve, d.badge)
}

// ReloadProjects picks up projects added or edited on the Projects tab.
func (d *Dashboard) ReloadProjects() {
	projects, err := d.storage.LoadProjects()
	if err != nil {
		return
	}
	d.projects = projects
	if d.projectSelect == nil {
		return
	}
	selected := d.projectSelect.Selected
	d.projectSelect.SetOptions(d.projectOptions())
	found := false
	for _, o := range d.projectSelect.Options {
		if o == selected {
			found = true
			break
		}
	}
	if !found {
		d.projectSelect.SetSelected(lang.L("none"))
	}
	if d.refreshList != nil {
		d.refreshList()
	}
}

func (d *Dashboard) projectOptions() []string {
	options := []string{lang.L("none")}
	for _, p := range d.projects {
		options = append(options, p.Name)
	}
	return options
}

func (d *Dashboard) projectIDByName(name string) string {
	for _, p := range d.projects {
		if p.Name == name {
			return p.ID
		}
	}
	return ""
}

func (d *Dashboard) projectName(id string) string {
	if id == "" {
		return ""
	}
	for _, p := range d.projects {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}

func (d *Dashboard) projectColor(id string) string {
	if id == "" {
		return ""
	}
	for _, p := range d.projects {
		if p.ID == id {
			return p.ColorHex
		}
	}
	return ""
}

// StopTicker stops the background ticker goroutine to prevent memory leaks.
// Call this when the Dashboard is being destroyed or rebuilt.
func (d *Dashboard) StopTicker() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopTicker != nil {
		close(d.stopTicker)
		d.stopTicker = nil
	}
}

// showSaveError displays an error dialog when saving fails.
func (d *Dashboard) showSaveError(err error) {
	parentWindow := safeGetMainWindow()
	if parentWindow == nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	dialog.ShowError(fmt.Errorf("%s: %w", lang.L("save_error"), err), parentWindow)
}

func (d *Dashboard) SetupShortcuts(w fyne.Window) {
	// Start/Stop Timer: Ctrl+S
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}, func(shortcut fyne.Shortcut) {
		d.startBtn.OnTapped()
	})

	// Pause/Resume Timer: Ctrl+P
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierControl}, func(shortcut fyne.Shortcut) {
		d.pauseBtn.OnTapped()
	})

	// Focus New Task: Ctrl+N
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl}, func(shortcut fyne.Shortcut) {
		w.Canvas().Focus(d.taskEntry)
	})

	// Show Window: Ctrl+Shift+W (to avoid conflict with common close window)
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyW, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(shortcut fyne.Shortcut) {
		w.Show()
	})
}

func (d *Dashboard) updateButtons() {
	if d.startBtn == nil {
		return
	}
	activeState := d.GetActiveState()
	if activeState == models.TaskStateRunning {
		d.startBtn.SetText(lang.L("stop"))
		d.startBtn.SetIcon(theme.MediaStopIcon())
		d.startBtn.Importance = widget.DangerImportance
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("pause"))
		d.pauseBtn.SetIcon(theme.MediaPauseIcon())
		d.pauseBtn.Enable()
	} else if activeState == models.TaskStatePaused {
		d.startBtn.SetText(lang.L("stop"))
		d.startBtn.SetIcon(theme.MediaStopIcon())
		d.startBtn.Importance = widget.DangerImportance
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("resume"))
		d.pauseBtn.SetIcon(theme.MediaPlayIcon())
		d.pauseBtn.Enable()
	} else {
		d.startBtn.SetText(lang.L("start"))
		d.startBtn.SetIcon(theme.MediaPlayIcon())
		d.startBtn.Importance = widget.HighImportance
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("pause"))
		d.pauseBtn.SetIcon(theme.MediaPauseIcon())
		d.pauseBtn.Disable()
	}
	d.startBtn.Refresh()
	d.updateStatus(activeState)
}

// updateStatus shows what is being tracked above the timer and in the app bar.
func (d *Dashboard) updateStatus(state int) {
	desc := d.activeDescription()
	status, dot := lang.L("ready_to_track"), ""
	switch state {
	case models.TaskStateRunning:
		status, dot = lang.L("tracking"), stateColorRunning
	case models.TaskStatePaused:
		status, dot = lang.L("paused"), stateColorPaused
	}
	if dot != "" && desc != "" {
		status += ": " + desc
	}
	if d.statusLabel != nil {
		d.statusSwatch.SetHex(dot)
		d.statusLabel.SetText(status)
	}
	if d.badge != nil {
		if dot == "" {
			d.badge.Hide()
		} else {
			d.badgeSwatch.SetHex(dot)
			d.badge.Show()
		}
	}
}

// activeDescription returns the description of the active task, if it is in
// today's list.
func (d *Dashboard) activeDescription() string {
	id := d.GetActiveID()
	if id == "" {
		return ""
	}
	for _, e := range d.todayEntries {
		if e.ID == id {
			return e.Description
		}
	}
	return ""
}

func (d *Dashboard) checkForActiveTask() {
	// Try LoadAppState first
	state, err := d.storage.LoadAppState()
	if err == nil && state.ActiveTaskID != "" {
		entries, _ := d.storage.LoadEntries(state.ActiveTaskDate)
		for _, e := range entries {
			if e.ID == state.ActiveTaskID {
				d.SetActiveID(e.ID)
				d.mu.Lock()
				d.activeOriginalStart = e.StartTime
				d.activeLastStart = state.LastStartTime
				d.mu.Unlock()
				d.SetAccumulated(e.Accumulated)
				state := e.State
				// If state is blank (legacy), assume running
				if state == 0 {
					state = models.TaskStateRunning
				}
				d.SetActiveState(state)

				d.updateButtons()
				return
			}
		}
		// Entry not found - log and clear state
		fmt.Printf("warning: active task %s not found; clearing state\n", state.ActiveTaskID)
		d.storage.ClearAppState()
	}

	// Fallback: Check today's entries for any running task (legacy support)
	entries, _ := d.storage.LoadEntries(time.Now())
	for _, e := range entries {
		if e.EndTime.IsZero() {
			d.SetActiveID(e.ID)
			d.mu.Lock()
			d.activeOriginalStart = e.StartTime
			d.activeLastStart = e.StartTime // Assume started just now if legacy? Or original start.
			d.mu.Unlock()
			d.SetAccumulated(0)
			d.SetActiveState(models.TaskStateRunning)

			// Save migrated state
			d.saveState()
			d.updateButtons()
			return
		}
	}

	d.SetActiveState(models.TaskStateStopped)
	d.updateButtons()
}

func (d *Dashboard) saveState() {
	d.mu.RLock()
	activeID := d.activeID
	activeOriginalStart := d.activeOriginalStart
	activeLastStart := d.activeLastStart
	d.mu.RUnlock()

	d.storage.SaveAppState(store.AppState{
		ActiveTaskID:   activeID,
		ActiveTaskDate: activeOriginalStart,
		LastStartTime:  activeLastStart,
	})
}

func (d *Dashboard) updateActiveEntry() error {
	// Helper to update the persistent entry with current in-memory values (accumulated, state)
	d.mu.RLock()
	activeOriginalStart := d.activeOriginalStart
	activeID := d.activeID
	activeState := d.activeState
	accumulated := d.accumulated
	d.mu.RUnlock()

	entries, err := d.storage.LoadEntries(activeOriginalStart)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID == activeID {
			e.State = activeState
			e.Accumulated = accumulated
			// StartTime remains original
			return d.storage.SaveEntry(e)
		}
	}
	return nil
}

func (d *Dashboard) StartTask(desc, projectID string, tags []string) {
	// If another task is running, stop it
	if d.GetActiveID() != "" {
		d.StopTask()
	}

	now := time.Now()
	entry := models.TimeEntry{
		ID:          uuid.New().String(),
		Description: desc,
		ProjectID:   projectID,
		Tags:        tags,
		StartTime:   now,
		State:       models.TaskStateRunning,
		Accumulated: 0,
	}

	if err := d.storage.SaveEntry(entry); err != nil {
		d.showSaveError(err)
		return
	}

	d.SetActiveID(entry.ID)
	d.mu.Lock()
	d.activeOriginalStart = now
	d.activeLastStart = now
	d.mu.Unlock()
	d.SetAccumulated(0)
	d.SetActiveState(models.TaskStateRunning)

	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) PauseTask() {
	if d.GetActiveState() != models.TaskStateRunning {
		return
	}
	now := time.Now()

	d.mu.Lock()
	prevAccumulated := d.accumulated
	prevState := d.activeState
	prevLastStart := d.activeLastStart
	d.accumulated += int64(now.Sub(d.activeLastStart).Seconds())
	d.activeState = models.TaskStatePaused
	d.mu.Unlock()

	if err := d.updateActiveEntry(); err != nil {
		// Revert state on error
		d.mu.Lock()
		d.accumulated = prevAccumulated
		d.activeState = prevState
		d.activeLastStart = prevLastStart
		d.mu.Unlock()
		d.showSaveError(err)
		return
	}
	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) ResumeTask() {
	if d.GetActiveState() != models.TaskStatePaused {
		return
	}
	d.mu.Lock()
	prevLastStart := d.activeLastStart
	prevState := d.activeState

	d.activeLastStart = time.Now()
	d.activeState = models.TaskStateRunning
	d.mu.Unlock()

	if err := d.updateActiveEntry(); err != nil {
		// Revert state on error
		d.mu.Lock()
		d.activeLastStart = prevLastStart
		d.activeState = prevState
		d.mu.Unlock()
		d.showSaveError(err)
		return
	}
	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) TogglePause() {
	activeState := d.GetActiveState()
	if activeState == models.TaskStateRunning {
		d.PauseTask()
	} else if activeState == models.TaskStatePaused {
		d.ResumeTask()
	}
	d.refreshList()
}

func (d *Dashboard) StopTask() {
	if d.GetActiveID() == "" {
		return
	}

	now := time.Now()

	// Load entry to finalize
	d.mu.RLock()
	activeOriginalStart := d.activeOriginalStart
	activeID := d.activeID
	accumulated := d.accumulated
	activeState := d.activeState
	activeLastStart := d.activeLastStart
	d.mu.RUnlock()

	entries, err := d.storage.LoadEntries(activeOriginalStart)
	if err != nil {
		d.showSaveError(err)
		return
	}

	for _, e := range entries {
		if e.ID == activeID {
			// Calculate final duration
			finalDuration := accumulated
			if activeState == models.TaskStateRunning {
				finalDuration += int64(now.Sub(activeLastStart).Seconds())
			}

			e.EndTime = now
			e.Duration = finalDuration
			e.State = models.TaskStateStopped
			e.Accumulated = accumulated // Optional: keep this for record

			if err := d.storage.SaveEntry(e); err != nil {
				d.showSaveError(err)
				return
			}
			break
		}
	}

	// Clear state
	d.storage.ClearAppState()
	d.SetActiveID("")
	d.SetActiveState(models.TaskStateStopped)
	d.SetAccumulated(0)
	d.timerData.Set("00:00:00")

	d.updateButtons()
	d.refreshList()
}

func (d *Dashboard) showEditDialog(entry models.TimeEntry) {
	descEntry := widget.NewEntry()
	descEntry.SetText(entry.Description)

	tagsEntry := widget.NewEntry()
	tagsEntry.SetPlaceHolder(lang.L("category_hint"))
	if len(entry.Tags) > 0 {
		tagsEntry.SetText(fmt.Sprintf("%s", entry.Tags[0]))
		if len(entry.Tags) > 1 {
			for i := 1; i < len(entry.Tags); i++ {
				tagsEntry.SetText(fmt.Sprintf("%s, %s", tagsEntry.Text, entry.Tags[i]))
			}
		}
	}

	// Project selection dropdown
	projectOptions := []string{lang.L("none")}
	selectedProjectIndex := 0
	for i, p := range d.projects {
		projectOptions = append(projectOptions, p.Name)
		if p.ID == entry.ProjectID {
			selectedProjectIndex = i + 1 // +1 because "None" is at index 0
		}
	}
	projectSelect := widget.NewSelect(projectOptions, nil)
	if selectedProjectIndex < len(projectOptions) {
		projectSelect.SetSelectedIndex(selectedProjectIndex)
	}

	startEntry := widget.NewEntry()
	startEntry.SetText(entry.StartTime.Format("2006-01-02 15:04:05"))

	endEntry := widget.NewEntry()
	if !entry.EndTime.IsZero() {
		endEntry.SetText(entry.EndTime.Format("2006-01-02 15:04:05"))
	}

	items := []*widget.FormItem{
		widget.NewFormItem(lang.L("task_description"), descEntry),
		widget.NewFormItem(lang.L("project"), projectSelect),
		widget.NewFormItem(lang.L("add_category"), tagsEntry),
		widget.NewFormItem(lang.L("start_time"), startEntry),
		widget.NewFormItem(lang.L("end_time"), endEntry),
	}

	parentWindow := safeGetMainWindow()
	if parentWindow == nil {
		return
	}
	dlg := dialog.NewForm(lang.L("edit_task"), lang.L("save"), lang.L("cancel"), items, func(b bool) {
		if !b {
			return
		}

		newDesc := descEntry.Text
		newStart, err1 := time.Parse("2006-01-02 15:04:05", startEntry.Text)
		newEnd, err2 := time.Parse("2006-01-02 15:04:05", endEntry.Text)

		if err1 != nil || (endEntry.Text != "" && err2 != nil) {
			// Show error? For now just return
			fmt.Println(lang.L("error_parsing_time"))
			return
		}

		// Parse tags from comma-separated input
		var newTags []string
		if tagsEntry.Text != "" {
			for _, tag := range strings.Split(tagsEntry.Text, ",") {
				trimmed := strings.TrimSpace(tag)
				if trimmed != "" {
					newTags = append(newTags, trimmed)
				}
			}
		}

		// Get selected project ID
		newProjectID := ""
		if projectSelect.Selected != "" && projectSelect.Selected != lang.L("none") {
			for _, p := range d.projects {
				if p.Name == projectSelect.Selected {
					newProjectID = p.ID
					break
				}
			}
		}

		// Update entry
		oldEntry := entry
		entry.Description = newDesc
		entry.Tags = newTags
		entry.ProjectID = newProjectID
		entry.StartTime = newStart
		if endEntry.Text != "" {
			entry.EndTime = newEnd
			entry.Duration = int64(newEnd.Sub(newStart).Seconds())
			entry.State = models.TaskStateStopped
		}

		// If start date changed, we need to delete old and save new
		if oldEntry.StartTime.Format("2006-01-02") != entry.StartTime.Format("2006-01-02") {
			d.storage.DeleteEntry(oldEntry)
		}

		d.storage.SaveEntry(entry)
		d.refreshList()
	}, parentWindow)
	dlg.Resize(fyne.NewSize(parentWindow.Canvas().Size().Width, dlg.MinSize().Height))
	dlg.Show()
}

// getSelectedProjectID returns the project ID for the currently selected project.
// Returns empty string if "None" is selected.
func (d *Dashboard) getSelectedProjectID() string {
	selected := d.projectSelect.Selected
	if selected == lang.L("none") || selected == "" {
		return ""
	}

	// Find the project by name and return its ID
	for _, p := range d.projects {
		if p.Name == selected {
			return p.ID
		}
	}

	// Default to empty if project not found
	return ""
}

// parseCategoryInput parses comma-separated tags from the category entry
func (d *Dashboard) parseCategoryInput() []string {
	text := d.categoryEntry.Text
	if text == "" {
		return nil
	}

	var tags []string
	for _, tag := range strings.Split(text, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}
