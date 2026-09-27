package ui

import (
	"fmt"
	"image/color"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/utils"
	"github.com/spf13/viper"
	"github.com/sqweek/dialog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneDialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// openFile opens a file with the OS default application. It avoids building a
// file:// URL (Windows paths like C:\... produce an invalid URL, which would
// crash OpenURL). On Windows rundll32 is used so no console window is spawned.
func openFile(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

type Reports struct {
	storage      *store.Storage
	filterStates map[string]*FilterStateManager
	projects     []models.Project

	// refreshCurrent reloads the report tab that is shown
	refreshCurrent func()
	// projectSelectors are the project filters of every report tab
	projectSelectors []*widget.Select
}

func NewReports(s *store.Storage) *Reports {
	return &Reports{
		storage:      s,
		filterStates: make(map[string]*FilterStateManager),
	}
}

// Refresh reloads the shown report and the project filter options, picking
// up time tracked and projects edited since it was built.
func (r *Reports) Refresh() {
	if projects, err := r.storage.LoadProjects(); err == nil {
		r.projects = projects
	}
	options := []string{lang.L("all_projects"), lang.L("no_project")}
	for _, p := range r.projects {
		options = append(options, p.Name)
	}
	for _, sel := range r.projectSelectors {
		selected := sel.Selected
		sel.Options = options
		known := false
		for _, o := range options {
			if o == selected {
				known = true
				break
			}
		}
		if !known && sel.OnChanged != nil {
			// The selected project is gone; fall back to all projects.
			sel.SetSelected(lang.L("all_projects"))
		}
		sel.Refresh()
	}
	if r.refreshCurrent != nil {
		r.refreshCurrent()
	}
}

// labeledControl puts a small caption above a filter control.
func labeledControl(label string, control fyne.CanvasObject) fyne.CanvasObject {
	c := captionLabel(strings.TrimSuffix(strings.TrimSpace(label), ":"))
	box := container.NewVBox(c, control)
	box.Layout = &tightVBox{gap: -4}
	return box
}

// getFilterStateManager returns or creates a filter state manager for a tab
func (r *Reports) getFilterStateManager(tabName string) *FilterStateManager {
	if fsm, ok := r.filterStates[tabName]; ok {
		return fsm
	}
	fsm := NewFilterStateManager(fyne.CurrentApp(), tabName)
	r.filterStates[tabName] = fsm
	return fsm
}

// createResponsiveToolbar creates a toolbar that adapts to screen size
func (r *Reports) createResponsiveToolbar(
	canvas fyne.Canvas,
	navControls []fyne.CanvasObject,
	filterControls []fyne.CanvasObject,
	filterBadgeContainer *fyne.Container,
	onToggleFilters func(bool),
	filterState *FilterStateManager,
) fyne.CanvasObject {
	// Check screen size and build layout accordingly
	isCompact := IsCompactScreen(canvas)

	// Give the search entry and bare buttons a caption slot too, so every
	// filter lines up with the labelled selectors.
	filters := make([]fyne.CanvasObject, 0, len(filterControls))
	for _, ctrl := range filterControls {
		switch ctrl.(type) {
		case *widget.Entry:
			filters = append(filters, labeledControl(lang.L("search"), ctrl))
		case *widget.Button:
			filters = append(filters, labeledControl(" ", ctrl))
		default:
			filters = append(filters, ctrl)
		}
	}

	if isCompact {
		// Filter panel content (simple container, no header)
		filterContent := container.NewVBox(filters...)

		// Start hidden/shown based on saved state
		expanded := filterState.GetState().PanelExpanded
		if !expanded {
			filterContent.Hide()
		}

		// Filter toggle button with dropdown icon, aligned with navigation
		var filterBtn *widget.Button
		if expanded {
			filterBtn = widget.NewButtonWithIcon(lang.L("filters"), theme.MenuDropUpIcon(), nil)
		} else {
			filterBtn = widget.NewButtonWithIcon(lang.L("filters"), theme.MenuDropDownIcon(), nil)
		}

		filterBtn.OnTapped = func() {
			expanded = !expanded
			filterState.SetPanelExpanded(expanded)
			if expanded {
				filterContent.Show()
				filterBtn.SetIcon(theme.MenuDropUpIcon())
			} else {
				filterContent.Hide()
				filterBtn.SetIcon(theme.MenuDropDownIcon())
			}
			if onToggleFilters != nil {
				onToggleFilters(expanded)
			}
		}

		// Navigation row with filter toggle button aligned at the end
		navRow := container.NewHBox(navControls...)
		navRow.Add(layout.NewSpacer())
		navRow.Add(filterBtn)

		return container.NewVBox(
			NewSurface(container.NewVBox(navRow, filterContent)),
			filterBadgeContainer,
		)
	}

	// Full layout: navigation on top, then every filter side by side
	fullNavRow := container.NewHBox(navControls...)

	return container.NewVBox(
		NewSurface(container.NewVBox(
			fullNavRow,
			container.NewGridWithColumns(len(filters), filters...),
		)),
		filterBadgeContainer,
	)
}

// createFilterBadgeContainer creates a container for active filter badges
func (r *Reports) createFilterBadgeContainer(
	searchQuery string,
	selectedCategory string,
	defaultCategory string,
	onClearSearch func(),
	onClearCategory func(),
	onClearAll func(),
) *fyne.Container {
	badges := container.NewHBox()

	hasFilters := false

	if searchQuery != "" {
		hasFilters = true
		searchBadge := widget.NewButton(fmt.Sprintf("%s: %s", lang.L("search_tasks")[:6], searchQuery), nil)
		searchBadge.Importance = widget.MediumImportance
		clearSearchBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClearSearch)
		clearSearchBtn.Importance = widget.LowImportance
		badges.Add(container.NewHBox(searchBadge, clearSearchBtn))
	}

	if selectedCategory != "" && selectedCategory != defaultCategory {
		hasFilters = true
		catBadge := widget.NewButton(fmt.Sprintf("%s: %s", lang.L("category"), selectedCategory), nil)
		catBadge.Importance = widget.MediumImportance
		clearCatBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClearCategory)
		clearCatBtn.Importance = widget.LowImportance
		badges.Add(container.NewHBox(catBadge, clearCatBtn))
	}

	if hasFilters {
		clearAllBtn := widget.NewButtonWithIcon(lang.L("clear_all_filters"), theme.ContentClearIcon(), onClearAll)
		clearAllBtn.Importance = widget.LowImportance
		badges.Add(clearAllBtn)
	}

	return badges
}

// updateFilterBadges updates the filter badge container
func (r *Reports) updateFilterBadges(
	badgeContainer *fyne.Container,
	searchQuery string,
	selectedCategory string,
	defaultCategory string,
	onClearSearch func(),
	onClearCategory func(),
	onClearAll func(),
) {
	badgeContainer.Objects = nil

	hasFilters := false

	if searchQuery != "" {
		hasFilters = true
		searchBadge := widget.NewButton(fmt.Sprintf("Search: %s", searchQuery), nil)
		searchBadge.Importance = widget.MediumImportance
		clearSearchBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClearSearch)
		clearSearchBtn.Importance = widget.LowImportance
		badgeContainer.Add(container.NewHBox(searchBadge, clearSearchBtn))
	}

	if selectedCategory != "" && selectedCategory != defaultCategory {
		hasFilters = true
		catBadge := widget.NewButton(fmt.Sprintf("%s: %s", lang.L("category"), selectedCategory), nil)
		catBadge.Importance = widget.MediumImportance
		clearCatBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClearCategory)
		clearCatBtn.Importance = widget.LowImportance
		badgeContainer.Add(container.NewHBox(catBadge, clearCatBtn))
	}

	if hasFilters {
		clearAllBtn := widget.NewButtonWithIcon(lang.L("clear_all_filters"), theme.ContentClearIcon(), onClearAll)
		clearAllBtn.Importance = widget.LowImportance
		badgeContainer.Add(clearAllBtn)
	}

	badgeContainer.Refresh()
}

