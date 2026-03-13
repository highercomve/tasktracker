package ui

import (
	"fmt"
	"os"
	"sort"
	"time"

	"fyne.io/fyne/v2/lang"
	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/utils"
	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
	"github.com/spf13/viper"
)

var (
	blueColor  = color.Color{Red: 10, Green: 50, Blue: 100}
	greyColor  = color.Color{Red: 200, Green: 200, Blue: 200}
	whiteColor = color.Color{Red: 255, Green: 255, Blue: 255}
)

func GeneratePDF(path string, entries []models.TimeEntry, start, end time.Time, groupBy string) error {
	m := pdf.NewMaroto(consts.Portrait, consts.A4)
	m.SetPageMargins(20, 15, 20)

	// Background for the whole page (Optional, but can look nice)
	// m.SetBackgroundColor(color.Color{Red: 252, Green: 252, Blue: 252})

	// Header
	m.RegisterHeader(func() {
		m.Row(25, func() {
			m.Col(3, func() {
				// Try to load Icon.png as logo
				if _, err := os.Stat("Icon.png"); err == nil {
					_ = m.FileImage("Icon.png", props.Rect{
						Percent: 100,
						Center:  true,
					})
				}
			})
			m.ColSpace(3)
			m.Col(6, func() {
				m.Text(lang.L("report_title"), props.Text{
					Size:  18,
					Style: consts.Bold,
					Align: consts.Right,
					Color: blueColor,
				})
				m.Text(fmt.Sprintf("%s - %s", start.Format("2006-01-02"), end.Format("2006-01-02")), props.Text{
					Top:   8,
					Size:  12,
					Style: consts.Italic,
					Align: consts.Right,
				})
			})
		})
		m.Row(2, func() {})
		m.Line(1.0, props.Line{
			Color: blueColor,
		})
	})

	// Footer
	m.RegisterFooter(func() {
		m.Row(10, func() {
			m.Col(6, func() {
				m.Text(fmt.Sprintf("Generated on %s", time.Now().Format("2006-01-02 15:04")), props.Text{
					Top:  5,
					Size: 8,
				})
			})
			m.Col(6, func() {
				m.Text(fmt.Sprintf("Page %d", m.GetCurrentPage()), props.Text{
					Top:   5,
					Size:  8,
					Align: consts.Right,
				})
			})
		})
	})

	// Table Headers
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

	m.Row(15, func() {
		m.Col(12, func() {
			m.Text(lang.L("task_history"), props.Text{
				Top:   10,
				Style: consts.Bold,
				Size:  14,
				Color: blueColor,
			})
		})
	})

	if groupBy == service.GroupByNone {
		rows := [][]string{}
		for _, e := range entries {
			dur := time.Duration(e.Duration) * time.Second
			if e.EndTime.IsZero() {
				dur = time.Since(e.StartTime)
			}

			rows = append(rows, []string{
				e.StartTime.Format("2006-01-02"),
				e.Description,
				utils.FormatDuration(dur),
			})
		}

		m.TableList(headers, rows, props.TableList{
			HeaderProp: props.TableListContent{
				Size:      10,
				GridSizes: []uint{2, 7, 3},
				Color:     whiteColor,
			},
			ContentProp: props.TableListContent{
				Size:      9,
				GridSizes: []uint{2, 7, 3},
			},
			Align:                consts.Center,
			AlternatedBackground: &color.Color{Red: 245, Green: 245, Blue: 245},
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
					utils.FormatDuration(dur),
				})
			}

			title := ""
			if len(groupEntries) > 0 {
				title = service.GetGroupTitle(groupEntries[0].StartTime, groupBy)
			}

			// Group Header Row
			m.Row(8, func() {
				m.Col(12, func() {
					m.Text(title, props.Text{
						Top:   2,
						Style: consts.Bold,
						Size:  11,
						Color: blueColor,
					})
				})
			})

			m.TableList(headers, rows, props.TableList{
				HeaderProp: props.TableListContent{
					Size:      9,
					GridSizes: []uint{2, 7, 3},
				},
				ContentProp: props.TableListContent{
					Size:      9,
					GridSizes: []uint{2, 7, 3},
				},
				Align:                consts.Center,
				AlternatedBackground: &color.Color{Red: 248, Green: 248, Blue: 248},
				HeaderContentSpace:   1,
				Line:                 false,
			})

			// Subtotal Footer
			m.Row(8, func() {
				m.Col(12, func() {
					m.Text(fmt.Sprintf("%s: %s", lang.L("subtotal"), utils.FormatDuration(groupTotal)), props.Text{
						Style: consts.Bold,
						Align: consts.Right,
						Size:  9,
					})
				})
			})
			m.Row(4, func() {}) // Spacer
		}
	}

	m.Row(5, func() {})
	m.Line(1.0, props.Line{Color: blueColor})

	// Summary Section
	m.Row(10, func() {
		m.ColSpace(6)
		m.Col(3, func() {
			m.Text(lang.L("total_time"), props.Text{
				Style: consts.Bold,
				Size:  12,
				Align: consts.Left,
			})
		})
		m.Col(3, func() {
			m.Text(utils.FormatDuration(totalDuration), props.Text{
				Style: consts.Normal,
				Size:  12,
				Align: consts.Right,
			})
		})
	})

	// Billing summary in PDF
	hourlyRate := viper.GetFloat64("hourly_rate")
	if hourlyRate > 0 {
		billingConfig := service.BillingConfig{
			HourlyRate: hourlyRate,
			MaxHours:   viper.GetFloat64("max_hours"),
			ExtraRate:  viper.GetFloat64("extra_rate"),
		}

		periodDays := int(end.Sub(start).Hours()/24) + 1
		billing := service.CalculateBilling(totalDuration, billingConfig, periodDays)

		m.Row(12, func() {
			m.ColSpace(6)
			m.Col(3, func() {
				m.Text(lang.L("total_cost"), props.Text{
					Style: consts.Bold,
					Align: consts.Left,
					Size:  14,
					Color: blueColor,
				})
			})
			m.Col(3, func() {
				m.Text(fmt.Sprintf("%.2f", billing.TotalCost), props.Text{
					Style: consts.Bold,
					Align: consts.Right,
					Size:  14,
					Color: blueColor,
				})
			})
		})

		if billing.ExtraCost > 0 {
			m.Row(8, func() {
				m.ColSpace(6)
				m.Col(3, func() {
					m.Text(lang.L("standard_cost"), props.Text{
						Style: consts.Normal,
						Align: consts.Left,
						Size:  10,
					})
				})
				m.Col(3, func() {
					m.Text(fmt.Sprintf("%.2f", billing.StandardCost), props.Text{
						Style: consts.Normal,
						Align: consts.Right,
						Size:  10,
					})
				})
			})
			m.Row(8, func() {
				m.ColSpace(6)
				m.Col(3, func() {
					m.Text(lang.L("extra_cost"), props.Text{
						Style: consts.Normal,
						Align: consts.Left,
						Size:  10,
					})
				})
				m.Col(3, func() {
					m.Text(fmt.Sprintf("%.2f", billing.ExtraCost), props.Text{
						Style: consts.Normal,
						Align: consts.Right,
						Size:  10,
					})
				})
			})
		}
	}

	return m.OutputFileAndClose(path)
}
