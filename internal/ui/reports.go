package ui

import (
	"fmt"
	"time"

	"go-tracker/internal/models"
	"go-tracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type Reports struct {
	storage *store.Storage
}

func NewReports(s *store.Storage) *Reports {
	return &Reports{storage: s}
}

func (r *Reports) MakeUI() fyne.CanvasObject {
	dailyContent := widget.NewLabel("Select a date to view report")
	weeklyContent := widget.NewLabel("Select a week to view report")
	monthlyContent := widget.NewLabel("Select a month to view report")

	// Helper to refresh content
	refreshReport := func(content *widget.Label, start, end time.Time) {
		entries, _ := r.storage.LoadEntriesForRange(start, end)
		summary := summarizeEntries(entries)
		content.SetText(summary)
	}

	// Daily Tab
	var selectedDay = time.Now()
	dailyLabel := widget.NewLabel("")

	updateDaily := func() {
		dailyLabel.SetText("Report for " + selectedDay.Format("Mon, 02 Jan 2006"))
		refreshReport(dailyContent, selectedDay, selectedDay)
	}
	updateDaily() // Initial

	dailyTab := container.NewVBox(
		container.NewHBox(
			widget.NewButton("<", func() {
				selectedDay = selectedDay.AddDate(0, 0, -1)
				updateDaily()
			}),
			widget.NewButton("Today", func() {
				selectedDay = time.Now()
				updateDaily()
			}),
			widget.NewButton(">", func() {
				selectedDay = selectedDay.AddDate(0, 0, 1)
				updateDaily()
			}),
			layout.NewSpacer(),
			dailyLabel,
		),
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

	updateWeekly := func() {
		end := selectedWeekStart.AddDate(0, 0, 6)
		weeklyLabel.SetText(fmt.Sprintf("Week %s - %s", selectedWeekStart.Format("Jan 02"), end.Format("Jan 02")))
		refreshReport(weeklyContent, selectedWeekStart, end)
	}
	updateWeekly()

	weeklyTab := container.NewVBox(
		container.NewHBox(
			widget.NewButton("<", func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, -7)
				updateWeekly()
			}),
			widget.NewButton("This Week", func() {
				selectedWeekStart = getWeekStart(time.Now())
				updateWeekly()
			}),
			widget.NewButton(">", func() {
				selectedWeekStart = selectedWeekStart.AddDate(0, 0, 7)
				updateWeekly()
			}),
			layout.NewSpacer(),
			weeklyLabel,
		),
		weeklyContent,
	)

	// Monthly Tab
	getMonthStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	var selectedMonth = getMonthStart(time.Now())
	monthlyLabel := widget.NewLabel("")

	updateMonthly := func() {
		end := selectedMonth.AddDate(0, 1, -1)
		monthlyLabel.SetText("Report for " + selectedMonth.Format("January 2006"))
		refreshReport(monthlyContent, selectedMonth, end)
	}
	updateMonthly()

	monthlyTab := container.NewVBox(
		container.NewHBox(
			widget.NewButton("<", func() {
				selectedMonth = selectedMonth.AddDate(0, -1, 0)
				updateMonthly()
			}),
			widget.NewButton("This Month", func() {
				selectedMonth = getMonthStart(time.Now())
				updateMonthly()
			}),
			widget.NewButton(">", func() {
				selectedMonth = selectedMonth.AddDate(0, 1, 0)
				updateMonthly()
			}),
			layout.NewSpacer(),
			monthlyLabel,
		),
		monthlyContent,
	)

	return container.NewAppTabs(
		container.NewTabItem("Daily", dailyTab),
		container.NewTabItem("Weekly", weeklyTab),
		container.NewTabItem("Monthly", monthlyTab),
	)
}

func summarizeEntries(entries []models.TimeEntry) string {
	if len(entries) == 0 {
		return "No entries found."
	}

	sums := make(map[string]time.Duration)
	for _, e := range entries {
		dur := time.Duration(e.Duration) * time.Second
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		}
		sums[e.Description] += dur
	}

	result := "Summary:\n"
	var total time.Duration
	for desc, dur := range sums {
		result += fmt.Sprintf("- %s: %s\n", desc, formatDuration(dur))
		total += dur
	}
	result += fmt.Sprintf("\nTotal Time: %s", formatDuration(total))
	return result
}
