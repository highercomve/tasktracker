package ui

import (
	"net/url"
	"strings"

	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DefaultWindowSize fits the tracker, reports and settings without scrolling.
var DefaultWindowSize = fyne.NewSize(900, 700)

const repoURL = "https://github.com/highercomve/tasktracker"

// BuildMainContent builds the app bar, the tabs and the footer of the main
// window. It returns the dashboard so the caller can wire shortcuts and the
// system tray to it.
func BuildMainContent(w fyne.Window, s *store.Storage, userConfigFilePath string, icon fyne.Resource) (fyne.CanvasObject, *Dashboard) {
	dashboard := NewDashboard(s)
	reports := NewReports(s)
	projects := NewProjects(s)
	configUI := NewConfig(w, s, userConfigFilePath)

	trackerTab := container.NewTabItemWithIcon(lang.L("tracker_tab"), theme.HistoryIcon(), dashboard.MakeUI())
	reportsTab := container.NewTabItemWithIcon(lang.L("reports_tab"), theme.DocumentIcon(), reports.MakeUI())
	projectsTab := container.NewTabItemWithIcon(lang.L("projects_tab"), theme.FolderIcon(), projects.MakeUI())
	configTab := container.NewTabItemWithIcon(lang.L("config_tab"), theme.SettingsIcon(), configUI.MakeUI())

	tabs := container.NewAppTabs(trackerTab, reportsTab, projectsTab, configTab)
	// Other tabs may have changed the data, so reload it when a tab is shown.
	tabs.OnSelected = func(item *container.TabItem) {
		refreshTabTheme(item)
		switch item {
		case trackerTab:
			dashboard.ReloadProjects()
		case reportsTab:
			reports.Refresh()
		case projectsTab:
			projects.Refresh()
		}
	}

	header := buildHeader(icon, dashboard.StatusBadge())
	return container.NewBorder(header, buildFooter(), nil, nil, tabs), dashboard
}

// buildHeader is the app bar: icon, name and tagline, plus the running timer.
func buildHeader(icon fyne.Resource, status fyne.CanvasObject) fyne.CanvasObject {
	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(32, 32))

	title := widget.NewLabelWithStyle(lang.L("app_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	subtitle := captionLabel(lang.L("app_subtitle"))
	text := container.NewVBox(title, subtitle)
	text.Layout = &tightVBox{gap: -14}

	bar := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(img), container.NewCenter(text)),
		container.NewCenter(status),
	)
	return container.NewVBox(Inset(8, bar), widget.NewSeparator())
}

// buildFooter shows the version and a link to the project.
func buildFooter() fyne.CanvasObject {
	ver := captionLabel(versionText())
	link := widget.NewHyperlink("GitHub", mustParseURL(repoURL))
	link.SizeName = theme.SizeNameCaptionText
	return container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(Inset(0, ver), layout.NewSpacer(), link),
	)
}

// versionText formats the build version for display.
func versionText() string {
	v := strings.TrimPrefix(version.Version, "v")
	if v == "" || v[0] < '0' || v[0] > '9' {
		return lang.L("app_title") + " · " + lang.L("development_build")
	}
	return lang.L("app_title") + " v" + v
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
