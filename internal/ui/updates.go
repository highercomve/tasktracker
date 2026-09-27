package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/highercomve/tasktracker/internal/update"
	"github.com/highercomve/tasktracker/internal/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"
)

// updateCheckInterval limits automatic checks to one a day. The app often
// runs for days from the tray, so it is re-evaluated every hour.
const (
	updateCheckInterval = 24 * time.Hour
	updateCheckPoll     = time.Hour
)

// Preference keys, kept in the Fyne preferences rather than the YAML config.
const (
	prefUpdateCheckDisabled = "update_check_disabled"
	prefUpdateLastCheck     = "update_last_check"
	prefUpdateSkipped       = "update_skipped_version"
)

// Updater offers new releases in the GUI: a daily background check, a footer
// link, the Updates section in Configuration and the update dialog.
type Updater struct {
	app    fyne.App
	window fyne.Window

	pending *update.Release
	status  *update.Status
	err     error
	checked bool // a check finished during this session

	link         *widget.Hyperlink
	listeners    []func()
	refreshPanel func()
}

// NewUpdater creates the updater for the main window.
func NewUpdater(a fyne.App, w fyne.Window) *Updater {
	u := &Updater{app: a, window: w}
	u.link = widget.NewHyperlink("", nil)
	u.link.OnTapped = u.showUpdateDialog
	u.link.TextStyle = fyne.TextStyle{Bold: true}
	u.link.SizeName = theme.SizeNameCaptionText
	u.link.Hide()
	return u
}

// Link is the footer link shown once an update is found.
func (u *Updater) Link() fyne.CanvasObject { return u.link }

// Start cleans up after a previous update and runs the automatic checks.
func (u *Updater) Start() {
	update.CleanupPrevious()
	if !update.IsReleaseVersion(version.Version) {
		return
	}
	go func() {
		for {
			if u.dueForCheck() {
				u.check()
			}
			time.Sleep(updateCheckPoll)
		}
	}()
}

func (u *Updater) autoCheckEnabled() bool {
	return !u.app.Preferences().Bool(prefUpdateCheckDisabled)
}

func (u *Updater) dueForCheck() bool {
	if !u.autoCheckEnabled() {
		return false
	}
	last, err := time.Parse(time.RFC3339, u.app.Preferences().String(prefUpdateLastCheck))
	return err != nil || time.Since(last) >= updateCheckInterval
}

// check runs an automatic check, offering an update the user hasn't skipped.
func (u *Updater) check() {
	st, err := u.fetch()
	if err != nil || st.Update == nil {
		return
	}
	fyne.Do(func() {
		if u.app.Preferences().String(prefUpdateSkipped) != st.Update.Version {
			u.showUpdateDialog()
		}
	})
}

// fetch checks the latest release and records the result: an available
// update shows the footer link and every listener is told.
func (u *Updater) fetch() (*update.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := update.Check(ctx, version.Version)
	if err == nil {
		u.app.Preferences().SetString(prefUpdateLastCheck, time.Now().Format(time.RFC3339))
	}
	fyne.Do(func() {
		u.status, u.err, u.checked = st, err, true
		if err == nil {
			u.pending = st.Update
		}
		if u.pending != nil {
			u.link.SetText(fmt.Sprintf(lang.L("update_available_link"), u.pending.Version))
			u.link.Show()
		} else {
			u.link.Hide()
		}
		for _, l := range u.listeners {
			l()
		}
	})
	return st, err
}

