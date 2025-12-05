package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func SetupTray(a fyne.App, w fyne.Window, icon fyne.Resource, d *Dashboard) {
	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("Task Tracker",
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}),
			fyne.NewMenuItem("Pause/Resume", func() {
				d.TogglePause()
			}),
			fyne.NewMenuItem("Stop", func() {
				d.StopTask()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
		desk.SetSystemTrayIcon(icon)
	}

	w.SetCloseIntercept(func() {
		w.Hide()
	})
}
