package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/highercomve/tasktracker/internal/i18n"
	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/utils"
	"github.com/spf13/viper"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// collect finds every object of type T under o, including inside widgets.
func collect[T any](o fyne.CanvasObject) []T {
	var out []T
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		if t, ok := o.(T); ok {
			out = append(out, t)
		}
		switch c := o.(type) {
		case *fyne.Container:
			for _, ch := range c.Objects {
				walk(ch)
			}
		case fyne.Widget:
			for _, ch := range test.WidgetRenderer(c).Objects() {
				walk(ch)
			}
		}
	}
	walk(o)
	return out
}

func buttonWithText(t *testing.T, o fyne.CanvasObject, text string) *widget.Button {
	t.Helper()
	for _, b := range collect[*widget.Button](o) {
		if b.Text == text {
			return b
		}
	}
	t.Fatalf("no %q button", text)
	return nil
}

func buttonWithIcon(t *testing.T, o fyne.CanvasObject, icon fyne.Resource) *widget.Button {
	t.Helper()
	for _, b := range collect[*widget.Button](o) {
		if b.Icon != nil && b.Icon.Name() == icon.Name() {
			return b
		}
	}
	t.Fatalf("no button with icon %s", icon.Name())
	return nil
}

func selectShowing(t *testing.T, o fyne.CanvasObject, selected string) *widget.Select {
	t.Helper()
	for _, s := range collect[*widget.Select](o) {
		if s.Selected == selected {
			return s
		}
	}
	t.Fatalf("no select showing %q", selected)
	return nil
}

// statValues are the values of the stat tiles: total time, entries, cost.
func statValues(o fyne.CanvasObject) []string {
	var v []string
	for _, l := range collect[*widget.Label](o) {
		if l.SizeName == sizeNameStat {
			v = append(v, l.Text)
		}
	}
	return v
}

func reportList(t *testing.T, o fyne.CanvasObject) *widget.List {
	t.Helper()
	lists := collect[*widget.List](o)
	if len(lists) != 1 {
		t.Fatalf("found %d report lists", len(lists))
	}
	return lists[0]
}

type reportFixture struct {
	storage  *store.Storage
	window   fyne.Window
	tabs     *container.AppTabs
	projects []models.Project
	now      time.Time
}

func newReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	a := test.NewTempApp(t)
	a.Settings().SetTheme(CurrentTheme())
	lang.AddTranslationsFS(i18n.TranslationsFS, "translations")
	viper.Set("hourly_rate", 60.0)
	t.Cleanup(func() { viper.Set("hourly_rate", 0.0) })

	s := store.NewStorage(t.TempDir())
	p1 := service.CreateProject("Pantavisor", "", "#3B82F6")
	p2 := service.CreateProject("PvFlasher", "", "#10B981")
	projects := []models.Project{p1, p2}
	if err := s.SaveProjects(projects); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, now.Location())
	add := func(d time.Time, desc, pid string, tags []string, mins int) {
		t.Helper()
		err := s.SaveEntry(models.TimeEntry{ID: uuid.NewString(), Description: desc, ProjectID: pid, Tags: tags,
			StartTime: d, EndTime: d.Add(time.Duration(mins) * time.Minute), Duration: int64(mins * 60),
			State: models.TaskStateStopped})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(day, "Review PR", p1.ID, []string{"review"}, 60)
	add(day.Add(2*time.Hour), "Write bmap code", p2.ID, []string{"dev"}, 90)
	add(day.Add(4*time.Hour), "Client call", "", []string{"meeting"}, 30)
	add(day.Add(5*time.Hour), "Release notes", "", nil, 15)
	add(day.AddDate(0, 0, -1), "Yesterday work", p1.ID, []string{"dev"}, 120)
	add(day.AddDate(0, 0, -40), "Old work", p2.ID, []string{"dev"}, 45)

	test.NewTempWindow(t, container.NewStack())
	w := safeGetMainWindow() // dialogs open on this window
	w.Resize(fyne.NewSize(900, 700))
	for _, x := range a.Driver().AllWindows() {
		x.Resize(fyne.NewSize(900, 700))
	}
	r := NewReports(s)
	w.SetContent(r.MakeUI())
	time.Sleep(300 * time.Millisecond) // toolbars are built after a short delay

	return &reportFixture{storage: s, window: w, tabs: w.Content().(*container.AppTabs), projects: projects, now: now}
}

func (f *reportFixture) tab(i int) fyne.CanvasObject {
	f.tabs.SelectIndex(i)
	time.Sleep(150 * time.Millisecond)
	return f.tabs.Items[i].Content
}