// updateFilterBadgesWithProject updates the filter badge container including project filter
func (r *Reports) updateFilterBadgesWithProject(
	badgeContainer *fyne.Container,
	searchQuery string,
	selectedCategory string,
	selectedProject string,
	defaultCategory string,
	defaultProject string,
	onClearSearch func(),
	onClearCategory func(),
	onClearProject func(),
	onClearAll func(),
) {
	badgeContainer.Objects = nil

	hasFilters := false

	if searchQuery != "" {
		hasFilters = true
		badgeContainer.Add(filterChip(fmt.Sprintf(lang.L("search_filter"), searchQuery), onClearSearch))
	}

	if selectedCategory != "" && selectedCategory != defaultCategory {
		hasFilters = true
		badgeContainer.Add(filterChip(fmt.Sprintf("%s: %s", lang.L("category"), selectedCategory), onClearCategory))
	}

	if selectedProject != "" && selectedProject != defaultProject {
		hasFilters = true
		badgeContainer.Add(filterChip(fmt.Sprintf("%s: %s", lang.L("project"), selectedProject), onClearProject))
	}

	if hasFilters {
		clearAllBtn := widget.NewButtonWithIcon(lang.L("clear_all_filters"), theme.ContentClearIcon(), onClearAll)
		clearAllBtn.Importance = widget.LowImportance
		badgeContainer.Add(clearAllBtn)
	}

	badgeContainer.Refresh()
}

