package ui

import (
	"testing"
	"time"

	"github.com/highercomve/tasktracker/internal/i18n"
	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func newUITest(t *testing.T) (*store.Storage, fyne.Window) {
	t.Helper()
	a := test.NewTempApp(t)
	a.Settings().SetTheme(CurrentTheme())
	lang.AddTranslationsFS(i18n.TranslationsFS, "translations")
	test.NewTempWindow(t, container.NewStack())
	w := safeGetMainWindow() // dialogs open on this window
	for _, x := range a.Driver().AllWindows() {
		x.Resize(fyne.NewSize(900, 700))
	}
	return store.NewStorage(t.TempDir()), w
}

func TestTrackerStartPauseResumeStop(t *testing.T) {
	s, w := newUITest(t)
	p := service.CreateProject("Pantavisor", "", "#3B82F6")
	s.SaveProjects([]models.Project{p})

	d := NewDashboard(s)
	w.SetContent(d.MakeUI())
	defer d.StopTicker()

	// Start with a project and categories.
	d.projectSelect.SetSelected("Pantavisor")
	d.categoryEntry.SetText("dev, review")
	test.Type(d.taskEntry, "Write tests")
	d.taskEntry.OnSubmitted(d.taskEntry.Text)
	if d.GetActiveState() != models.TaskStateRunning || d.startBtn.Text != lang.L("stop") || d.pauseBtn.Disabled() {
		t.Fatalf("after start: state %d, start button %q", d.GetActiveState(), d.startBtn.Text)
	}
	if d.statusLabel.Text != lang.L("tracking")+": Write tests" {
		t.Errorf("status = %q", d.statusLabel.Text)
	}
	entries, _ := s.LoadEntries(time.Now())
	if len(entries) != 1 || entries[0].ProjectID != p.ID || len(entries[0].Tags) != 2 {
		t.Fatalf("saved entry = %+v", entries)
	}

	// Pause and resume, from the button and the tray.
	test.Tap(d.pauseBtn)
	if d.GetActiveState() != models.TaskStatePaused || d.pauseBtn.Text != lang.L("resume") {
		t.Errorf("after pause: state %d, pause button %q", d.GetActiveState(), d.pauseBtn.Text)
	}
	d.TogglePause()
	if d.GetActiveState() != models.TaskStateRunning {
		t.Errorf("after tray resume: state %d", d.GetActiveState())
	}

	// Stop.
	test.Tap(d.startBtn)
	if d.GetActiveState() != models.TaskStateStopped || d.startBtn.Text != lang.L("start") || !d.pauseBtn.Disabled() {
		t.Errorf("after stop: state %d, start button %q", d.GetActiveState(), d.startBtn.Text)
	}
	entries, _ = s.LoadEntries(time.Now())
	if entries[0].State != models.TaskStateStopped || entries[0].EndTime.IsZero() {
		t.Errorf("stopped entry = %+v", entries[0])
	}
	if state, _ := s.LoadAppState(); state.ActiveTaskID != "" {
		t.Errorf("active task still saved: %+v", state)
	}

	// Search the list.
	d.searchEntry.SetText("nothing like this")
	if len(d.taskList) != 0 {
		t.Errorf("search kept %d tasks", len(d.taskList))
	}
	d.searchEntry.SetText("")
	if len(d.taskList) != 1 {
		t.Errorf("list has %d tasks", len(d.taskList))
	}
}

func TestTrackerResumesActiveTask(t *testing.T) {
	s, w := newUITest(t)
	d := NewDashboard(s)
	w.SetContent(d.MakeUI())
	d.StartTask("Survives a restart", "", nil)
	d.StopTicker()

	// A new dashboard (as after a restart or self-update) picks it up.
	d2 := NewDashboard(s)
	w.SetContent(d2.MakeUI())
	defer d2.StopTicker()
	if d2.GetActiveState() != models.TaskStateRunning || d2.activeDescription() != "Survives a restart" {
		t.Errorf("restored state %d, task %q", d2.GetActiveState(), d2.activeDescription())
	}
}

func TestProjectsCreateEditDelete(t *testing.T) {
	s, w := newUITest(t)
	p := NewProjects(s)
	w.SetContent(p.MakeUI())

	// Create, picking a preset colour.
	test.Tap(buttonWithText(t, w.Content(), lang.L("new_project")))
	dlg := w.Canvas().Overlays().Top()
	if dlg == nil {
		t.Fatal("create dialog did not open")
	}
	entries := collect[*widget.Entry](dlg)
	entries[0].SetText("Consulting")
	entries[1].SetText("Client work")
	var preset *widget.Button
	for _, b := range collect[*widget.Button](dlg) {
		if b.Text == "" && b.Icon == nil {
			preset = b // first colour swatch
			break
		}
	}
	test.Tap(preset)
	if entries[2].Text != projectPalette[0] {
		t.Errorf("preset set colour %q", entries[2].Text)
	}
	test.Tap(buttonWithText(t, dlg, lang.L("create")))
	projects, _ := s.LoadProjects()
	if len(projects) != 1 || projects[0].Name != "Consulting" || projects[0].ColorHex != projectPalette[0] {
		t.Fatalf("saved projects = %+v", projects)
	}

	// A task assigned to it counts in the row, and survives the delete.
	s.SaveEntry(models.TimeEntry{ID: "t1", Description: "Call", ProjectID: projects[0].ID,
		StartTime: time.Now().Add(-time.Hour), EndTime: time.Now(), Duration: 3600, State: models.TaskStateStopped})
	p.Refresh()
	if st := p.cachedProjectStats(projects[0].ID); st.EntryCount != 1 || st.TotalTime != time.Hour {
		t.Errorf("stats = %+v", st)
	}

	// Edit.
	rows := collect[*projectRow](w.Content())
	if len(rows) != 1 {
		t.Fatalf("%d project rows", len(rows))
	}
	test.Tap(rows[0].edit)
	dlg = w.Canvas().Overlays().Top()
	collect[*widget.Entry](dlg)[0].SetText("Consulting Inc")
	test.Tap(buttonWithText(t, dlg, lang.L("save")))
	if projects, _ = s.LoadProjects(); projects[0].Name != "Consulting Inc" {
		t.Errorf("edited name = %q", projects[0].Name)
	}

	// Delete: the task is unassigned, not deleted.
	test.Tap(collect[*projectRow](w.Content())[0].del)
	test.Tap(buttonWithText(t, w.Canvas().Overlays().Top(), "Yes"))
	if projects, _ = s.LoadProjects(); len(projects) != 0 {
		t.Errorf("projects after delete = %+v", projects)
	}
	tasks, _ := s.LoadEntries(time.Now())
	if len(tasks) != 1 || tasks[0].ProjectID != "" {
		t.Errorf("tasks after project delete = %+v", tasks)
	}
}
