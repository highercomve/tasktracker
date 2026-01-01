package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
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
	refreshReport := func(content *fyne.Container, start, end time.Time, groupBy string, refreshFunc func()) {
		entries, _ := r.storage.LoadEntriesForRange(start, end)
		reportUI := r.renderHistory(entries, groupBy, refreshFunc)
		content.Objects = []fyne.CanvasObject{reportUI}
		content.Refresh()
	}

	createExportButton := func(getRange func() (time.Time, time.Time), getGroupBy func() string) *widget.Button {
		return widget.NewButtonWithIcon(lang.L("export_pdf"), theme.DocumentSaveIcon(), func() {
			start, end := getRange()
			groupBy := getGroupBy()

			// Initial filename suggestion
			filename := fmt.Sprintf("report_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))

			dlg := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
					return
				}
				if writer == nil {
					return
				}
				defer writer.Close()

				path := writer.URI().Path()
				entries, err := r.storage.LoadEntriesForRange(start, end)
				if err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
					return
				}

				if err := GeneratePDF(path, entries, start, end, groupBy); err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
				} else {
					dialog.ShowInformation(lang.L("success"), lang.L("pdf_saved"), fyne.CurrentApp().Driver().AllWindows()[0])
				}
			}, fyne.CurrentApp().Driver().AllWindows()[0])

			dlg.SetFileName(filename)
			dlg.Show()
		})
	}

	// Helper to create GroupBy selector
	createGroupBySelector := func(onChange func(string)) *widget.Select {
		s := widget.NewSelect([]string{lang.L("none"), lang.L("daily"), lang.L("weekly")}, onChange)
		s.SetSelected(lang.L("none"))
		return s
	}

	// Daily Tab
	var selectedDay = time.Now()
	dailyLabel := widget.NewLabel("")

	var updateDaily func()
	updateDaily = func() {
		dailyLabel.SetText(lang.L("report_for") + selectedDay.Format("Mon, 02 Jan 2006"))
		refreshReport(dailyContent, selectedDay, selectedDay, service.GroupByNone, updateDaily)
	}

	dailyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedDay = selectedDay.AddDate(0, 0, -1)
				updateDaily()
			}),
			widget.NewButton(lang.L("today"), func() {
				selectedDay = time.Now()
				updateDaily()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedDay = selectedDay.AddDate(0, 0, 1)
				updateDaily()
			}),
			layout.NewSpacer(),
			dailyLabel,
			createExportButton(func() (time.Time, time.Time) {
				return selectedDay, selectedDay
			}, func() string {
				return service.GroupByNone
			}),
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
	weeklyGroupBy := service.GroupByNone

	var updateWeekly func()
	updateWeekly = func() {
		end := selectedWeekStart.AddDate(0, 0, 6)
		weeklyLabel.SetText(fmt.Sprintf("%s %s - %s", lang.L("week"), selectedWeekStart.Format("Jan 02"), end.Format("Jan 02")))
		refreshReport(weeklyContent, selectedWeekStart, end, weeklyGroupBy, updateWeekly)
	}

	weeklySelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			weeklyGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			weeklyGroupBy = service.GroupByWeek
		} else {
			weeklyGroupBy = service.GroupByNone
		}
		updateWeekly()
	})

	weeklyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, -7)
				updateWeekly()
			}),
			widget.NewButton(lang.L("this_week"), func() {
				selectedWeekStart = getWeekStart(time.Now())
				updateWeekly()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, 7)
				updateWeekly()
			}),
			layout.NewSpacer(),
			weeklyLabel,
			widget.NewLabel(lang.L("group_by")),
			weeklySelector,
			createExportButton(func() (time.Time, time.Time) {
				return selectedWeekStart, selectedWeekStart.AddDate(0, 0, 6)
			}, func() string {
				return weeklyGroupBy
			}),
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
	monthlyGroupBy := service.GroupByNone

	var updateMonthly func()
	updateMonthly = func() {
		end := selectedMonth.AddDate(0, 1, -1)
		monthlyLabel.SetText(lang.L("report_for") + selectedMonth.Format("January 2006"))
		refreshReport(monthlyContent, selectedMonth, end, monthlyGroupBy, updateMonthly)
	}

	monthlySelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			monthlyGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			monthlyGroupBy = service.GroupByWeekOfMonth
		} else {
			monthlyGroupBy = service.GroupByNone
		}
		updateMonthly()
	})

	monthlyTab := container.NewBorder(
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				selectedMonth = selectedMonth.AddDate(0, -1, 0)
				updateMonthly()
			}),
			widget.NewButton(lang.L("this_month"), func() {
				selectedMonth = getMonthStart(time.Now())
				updateMonthly()
			}),
			widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
				selectedMonth = selectedMonth.AddDate(0, 1, 0)
				updateMonthly()
			}),
			layout.NewSpacer(),
			monthlyLabel,
			widget.NewLabel(lang.L("group_by")),
			monthlySelector,
			createExportButton(func() (time.Time, time.Time) {
				return selectedMonth, selectedMonth.AddDate(0, 1, -1)
			}, func() string {
				return monthlyGroupBy
			}),
		),
		nil, nil, nil,
		monthlyContent,
	)

	// Custom Range Tab
	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()
	customGroupBy := service.GroupByNone

	var startBtn, endBtn *widget.Button

	var updateCustom func()
	updateCustom = func() {
		startBtn.SetText(startDate.Format("2006-01-02"))
		endBtn.SetText(endDate.Format("2006-01-02"))
		refreshReport(customContent, startDate, endDate, customGroupBy, updateCustom)
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
			d = dialog.NewCustom(lang.L("select_date"), lang.L("cancel"), container.NewPadded(cal), wins[0])
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

	customSelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			customGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			customGroupBy = service.GroupByWeek
		} else {
			customGroupBy = service.GroupByNone
		}
		updateCustom()
	})

	customTab := container.NewBorder(
		container.NewHBox(
			widget.NewLabel(lang.L("from")), startBtn,
			widget.NewLabel(lang.L("to")), endBtn,
			layout.NewSpacer(),
			widget.NewLabel(lang.L("group_by")),
			customSelector,
			widget.NewButtonWithIcon(lang.L("refresh"), theme.ViewRefreshIcon(), func() {
				updateCustom()
			}),
			createExportButton(func() (time.Time, time.Time) {
				return startDate, endDate
			}, func() string {
				return customGroupBy
			}),
		),
		nil, nil, nil,
		customContent,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(lang.L("daily"), dailyTab),
		container.NewTabItem(lang.L("weekly"), weeklyTab),
		container.NewTabItem(lang.L("monthly"), monthlyTab),
		container.NewTabItem(lang.L("custom_range"), customTab),
	)

	tabs.OnSelected = func(item *container.TabItem) {
		switch item.Text {
		case lang.L("daily"):
			updateDaily()
		case lang.L("weekly"):
			updateWeekly()
		case lang.L("monthly"):
			updateMonthly()
		case lang.L("custom_range"):
			updateCustom()
		}
	}
	// Select initial tab to trigger data load
	tabs.SelectIndex(0)

	return tabs
}

