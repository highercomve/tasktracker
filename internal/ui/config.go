package ui

import (
	"path/filepath"

	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"
)

type Config struct {
	window           fyne.Window
	storage          *store.Storage
	userConfigFilePath string
}

func NewConfig(w fyne.Window, s *store.Storage, userConfigFilePath string) *Config {
	return &Config{window: w, storage: s, userConfigFilePath: userConfigFilePath}
}

func (c *Config) MakeUI() fyne.CanvasObject {
	dataFolder := viper.GetString("data_folder")
	entry := widget.NewEntry()
	entry.SetText(dataFolder)

	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, c.window)
				return
			}
			if uri == nil {
				return
			}
			entry.SetText(uri.Path())
		}, c.window).Show()
	})

	folderContainer := container.NewBorder(nil, nil, nil, browseBtn, entry)

	saveBtn := widget.NewButton(lang.L("save_configuration"), func() {
		newDataFolder := entry.Text
		if newDataFolder == "" {
			dialog.ShowError(filepath.ErrBadPattern, c.window)
			return
		}

		oldDataFolder := c.storage.BaseDir

		saveConfig := func() {
			viper.Set("data_folder", newDataFolder)
			err := viper.WriteConfigAs(c.userConfigFilePath)
			if err != nil {
				dialog.ShowError(err, c.window)
				return
			}
			dialog.ShowInformation(lang.L("success"), lang.L("config_saved"), c.window)
		}

		if newDataFolder != oldDataFolder {
			// Ask user
			var d dialog.Dialog
			
			moveBtn := widget.NewButton(lang.L("move_existing_data"), func() {
				d.Hide()
				if err := c.storage.MoveData(newDataFolder); err != nil {
					dialog.ShowError(err, c.window)
					return
				}
				saveConfig()
			})
			
			freshBtn := widget.NewButton(lang.L("start_fresh"), func() {
				d.Hide()
				c.storage.UpdateBaseDir(newDataFolder)
				saveConfig()
			})

			content := container.NewVBox(
				widget.NewLabel(lang.L("data_folder_changed_msg")),
				container.NewHBox(moveBtn, freshBtn),
			)

			d = dialog.NewCustom(lang.L("data_folder_changed_title"), lang.L("cancel"), content, c.window)
			d.Show()
			return
		}

		// Same folder, just save (maybe other settings in future)
		saveConfig()
	})

	eraseBtn := widget.NewButtonWithIcon(lang.L("erase_all_history"), theme.DeleteIcon(), func() {
		dialog.ShowConfirm(lang.L("erase_all_history"), lang.L("erase_history_confirm"), func(confirmed bool) {
			if confirmed {
				if err := c.storage.DeleteAllEntries(); err != nil {
					dialog.ShowError(err, c.window)
				} else {
					dialog.ShowInformation(lang.L("success"), lang.L("history_erased"), c.window)
				}
			}
		}, c.window)
	})
	eraseBtn.Importance = widget.DangerImportance

	quitBtn := widget.NewButtonWithIcon(lang.L("quit_application"), theme.LogoutIcon(), func() {
		fyne.CurrentApp().Quit()
	})

	return container.NewVBox(
		widget.NewLabel(lang.L("config_tab")),
		widget.NewForm(
			widget.NewFormItem(lang.L("data_folder"), folderContainer),
		),
		saveBtn,
		widget.NewSeparator(),
		eraseBtn,
		widget.NewSeparator(),
		quitBtn,
	)
}
