package ui

import (
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2/lang"
	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

func GeneratePDF(path string, entries []models.TimeEntry, start, end time.Time, groupBy string) error {
	m := pdf.NewMaroto(consts.Portrait, consts.A4)
	m.SetPageMargins(20, 10, 20)

	// Header
	m.RegisterHeader(func() {
		m.Row(10, func() {
			m.Col(12, func() {
				m.Text(lang.L("report_title"), props.Text{
					Top:   3,
					Style: consts.Bold,
					Align: consts.Center,
					Size:  16,
				})
			})
		})
		m.Row(10, func() {
			m.Col(12, func() {
				dateRange := fmt.Sprintf("%s - %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
				m.Text(dateRange, props.Text{
					Top:   3,
					Style: consts.Normal,
					Align: consts.Center,
					Size:  12,
				})
			})
		})
	})

	// Table Header
	headers := []string{
		lang.L("date"),
		lang.L("task_description"),
		lang.L("duration"),
	}

	// Calculate total duration
	var totalDuration time.Duration
	for _, e := range entries {
		dur := time.Duration(e.Duration) * time.Second
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		}
		totalDuration += dur
	}

	m.Row(10, func() {
		m.Col(12, func() {
			m.Text(lang.L("task_history"), props.Text{
				Top:   5,
				Style: consts.Bold,
				Size:  14,
			})
		})
	})

	// Helper to determine group key and title
	/*
	   REMOVED local definitions of getGroupKey and getGroupTitle
	   because they are now available in internal/ui/grouping.go (same package)
	*/

	if groupBy == service.GroupByNone {
		// Content
		rows := [][]string{}

		for _, e := range entries {
			dur := time.Duration(e.Duration) * time.Second
			if e.EndTime.IsZero() {
				dur = time.Since(e.StartTime)
			}

			rows = append(rows, []string{
				e.StartTime.Format("2006-01-02"),
				e.Description,
				formatDuration(dur),
			})
		}

		m.TableList(headers, rows, props.TableList{
			HeaderProp: props.TableListContent{
				Size:      10,
				GridSizes: []uint{3, 6, 3},
			},
			ContentProp: props.TableListContent{
				Size:      10,
				GridSizes: []uint{3, 6, 3},
			},
			Align:                consts.Center,
			AlternatedBackground: &color.Color{Red: 240, Green: 240, Blue: 240},
			HeaderContentSpace:   1,
			Line:                 false,
		})
	} else {
		// Group logic
		groups := make(map[string][]models.TimeEntry)
		var keys []string

		for _, e := range entries {
			key := service.GetGroupKey(e.StartTime, groupBy)
			if _, exists := groups[key]; !exists {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], e)
		}

		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		for _, key := range keys {
			groupEntries := groups[key]
			var groupTotal time.Duration
			rows := [][]string{}

			for _, e := range groupEntries {
				dur := time.Duration(e.Duration) * time.Second
				if e.EndTime.IsZero() {
					dur = time.Since(e.StartTime)
				}
				groupTotal += dur

				rows = append(rows, []string{
					e.StartTime.Format("2006-01-02"),
					e.Description,
					formatDuration(dur),
				})
			}

			title := ""
			if len(groupEntries) > 0 {
				title = service.GetGroupTitle(groupEntries[0].StartTime, groupBy)
			}
			// Header Title Only (Total in footer now)
			headerTitle := title

			m.Row(10, func() {
				m.Col(12, func() {
					m.Text(headerTitle, props.Text{
						Top:   5,
						Style: consts.Bold,
						Size:  12,
						Align: consts.Left,
					})
				})
			})

			m.TableList(headers, rows, props.TableList{
				HeaderProp: props.TableListContent{
					Size:      10,
					GridSizes: []uint{3, 6, 3},
				},
				ContentProp: props.TableListContent{
					Size:      10,
					GridSizes: []uint{3, 6, 3},
				},
				Align:                consts.Center,
				AlternatedBackground: &color.Color{Red: 240, Green: 240, Blue: 240},
				HeaderContentSpace:   1,
				Line:                 false,
			})

			// Subtotal Footer
			m.Row(10, func() {
				m.Col(12, func() {
					m.Text(fmt.Sprintf("%s: %s", lang.L("subtotal"), formatDuration(groupTotal)), props.Text{
						Top:   0,
						Style: consts.Bold,
						Align: consts.Right,
						Size:  10,
					})
				})
			})

			// Add some space after table
			m.Row(5, func() {})
		}
	}

	// Summary
	m.Row(20, func() {
		m.Col(12, func() {
			m.Text(fmt.Sprintf("%s: %s", lang.L("total_time"), formatDuration(totalDuration)), props.Text{
				Top:   10,
				Style: consts.Bold,
				Align: consts.Right,
				Size:  12,
			})
		})
	})

	return m.OutputFileAndClose(path)
}