type ListItem struct {
	IsHeader bool
	IsFooter bool
	Header   string
	Entry    models.TimeEntry
}

func (r *Reports) renderHistory(entries []models.TimeEntry, groupBy string, onRefresh func()) fyne.CanvasObject {
	if len(entries) == 0 {
		return widget.NewLabel(lang.L("no_entries"))
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

	summaryText := fmt.Sprintf(lang.L("total_time")+"%s\n", formatDuration(total))
	for desc, dur := range sums {
		summaryText += fmt.Sprintf("- %s: %s\n", desc, formatDuration(dur))
	}
	summaryLabel := widget.NewLabel(summaryText)

	// Build List Items based on Grouping
	var listItems []ListItem

	if groupBy == service.GroupByNone {
		for i := len(entries) - 1; i >= 0; i-- {
			listItems = append(listItems, ListItem{IsHeader: false, Entry: entries[i]})
		}
	} else {
		// Group entries
		groups := make(map[string][]models.TimeEntry)
		var keys []string

		for _, e := range entries {
			key := service.GetGroupKey(e.StartTime, groupBy)
			if _, exists := groups[key]; !exists {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], e)
		}

		// Sort keys (reverse chronological)
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		for _, key := range keys {
			groupEntries := groups[key]
			// Calculate group total
			var groupTotal time.Duration
			for _, e := range groupEntries {
				dur := time.Duration(e.Duration) * time.Second
				if e.EndTime.IsZero() {
					dur = time.Since(e.StartTime)
				}
				groupTotal += dur
			}

			// Add Header
			title := ""
			if len(groupEntries) > 0 {
				title = service.GetGroupTitle(groupEntries[0].StartTime, groupBy)
			}
			headerTitle := title
			listItems = append(listItems, ListItem{IsHeader: true, Header: headerTitle})

			// Add Entries (reverse order within group)
			for i := len(groupEntries) - 1; i >= 0; i-- {
				listItems = append(listItems, ListItem{IsHeader: false, Entry: groupEntries[i]})
			}

			// Add Footer (Subtotal)
			subtotalTitle := fmt.Sprintf("%s: %s", lang.L("subtotal"), formatDuration(groupTotal))
			listItems = append(listItems, ListItem{IsFooter: true, Header: subtotalTitle})
		}
	}

	listView := widget.NewList(
		func() int { return len(listItems) },
		func() fyne.CanvasObject {
			// Container that holds layouts, hidden/shown via object type
			// Header View
			headerLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

			// Footer View (Subtotal)
			footerLabel := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true, Italic: true})

			// Task View
			taskContainer := container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				container.NewVBox(
					widget.NewLabelWithStyle(lang.L("title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle(lang.L("date"), fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
				))

			return container.NewMax(headerLabel, footerLabel, taskContainer)
		},
		func(i int, o fyne.CanvasObject) {
			item := listItems[i]
			containerBox := o.(*fyne.Container)
			headerLabel := containerBox.Objects[0].(*widget.Label)
			footerLabel := containerBox.Objects[1].(*widget.Label)
			taskBox := containerBox.Objects[2].(*fyne.Container)

			if item.IsHeader {
				headerLabel.Show()
				footerLabel.Hide()
				taskBox.Hide()
				headerLabel.SetText(item.Header)
			} else if item.IsFooter {
				headerLabel.Hide()
				footerLabel.Show()
				taskBox.Hide()
				footerLabel.SetText(item.Header)
			} else {
				headerLabel.Hide()
				footerLabel.Hide()
				taskBox.Show()

				entry := item.Entry

				// Extract sub-widgets from taskBox
				rightBox := taskBox.Objects[1].(*fyne.Container)
				durLabel := rightBox.Objects[0].(*widget.Label)
				editBtn := rightBox.Objects[1].(*widget.Button)
				delBtn := rightBox.Objects[2].(*widget.Button)

				infoBox := taskBox.Objects[0].(*fyne.Container)
				titleLabel := infoBox.Objects[0].(*widget.Label)
				dateLabel := infoBox.Objects[1].(*widget.Label)

				titleLabel.SetText(entry.Description)
				dateLabel.SetText(entry.StartTime.Format("Mon, 02 Jan 15:04"))

				dur := time.Duration(entry.Duration) * time.Second
				if entry.EndTime.IsZero() {
					dur = time.Since(entry.StartTime)
					durLabel.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
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
					dialog.ShowConfirm(lang.L("confirm_deletion"), lang.L("confirm_delete_task"), func(confirmed bool) {
						if !confirmed {
							return
						}
						r.storage.DeleteEntry(entry)
						onRefresh()
					}, parentWindow)
				}
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