func (r *Reports) MakeUI() fyne.CanvasObject {
	// Load projects
	projects, err := r.storage.LoadProjects()
	if err == nil {
		r.projects = projects
	}

	// Content containers
	dailyContent := container.NewStack()
	weeklyContent := container.NewStack()
	monthlyContent := container.NewStack()
	customContent := container.NewStack()

	// Helper to refresh content
	refreshReport := func(content *fyne.Container, start, end time.Time, groupBy string, selectedCategory string, selectedProject string, searchQuery string, refreshFunc func()) {
		entries, _ := r.storage.LoadEntriesForRange(start, end)
		// Filter by search query
		if searchQuery != "" {
			entries = service.FilterTasks(entries, searchQuery)
		}
		// Filter by category if selected
		if selectedCategory != "" && selectedCategory != lang.L("all_categories") {
			entries = service.FilterByCategory(entries, selectedCategory)
		}
		// Filter by project if selected
		if selectedProject != "" && selectedProject != lang.L("all_projects") {
			if selectedProject == lang.L("no_project") {
				entries = service.FilterByProject(entries, "unassigned")
			} else {
				// Find project ID by name
				for _, p := range r.projects {
					if p.Name == selectedProject {
						entries = service.FilterByProject(entries, p.ID)
						break
					}
				}
			}
		}
		reportUI := r.renderHistory(entries, groupBy, start, end, refreshFunc)
		content.Objects = []fyne.CanvasObject{reportUI}
		content.Refresh()
	}

	// Helper to build project options
	buildProjectOptions := func() []string {
		options := []string{lang.L("all_projects"), lang.L("no_project")}
		for _, p := range r.projects {
			options = append(options, p.Name)
		}
		return options
	}

	createExportButton := func(getRange func() (time.Time, time.Time), getGroupBy func() string) *widget.Button {
		btn := widget.NewButtonWithIcon(lang.L("export_pdf"), theme.DocumentSaveIcon(), func() {
			start, end := getRange()
			groupBy := getGroupBy()

			// Initial filename suggestion
			filename := fmt.Sprintf("report_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))

			path, err := dialog.File().Title(lang.L("export_pdf")).SetStartFile(filename).Filter("PDF files", "pdf").Save()
			if err != nil {
				if err != dialog.ErrCancelled {
					fyneDialog.ShowError(err, safeGetMainWindow())
				}
				return
			}

			if path == "" {
				return
			}

			entries, err := r.storage.LoadEntriesForRange(start, end)
			if err != nil {
				fyneDialog.ShowError(err, safeGetMainWindow())
				return
			}

			if err := GeneratePDF(path, entries, start, end, groupBy); err != nil {
				fyneDialog.ShowError(err, safeGetMainWindow())
			} else {
				fyneDialog.ShowConfirm(lang.L("success"), lang.L("pdf_saved")+"\n"+lang.L("open_file_question"), func(open bool) {
					if open {
						if err := openFile(path); err != nil {
							fyneDialog.ShowError(err, safeGetMainWindow())
						}
					}
				}, safeGetMainWindow())
			}
		})
		btn.Importance = widget.HighImportance
		return btn
	}

	// Helper to create GroupBy selector
	createGroupBySelector := func(onChange func(string)) *widget.Select {
		s := widget.NewSelect([]string{lang.L("none"), lang.L("daily"), lang.L("weekly"), lang.L("project")}, onChange)
		s.SetSelected(lang.L("none"))
		return s
	}

	// Helper to build category options from entries
	buildCategoryOptions := func(entries []models.TimeEntry) []string {
		categories := service.ExtractCategories(entries)
		options := []string{lang.L("all_categories")}

		// Add untagged if any entries lack tags
		hasUntagged := false
		for _, e := range entries {
			if len(e.Tags) == 0 || e.Tags[0] == "" {
				hasUntagged = true
				break
			}
		}
		if hasUntagged {
			options = append(options, lang.L("untagged"))
		}

		// Add categories
		options = append(options, categories...)
		return options
	}

	// Helper to update Category selector with new options
	updateCategorySelector := func(selector *widget.Select, entries []models.TimeEntry, onChange func(string)) {
		options := buildCategoryOptions(entries)
		selector.Options = options
		// Reset to the default without firing onChange. Fyne's SetSelected always
		// invokes OnChanged, which would clobber (and persist) the caller's
		// saved/selected category before it has a chance to be restored.
		selector.OnChanged = nil
		selector.SetSelected(lang.L("all_categories"))
		selector.OnChanged = onChange
		selector.Refresh()
	}

	// Daily Tab
	var selectedDay = time.Now()
	dailyLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var dailySelectedCategory = lang.L("all_categories")
	var dailySelectedProject = lang.L("all_projects")
	dailyFilterState := r.getFilterStateManager("daily")

	// Initialize selector with default option BEFORE container is built
	dailyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	dailyCategorySelector.SetSelected(lang.L("all_categories"))
	dailyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
	r.projectSelectors = append(r.projectSelectors, dailyProjectSelector)
	dailyProjectSelector.SetSelected(lang.L("all_projects"))
	dailySearchEntry := widget.NewEntry()
	dailySearchEntry.PlaceHolder = lang.L("search_tasks")

	// Restore saved filter state
	savedDailyState := dailyFilterState.GetState()
	if savedDailyState.SearchQuery != "" {
		dailySearchEntry.SetText(savedDailyState.SearchQuery)
	}
	if savedDailyState.SelectedCategory != "" {
		dailySelectedCategory = savedDailyState.SelectedCategory
	}
	if savedDailyState.SelectedProject != "" {
		dailySelectedProject = savedDailyState.SelectedProject
	}

	// Filter badges container for daily tab
	dailyBadgeContainer := container.NewHBox()

	var updateDaily func()
	updateDaily = func() {
		dailyLabel.SetText(lang.L("report_for") + selectedDay.Format("Mon, 02 Jan 2006"))
		entries, _ := r.storage.LoadEntriesForRange(selectedDay, selectedDay)
		updateCategorySelector(dailyCategorySelector, entries, func(s string) {
			dailySelectedCategory = s
			dailyFilterState.SetSelectedCategory(s)
			refreshReport(dailyContent, selectedDay, selectedDay, service.GroupByNone, dailySelectedCategory, dailySelectedProject, dailySearchEntry.Text, updateDaily)
			r.updateFilterBadgesWithProject(dailyBadgeContainer, dailySearchEntry.Text, dailySelectedCategory, dailySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
				func() { dailySearchEntry.SetText(""); dailyFilterState.SetSearchQuery(""); updateDaily() },
				func() {
					dailyCategorySelector.SetSelected(lang.L("all_categories"))
					dailySelectedCategory = lang.L("all_categories")
					dailyFilterState.SetSelectedCategory(lang.L("all_categories"))
					updateDaily()
				},
				func() {
					dailyProjectSelector.SetSelected(lang.L("all_projects"))
					dailySelectedProject = lang.L("all_projects")
					dailyFilterState.SetSelectedProject(lang.L("all_projects"))
					updateDaily()
				},
				func() {
					dailySearchEntry.SetText("")
					dailyCategorySelector.SetSelected(lang.L("all_categories"))
					dailyProjectSelector.SetSelected(lang.L("all_projects"))
					dailySelectedCategory = lang.L("all_categories")
					dailySelectedProject = lang.L("all_projects")
					dailyFilterState.ClearFilters(lang.L("all_categories"))
					updateDaily()
				},
			)
		})
		// Set saved category after options are populated
		if dailySelectedCategory != lang.L("all_categories") {
			dailyCategorySelector.SetSelected(dailySelectedCategory)
		}
		if dailySelectedProject != lang.L("all_projects") && dailyProjectSelector.Selected != dailySelectedProject {
			dailyProjectSelector.SetSelected(dailySelectedProject)
		}
		refreshReport(dailyContent, selectedDay, selectedDay, service.GroupByNone, dailySelectedCategory, dailySelectedProject, dailySearchEntry.Text, updateDaily)
		r.updateFilterBadgesWithProject(dailyBadgeContainer, dailySearchEntry.Text, dailySelectedCategory, dailySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
			func() { dailySearchEntry.SetText(""); dailyFilterState.SetSearchQuery(""); updateDaily() },
			func() {
				dailyCategorySelector.SetSelected(lang.L("all_categories"))
				dailySelectedCategory = lang.L("all_categories")
				dailyFilterState.SetSelectedCategory(lang.L("all_categories"))
				updateDaily()
			},
			func() {
				dailyProjectSelector.SetSelected(lang.L("all_projects"))
				dailySelectedProject = lang.L("all_projects")
				dailyFilterState.SetSelectedProject(lang.L("all_projects"))
				updateDaily()
			},
			func() {
				dailySearchEntry.SetText("")
				dailyCategorySelector.SetSelected(lang.L("all_categories"))
				dailyProjectSelector.SetSelected(lang.L("all_projects"))
				dailySelectedCategory = lang.L("all_categories")
				dailySelectedProject = lang.L("all_projects")
				dailyFilterState.ClearFilters(lang.L("all_categories"))
				updateDaily()
			},
		)
	}
	dailySearchEntry.OnChanged = func(s string) {
		dailyFilterState.SetSearchQuery(s)
		updateDaily()
	}
	dailyProjectSelector.OnChanged = func(s string) {
		dailySelectedProject = s
		dailyFilterState.SetSelectedProject(s)
		updateDaily()
	}

	// Navigation controls for daily tab
	dailyNavControls := []fyne.CanvasObject{
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			selectedDay = selectedDay.AddDate(0, 0, -1)
			updateDaily()
		}),
		widget.NewButton(lang.L("today"), func() {
			selectedDay = time.Now()
			updateDaily()
		}),
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
			selectedDay = selectedDay.AddDate(0, 0, 1)
			updateDaily()
		}),
		dailyLabel,
		layout.NewSpacer(),
		createExportButton(func() (time.Time, time.Time) {
			return selectedDay, selectedDay
		}, func() string {
			return service.GroupByNone
		}),
	}

	// Filter controls for daily tab
	dailyFilterControls := []fyne.CanvasObject{
		dailySearchEntry,
		labeledControl(lang.L("filter_by_category"), dailyCategorySelector),
		labeledControl(lang.L("filter_by_project"), dailyProjectSelector),
	}

	// Create responsive toolbar for daily tab
	dailyToolbarContainer := container.NewVBox()
	var rebuildDailyToolbar func()
	rebuildDailyToolbar = func() {
		window := safeGetMainWindow()
		if window == nil {
			return
		}
		canvas := window.Canvas()
		toolbar := r.createResponsiveToolbar(canvas, dailyNavControls, dailyFilterControls, dailyBadgeContainer, nil, dailyFilterState)
		dailyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		dailyToolbarContainer.Refresh()
	}

	dailyTab := container.NewBorder(
		dailyToolbarContainer,
		nil, nil, nil,
		dailyContent,
	)

	// Build the toolbar now when the window exists (it sizes the layout);
	// otherwise shortly after, once the window is ready.
	if safeGetMainWindow() != nil {
		rebuildDailyToolbar()
	} else {
		go func() {
			time.Sleep(100 * time.Millisecond)
			fyne.Do(rebuildDailyToolbar)
		}()
	}

	// Weekly Tab
	getWeekStart := func(t time.Time) time.Time {
		offset := int(t.Weekday())
		if offset == 0 {
			offset = 7
		}
		return t.AddDate(0, 0, -offset+1)
	}
	var selectedWeekStart = getWeekStart(time.Now())
	weeklyLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	weeklyGroupBy := service.GroupByNone
	var weeklySelectedCategory = lang.L("all_categories")
	var weeklySelectedProject = lang.L("all_projects")
	weeklyFilterState := r.getFilterStateManager("weekly")

	// Initialize selector with default option BEFORE container is built
	weeklyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	weeklyCategorySelector.SetSelected(lang.L("all_categories"))
	weeklyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
	r.projectSelectors = append(r.projectSelectors, weeklyProjectSelector)
	weeklyProjectSelector.SetSelected(lang.L("all_projects"))
	weeklySearchEntry := widget.NewEntry()
	weeklySearchEntry.PlaceHolder = lang.L("search_tasks")

	// Restore saved filter state
	savedWeeklyState := weeklyFilterState.GetState()
	if savedWeeklyState.SearchQuery != "" {
		weeklySearchEntry.SetText(savedWeeklyState.SearchQuery)
	}
	if savedWeeklyState.SelectedCategory != "" {
		weeklySelectedCategory = savedWeeklyState.SelectedCategory
	}
	if savedWeeklyState.SelectedProject != "" {
		weeklySelectedProject = savedWeeklyState.SelectedProject
	}
	if savedWeeklyState.GroupBy != "" {
		weeklyGroupBy = savedWeeklyState.GroupBy
	}

	// Filter badges container for weekly tab
	weeklyBadgeContainer := container.NewHBox()

	var updateWeekly func()
	updateWeekly = func() {
		end := selectedWeekStart.AddDate(0, 0, 6)
		weeklyLabel.SetText(fmt.Sprintf("%s %s - %s", lang.L("week"), selectedWeekStart.Format("Jan 02"), end.Format("Jan 02")))
		entries, _ := r.storage.LoadEntriesForRange(selectedWeekStart, end)
		updateCategorySelector(weeklyCategorySelector, entries, func(s string) {
			weeklySelectedCategory = s
			weeklyFilterState.SetSelectedCategory(s)
			refreshReport(weeklyContent, selectedWeekStart, end, weeklyGroupBy, weeklySelectedCategory, weeklySelectedProject, weeklySearchEntry.Text, updateWeekly)
			r.updateFilterBadgesWithProject(weeklyBadgeContainer, weeklySearchEntry.Text, weeklySelectedCategory, weeklySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
				func() { weeklySearchEntry.SetText(""); weeklyFilterState.SetSearchQuery(""); updateWeekly() },
				func() {
					weeklyCategorySelector.SetSelected(lang.L("all_categories"))
					weeklySelectedCategory = lang.L("all_categories")
					weeklyFilterState.SetSelectedCategory(lang.L("all_categories"))
					updateWeekly()
				},
				func() {
					weeklyProjectSelector.SetSelected(lang.L("all_projects"))
					weeklySelectedProject = lang.L("all_projects")
					weeklyFilterState.SetSelectedProject(lang.L("all_projects"))
					updateWeekly()
				},
				func() {
					weeklySearchEntry.SetText("")
					weeklyCategorySelector.SetSelected(lang.L("all_categories"))
					weeklyProjectSelector.SetSelected(lang.L("all_projects"))
					weeklySelectedCategory = lang.L("all_categories")
					weeklySelectedProject = lang.L("all_projects")
					weeklyFilterState.ClearFilters(lang.L("all_categories"))
					updateWeekly()
				},
			)
		})
		// Set saved filters after options are populated
		if weeklySelectedCategory != lang.L("all_categories") {
			weeklyCategorySelector.SetSelected(weeklySelectedCategory)
		}
		if weeklySelectedProject != lang.L("all_projects") && weeklyProjectSelector.Selected != weeklySelectedProject {
			weeklyProjectSelector.SetSelected(weeklySelectedProject)
		}
		refreshReport(weeklyContent, selectedWeekStart, end, weeklyGroupBy, weeklySelectedCategory, weeklySelectedProject, weeklySearchEntry.Text, updateWeekly)
		r.updateFilterBadgesWithProject(weeklyBadgeContainer, weeklySearchEntry.Text, weeklySelectedCategory, weeklySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
			func() { weeklySearchEntry.SetText(""); weeklyFilterState.SetSearchQuery(""); updateWeekly() },
			func() {
				weeklyCategorySelector.SetSelected(lang.L("all_categories"))
				weeklySelectedCategory = lang.L("all_categories")
				weeklyFilterState.SetSelectedCategory(lang.L("all_categories"))
				updateWeekly()
			},
			func() {
				weeklyProjectSelector.SetSelected(lang.L("all_projects"))
				weeklySelectedProject = lang.L("all_projects")
				weeklyFilterState.SetSelectedProject(lang.L("all_projects"))
				updateWeekly()
			},
			func() {
				weeklySearchEntry.SetText("")
				weeklyCategorySelector.SetSelected(lang.L("all_categories"))
				weeklyProjectSelector.SetSelected(lang.L("all_projects"))
				weeklySelectedCategory = lang.L("all_categories")
				weeklySelectedProject = lang.L("all_projects")
				weeklyFilterState.ClearFilters(lang.L("all_categories"))
				updateWeekly()
			},
		)
	}
	weeklySearchEntry.OnChanged = func(s string) {
		weeklyFilterState.SetSearchQuery(s)
		updateWeekly()
	}
	weeklyProjectSelector.OnChanged = func(s string) {
		weeklySelectedProject = s
		weeklyFilterState.SetSelectedProject(s)
		updateWeekly()
	}

	weeklySelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			weeklyGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			weeklyGroupBy = service.GroupByWeek
		} else if s == lang.L("project") {
			weeklyGroupBy = service.GroupByProject
		} else {
			weeklyGroupBy = service.GroupByNone
		}
		weeklyFilterState.SetGroupBy(weeklyGroupBy)
		end := selectedWeekStart.AddDate(0, 0, 6)
		refreshReport(weeklyContent, selectedWeekStart, end, weeklyGroupBy, weeklySelectedCategory, weeklySelectedProject, weeklySearchEntry.Text, updateWeekly)
	})

	// Restore saved group by
	if savedWeeklyState.GroupBy == service.GroupByDay {
		weeklySelector.SetSelected(lang.L("daily"))
	} else if savedWeeklyState.GroupBy == service.GroupByWeek {
		weeklySelector.SetSelected(lang.L("weekly"))
	} else if savedWeeklyState.GroupBy == service.GroupByProject {
		weeklySelector.SetSelected(lang.L("project"))
	}

	// Navigation controls for weekly tab
	weeklyNavControls := []fyne.CanvasObject{
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			selectedWeekStart = selectedWeekStart.AddDate(0, 0, -7)
			updateWeekly()
		}),
		widget.NewButton(lang.L("this_week"), func() {
			selectedWeekStart = getWeekStart(time.Now())
			updateWeekly()
		}),
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
			selectedWeekStart = selectedWeekStart.AddDate(0, 0, 7)
			updateWeekly()
		}),
		weeklyLabel,
		layout.NewSpacer(),
		createExportButton(func() (time.Time, time.Time) {
			return selectedWeekStart, selectedWeekStart.AddDate(0, 0, 6)
		}, func() string {
			return weeklyGroupBy
		}),
	}

	// Filter controls for weekly tab
	weeklyFilterControls := []fyne.CanvasObject{
		weeklySearchEntry,
		labeledControl(lang.L("group_by"), weeklySelector),
		labeledControl(lang.L("filter_by_category"), weeklyCategorySelector),
		labeledControl(lang.L("filter_by_project"), weeklyProjectSelector),
	}

	// Create responsive toolbar for weekly tab
	weeklyToolbarContainer := container.NewVBox()
	var rebuildWeeklyToolbar func()
	rebuildWeeklyToolbar = func() {
		window := safeGetMainWindow()
		if window == nil {
			return
		}
		canvas := window.Canvas()
		toolbar := r.createResponsiveToolbar(canvas, weeklyNavControls, weeklyFilterControls, weeklyBadgeContainer, nil, weeklyFilterState)
		weeklyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		weeklyToolbarContainer.Refresh()
	}

	weeklyTab := container.NewBorder(
		weeklyToolbarContainer,
		nil, nil, nil,
		weeklyContent,
	)

	// Build the toolbar now when the window exists (it sizes the layout);
	// otherwise shortly after, once the window is ready.
	if safeGetMainWindow() != nil {
		rebuildWeeklyToolbar()
	} else {
		go func() {
			time.Sleep(100 * time.Millisecond)
			fyne.Do(rebuildWeeklyToolbar)
		}()
	}

	// Monthly Tab
	getMonthStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	var selectedMonth = getMonthStart(time.Now())
	monthlyLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	monthlyGroupBy := service.GroupByNone
	var monthlySelectedCategory = lang.L("all_categories")
	var monthlySelectedProject = lang.L("all_projects")
	monthlyFilterState := r.getFilterStateManager("monthly")

	// Initialize selector with default option BEFORE container is built
	monthlyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	monthlyCategorySelector.SetSelected(lang.L("all_categories"))
	monthlyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
	r.projectSelectors = append(r.projectSelectors, monthlyProjectSelector)
	monthlyProjectSelector.SetSelected(lang.L("all_projects"))
	monthlySearchEntry := widget.NewEntry()
	monthlySearchEntry.PlaceHolder = lang.L("search_tasks")

	// Restore saved filter state
	savedMonthlyState := monthlyFilterState.GetState()
	if savedMonthlyState.SearchQuery != "" {
		monthlySearchEntry.SetText(savedMonthlyState.SearchQuery)
	}
	if savedMonthlyState.SelectedCategory != "" {
		monthlySelectedCategory = savedMonthlyState.SelectedCategory
	}
	if savedMonthlyState.SelectedProject != "" {
		monthlySelectedProject = savedMonthlyState.SelectedProject
	}
	if savedMonthlyState.GroupBy != "" {
		monthlyGroupBy = savedMonthlyState.GroupBy
	}

	// Filter badges container for monthly tab
	monthlyBadgeContainer := container.NewHBox()

	var updateMonthly func()
	updateMonthly = func() {
		end := selectedMonth.AddDate(0, 1, -1)
		monthlyLabel.SetText(lang.L("report_for") + selectedMonth.Format("January 2006"))
		entries, _ := r.storage.LoadEntriesForRange(selectedMonth, end)
		updateCategorySelector(monthlyCategorySelector, entries, func(s string) {
			monthlySelectedCategory = s
			monthlyFilterState.SetSelectedCategory(s)
			refreshReport(monthlyContent, selectedMonth, end, monthlyGroupBy, monthlySelectedCategory, monthlySelectedProject, monthlySearchEntry.Text, updateMonthly)
			r.updateFilterBadgesWithProject(monthlyBadgeContainer, monthlySearchEntry.Text, monthlySelectedCategory, monthlySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
				func() { monthlySearchEntry.SetText(""); monthlyFilterState.SetSearchQuery(""); updateMonthly() },
				func() {
					monthlyCategorySelector.SetSelected(lang.L("all_categories"))
					monthlySelectedCategory = lang.L("all_categories")
					monthlyFilterState.SetSelectedCategory(lang.L("all_categories"))
					updateMonthly()
				},
				func() {
					monthlyProjectSelector.SetSelected(lang.L("all_projects"))
					monthlySelectedProject = lang.L("all_projects")
					monthlyFilterState.SetSelectedProject(lang.L("all_projects"))
					updateMonthly()
				},
				func() {
					monthlySearchEntry.SetText("")
					monthlyCategorySelector.SetSelected(lang.L("all_categories"))
					monthlyProjectSelector.SetSelected(lang.L("all_projects"))
					monthlySelectedCategory = lang.L("all_categories")
					monthlySelectedProject = lang.L("all_projects")
					monthlyFilterState.ClearFilters(lang.L("all_categories"))
					updateMonthly()
				},
			)
		})
		// Set saved filters after options are populated
		if monthlySelectedCategory != lang.L("all_categories") {
			monthlyCategorySelector.SetSelected(monthlySelectedCategory)
		}
		if monthlySelectedProject != lang.L("all_projects") && monthlyProjectSelector.Selected != monthlySelectedProject {
			monthlyProjectSelector.SetSelected(monthlySelectedProject)
		}
		refreshReport(monthlyContent, selectedMonth, end, monthlyGroupBy, monthlySelectedCategory, monthlySelectedProject, monthlySearchEntry.Text, updateMonthly)
		r.updateFilterBadgesWithProject(monthlyBadgeContainer, monthlySearchEntry.Text, monthlySelectedCategory, monthlySelectedProject, lang.L("all_categories"), lang.L("all_projects"),
			func() { monthlySearchEntry.SetText(""); monthlyFilterState.SetSearchQuery(""); updateMonthly() },
			func() {
				monthlyCategorySelector.SetSelected(lang.L("all_categories"))
				monthlySelectedCategory = lang.L("all_categories")
				monthlyFilterState.SetSelectedCategory(lang.L("all_categories"))
				updateMonthly()
			},
			func() {
				monthlyProjectSelector.SetSelected(lang.L("all_projects"))
				monthlySelectedProject = lang.L("all_projects")
				monthlyFilterState.SetSelectedProject(lang.L("all_projects"))
				updateMonthly()
			},
			func() {
				monthlySearchEntry.SetText("")
				monthlyCategorySelector.SetSelected(lang.L("all_categories"))
				monthlyProjectSelector.SetSelected(lang.L("all_projects"))
				monthlySelectedCategory = lang.L("all_categories")
				monthlySelectedProject = lang.L("all_projects")
				monthlyFilterState.ClearFilters(lang.L("all_categories"))
				updateMonthly()
			},
		)
	}
	monthlySearchEntry.OnChanged = func(s string) {
		monthlyFilterState.SetSearchQuery(s)
		updateMonthly()
	}
	monthlyProjectSelector.OnChanged = func(s string) {
		monthlySelectedProject = s
		monthlyFilterState.SetSelectedProject(s)
		updateMonthly()
	}

	monthlySelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			monthlyGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			monthlyGroupBy = service.GroupByWeekOfMonth
		} else if s == lang.L("project") {
			monthlyGroupBy = service.GroupByProject
		} else {
			monthlyGroupBy = service.GroupByNone
		}
		monthlyFilterState.SetGroupBy(monthlyGroupBy)
		end := selectedMonth.AddDate(0, 1, -1)
		refreshReport(monthlyContent, selectedMonth, end, monthlyGroupBy, monthlySelectedCategory, monthlySelectedProject, monthlySearchEntry.Text, updateMonthly)
	})

	// Restore saved group by
	if savedMonthlyState.GroupBy == service.GroupByDay {
		monthlySelector.SetSelected(lang.L("daily"))
	} else if savedMonthlyState.GroupBy == service.GroupByWeekOfMonth {
		monthlySelector.SetSelected(lang.L("weekly"))
	} else if savedMonthlyState.GroupBy == service.GroupByProject {
		monthlySelector.SetSelected(lang.L("project"))
	}

	// Navigation controls for monthly tab
	monthlyNavControls := []fyne.CanvasObject{
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			selectedMonth = selectedMonth.AddDate(0, -1, 0)
			updateMonthly()
		}),
		widget.NewButton(lang.L("this_month"), func() {
			selectedMonth = getMonthStart(time.Now())
			updateMonthly()
		}),
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
			selectedMonth = selectedMonth.AddDate(0, 1, 0)
			updateMonthly()
		}),
		monthlyLabel,
		layout.NewSpacer(),
		createExportButton(func() (time.Time, time.Time) {
			return selectedMonth, selectedMonth.AddDate(0, 1, -1)
		}, func() string {
			return monthlyGroupBy
		}),
	}

	// Filter controls for monthly tab
	monthlyFilterControls := []fyne.CanvasObject{
		monthlySearchEntry,
		labeledControl(lang.L("group_by"), monthlySelector),
		labeledControl(lang.L("filter_by_category"), monthlyCategorySelector),
		labeledControl(lang.L("filter_by_project"), monthlyProjectSelector),
	}

	// Create responsive toolbar for monthly tab
	monthlyToolbarContainer := container.NewVBox()
	var rebuildMonthlyToolbar func()
	rebuildMonthlyToolbar = func() {
		window := safeGetMainWindow()
		if window == nil {
			return
		}
		canvas := window.Canvas()
		toolbar := r.createResponsiveToolbar(canvas, monthlyNavControls, monthlyFilterControls, monthlyBadgeContainer, nil, monthlyFilterState)
		monthlyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		monthlyToolbarContainer.Refresh()
	}

	monthlyTab := container.NewBorder(
		monthlyToolbarContainer,
		nil, nil, nil,
		monthlyContent,
	)

	// Build the toolbar now when the window exists (it sizes the layout);
	// otherwise shortly after, once the window is ready.
	if safeGetMainWindow() != nil {
		rebuildMonthlyToolbar()
	} else {
		go func() {
			time.Sleep(100 * time.Millisecond)
			fyne.Do(rebuildMonthlyToolbar)
		}()
	}

	// Custom Range Tab
	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()
	customGroupBy := service.GroupByNone
	var customSelectedCategory = lang.L("all_categories")
	var customSelectedProject = lang.L("all_projects")
	customFilterState := r.getFilterStateManager("custom")

	// Initialize selector with default option BEFORE container is built
	customCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	customCategorySelector.SetSelected(lang.L("all_categories"))
	customProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
	r.projectSelectors = append(r.projectSelectors, customProjectSelector)
	customProjectSelector.SetSelected(lang.L("all_projects"))
	customSearchEntry := widget.NewEntry()
	customSearchEntry.PlaceHolder = lang.L("search_tasks")

	// Restore saved filter state
	savedCustomState := customFilterState.GetState()
	if savedCustomState.SearchQuery != "" {
		customSearchEntry.SetText(savedCustomState.SearchQuery)
	}
	if savedCustomState.SelectedCategory != "" {
		customSelectedCategory = savedCustomState.SelectedCategory
	}
	if savedCustomState.SelectedProject != "" {
		customSelectedProject = savedCustomState.SelectedProject
	}
	if savedCustomState.GroupBy != "" {
		customGroupBy = savedCustomState.GroupBy
	}

	// Filter badges container for custom tab
	customBadgeContainer := container.NewHBox()

	var startBtn, endBtn *widget.Button

	var updateCustom func()
	updateCustom = func() {
		startBtn.SetText(startDate.Format("2006-01-02"))
		endBtn.SetText(endDate.Format("2006-01-02"))
		entries, _ := r.storage.LoadEntriesForRange(startDate, endDate)
		updateCategorySelector(customCategorySelector, entries, func(s string) {
			customSelectedCategory = s
			customFilterState.SetSelectedCategory(s)
			refreshReport(customContent, startDate, endDate, customGroupBy, customSelectedCategory, customSelectedProject, customSearchEntry.Text, updateCustom)
			r.updateFilterBadgesWithProject(customBadgeContainer, customSearchEntry.Text, customSelectedCategory, customSelectedProject, lang.L("all_categories"), lang.L("all_projects"),
				func() { customSearchEntry.SetText(""); customFilterState.SetSearchQuery(""); updateCustom() },
				func() {
					customCategorySelector.SetSelected(lang.L("all_categories"))
					customSelectedCategory = lang.L("all_categories")
					customFilterState.SetSelectedCategory(lang.L("all_categories"))
					updateCustom()
				},
				func() {
					customProjectSelector.SetSelected(lang.L("all_projects"))
					customSelectedProject = lang.L("all_projects")
					customFilterState.SetSelectedProject(lang.L("all_projects"))
					updateCustom()
				},
				func() {
					customSearchEntry.SetText("")
					customCategorySelector.SetSelected(lang.L("all_categories"))
					customProjectSelector.SetSelected(lang.L("all_projects"))
					customSelectedCategory = lang.L("all_categories")
					customSelectedProject = lang.L("all_projects")
					customFilterState.ClearFilters(lang.L("all_categories"))
					updateCustom()
				},
			)
		})
		// Set saved filters after options are populated
		if customSelectedCategory != lang.L("all_categories") {
			customCategorySelector.SetSelected(customSelectedCategory)
		}
		if customSelectedProject != lang.L("all_projects") && customProjectSelector.Selected != customSelectedProject {
			customProjectSelector.SetSelected(customSelectedProject)
		}
		refreshReport(customContent, startDate, endDate, customGroupBy, customSelectedCategory, customSelectedProject, customSearchEntry.Text, updateCustom)
		r.updateFilterBadgesWithProject(customBadgeContainer, customSearchEntry.Text, customSelectedCategory, customSelectedProject, lang.L("all_categories"), lang.L("all_projects"),
			func() { customSearchEntry.SetText(""); customFilterState.SetSearchQuery(""); updateCustom() },
			func() {
				customCategorySelector.SetSelected(lang.L("all_categories"))
				customSelectedCategory = lang.L("all_categories")
				customFilterState.SetSelectedCategory(lang.L("all_categories"))
				updateCustom()
			},
			func() {
				customProjectSelector.SetSelected(lang.L("all_projects"))
				customSelectedProject = lang.L("all_projects")
				customFilterState.SetSelectedProject(lang.L("all_projects"))
				updateCustom()
			},
			func() {
				customSearchEntry.SetText("")
				customCategorySelector.SetSelected(lang.L("all_categories"))
				customProjectSelector.SetSelected(lang.L("all_projects"))
				customSelectedCategory = lang.L("all_categories")
				customSelectedProject = lang.L("all_projects")
				customFilterState.ClearFilters(lang.L("all_categories"))
				updateCustom()
			},
		)
	}
	customSearchEntry.OnChanged = func(s string) {
		customFilterState.SetSearchQuery(s)
		updateCustom()
	}
	customProjectSelector.OnChanged = func(s string) {
		customSelectedProject = s
		customFilterState.SetSelectedProject(s)
		updateCustom()
	}

	pickDate := func(current time.Time, onSelect func(time.Time)) {
		var d fyneDialog.Dialog
		cal := widget.NewCalendar(current, func(t time.Time) {
			onSelect(t)
			if d != nil {
				d.Hide()
			}
		})

		// We need to find the parent window
		wins := fyne.CurrentApp().Driver().AllWindows()
		if len(wins) > 0 {
			d = fyneDialog.NewCustom(lang.L("select_date"), lang.L("cancel"), container.NewPadded(cal), wins[0])
			d.Resize(fyne.NewSize(300, 300))
			d.Show()
		}
	}

	startBtn = widget.NewButton(startDate.Format("2006-01-02"), func() {
		pickDate(startDate, func(t time.Time) {
			startDate = t
			updateCustom()
		})
	})

	endBtn = widget.NewButton(endDate.Format("2006-01-02"), func() {
		pickDate(endDate, func(t time.Time) {
			endDate = t
			updateCustom()
		})
	})

	customSelector := createGroupBySelector(func(s string) {
		if s == lang.L("daily") {
			customGroupBy = service.GroupByDay
		} else if s == lang.L("weekly") {
			customGroupBy = service.GroupByWeek
		} else if s == lang.L("project") {
			customGroupBy = service.GroupByProject
		} else {
			customGroupBy = service.GroupByNone
		}
		customFilterState.SetGroupBy(customGroupBy)
		refreshReport(customContent, startDate, endDate, customGroupBy, customSelectedCategory, customSelectedProject, customSearchEntry.Text, updateCustom)
	})

	// Restore saved group by
	if savedCustomState.GroupBy == service.GroupByDay {
		customSelector.SetSelected(lang.L("daily"))
	} else if savedCustomState.GroupBy == service.GroupByWeek {
		customSelector.SetSelected(lang.L("weekly"))
	} else if savedCustomState.GroupBy == service.GroupByProject {
		customSelector.SetSelected(lang.L("project"))
	}

	// Quick date range buttons
	lastWeekBtn := widget.NewButton(lang.L("last_week"), func() {
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -7)
		updateCustom()
	})

	lastMonthBtn := widget.NewButton(lang.L("last_month"), func() {
		endDate = time.Now()
		startDate = endDate.AddDate(0, -1, 0)
		updateCustom()
	})

	last3MonthsBtn := widget.NewButton(lang.L("last_3_months"), func() {
		endDate = time.Now()
		startDate = endDate.AddDate(0, -3, 0)
		updateCustom()
	})

	allTimeBtn := widget.NewButton(lang.L("all_time"), func() {
		endDate = time.Now()
		startDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		updateCustom()
	})

	// Navigation controls for custom tab (date range selection)
	customNavControls := []fyne.CanvasObject{
		widget.NewLabel(lang.L("from")), startBtn,
		widget.NewLabel(lang.L("to")), endBtn,
		lastWeekBtn, lastMonthBtn, last3MonthsBtn, allTimeBtn,
		layout.NewSpacer(),
		createExportButton(func() (time.Time, time.Time) {
			return startDate, endDate
		}, func() string {
			return customGroupBy
		}),
	}

	// Filter controls for custom tab
	customFilterControls := []fyne.CanvasObject{
		customSearchEntry,
		labeledControl(lang.L("group_by"), customSelector),
		labeledControl(lang.L("filter_by_category"), customCategorySelector),
		labeledControl(lang.L("filter_by_project"), customProjectSelector),
		widget.NewButtonWithIcon(lang.L("refresh"), theme.ViewRefreshIcon(), func() {
			updateCustom()
		}),
	}

	// Create responsive toolbar for custom tab
	customToolbarContainer := container.NewVBox()
	var rebuildCustomToolbar func()
	rebuildCustomToolbar = func() {
		window := safeGetMainWindow()
		if window == nil {
			return
		}
		canvas := window.Canvas()
		toolbar := r.createResponsiveToolbar(canvas, customNavControls, customFilterControls, customBadgeContainer, nil, customFilterState)
		customToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		customToolbarContainer.Refresh()
	}

	customTab := container.NewBorder(
		customToolbarContainer,
		nil, nil, nil,
		customContent,
	)

	// Build the toolbar now when the window exists (it sizes the layout);
	// otherwise shortly after, once the window is ready.
	if safeGetMainWindow() != nil {
		rebuildCustomToolbar()
	} else {
		go func() {
			time.Sleep(100 * time.Millisecond)
			fyne.Do(rebuildCustomToolbar)
		}()
	}

	tabs := container.NewAppTabs(
		container.NewTabItem(lang.L("daily"), Inset(12, dailyTab)),
		container.NewTabItem(lang.L("weekly"), Inset(12, weeklyTab)),
		container.NewTabItem(lang.L("monthly"), Inset(12, monthlyTab)),
		container.NewTabItem(lang.L("custom_range"), Inset(12, customTab)),
	)

	tabs.OnSelected = func(item *container.TabItem) {
		refreshTabTheme(item)
		switch item.Text {
		case lang.L("daily"):
			r.refreshCurrent = updateDaily
		case lang.L("weekly"):
			r.refreshCurrent = updateWeekly
		case lang.L("monthly"):
			r.refreshCurrent = updateMonthly
		case lang.L("custom_range"):
			r.refreshCurrent = updateCustom
		}
		r.refreshCurrent()
	}
	// The first tab is already selected, so OnSelected does not fire for it:
	// load it directly.
	tabs.SelectIndex(0)
	r.refreshCurrent = updateDaily
	updateDaily()

	return tabs
}

