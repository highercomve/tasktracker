package main

import (
	_ "embed" // Required for go:embed

	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"

	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/ui"
	"github.com/highercomve/tasktracker/internal/updater"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
)

//go:embed Icon.png
var embeddedIconBytes []byte

var userConfigFilePath string

func setupViper() error {
	viper.SetConfigName("tasktracker") // name of config file (without extension)
	viper.SetConfigType("yaml")        // or viper.SetConfigType("YAML")

	// Determine the user config directory
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("error getting user home directory: %w", err)
		}
		if runtime.GOOS == "windows" {
			configHome = filepath.Join(homeDir, "AppData", "Roaming")
		} else {
			configHome = filepath.Join(homeDir, ".config")
		}
	}

	// Set the full path to the user's config file
	userConfigFilePath = filepath.Join(configHome, "tasktracker", "tasktracker.yml")
	viper.SetConfigFile(userConfigFilePath)

	// Ensure the config directory exists
	err := os.MkdirAll(filepath.Dir(userConfigFilePath), 0755)
	if err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	viper.SetDefault("data_folder", "./data")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			log.Println("Config file not found; creating one with default values")
			if err := viper.WriteConfigAs(userConfigFilePath); err != nil {
				return fmt.Errorf("error creating config file: %w", err)
			}
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}
	return nil
}

func main() {
	os.Setenv("FYNE_SCALE", "auto")

	go func() {
		// Call self-update check at startup
		err := updater.SelfUpdate("highercomve", "tasktracker") // Replace with actual GitHub owner and repo
		if err != nil {
			log.Printf("Self-update failed: %v", err) // Use log for errors
		}
	}()

	a := app.NewWithID("com.highercomve.task-tracker")
	a.Settings().SetTheme(theme.DarkTheme())

	// Convert embedded bytes to a Fyne Resource
	iconResource := fyne.NewStaticResource("myappicon.png", embeddedIconBytes)
	a.SetIcon(iconResource)

	w := a.NewWindow("Task Tracker")
	w.Resize(fyne.NewSize(400, 600))

	if err := setupViper(); err != nil {
		dialog.ShowError(err, w)
		w.ShowAndRun()
		return
	}

	storage := store.NewStorage(viper.GetString("data_folder"))
	dashboard := ui.NewDashboard(storage)
	reports := ui.NewReports(storage)
	configUI := ui.NewConfig(w, storage, userConfigFilePath)

	tabs := container.NewAppTabs(
		container.NewTabItem("Tracker", dashboard.MakeUI()),
		container.NewTabItem("Reports", reports.MakeUI()),
		container.NewTabItem("Config", configUI.MakeUI()),
	)

	w.SetContent(tabs)

	ui.SetupTray(a, w, iconResource, dashboard)

	w.ShowAndRun()
}