// expected returns the stat tile values for entries.
func expected(entries []models.TimeEntry, days int) []string {
	var total time.Duration
	for _, e := range entries {
		total += time.Duration(e.Duration) * time.Second
	}
	cost := service.CalculateBilling(total, service.BillingConfig{HourlyRate: 60}, days).TotalCost
	return []string{utils.FormatDuration(total), fmt.Sprint(len(entries)), fmt.Sprintf("%.2f", cost)}
}

func (f *reportFixture) entries(t *testing.T, start, end time.Time) []models.TimeEntry {
	t.Helper()
	e, err := f.storage.LoadEntriesForRange(start, end)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func assertStats(t *testing.T, o fyne.CanvasObject, want []string) {
	t.Helper()
	if got := statValues(o); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("stat tiles = %v, want %v", got, want)
	}
}

func TestDailyReport(t *testing.T) {
	f := newReportFixture(t)
	daily := f.tab(0)

	// Loaded on start, without switching sub-tabs first.
	today := f.entries(t, f.now, f.now)
	assertStats(t, daily, expected(today, 1))
	if n := reportList(t, daily).Length(); n != 4 {
		t.Errorf("daily list has %d rows, want 4", n)
	}

	// Previous day, then back to today.
	test.Tap(buttonWithIcon(t, daily, theme.NavigateBackIcon()))
	yesterday := f.now.AddDate(0, 0, -1)
	assertStats(t, daily, expected(f.entries(t, yesterday, yesterday), 1))
	test.Tap(buttonWithText(t, daily, lang.L("today")))
	assertStats(t, daily, expected(today, 1))

	// Search, then clear it from its chip.
	search := collect[*widget.Entry](daily)[0]
	search.SetText("review")
	if n := reportList(t, daily).Length(); n != 1 {
		t.Errorf("search: %d rows, want 1", n)
	}
	test.Tap(buttonWithIcon(t, daily, theme.CancelIcon()))
	if search.Text != "" || reportList(t, daily).Length() != 4 {
		t.Errorf("clearing the search chip left %q and %d rows", search.Text, reportList(t, daily).Length())
	}

	// Category filter.
	selectShowing(t, daily, lang.L("all_categories")).SetSelected("meeting")
	if n := reportList(t, daily).Length(); n != 1 {
		t.Errorf("category filter: %d rows, want 1", n)
	}
	test.Tap(buttonWithText(t, daily, lang.L("clear_all_filters")))
	if n := reportList(t, daily).Length(); n != 4 {
		t.Errorf("after Clear All: %d rows, want 4", n)
	}

	// Project filters, including tasks without a project.
	selectShowing(t, daily, lang.L("all_projects")).SetSelected("PvFlasher")
	if n := reportList(t, daily).Length(); n != 1 {
		t.Errorf("project filter: %d rows, want 1", n)
	}
	selectShowing(t, daily, "PvFlasher").SetSelected(lang.L("no_project"))
	if n := reportList(t, daily).Length(); n != 2 {
		t.Errorf("no-project filter: %d rows, want 2", n)
	}
	test.Tap(buttonWithText(t, daily, lang.L("clear_all_filters")))
	if n := reportList(t, daily).Length(); n != 4 {
		t.Errorf("after Clear All: %d rows, want 4", n)
	}
}

func TestReportGroupingAndTabs(t *testing.T) {
	f := newReportFixture(t)

	// Weekly: grouping adds a header and a subtotal row per group.
	weekly := f.tab(1)
	offset := int(f.now.Weekday())
	if offset == 0 {
		offset = 7
	}
	weekStart := f.now.AddDate(0, 0, -offset+1)
	week := f.entries(t, weekStart, weekStart.AddDate(0, 0, 6))
	assertStats(t, weekly, expected(week, 7))
	if n := reportList(t, weekly).Length(); n != len(week) {
		t.Errorf("weekly list has %d rows, want %d", n, len(week))
	}
	groupBy := selectShowing(t, weekly, lang.L("none"))
	groupBy.SetSelected(lang.L("project"))
	if n, want := reportList(t, weekly).Length(), len(week)+2*len(service.GroupByProjectID(week)); n != want {
		t.Errorf("grouped by project: %d rows, want %d", n, want)
	}
	groupBy.SetSelected(lang.L("daily"))
	days := map[string]bool{}
	for _, e := range week {
		days[e.StartTime.Format("2006-01-02")] = true
	}
	if n, want := reportList(t, weekly).Length(), len(week)+2*len(days); n != want {
		t.Errorf("grouped by day: %d rows, want %d", n, want)
	}
	groupBy.SetSelected(lang.L("none"))

	// Monthly.
	monthly := f.tab(2)
	monthStart := time.Date(f.now.Year(), f.now.Month(), 1, 0, 0, 0, 0, f.now.Location())
	month := f.entries(t, monthStart, monthStart.AddDate(0, 1, -1))
	if got := statValues(monthly); len(got) != 3 || got[1] != fmt.Sprint(len(month)) {
		t.Errorf("monthly tiles = %v, want %d entries", got, len(month))
	}

	// Custom range: All Time reaches the 40-day-old entry.
	custom := f.tab(3)
	test.Tap(buttonWithText(t, custom, lang.L("all_time")))
	if got := statValues(custom); len(got) != 3 || got[1] != "6" {
		t.Errorf("all time tiles = %v, want 6 entries", got)
	}
	test.Tap(buttonWithText(t, custom, lang.L("last_week")))
	if got := statValues(custom); len(got) != 3 || got[1] != "5" {
		t.Errorf("last week tiles = %v, want 5 entries", got)
	}
}

