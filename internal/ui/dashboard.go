package ui

import (
	"fmt"
	"image/color"
	"strings"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type Dashboard struct {
	storage   *store.Storage
	timerData binding.String
	taskList  []models.TimeEntry

	// State
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
}

func NewDashboard(s *store.Storage) *Dashboard {
	return &Dashboard{
		storage:      s,
		timerData:    binding.NewString(),
		lastActivity: time.Now(),
	}
}

func (d *Dashboard) RegisterActivity() {
	d.lastActivity = time.Now()
}

func (d *Dashboard) checkIdle() {
	if d.activeState != models.TaskStateRunning || d.isIdleDialogShowing {
		return
	}

	if !viper.GetBool("idle_detection") {
		return
	}

	threshold := viper.GetInt("idle_threshold")
	if threshold <= 0 {
		threshold = 5 // Default 5 minutes
	}

	idleTime := time.Since(d.lastActivity)
	if idleTime > time.Duration(threshold)*time.Minute {
		d.isIdleDialogShowing = true

		parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]

		// Round idle time to minutes for the message
		idleMinutes := int(idleTime.Minutes())

		msg := fmt.Sprintf(lang.L("idle_detected_msg"), idleMinutes)

		dialog.ShowCustomConfirm(
			lang.L("idle_detected_title"),
			lang.L("keep_idle_time"),
			lang.L("discard_idle_time"),
			widget.NewLabel(msg),
			func(keep bool) {
				d.isIdleDialogShowing = false
				d.RegisterActivity() // Reset activity after dialog

				if !keep {
					// Discard idle time
					// Subtract idleTime from the current run
					// A simpler way: set activeLastStart to now
					d.activeLastStart = time.Now()
					// We might also want to subtract any accumulated time if the idle period spanned across multiple runs,
					// but usually idle is within the current run.
					d.refreshList()
				}
			},
			parentWindow,
		)
	}
}

