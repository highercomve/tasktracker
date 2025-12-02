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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
)

type Dashboard struct {
	storage     *store.Storage
	timerData   binding.String
	entries     binding.ExternalStringList
	taskList    []models.TimeEntry
	activeID    string
	activeStart time.Time
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
	entry.PlaceHolder = "What are you working on?"

	// Start/Stop Button
	var btn *widget.Button
	btn = widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), func() {
		if d.activeID != "" {
			// Stop
			d.stopTask()
			btn.SetText("Start")
			btn.SetIcon(theme.MediaPlayIcon())
		} else {
			// Start
			if entry.Text == "" {
				return
			}
			d.startTask(entry.Text)
			btn.SetText("Stop")
			btn.SetIcon(theme.MediaStopIcon())
			entry.SetText("")
		}
		d.refreshList()
	})

	// List
	// Let's use simple list for MVP
	simpleList := widget.NewList(
		func() int { return len(d.taskList) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				widget.NewLabel("Title"))
		},
		func(i int, o fyne.CanvasObject) {
			entry := d.taskList[len(d.taskList)-1-i] // Reverse order
			box := o.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			rightBox := box.Objects[1].(*fyne.Container)
			dur := rightBox.Objects[0].(*widget.Label)
			editBtn := rightBox.Objects[1].(*widget.Button)
			delBtn := rightBox.Objects[2].(*widget.Button)

			title.SetText(entry.Description)
			if entry.EndTime.IsZero() {
				dur.SetText(formatDuration(time.Since(entry.StartTime)))
				dur.TextStyle = fyne.TextStyle{Italic: true}
				editBtn.Disable() // Disable edit for running tasks for now (simpler)
			} else {
				dur.SetText(formatDuration(time.Duration(entry.Duration) * time.Second))
				dur.TextStyle = fyne.TextStyle{Italic: false}
				editBtn.Enable()
			}

			editBtn.OnTapped = func() {
				d.showEditDialog(entry)
			}
			delBtn.OnTapped = func() {
				parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
				dialog.ShowConfirm("Confirm Deletion", "Are you sure you want to delete this task?", func(confirmed bool) {
					if !confirmed {
						return
					}
					d.storage.DeleteEntry(entry)
					d.refreshList()
				}, parentWindow)
			}
		},
	)

	d.refreshList = func() {
		entries, _ := d.storage.LoadEntries(time.Now())
		d.taskList = entries
		simpleList.Refresh()
	}
	d.refreshList() // Initial load

	// Ticker
	// Ticker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			// fmt.Println("Ticker fired. ActiveID:", d.activeID)
			fyne.Do(func() {
				if d.activeID != "" {
					dur := time.Since(d.activeStart)
					// fmt.Println("Updating timer:", dur)
					d.timerData.Set(formatDuration(dur))
				}
				simpleList.Refresh()
			})
		}
	}()

	// Check for active task on load
	d.checkForActiveTask(btn)

	return container.NewBorder(
		container.NewVBox(timerLabel, container.NewBorder(nil, nil, nil, btn, entry)),
		nil, nil, nil,
		simpleList,
	)
}

func (d *Dashboard) checkForActiveTask(btn *widget.Button) {
	entries, _ := d.storage.LoadEntries(time.Now())
	for _, e := range entries {
		if e.EndTime.IsZero() {
			d.activeID = e.ID
			d.activeStart = e.StartTime

			btn.SetText("Stop")
			btn.SetIcon(theme.MediaStopIcon())
			return
		}
	}
}

func (d *Dashboard) startTask(desc string) {
	// Stop existing
	d.storage.StopActiveTask(time.Now())

	entry := models.TimeEntry{
		ID:          uuid.New().String(),
		Description: desc,
		StartTime:   time.Now(),
	}
	d.storage.SaveEntry(entry)
	d.activeID = entry.ID
	d.activeStart = entry.StartTime
}

func (d *Dashboard) stopTask() {
	d.storage.StopActiveTask(time.Now())
	d.activeID = ""
	d.activeStart = time.Time{}
	d.timerData.Set("00:00:00")
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
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Start Time", startEntry),
		widget.NewFormItem("End Time", endEntry),
	}

	parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
	dlg := dialog.NewForm("Edit Task", "Save", "Cancel", items, func(b bool) {
		if !b {
			return
		}

		newDesc := descEntry.Text
		newStart, err1 := time.Parse("2006-01-02 15:04:05", startEntry.Text)
		newEnd, err2 := time.Parse("2006-01-02 15:04:05", endEntry.Text)

		if err1 != nil || (endEntry.Text != "" && err2 != nil) {
			// Show error? For now just return
			fmt.Println("Error parsing time")
			return
		}

		// Update entry
		oldEntry := entry
		entry.Description = newDesc
		entry.StartTime = newStart
		if endEntry.Text != "" {
			entry.EndTime = newEnd
			entry.Duration = int64(newEnd.Sub(newStart).Seconds())
		}

		// If start date changed, we need to delete old and save new
		// Simple check: if filename differs
		// But we don't have access to filename helper here easily.
		// Just compare YYYY-MM-DD
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
