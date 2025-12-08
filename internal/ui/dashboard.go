package ui

import (
	"fmt"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
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

	// UI
	startBtn    *widget.Button
	pauseBtn    *widget.Button
	refreshList func()
}

func NewDashboard(s *store.Storage) *Dashboard {
	return &Dashboard{
		storage:   s,
		timerData: binding.NewString(),
	}
}

func (d *Dashboard) MakeUI() fyne.CanvasObject {
	d.timerData.Set("00:00:00")

	// Timer Label
	timerLabel := widget.NewLabelWithData(d.timerData)
	timerLabel.TextStyle = fyne.TextStyle{Bold: true}
	timerLabel.Alignment = fyne.TextAlignCenter

	// Input
	entry := widget.NewEntry()
	entry.PlaceHolder = lang.L("what_working_on")

	// Buttons
	d.startBtn = widget.NewButtonWithIcon(lang.L("start"), theme.MediaPlayIcon(), nil)
	d.pauseBtn = widget.NewButtonWithIcon(lang.L("pause"), theme.MediaPauseIcon(), nil)
	d.pauseBtn.Disable() // Initially disabled

	d.startBtn.OnTapped = func() {
		if d.activeState == models.TaskStateRunning || d.activeState == models.TaskStatePaused {
			// Stop
			d.StopTask()
			entry.SetText("")
		} else {
			// Start
			if entry.Text == "" {
				return
			}
			d.StartTask(entry.Text)
			entry.SetText("")
		}
		d.refreshList()
	}

	d.pauseBtn.OnTapped = func() {
		if d.activeState == models.TaskStateRunning {
			d.PauseTask()
		} else if d.activeState == models.TaskStatePaused {
			d.ResumeTask()
		}
		d.refreshList()
	}

	entry.OnSubmitted = func(text string) {
		if text == "" {
			return
		}
		d.StartTask(text)
		entry.SetText("")
		d.refreshList()
	}

	// List
	simpleList := widget.NewList(
		func() int { return len(d.taskList) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				widget.NewLabel(lang.L("title")))
		},
		func(i int, o fyne.CanvasObject) {
			// Safety check
			if i >= len(d.taskList) {
				return
			}
			entry := d.taskList[len(d.taskList)-1-i] // Reverse order
			box := o.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			rightBox := box.Objects[1].(*fyne.Container)
			dur := rightBox.Objects[0].(*widget.Label)
			editBtn := rightBox.Objects[1].(*widget.Button)
			delBtn := rightBox.Objects[2].(*widget.Button)

			title.SetText(entry.Description)

			// Calculate duration for display
			if entry.ID == d.activeID {
				// Active task - use in-memory state for live update
				currentDur := time.Duration(d.accumulated) * time.Second
				if d.activeState == models.TaskStateRunning {
					currentDur += time.Since(d.activeLastStart)
				}
				dur.SetText(formatDuration(currentDur))
				dur.TextStyle = fyne.TextStyle{Italic: true}
				editBtn.Disable()
			} else {
				// History items
				if entry.State == models.TaskStatePaused {
					dur.SetText(formatDuration(time.Duration(entry.Accumulated) * time.Second))
					dur.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
				} else if entry.State == models.TaskStateRunning {
					// Should technically not happen for non-active tasks unless multiple running (bug)
					// or if activeID mismatch.
					dur.SetText(lang.L("running"))
					dur.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
				} else {
					dur.SetText(formatDuration(time.Duration(entry.Duration) * time.Second))
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
		d.taskList = entries
		simpleList.Refresh()
		d.updateButtons()
	}

	// Ticker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			fyne.Do(func() {
				if d.activeState == models.TaskStateRunning {
					dur := time.Duration(d.accumulated)*time.Second + time.Since(d.activeLastStart)
					d.timerData.Set(formatDuration(dur))
				} else if d.activeState == models.TaskStatePaused {
					d.timerData.Set(formatDuration(time.Duration(d.accumulated) * time.Second))
				} else {
					// Stopped
					d.timerData.Set("00:00:00")
				}
				simpleList.Refresh()
			})
		}
	}()

	// Check for active task on load
	d.checkForActiveTask()
	d.refreshList() // Initial load

	return container.NewBorder(
		container.NewVBox(timerLabel, container.NewBorder(nil, nil, nil, container.NewHBox(d.startBtn, d.pauseBtn), entry)),
		nil, nil, nil,
		simpleList,
	)
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

func (d *Dashboard) updateActiveEntry() {
	// Helper to update the persistent entry with current in-memory values (accumulated, state)
	entries, err := d.storage.LoadEntries(d.activeOriginalStart)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.ID == d.activeID {
			e.State = d.activeState
			e.Accumulated = d.accumulated
			// StartTime remains original
			d.storage.SaveEntry(e)
			return
		}
	}
}

func (d *Dashboard) StartTask(desc string) {
	// If another task is running, stop it
	if d.activeID != "" {
		d.StopTask()
	}

	now := time.Now()
	entry := models.TimeEntry{
		ID:          uuid.New().String(),
		Description: desc,
		StartTime:   now,
		State:       models.TaskStateRunning,
		Accumulated: 0,
	}
	d.storage.SaveEntry(entry)

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
	d.accumulated += int64(now.Sub(d.activeLastStart).Seconds())
	d.activeState = models.TaskStatePaused

	d.updateActiveEntry()
	d.saveState()
	d.updateButtons()
}

func (d *Dashboard) ResumeTask() {
	if d.activeState != models.TaskStatePaused {
		return
	}
	d.activeLastStart = time.Now()
	d.activeState = models.TaskStateRunning

	d.updateActiveEntry()
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
	if err == nil {
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

				d.storage.SaveEntry(e)
				break
			}
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

	startEntry := widget.NewEntry()
	startEntry.SetText(entry.StartTime.Format("2006-01-02 15:04:05"))

	endEntry := widget.NewEntry()
	if !entry.EndTime.IsZero() {
		endEntry.SetText(entry.EndTime.Format("2006-01-02 15:04:05"))
	}

	items := []*widget.FormItem{
		widget.NewFormItem(lang.L("task_description"), descEntry),
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

		// Update entry
		oldEntry := entry
		entry.Description = newDesc
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

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