type ListItem struct {
	IsHeader bool
	IsFooter bool
	Header   string
	Entry    models.TimeEntry
}

func (r *Reports) renderHistory(entries []models.TimeEntry, groupBy string, start, end time.Time, onRefresh func()) fyne.CanvasObject {
	if len(entries) == 0 {
		return emptyState(theme.DocumentIcon(), lang.L("no_entries"), "")
	}

	projects, _ := r.storage.LoadProjects()
	projectByID := make(map[string]models.Project, len(projects))
	for _, p := range projects {
		projectByID[p.ID] = p
	}
	projectName := func(id string) string {
		if id == "unassigned" {
			return lang.L("unassigned")
		}
		if p, ok := projectByID[id]; ok {
			return p.Name
		}
		return id
	}

	// Summary
	sums := make(map[string]time.Duration)
	categoryTotals := service.GetCategoryTotals(entries)
	var total time.Duration
	for _, e := range entries {
		dur := entryDuration(e)
		sums[e.Description] += dur
		total += dur
	}

	_, totalTile := statTile(lang.L("stat_total_time"), utils.FormatDuration(total))
	_, entriesTile := statTile(lang.L("stat_entries"), fmt.Sprintf("%d", len(entries)))
	tiles := []fyne.CanvasObject{totalTile, entriesTile}

	// Billing calculation
	hourlyRate := viper.GetFloat64("hourly_rate")
	if hourlyRate > 0 {
		billingConfig := service.BillingConfig{
			HourlyRate: hourlyRate,
			MaxHours:   viper.GetFloat64("max_hours"),
			ExtraRate:  viper.GetFloat64("extra_rate"),
		}

		// Calculate period days
		periodDays := 0
		if !start.IsZero() && !end.IsZero() {
			periodDays = int(end.Sub(start).Hours()/24) + 1
		}

		billing := service.CalculateBilling(total, billingConfig, periodDays)
		caption := lang.L("stat_total_cost")
		if billing.ExtraCost > 0 {
			caption = fmt.Sprintf("%s  (%s%.2f · %s%.2f)", caption,
				lang.L("standard_cost"), billing.StandardCost, lang.L("extra_cost"), billing.ExtraCost)
		}
		_, costTile := statTile(caption, fmt.Sprintf("%.2f", billing.TotalCost))
		tiles = append(tiles, costTile)
	}

	// Breakdown: by project when grouping by project, otherwise by category
	// when there is more than one.
	var breakdownTitle string
	var breakdown []breakdownItem
	if groupBy == service.GroupByProject {
		projectTotals := service.GetProjectTotals(entries)
		if len(projectTotals) > 0 {
			breakdownTitle = lang.L("by_project")
			for projID, d := range projectTotals {
				item := breakdownItem{label: projectName(projID), dur: d}
				if p, ok := projectByID[projID]; ok {
					item.color = p.ColorHex
				}
				breakdown = append(breakdown, item)
			}
			sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].label < breakdown[j].label })
		}
	} else if len(categoryTotals) > 1 {
		breakdownTitle = lang.L("by_category")
		for cat, d := range categoryTotals {
			breakdown = append(breakdown, breakdownItem{label: cat, dur: d, color: "-"})
		}
		sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].label < breakdown[j].label })
	}

	var byTask []breakdownItem
	for desc, d := range sums {
		byTask = append(byTask, breakdownItem{label: desc, dur: d, color: "-"})
	}
	sort.Slice(byTask, func(i, j int) bool {
		if byTask[i].dur != byTask[j].dur {
			return byTask[i].dur > byTask[j].dur
		}
		return byTask[i].label < byTask[j].label
	})

	summary := container.NewVBox()
	if breakdownTitle != "" {
		summary.Add(sectionTitle(breakdownTitle))
		summary.Add(breakdownList(breakdown, total))
		summary.Add(widget.NewSeparator())
	}
	summary.Add(sectionTitle(lang.L("by_task")))
	summary.Add(breakdownList(byTask, total))

	// Build List Items based on Grouping
	var listItems []ListItem

	if groupBy == service.GroupByNone {
		for i := len(entries) - 1; i >= 0; i-- {
			listItems = append(listItems, ListItem{IsHeader: false, Entry: entries[i]})
		}
	} else if groupBy == service.GroupByProject {
		// Group by project
		projectGroups := service.GroupByProjectID(entries)

		// Get sorted project IDs
		var projectIDs []string
		for projID := range projectGroups {
			projectIDs = append(projectIDs, projID)
		}

		// Sort by project name
		sort.Slice(projectIDs, func(i, j int) bool {
			return projectName(projectIDs[i]) < projectName(projectIDs[j])
		})

		for _, projID := range projectIDs {
			groupEntries := projectGroups[projID]

			// Calculate group total
			var groupTotal time.Duration
			for _, e := range groupEntries {
				groupTotal += entryDuration(e)
			}

			// Add Header
			listItems = append(listItems, ListItem{IsHeader: true, Header: projectName(projID)})

			// Add Entries (reverse order within group)
			for i := len(groupEntries) - 1; i >= 0; i-- {
				listItems = append(listItems, ListItem{IsHeader: false, Entry: groupEntries[i]})
			}

			// Add Footer (Subtotal)
			subtotalTitle := fmt.Sprintf("%s: %s", lang.L("subtotal"), utils.FormatDuration(groupTotal))
			listItems = append(listItems, ListItem{IsFooter: true, Header: subtotalTitle})
		}
	} else {
		// Group entries by time-based keys
		groups := make(map[string][]models.TimeEntry)
		var keys []string

		for _, e := range entries {
			key := service.GetGroupKey(e.StartTime, groupBy)
			if _, exists := groups[key]; !exists {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], e)
		}

		// Sort keys (reverse chronological)
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		for _, key := range keys {
			groupEntries := groups[key]
			// Calculate group total
			var groupTotal time.Duration
			for _, e := range groupEntries {
				groupTotal += entryDuration(e)
			}

			// Add Header
			title := ""
			if len(groupEntries) > 0 {
				title = service.GetGroupTitle(groupEntries[0].StartTime, groupBy)
			}
			headerTitle := title
			listItems = append(listItems, ListItem{IsHeader: true, Header: headerTitle})

			// Add Entries (reverse order within group)
			for i := len(groupEntries) - 1; i >= 0; i-- {
				listItems = append(listItems, ListItem{IsHeader: false, Entry: groupEntries[i]})
			}

			// Add Footer (Subtotal)
			subtotalTitle := fmt.Sprintf("%s: %s", lang.L("subtotal"), utils.FormatDuration(groupTotal))
			listItems = append(listItems, ListItem{IsFooter: true, Header: subtotalTitle})
		}
	}

	listView := widget.NewList(
		func() int { return len(listItems) },
		func() fyne.CanvasObject {
			// Container that holds layouts, hidden/shown via object type
			// Header View
			headerLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			headerLabel.Importance = widget.HighImportance

			// Footer View (Subtotal)
			footerLabel := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true, Monospace: true})
			footerLabel.Importance = widget.LowImportance

			// Task View
			return container.NewStack(headerLabel, footerLabel, newTaskRow())
		},
		func(i int, o fyne.CanvasObject) {
			item := listItems[i]
			containerBox := o.(*fyne.Container)
			headerLabel := containerBox.Objects[0].(*widget.Label)
			footerLabel := containerBox.Objects[1].(*widget.Label)
			row := containerBox.Objects[2].(*taskRow)

			if item.IsHeader {
				headerLabel.Show()
				footerLabel.Hide()
				row.Hide()
				headerLabel.SetText(item.Header)
			} else if item.IsFooter {
				headerLabel.Hide()
				footerLabel.Show()
				row.Hide()
				footerLabel.SetText(item.Header)
			} else {
				headerLabel.Hide()
				footerLabel.Hide()
				row.Show()

				entry := item.Entry

				// Display project name if assigned
				name, hex := "", ""
				if p, ok := projectByID[entry.ProjectID]; ok {
					name, hex = p.Name, p.ColorHex
				}
				row.title.SetText(entry.Description)
				row.swatch.SetHex(hex)
				row.meta.SetText(entryMeta(entry, name, "Mon, 02 Jan 15:04"))

				row.setDuration(utils.FormatDuration(entryDuration(entry)), entry.EndTime.IsZero())
				if entry.EndTime.IsZero() {
					row.edit.Disable()
				} else {
					row.edit.Enable()
				}

				row.edit.OnTapped = func() {
					r.showEditDialog(entry, onRefresh)
				}
				row.del.OnTapped = func() {
					parentWindow := safeGetMainWindow()
					if parentWindow == nil {
						return
					}
					fyneDialog.ShowConfirm(lang.L("confirm_deletion"), lang.L("confirm_delete_task"), func(confirmed bool) {
						if !confirmed {
							return
						}
						r.storage.DeleteEntry(entry)
						onRefresh()
					}, parentWindow)
				}
			}
		},
	)
	// Rows are not selectable; the buttons carry the actions.
	listView.OnSelected = func(id widget.ListItemID) { listView.UnselectAll() }
	// Group headers and subtotals are a single line, so keep them compact.
	lineHeight := widget.NewLabel("X").MinSize().Height
	for i, item := range listItems {
		if item.IsHeader || item.IsFooter {
			listView.SetItemHeight(i, lineHeight)
		}
	}

	entriesPanel := newSurfaceWithInset(listView, 4)
	summaryPanel := NewSurface(container.NewVScroll(summary))

	// Entries and summary side by side on wide windows, stacked on narrow ones.
	var body *container.Split
	if w := safeGetMainWindow(); w != nil && IsCompactScreen(w.Canvas()) {
		body = container.NewVSplit(summaryPanel, entriesPanel)
		body.Offset = 0.35
	} else {
		body = container.NewHSplit(entriesPanel, summaryPanel)
		body.Offset = 0.62
	}

	return container.NewBorder(
		Inset(0, container.NewGridWithColumns(len(tiles), tiles...)),
		nil, nil, nil,
		body,
	)
}

