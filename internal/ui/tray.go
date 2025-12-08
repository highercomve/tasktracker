package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
)

func SetupTray(a fyne.App, w fyne.Window, icon fyne.Resource, d *Dashboard) {
	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu(lang.L("app_title"),
			fyne.NewMenuItem(lang.L("show"), func() {
				w.Show()
			}),
			fyne.NewMenuItem(lang.L("pause_resume"), func() {
				d.TogglePause()
			}),
			fyne.NewMenuItem(lang.L("stop"), func() {
				d.StopTask()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(lang.L("quit"), func() {
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
