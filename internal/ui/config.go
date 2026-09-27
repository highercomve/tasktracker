package ui

import (
	"fmt"
	"path/filepath"

	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneDialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"
	"github.com/sqweek/dialog"
)

type Config struct {
	window             fyne.Window
	storage            *store.Storage
	userConfigFilePath string
	updater            *Updater
}

func NewConfig(w fyne.Window, s *store.Storage, userConfigFilePath string) *Config {
	return &Config{window: w, storage: s, userConfigFilePath: userConfigFilePath}
}

func (c *Config) MakeUI() fyne.CanvasObject {
	dataFolder := viper.GetString("data_folder")
	entry := widget.NewEntry()
	entry.SetText(dataFolder)

	idleEnabled := viper.GetBool("idle_detection")
	idleCheck := widget.NewCheck(lang.L("idle_detection"), nil)
	idleCheck.SetChecked(idleEnabled)

	idleThreshold := viper.GetInt("idle_threshold")
	if idleThreshold <= 0 {
		idleThreshold = 5
	}
	thresholdEntry := widget.NewEntry()
	thresholdEntry.SetText(fmt.Sprintf("%d", idleThreshold))

	hourlyRate := viper.GetFloat64("hourly_rate")
	hourlyRateEntry := widget.NewEntry()
	hourlyRateEntry.SetText(fmt.Sprintf("%.2f", hourlyRate))

	maxHours := viper.GetFloat64("max_hours")
	maxHoursEntry := widget.NewEntry()
	maxHoursEntry.SetText(fmt.Sprintf("%.2f", maxHours))

	extraRate := viper.GetFloat64("extra_rate")
	extraRateEntry := widget.NewEntry()
	extraRateEntry.SetText(fmt.Sprintf("%.2f", extraRate))

	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		path, err := dialog.Directory().Title(lang.L("data_folder")).Browse()
		if err != nil {
			if err != dialog.ErrCancelled {
				fyneDialog.ShowError(err, c.window)
			}
			return
		}
		if path != "" {
			entry.SetText(path)
		}
	})

	folderContainer := container.NewBorder(nil, nil, nil, browseBtn, entry)

	saveBtn := widget.NewButton(lang.L("save_configuration"), func() {
		newDataFolder := entry.Text
		if newDataFolder == "" {
			fyneDialog.ShowError(filepath.ErrBadPattern, c.window)
			return
		}

		newIdleEnabled := idleCheck.Checked
		newIdleThreshold := 5
		fmt.Sscanf(thresholdEntry.Text, "%d", &newIdleThreshold)

		var newHourlyRate, newMaxHours, newExtraRate float64
		fmt.Sscanf(hourlyRateEntry.Text, "%f", &newHourlyRate)
		fmt.Sscanf(maxHoursEntry.Text, "%f", &newMaxHours)
		fmt.Sscanf(extraRateEntry.Text, "%f", &newExtraRate)

		oldDataFolder := c.storage.BaseDir

		saveConfig := func() {
			viper.Set("data_folder", newDataFolder)
			viper.Set("idle_detection", newIdleEnabled)
			viper.Set("idle_threshold", newIdleThreshold)
			viper.Set("hourly_rate", newHourlyRate)
			viper.Set("max_hours", newMaxHours)
			viper.Set("extra_rate", newExtraRate)
			err := viper.WriteConfigAs(c.userConfigFilePath)
			if err != nil {
				fyneDialog.ShowError(err, c.window)
				return
			}
			fyneDialog.ShowInformation(lang.L("success"), lang.L("config_saved"), c.window)
		}

		if newDataFolder != oldDataFolder {
			// Ask user
			var d fyneDialog.Dialog

			moveBtn := widget.NewButton(lang.L("move_existing_data"), func() {
				d.Hide()
				if err := c.storage.MoveData(newDataFolder); err != nil {
					fyneDialog.ShowError(err, c.window)
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

			d = fyneDialog.NewCustom(lang.L("data_folder_changed_title"), lang.L("cancel"), content, c.window)
			d.Show()
			return
		}

		// Same folder, just save (maybe other settings in future)
		saveConfig()
	})

	saveBtn.Importance = widget.HighImportance
	saveBtn.SetIcon(theme.DocumentSaveIcon())

	eraseBtn := widget.NewButtonWithIcon(lang.L("erase_all_history"), theme.DeleteIcon(), func() {
		d := fyneDialog.NewConfirm(lang.L("erase_all_history"), lang.L("erase_history_confirm"), func(confirmed bool) {
			if confirmed {
				if err := c.storage.DeleteAllEntries(); err != nil {
					fyneDialog.ShowError(err, c.window)
				} else {
					fyneDialog.ShowInformation(lang.L("success"), lang.L("history_erased"), c.window)
				}
			}
		}, c.window)
		d.SetConfirmImportance(widget.DangerImportance)
		d.Show()
	})
	eraseBtn.Importance = widget.DangerImportance

	quitBtn := widget.NewButtonWithIcon(lang.L("quit_application"), theme.LogoutIcon(), func() {
		_ = viper.WriteConfigAs(c.userConfigFilePath)
		fyne.CurrentApp().Quit()
	})

	general := NewSurface(container.NewVBox(
		sectionTitle(lang.L("general_settings")),
		widget.NewForm(
			widget.NewFormItem(lang.L("data_folder"), folderContainer),
			widget.NewFormItem(lang.L("idle_detection"), idleCheck),
			widget.NewFormItem(lang.L("idle_threshold"), thresholdEntry),
		),
	))

	billing := NewSurface(container.NewVBox(
		sectionTitle(lang.L("billing_settings")),
		widget.NewForm(
			widget.NewFormItem(lang.L("hourly_rate"), hourlyRateEntry),
			widget.NewFormItem(lang.L("max_hours"), maxHoursEntry),
			widget.NewFormItem(lang.L("extra_rate"), extraRateEntry),
		),
	))

	appearance := NewSurface(container.NewVBox(
		sectionTitle(lang.L("appearance")),
		widget.NewForm(widget.NewFormItem(lang.L("theme"), c.themeSelector())),
	))

	updatesPanel := fyne.CanvasObject(container.NewVBox())
	if c.updater != nil {
		updatesPanel = NewSurface(c.updater.Panel())
	}

	danger := NewSurface(container.NewVBox(
		sectionTitle(lang.L("danger_zone")),
		eraseBtn,
		quitBtn,
	))

	settings := container.NewVBox(general, billing, container.NewHBox(layout.NewSpacer(), saveBtn))
	return Inset(12, container.NewVScroll(newAdaptiveColumns(760, settings, container.NewVBox(appearance, updatesPanel, danger))))
}

// themeSelector switches between following the desktop, light and dark. The
// choice applies and is saved straight away.
func (c *Config) themeSelector() fyne.CanvasObject {
	prefs := []string{ThemeSystem, ThemeLight, ThemeDark}
	labels := []string{lang.L("theme_system"), lang.L("theme_light"), lang.L("theme_dark")}

	radio := widget.NewRadioGroup(labels, nil)
	radio.Required = true
	current := ThemePreference()
	for i, p := range prefs {
		if p == current {
			radio.SetSelected(labels[i])
		}
	}
	radio.OnChanged = func(sel string) {
		for i, l := range labels {
			if l != sel {
				continue
			}
			viper.Set("theme", prefs[i])
			ApplyTheme(fyne.CurrentApp())
			if err := viper.WriteConfigAs(c.userConfigFilePath); err != nil {
				fyneDialog.ShowError(err, c.window)
			}
		}
	}
	return radio
}
