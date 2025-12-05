package ui

import (
	"path/filepath"

	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

	saveBtn := widget.NewButton("Save Configuration", func() {
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
			dialog.ShowInformation("Success", "Configuration saved successfully.\nData folder updated.", c.window)
		}

		if newDataFolder != oldDataFolder {
			// Ask user
			var d dialog.Dialog
			
			moveBtn := widget.NewButton("Move Existing Data", func() {
				d.Hide()
				if err := c.storage.MoveData(newDataFolder); err != nil {
					dialog.ShowError(err, c.window)
					return
				}
				saveConfig()
			})
			
			freshBtn := widget.NewButton("Start Fresh", func() {
				d.Hide()
				c.storage.UpdateBaseDir(newDataFolder)
				saveConfig()
			})

			content := container.NewVBox(
				widget.NewLabel("You have changed the data folder.\nDo you want to move existing data to the new location?"),
				container.NewHBox(moveBtn, freshBtn),
			)

			d = dialog.NewCustom("Data Folder Changed", "Cancel", content, c.window)
			d.Show()
			return
		}

		// Same folder, just save (maybe other settings in future)
		saveConfig()
	})

	eraseBtn := widget.NewButtonWithIcon("Erase All History", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Erase All History", "Are you sure you want to delete ALL task history? This action cannot be undone.", func(confirmed bool) {
			if confirmed {
				if err := c.storage.DeleteAllEntries(); err != nil {
					dialog.ShowError(err, c.window)
				} else {
					dialog.ShowInformation("Success", "All history has been erased.", c.window)
				}
			}
		}, c.window)
	})
	eraseBtn.Importance = widget.DangerImportance

	return container.NewVBox(
		widget.NewLabel("Configuration"),
		widget.NewForm(
			widget.NewFormItem("Data Folder", folderContainer),
		),
		saveBtn,
		widget.NewSeparator(),
		eraseBtn,
	)
}