func TestReportEditAndDelete(t *testing.T) {
	f := newReportFixture(t)
	daily := f.tab(0)

	row := func(title string) *taskRow {
		for _, r := range collect[*taskRow](daily) {
			if r.title.Text == title {
				return r
			}
		}
		t.Fatalf("no row %q", title)
		return nil
	}

	// Edit the description.
	test.Tap(row("Client call").edit)
	dlg := f.window.Canvas().Overlays().Top()
	if dlg == nil {
		t.Fatal("edit dialog did not open")
	}
	collect[*widget.Entry](dlg)[0].SetText("Client call (edited)")
	test.Tap(buttonWithText(t, dlg, lang.L("save")))
	found := false
	for _, e := range f.entries(t, f.now, f.now) {
		found = found || e.Description == "Client call (edited)"
	}
	if !found {
		t.Error("edited description was not saved")
	}
	row("Client call (edited)") // the report shows it

	// Delete after confirming.
	test.Tap(row("Release notes").del)
	dlg = f.window.Canvas().Overlays().Top()
	if dlg == nil {
		t.Fatal("delete confirmation did not open")
	}
	test.Tap(buttonWithText(t, dlg, "Yes"))
	if n := len(f.entries(t, f.now, f.now)); n != 3 {
		t.Errorf("%d entries left today, want 3", n)
	}
	if n := reportList(t, daily).Length(); n != 3 {
		t.Errorf("report shows %d rows, want 3", n)
	}
}

func TestReportFiltersAreRestored(t *testing.T) {
	f := newReportFixture(t)
	daily := f.tab(0)
	collect[*widget.Entry](daily)[0].SetText("bmap")

	// A new Reports view (as after a restart) restores the saved search.
	r := NewReports(f.storage)
	f.window.SetContent(r.MakeUI())
	time.Sleep(300 * time.Millisecond)
	daily = f.window.Content().(*container.AppTabs).Items[0].Content
	if got := collect[*widget.Entry](daily)[0].Text; got != "bmap" {
		t.Errorf("restored search = %q", got)
	}
	if n := reportList(t, daily).Length(); n != 1 {
		t.Errorf("restored search shows %d rows, want 1", n)
	}
}

func TestReportCompactLayout(t *testing.T) {
	f := newReportFixture(t)
	for _, x := range fyne.CurrentApp().Driver().AllWindows() {
		x.Resize(fyne.NewSize(420, 700))
	}
	f.window.SetContent(NewReports(f.storage).MakeUI())
	time.Sleep(300 * time.Millisecond)
	daily := f.window.Content().(*container.AppTabs).Items[0].Content

	// Filters fold into a toggle on narrow windows.
	toggle := buttonWithText(t, daily, lang.L("filters"))
	before := len(collect[*widget.Select](daily))
	test.Tap(toggle)
	if after := len(collect[*widget.Select](daily)); after == before {
		t.Errorf("filter toggle did not change the visible filters (%d)", after)
	}
	if n := reportList(t, daily).Length(); n != 4 {
		t.Errorf("compact daily list has %d rows, want 4", n)
	}
}

func TestPDFExport(t *testing.T) {
	f := newReportFixture(t)
	start, end := f.now.AddDate(0, 0, -60), f.now
	entries := f.entries(t, start, end)
	for _, g := range []string{service.GroupByNone, service.GroupByDay, service.GroupByWeek, service.GroupByProject} {
		path := filepath.Join(t.TempDir(), "report-"+g+".pdf")
		if err := GeneratePDF(path, entries, start, end, g); err != nil {
			t.Fatalf("group %q: %v", g, err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() < 1000 {
			t.Errorf("group %q: PDF missing or empty (%v)", g, err)
		}
	}
}