// Panel is the Updates section of the Configuration tab. It shows the running
// and latest versions, with the update action when one is available.
func (u *Updater) Panel() fyne.CanvasObject {
	current := widget.NewLabelWithStyle(currentVersionText(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	latest := widget.NewLabel("—")
	state := widget.NewLabel("")
	state.Importance = widget.LowImportance
	state.SizeName = theme.SizeNameCaptionText
	state.Wrapping = fyne.TextWrapWord
	spinner := widget.NewActivity()
	spinner.Hide()

	action := widget.NewButtonWithIcon(lang.L("check_for_updates"), theme.ViewRefreshIcon(), nil)

	var refresh func()
	show := func() {
		spinner.Stop()
		spinner.Hide()
		action.Enable()
		action.Importance = widget.MediumImportance
		action.SetText(lang.L("check_again"))
		action.SetIcon(theme.ViewRefreshIcon())
		action.OnTapped = refresh
		st, err := u.status, u.err
		switch {
		case err != nil:
			latest.SetText(lang.L("unknown"))
			state.SetText(lang.L("update_server_unreachable"))
		case st == nil:
			latest.SetText("—")
			state.SetText("")
			action.SetText(lang.L("check_for_updates"))
		case st.DevBuild:
			latest.SetText("v" + st.Latest)
			state.SetText(lang.L("update_dev_build"))
		case st.Update == nil:
			latest.SetText("v" + st.Latest)
			state.SetText(lang.L("update_up_to_date"))
		default:
			latest.SetText("v" + st.Latest)
			if st.Update.Install.CanSelfUpdate {
				state.SetText(lang.L("update_available_state"))
			} else {
				state.SetText(fmt.Sprintf(lang.L("update_available_managed"), st.Update.Install.Reason))
			}
			action.Importance = widget.HighImportance
			action.SetText(fmt.Sprintf(lang.L("update_to"), st.Update.Version))
			action.SetIcon(theme.DownloadIcon())
			action.OnTapped = u.showUpdateDialog
		}
		action.Refresh()
	}
	refresh = func() {
		latest.SetText(lang.L("checking"))
		state.SetText("")
		spinner.Show()
		spinner.Start()
		action.Disable()
		go u.fetch()
	}
	u.listeners = append(u.listeners, show)
	u.refreshPanel = refresh
	show()

	auto := widget.NewCheck(lang.L("check_updates_automatically"), func(on bool) {
		u.app.Preferences().SetBool(prefUpdateCheckDisabled, !on)
	})
	auto.SetChecked(u.autoCheckEnabled())
	if !update.IsReleaseVersion(version.Version) {
		auto.Disable()
	}

	form := widget.NewForm(
		widget.NewFormItem(lang.L("current_version"), current),
		widget.NewFormItem(lang.L("latest_version"), container.NewHBox(latest, spinner)),
	)
	return container.NewVBox(
		sectionTitle(lang.L("updates")),
		form,
		state,
		container.NewHBox(action),
		auto,
	)
}

// CheckOnce runs a check the first time the Updates section is shown, so it
// shows the latest version without waiting for the daily check.
func (u *Updater) CheckOnce() {
	if u.checked {
		return
	}
	u.checked = true
	if u.refreshPanel != nil {
		u.refreshPanel()
		return
	}
	go u.fetch()
}

// showUpdateDialog presents the release notes and the install choice.
func (u *Updater) showUpdateDialog() {
	rel := u.pending
	if rel == nil {
		return
	}

	intro := widget.NewLabel(fmt.Sprintf(lang.L("update_intro"), rel.Version, currentVersionText()))
	intro.Wrapping = fyne.TextWrapWord

	notesText := rel.Notes
	if notesText == "" {
		notesText = lang.L("update_no_notes")
	}
	notes := widget.NewRichTextFromMarkdown(notesText)
	notes.Wrapping = fyne.TextWrapWord
	notesScroll := container.NewVScroll(notes)
	notesScroll.SetMinSize(fyne.NewSize(460, 180))

	content := container.NewVBox(intro, widget.NewSeparator(), notesScroll)
	d := dialog.NewCustomWithoutButtons(lang.L("update_title"), content, u.window)

	later := widget.NewButton(lang.L("later"), d.Hide)
	skip := widget.NewButton(lang.L("skip_version"), func() {
		u.app.Preferences().SetString(prefUpdateSkipped, rel.Version)
		d.Hide()
	})

	var primary *widget.Button
	if rel.Install.CanSelfUpdate {
		note := captionLabel(lang.L("update_timer_note"))
		note.Wrapping = fyne.TextWrapWord
		content.Add(note)
		primary = widget.NewButtonWithIcon(lang.L("install_and_restart"), theme.DownloadIcon(), func() {
			d.Hide()
			u.install(rel)
		})
	} else {
		// Flatpak, package-managed or read-only installs are updated by other means.
		note := widget.NewLabel(fmt.Sprintf(lang.L("update_cannot_self"), rel.Install.Reason))
		note.Wrapping = fyne.TextWrapWord
		note.Importance = widget.LowImportance
		content.Add(note)
		primary = widget.NewButtonWithIcon(lang.L("open_download_page"), theme.ComputerIcon(), func() {
			_ = u.app.OpenURL(mustParseURL(update.ReleasesPage))
			d.Hide()
		})
	}
	primary.Importance = widget.HighImportance

	d.SetButtons([]fyne.CanvasObject{skip, later, primary})
	d.Resize(fyne.NewSize(520, 0))
	d.Show()
}

// install downloads, verifies and installs the update, then restarts. The
// active task is kept in the data folder, so its timer carries on.
func (u *Updater) install(rel *update.Release) {
	bar := widget.NewProgressBar()
	status := widget.NewLabel(lang.L("downloading"))
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn := widget.NewButton(lang.L("cancel"), cancel)

	d := dialog.NewCustomWithoutButtons(lang.L("updating_title"),
		container.NewVBox(status, minWidth(400, bar)), u.window)
	d.SetButtons([]fyne.CanvasObject{cancelBtn})
	d.Show()

	go func() {
		defer cancel()
		err := update.Apply(ctx, rel, func(done, total int64) {
			fyne.Do(func() {
				if total > 0 {
					bar.SetValue(float64(done) / float64(total))
				}
				if total > 0 && done >= total {
					// Verifying and unpacking follow the download.
					status.SetText(lang.L("installing"))
					cancelBtn.Disable()
				} else {
					status.SetText(fmt.Sprintf(lang.L("downloading_progress"), formatBytes(done), formatBytes(total)))
				}
			})
		})
		fyne.Do(func() {
			d.Hide()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				dialog.ShowError(fmt.Errorf("%s: %w", lang.L("update_failed"), err), u.window)
				return
			}
			_ = viper.WriteConfigAs(viper.ConfigFileUsed())
			if err := update.Relaunch(rel.Install); err != nil {
				dialog.ShowInformation(lang.L("update_installed_title"),
					fmt.Sprintf(lang.L("update_installed_restart"), rel.Version), u.window)
				return
			}
			u.app.Quit()
		})
	}()
}

// currentVersionText is the running version, e.g. "v0.2.0".
func currentVersionText() string {
	v := strings.TrimPrefix(version.Version, "v")
	if v == "" || v[0] < '0' || v[0] > '9' {
		return lang.L("development_build")
	}
	return "v" + v
}

// formatBytes renders a size such as "12.3 MB"; unknown sizes show "?".
func formatBytes(n int64) string {
	if n < 0 {
		return "?"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