func (d *Dashboard) MakeUI() fyne.CanvasObject {
	d.timerData.Set("00:00:00")

	// Timer Label
	timerLabel := widget.NewLabelWithData(d.timerData)
	timerLabel.TextStyle = fyne.TextStyle{Bold: true}
	timerLabel.Alignment = fyne.TextAlignCenter

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
	projectOptions := []string{lang.L("none")}
	for _, p := range d.projects {
		projectOptions = append(projectOptions, p.Name)
	}
	d.projectSelect = widget.NewSelect(projectOptions, nil)
	d.projectSelect.SetSelected(lang.L("none"))
	d.projectSelect.PlaceHolder = lang.L("select_project")

	// Create color indicator for selected project
	projectColorIndicator := canvas.NewRectangle(color.Transparent)
	projectColorIndicator.SetMinSize(fyne.NewSize(12, 12))
	projectColorIndicator.CornerRadius = 6

	// Update color indicator when project changes
	d.projectSelect.OnChanged = func(selected string) {
		if selected == lang.L("none") || selected == "" {
			projectColorIndicator.FillColor = color.Transparent
		} else {
			for _, p := range d.projects {
				if p.Name == selected && p.ColorHex != "" {
					projectColorIndicator.FillColor = utils.ParseHexColor(p.ColorHex)
					break
				}
			}
		}
		projectColorIndicator.Refresh()
	}

	// Buttons
	d.startBtn = widget.NewButtonWithIcon(lang.L("start"), theme.MediaPlayIcon(), nil)
	d.pauseBtn = widget.NewButtonWithIcon(lang.L("pause"), theme.MediaPauseIcon(), nil)
	d.pauseBtn.Disable() // Initially disabled

	d.startBtn.OnTapped = func() {
		d.RegisterActivity()
		if d.activeState == models.TaskStateRunning || d.activeState == models.TaskStatePaused {
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
		if d.activeState == models.TaskStateRunning {
			d.PauseTask()
		} else if d.activeState == models.TaskStatePaused {
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
	d.searchEntry.OnChanged = func(s string) {
		d.refreshList()
	}

	// List
	simpleList := widget.NewList(
		func() int { return len(d.taskList) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				container.NewVBox(
					widget.NewLabel(lang.L("title")),
					widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
				))
		},
		func(i int, o fyne.CanvasObject) {
			// Safety check
			if i >= len(d.taskList) {
				return
			}
			entry := d.taskList[len(d.taskList)-1-i] // Reverse order
			box := o.(*fyne.Container)
			leftBox := box.Objects[0].(*fyne.Container)
			title := leftBox.Objects[0].(*widget.Label)
			projectLabel := leftBox.Objects[1].(*widget.Label)
			rightBox := box.Objects[1].(*fyne.Container)
			dur := rightBox.Objects[0].(*widget.Label)
			editBtn := rightBox.Objects[1].(*widget.Button)
			delBtn := rightBox.Objects[2].(*widget.Button)

			title.SetText(entry.Description)

			// Display project name if assigned
			if entry.ProjectID != "" {
				for _, p := range d.projects {
					if p.ID == entry.ProjectID {
						projectLabel.SetText(p.Name)
						projectLabel.Show()
						break
					}
				}
			} else {
				projectLabel.SetText("")
				projectLabel.Hide()
			}

			// Calculate duration for display
			if entry.ID == d.activeID {
				// Active task - use in-memory state for live update
				currentDur := time.Duration(d.accumulated) * time.Second
				if d.activeState == models.TaskStateRunning {
					currentDur += time.Since(d.activeLastStart)
				}
				dur.SetText(utils.FormatDuration(currentDur))
				dur.TextStyle = fyne.TextStyle{Italic: true}
				editBtn.Disable()
			} else {
				// History items
				if entry.State == models.TaskStatePaused {
					dur.SetText(utils.FormatDuration(time.Duration(entry.Accumulated) * time.Second))
					dur.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
				} else if entry.State == models.TaskStateRunning {
					// Should technically not happen for non-active tasks unless multiple running (bug)
					// or if activeID mismatch.
					dur.SetText(lang.L("running"))
					dur.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
				} else {
					dur.SetText(utils.FormatDuration(time.Duration(entry.Duration) * time.Second))
					dur.TextStyle = fyne.TextStyle{Italic: false}
					editBtn.Enable()
				}
			}

			editBtn.OnTapped = func() {
				d.showEditDialog(entry)
			}
			delBtn.OnTapped = func() {
				parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
				dialog.ShowConfirm(lang.L("confirm_deletion"), lang.L("confirm_delete_task"), func(confirmed bool) {
					if !confirmed {
						return
					}

					// If deleting the active task, clear the active state
					if entry.ID == d.activeID {
						d.storage.ClearAppState()
						d.activeID = ""
						d.activeState = models.TaskStateStopped
						d.accumulated = 0
						d.timerData.Set("00:00:00")
						d.updateButtons()
					}

					d.storage.DeleteEntry(entry)
					d.refreshList()
				}, parentWindow)
			}
		},
	)

	d.refreshList = func() {
		// Load today or active date?
		// If active task is from yesterday, we might want to see it.
		// But dashboard usually shows "Today".
		// Let's stick to Today for the list.
		entries, _ := d.storage.LoadEntries(time.Now())
		if d.searchEntry.Text != "" {
			entries = service.FilterTasks(entries, d.searchEntry.Text)
		}
		d.taskList = entries
		simpleList.Refresh()
		d.updateButtons()
	}

	// Ticker with lifecycle management
	d.stopTicker = make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopTicker:
				return
			case <-ticker.C:
				fyne.Do(func() {
					if d.activeState == models.TaskStateRunning {
						dur := time.Duration(d.accumulated)*time.Second + time.Since(d.activeLastStart)
						d.timerData.Set(utils.FormatDuration(dur))
						d.checkIdle()
					} else if d.activeState == models.TaskStatePaused {
						d.timerData.Set(utils.FormatDuration(time.Duration(d.accumulated) * time.Second))
					} else {
						// Stopped
						d.timerData.Set("00:00:00")
					}
					simpleList.Refresh()
				})
			}
		}
	}()

	// Check for active task on load
	d.checkForActiveTask()
	d.refreshList() // Initial load

	// Project selector with color indicator
	projectSelectorWithColor := container.NewHBox(
		projectColorIndicator,
		d.projectSelect,
	)

	// Category input with icon
	categoryIcon := widget.NewIcon(theme.ListIcon())
	categoryWithIcon := container.NewBorder(nil, nil, categoryIcon, nil, d.categoryEntry)

	// Compact input row for project and category
	inputDetailsRow := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewIcon(theme.FolderIcon()), nil, projectSelectorWithColor),
		categoryWithIcon,
	)

	// Main input area with task entry and buttons
	taskInputRow := container.NewBorder(nil, nil, nil, container.NewHBox(d.startBtn, d.pauseBtn), d.taskEntry)

	return container.NewBorder(
		container.NewVBox(
			timerLabel,
			taskInputRow,
			inputDetailsRow,
			layout.NewSpacer(),
			d.searchEntry,
		),
		nil, nil, nil,
		simpleList,
	)
}

// StopTicker stops the background ticker goroutine to prevent memory leaks.
// Call this when the Dashboard is being destroyed or rebuilt.
func (d *Dashboard) StopTicker() {
	if d.stopTicker != nil {
		close(d.stopTicker)
		d.stopTicker = nil
	}
}

// showSaveError displays an error dialog when saving fails.
func (d *Dashboard) showSaveError(err error) {
	parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
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
	if d.activeState == models.TaskStateRunning {
		d.startBtn.SetText(lang.L("stop"))
		d.startBtn.SetIcon(theme.MediaStopIcon())
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("pause"))
		d.pauseBtn.SetIcon(theme.MediaPauseIcon())
		d.pauseBtn.Enable()
	} else if d.activeState == models.TaskStatePaused {
		d.startBtn.SetText(lang.L("stop"))
		d.startBtn.SetIcon(theme.MediaStopIcon())
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("resume"))
		d.pauseBtn.SetIcon(theme.MediaPlayIcon())
		d.pauseBtn.Enable()
	} else {
		d.startBtn.SetText(lang.L("start"))
		d.startBtn.SetIcon(theme.MediaPlayIcon())
		d.startBtn.Enable()

		d.pauseBtn.SetText(lang.L("pause"))
		d.pauseBtn.SetIcon(theme.MediaPauseIcon())
		d.pauseBtn.Disable()
	}
}

