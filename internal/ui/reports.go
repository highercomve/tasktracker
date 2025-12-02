package ui

import (
	"fmt"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Reports struct {
	storage *store.Storage
}

func NewReports(s *store.Storage) *Reports {
	return &Reports{storage: s}
}

func (r *Reports) MakeUI() fyne.CanvasObject {
	// Content containers
	dailyContent := container.NewStack()
	weeklyContent := container.NewStack()
	monthlyContent := container.NewStack()
	customContent := container.NewStack()

	// Helper to refresh content
	refreshReport := func(content *fyne.Container, start, end time.Time, refreshFunc func()) {
		entries, _ := r.storage.LoadEntriesForRange(start, end)
		reportUI := r.renderHistory(entries, refreshFunc)
		content.Objects = []fyne.CanvasObject{reportUI}
		content.Refresh()
	}

	// Daily Tab
	var selectedDay = time.Now()
	dailyLabel := widget.NewLabel("")

	// Define update functions first so they can be used recursively if needed (though typically not needed for this pattern)
	// Actually, we need a stable reference to the update function to pass as a callback?
	// Yes, refreshReport needs to know WHAT to call to re-trigger the full update (re-load entries).
	
	var updateDaily func()
	updateDaily = func() {
		dailyLabel.SetText("Report for " + selectedDay.Format("Mon, 02 Jan 2006"))
		refreshReport(dailyContent, selectedDay, selectedDay, updateDaily)
	}
	updateDaily() // Initial

	dailyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedDay = selectedDay.AddDate(0, 0, -1)
				updateDaily()
			}),
			widget.NewButton("Today", func() {
				selectedDay = time.Now()
				updateDaily()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedDay = selectedDay.AddDate(0, 0, 1)
				updateDaily()
			}),
			layout.NewSpacer(),
			dailyLabel,
		),
		nil, nil, nil,
		dailyContent,
	)

	// Weekly Tab
	getWeekStart := func(t time.Time) time.Time {
		offset := int(t.Weekday())
		if offset == 0 {
			offset = 7
		}
		return t.AddDate(0, 0, -offset+1)
	}
	var selectedWeekStart = getWeekStart(time.Now())
	weeklyLabel := widget.NewLabel("")

	var updateWeekly func()
	updateWeekly = func() {
		end := selectedWeekStart.AddDate(0, 0, 6)
		weeklyLabel.SetText(fmt.Sprintf("Week %s - %s", selectedWeekStart.Format("Jan 02"), end.Format("Jan 02")))
		refreshReport(weeklyContent, selectedWeekStart, end, updateWeekly)
	}
	updateWeekly()

	weeklyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, -7)
				updateWeekly()
			}),
			widget.NewButton("This Week", func() {
				selectedWeekStart = getWeekStart(time.Now())
				updateWeekly()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, 7)
				updateWeekly()
			}),
			layout.NewSpacer(),
			weeklyLabel,
		),
		nil, nil, nil,
		weeklyContent,
	)

	// Monthly Tab
	getMonthStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	var selectedMonth = getMonthStart(time.Now())
	monthlyLabel := widget.NewLabel("")

	var updateMonthly func()
	updateMonthly = func() {
		end := selectedMonth.AddDate(0, 1, -1)
		monthlyLabel.SetText("Report for " + selectedMonth.Format("January 2006"))
		refreshReport(monthlyContent, selectedMonth, end, updateMonthly)
	}
	updateMonthly()

	monthlyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedMonth = selectedMonth.AddDate(0, -1, 0)
				updateMonthly()
			}),
			widget.NewButton("This Month", func() {
				selectedMonth = getMonthStart(time.Now())
				updateMonthly()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedMonth = selectedMonth.AddDate(0, 1, 0)
				updateMonthly()
			}),
			layout.NewSpacer(),
			monthlyLabel,
		),
		nil, nil, nil,
		monthlyContent,
	)

	// Custom Range Tab
	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()
	
	var startBtn, endBtn *widget.Button

	var updateCustom func()
	updateCustom = func() {
		startBtn.SetText(startDate.Format("2006-01-02"))
		endBtn.SetText(endDate.Format("2006-01-02"))
		refreshReport(customContent, startDate, endDate, updateCustom)
	}

	pickDate := func(current time.Time, onSelect func(time.Time)) {
		var d dialog.Dialog
		cal := widget.NewCalendar(current, func(t time.Time) {
			onSelect(t)
			if d != nil {
				d.Hide()
			}
		})
		
		// We need to find the parent window
		wins := fyne.CurrentApp().Driver().AllWindows()
		if len(wins) > 0 {
			d = dialog.NewCustom("Select Date", "Cancel", container.NewPadded(cal), wins[0])
			d.Resize(fyne.NewSize(300, 300))
			d.Show()
		}
	}

	startBtn = widget.NewButton(startDate.Format("2006-01-02"), func() {
		pickDate(startDate, func(t time.Time) {
			startDate = t
			updateCustom()
		})
	})

	endBtn = widget.NewButton(endDate.Format("2006-01-02"), func() {
		pickDate(endDate, func(t time.Time) {
			endDate = t
			updateCustom()
		})
	})

	updateCustom() // Initial

	customTab := container.NewBorder(
		container.NewHBox(
			widget.NewLabel("From:"), startBtn,
			widget.NewLabel("To:"), endBtn,
			layout.NewSpacer(),
			widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
				updateCustom()
			}),
		),
		nil, nil, nil,
		customContent,
	)

	return container.NewAppTabs(
		container.NewTabItem("Daily", dailyTab),
		container.NewTabItem("Weekly", weeklyTab),
		container.NewTabItem("Monthly", monthlyTab),
		container.NewTabItem("Custom Range", customTab),
	)
}