// breakdownItem is one line of a report summary.
type breakdownItem struct {
	label string
	dur   time.Duration
	// color is a project colour; "" shows a hollow marker, "-" no marker.
	color string
}

// breakdownList lists labels with their time and a bar showing their share
// of the total.
func breakdownList(items []breakdownItem, total time.Duration) fyne.CanvasObject {
	box := container.NewVBox()
	for _, it := range items {
		label := widget.NewLabel(it.label)
		label.Truncation = fyne.TextTruncateEllipsis
		dur := widget.NewLabelWithStyle(utils.FormatDuration(it.dur), fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true})

		var left fyne.CanvasObject
		if it.color != "-" {
			left = container.NewCenter(NewSwatch(it.color, 10))
		}
		ratio := float32(0)
		if total > 0 {
			ratio = float32(it.dur) / float32(total)
		}
		line := container.NewBorder(nil, nil, left, dur, label)
		row := container.NewVBox(line, newRatioBar(ratio, it.color))
		row.Layout = &tightVBox{gap: -4}
		box.Add(row)
	}
	return box
}

// ratioBar is a thin horizontal bar filled to a fraction of its width.
type ratioBar struct {
	widget.BaseWidget
	ratio float32
	hex   string
}

func newRatioBar(ratio float32, hex string) *ratioBar {
	b := &ratioBar{ratio: ratio, hex: hex}
	b.ExtendBaseWidget(b)
	return b
}

