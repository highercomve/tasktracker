package ui

import (
	_ "embed"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/version"
)

//go:embed CHANGELOG.md
var changelogData string

func CheckVersion(w fyne.Window, s *store.Storage) {
	appState, _ := s.LoadAppState()
	// appState is zero-valued if error (LastRunVersion == ""), which triggers the update below.

	currentVersion := version.Version
	lastRunVersion := appState.LastRunVersion

	if lastRunVersion != currentVersion {
		// Version changed or first run
		showWelcomeDialog(w, currentVersion)
		updateVersion(s, currentVersion)
	}
}

func updateVersion(s *store.Storage, v string) {
	state, _ := s.LoadAppState()
	state.LastRunVersion = v
	s.SaveAppState(state)
}

func showWelcomeDialog(w fyne.Window, v string) {
	notes := parseChangelog(v)
	if notes == "" {
		return
	}

	content := widget.NewRichTextFromMarkdown(notes)

	// Wrap in a scroll container
	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	dlg := dialog.NewCustom("What's New in "+v, "Close", scroll, w)
	dlg.Resize(fyne.NewSize(500, 400))
	dlg.Show()
}

func parseChangelog(v string) string {
	// Simple parser to find the section for version v
	// Assumes format:
	// ## [vX.Y.Z]... or ## vX.Y.Z ...
	// Content until next ##

	lines := strings.Split(changelogData, "\n")
	var extracted []string
	capture := false

	// Cleanup version string just in case (remove 'v' prefix if double)
	searchVer := v
	if !strings.HasPrefix(v, "v") {
		// Changelog usually has tags like v1.0.0, but let's check
		// The template I used: ## [{{ .Tag.Name }}] ...
		// If version.Version is "1.0.0", tag might be "v1.0.0"
		// If version.Version is "v1.0.0", tag matches.
	}

	// Helper to check if line starts a version block
	isVersionHeader := func(line string, ver string) bool {
		if !strings.HasPrefix(line, "## ") {
			return false
		}
		// Check for [ver] or ver
		// ## [v1.0.0]...
		// ## v1.0.0...
		return strings.Contains(line, "["+ver+"]") || strings.Contains(line, " "+ver+" ") || strings.HasSuffix(line, " "+ver)
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// If we were capturing, we stop now as we hit the next version
			if capture {
				break
			}
			// Check if this is our version
			// Try both with and without 'v' prefix if needed
			if isVersionHeader(line, searchVer) || (!strings.HasPrefix(searchVer, "v") && isVersionHeader(line, "v"+searchVer)) {
				capture = true
				// We don't include the header itself in the welcome message, usually redundant with dialog title
				// But maybe we want it? Let's skip it.
				continue
			}
		}

		if capture {
			extracted = append(extracted, line)
		}
	}

	return strings.Join(extracted, "\n")
}