func (r *Reports) renderHistory(entries []models.TimeEntry, onRefresh func()) fyne.CanvasObject {
	if len(entries) == 0 {
		return widget.NewLabel("No entries found for this period.")
	}

	// Summary
	sums := make(map[string]time.Duration)
	var total time.Duration
	for _, e := range entries {
		dur := time.Duration(e.Duration) * time.Second
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		}
		sums[e.Description] += dur
		total += dur
	}

	summaryText := fmt.Sprintf("Total Time: %s\n", formatDuration(total))
	for desc, dur := range sums {
		summaryText += fmt.Sprintf("- %s: %s\n", desc, formatDuration(dur))
	}
	summaryLabel := widget.NewLabel(summaryText)

	// List
	listData := entries // local copy
	
	listView := widget.NewList(
		func() int { return len(listData) },
		func() fyne.CanvasObject {
			// Reusing layout similar to dashboard
			return container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				container.NewVBox(
					widget.NewLabelWithStyle("Title", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle("Date", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
				))
		},
		func(i int, o fyne.CanvasObject) {
			// Reverse order for display? Or chronological?
			// Usually history is newest first.
			entry := listData[len(listData)-1-i] 
			
			box := o.(*fyne.Container)
			rightBox := box.Objects[1].(*fyne.Container)
			durLabel := rightBox.Objects[0].(*widget.Label)
			editBtn := rightBox.Objects[1].(*widget.Button)
			delBtn := rightBox.Objects[2].(*widget.Button)
			
			infoBox := box.Objects[0].(*fyne.Container)
			titleLabel := infoBox.Objects[0].(*widget.Label)
			dateLabel := infoBox.Objects[1].(*widget.Label)

			titleLabel.SetText(entry.Description)
			dateLabel.SetText(entry.StartTime.Format("Mon, 02 Jan 15:04"))

			dur := time.Duration(entry.Duration) * time.Second
			if entry.EndTime.IsZero() {
				dur = time.Since(entry.StartTime)
				durLabel.TextStyle = fyne.TextStyle{Italic: true}
				editBtn.Disable() // Disable edit for running tasks
			} else {
				durLabel.TextStyle = fyne.TextStyle{}
				editBtn.Enable()
			}
			durLabel.SetText(formatDuration(dur))
			
			editBtn.OnTapped = func() {
				r.showEditDialog(entry, onRefresh)
			}
			delBtn.OnTapped = func() {
				parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
				dialog.ShowConfirm("Confirm Deletion", "Are you sure you want to delete this task?", func(confirmed bool) {
					if !confirmed {
						return
					}
					r.storage.DeleteEntry(entry)
					onRefresh()
				}, parentWindow)
			}
		},
	)

	return container.NewBorder(
		container.NewVBox(summaryLabel, widget.NewSeparator()), 
		nil, nil, nil, 
		listView,
	)
}

func (r *Reports) showEditDialog(entry models.TimeEntry, onSuccess func()) {
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
		if oldEntry.StartTime.Format("2006-01-02") != entry.StartTime.Format("2006-01-02") {
			r.storage.DeleteEntry(oldEntry)
		}

		r.storage.SaveEntry(entry)
		onSuccess()
	}, parentWindow)
	dlg.Resize(fyne.NewSize(parentWindow.Canvas().Size().Width, dlg.MinSize().Height))
	dlg.Show()
}