func (b *ratioBar) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(color.Transparent)
	track.CornerRadius = 2
	fill := canvas.NewRectangle(color.Transparent)
	fill.CornerRadius = 2
	r := &ratioBarRenderer{b: b, track: track, fill: fill}
	r.Refresh()
	return r
}

type ratioBarRenderer struct {
	b     *ratioBar
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

func (r *ratioBarRenderer) Layout(size fyne.Size) {
	pad := theme.Padding()
	w := size.Width - 2*pad
	r.track.Move(fyne.NewPos(pad, 0))
	r.track.Resize(fyne.NewSize(w, size.Height))
	r.fill.Move(fyne.NewPos(pad, 0))
	r.fill.Resize(fyne.NewSize(w*r.b.ratio, size.Height))
}

func (r *ratioBarRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 4) }

func (r *ratioBarRenderer) Refresh() {
	r.track.FillColor = theme.Color(colorNameSubtle)
	if r.b.hex != "" && r.b.hex != "-" {
		r.fill.FillColor = utils.ParseHexColor(r.b.hex)
	} else {
		r.fill.FillColor = theme.Color(theme.ColorNamePrimary)
	}
	r.track.Refresh()
	r.fill.Refresh()
}

func (r *ratioBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill}
}

func (r *ratioBarRenderer) Destroy() {}