func (d *Dashboard) checkForActiveTask() {
	// Try LoadAppState first
	state, err := d.storage.LoadAppState()
	if err == nil && state.ActiveTaskID != "" {
		entries, _ := d.storage.LoadEntries(state.ActiveTaskDate)
		for _, e := range entries {
			if e.ID == state.ActiveTaskID {
				d.activeID = e.ID
				d.activeOriginalStart = e.StartTime
				d.activeLastStart = state.LastStartTime
				d.accumulated = e.Accumulated
				d.activeState = e.State

				// If state is blank (legacy), assume running
				if d.activeState == 0 {
					d.activeState = models.TaskStateRunning
				}

				d.updateButtons()
				return
			}
		}
	}

	// Fallback: Check today's entries for any running task (legacy support)
	entries, _ := d.storage.LoadEntries(time.Now())
	for _, e := range entries {
		if e.EndTime.IsZero() {
			d.activeID = e.ID
			d.activeOriginalStart = e.StartTime
			d.activeLastStart = e.StartTime // Assume started just now if legacy? Or original start.
			d.accumulated = 0
			d.activeState = models.TaskStateRunning

			// Save migrated state
			d.saveState()
			d.updateButtons()
			return
		}
	}

	d.activeState = models.TaskStateStopped
	d.updateButtons()
}

func (d *Dashboard) saveState() {
	d.storage.SaveAppState(store.AppState{
		ActiveTaskID:   d.activeID,
		ActiveTaskDate: d.activeOriginalStart,
		LastStartTime:  d.activeLastStart,
	})
}

func (d *Dashboard) updateActiveEntry() error {
	// Helper to update the persistent entry with current in-memory values (accumulated, state)
	entries, err := d.storage.LoadEntries(d.activeOriginalStart)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID == d.activeID {
			e.State = d.activeState
			e.Accumulated = d.accumulated
			// StartTime remains original
			return d.storage.SaveEntry(e)
		}
	}
	return nil
}

func (d *Dashboard) StartTask(desc, projectID string, tags []string) {
	// If another task is running, stop it
	if d.activeID != "" {
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

	d.activeID = entry.ID
	d.activeOriginalStart = now
	d.activeLastStart = now
	d.accumulated = 0
	d.activeState = models.TaskStateRunning

	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) PauseTask() {
	if d.activeState != models.TaskStateRunning {
		return
	}
	now := time.Now()
	prevAccumulated := d.accumulated
	prevState := d.activeState

	d.accumulated += int64(now.Sub(d.activeLastStart).Seconds())
	d.activeState = models.TaskStatePaused

	if err := d.updateActiveEntry(); err != nil {
		// Revert state on error
		d.accumulated = prevAccumulated
		d.activeState = prevState
		d.showSaveError(err)
		return
	}
	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) ResumeTask() {
	if d.activeState != models.TaskStatePaused {
		return
	}
	prevLastStart := d.activeLastStart
	prevState := d.activeState

	d.activeLastStart = time.Now()
	d.activeState = models.TaskStateRunning

	if err := d.updateActiveEntry(); err != nil {
		// Revert state on error
		d.activeLastStart = prevLastStart
		d.activeState = prevState
		d.showSaveError(err)
		return
	}
	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) TogglePause() {
	if d.activeState == models.TaskStateRunning {
		d.PauseTask()
	} else if d.activeState == models.TaskStatePaused {
		d.ResumeTask()
	}
	d.refreshList()
}

func (d *Dashboard) StopTask() {
	if d.activeID == "" {
		return
	}

	now := time.Now()

	// Load entry to finalize
	entries, err := d.storage.LoadEntries(d.activeOriginalStart)
	if err != nil {
		d.showSaveError(err)
		return
	}

	for _, e := range entries {
		if e.ID == d.activeID {
			// Calculate final duration
			finalDuration := d.accumulated
			if d.activeState == models.TaskStateRunning {
				finalDuration += int64(now.Sub(d.activeLastStart).Seconds())
			}

			e.EndTime = now
			e.Duration = finalDuration
			e.State = models.TaskStateStopped
			e.Accumulated = d.accumulated // Optional: keep this for record

			if err := d.storage.SaveEntry(e); err != nil {
				d.showSaveError(err)
				return
			}
			break
		}
	}

	// Clear state
	d.storage.ClearAppState()
	d.activeID = ""
	d.activeState = models.TaskStateStopped
	d.accumulated = 0
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

	parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
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