func (r *Reports) showEditDialog(entry models.TimeEntry, onSuccess func()) {
	descEntry := widget.NewEntry()
	descEntry.SetText(entry.Description)

	tagsEntry := widget.NewEntry()
	tagsEntry.SetPlaceHolder(lang.L("category_hint"))
	if len(entry.Tags) > 0 {
		tagsEntry.SetText(fmt.Sprintf("%s", entry.Tags[0]))
		if len(entry.Tags) > 1 {
			for i := 1; i < len(entry.Tags); i++ {
				tagsEntry.SetText(fmt.Sprintf("%s, %s", tagsEntry.Text, entry.Tags[i]))
			}
		}
	}

	// Project selection dropdown
	projectOptions := []string{lang.L("none")}
	selectedProjectIndex := 0
	for i, p := range r.projects {
		projectOptions = append(projectOptions, p.Name)
		if p.ID == entry.ProjectID {
			selectedProjectIndex = i + 1 // +1 because "None" is at index 0
		}
	}
	projectSelect := widget.NewSelect(projectOptions, nil)
	if selectedProjectIndex < len(projectOptions) {
		projectSelect.SetSelectedIndex(selectedProjectIndex)
	}

	startEntry := widget.NewEntry()
	startEntry.SetText(entry.StartTime.Format("2006-01-02 15:04:05"))

	endEntry := widget.NewEntry()
	if !entry.EndTime.IsZero() {
		endEntry.SetText(entry.EndTime.Format("2006-01-02 15:04:05"))
	}

	items := []*widget.FormItem{
		widget.NewFormItem(lang.L("task_description"), descEntry),
		widget.NewFormItem(lang.L("project"), projectSelect),
		widget.NewFormItem(lang.L("add_category"), tagsEntry),
		widget.NewFormItem(lang.L("start_time"), startEntry),
		widget.NewFormItem(lang.L("end_time"), endEntry),
	}

	parentWindow := safeGetMainWindow()
	if parentWindow == nil {
		return
	}
	dlg := fyneDialog.NewForm(lang.L("edit_task"), lang.L("save"), lang.L("cancel"), items, func(b bool) {
		if !b {
			return
		}

		newDesc := descEntry.Text
		newStart, err1 := time.Parse("2006-01-02 15:04:05", startEntry.Text)
		newEnd, err2 := time.Parse("2006-01-02 15:04:05", endEntry.Text)

		if err1 != nil || (endEntry.Text != "" && err2 != nil) {
			// Show error? For now just return
			fmt.Println(lang.L("error_parsing_time"))
			return
		}

		// Parse tags from comma-separated input
		var newTags []string
		if tagsEntry.Text != "" {
			for _, tag := range strings.Split(tagsEntry.Text, ",") {
				trimmed := strings.TrimSpace(tag)
				if trimmed != "" {
					newTags = append(newTags, trimmed)
				}
			}
		}

		// Get selected project ID
		newProjectID := ""
		if projectSelect.Selected != "" && projectSelect.Selected != lang.L("none") {
			for _, p := range r.projects {
				if p.Name == projectSelect.Selected {
					newProjectID = p.ID
					break
				}
			}
		}

		// Update entry
		oldEntry := entry
		entry.Description = newDesc
		entry.Tags = newTags
		entry.ProjectID = newProjectID
		entry.StartTime = newStart
		if endEntry.Text != "" {
			entry.EndTime = newEnd
			entry.Duration = int64(newEnd.Sub(newStart).Seconds())
			entry.State = models.TaskStateStopped
		}

		// If start date changed, we need to delete old and save new
		if oldEntry.StartTime.Format("2006-01-02") != entry.StartTime.Format("2006-01-02") {
			r.storage.DeleteEntry(oldEntry)
		}

		r.storage.SaveEntry(entry)
		onSuccess()
	}, parentWindow)
	dlg.Resize(fyne.NewSize(parentWindow.Canvas().Size().Width, dlg.MinSize().Height))
	dlg.Show()
}
