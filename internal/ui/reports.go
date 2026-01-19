package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Reports struct {
	storage      *store.Storage
	filterStates map[string]*FilterStateManager
	projects     []models.Project
}

func NewReports(s *store.Storage) *Reports {
	return &Reports{
		storage:      s,
		filterStates: make(map[string]*FilterStateManager),
	}
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

	if isCompact {
		// Filter panel content (simple container, no header)
		filterContent := container.NewVBox()
		for _, ctrl := range filterControls {
			filterContent.Add(ctrl)
		}

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
			navRow,
			filterContent,
			filterBadgeContainer,
		)
	}

	// Full layout: All controls in horizontal layout
	fullNavRow := container.NewHBox(navControls...)
	fullNavRow.Add(layout.NewSpacer())
	for _, ctrl := range filterControls {
		fullNavRow.Add(ctrl)
	}

	return container.NewVBox(
		fullNavRow,
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

	if selectedProject != "" && selectedProject != defaultProject {
		hasFilters = true
		projBadge := widget.NewButton(fmt.Sprintf("%s: %s", lang.L("project"), selectedProject), nil)
		projBadge.Importance = widget.MediumImportance
		clearProjBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClearProject)
		clearProjBtn.Importance = widget.LowImportance
		badgeContainer.Add(container.NewHBox(projBadge, clearProjBtn))
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
		reportUI := r.renderHistory(entries, groupBy, refreshFunc)
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
		return widget.NewButtonWithIcon(lang.L("export_pdf"), theme.DocumentSaveIcon(), func() {
			start, end := getRange()
			groupBy := getGroupBy()

			// Initial filename suggestion
			filename := fmt.Sprintf("report_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))

			dlg := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
					return
				}
				if writer == nil {
					return
				}
				defer writer.Close()

				path := writer.URI().Path()
				entries, err := r.storage.LoadEntriesForRange(start, end)
				if err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
					return
				}

				if err := GeneratePDF(path, entries, start, end, groupBy); err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
				} else {
					dialog.ShowInformation(lang.L("success"), lang.L("pdf_saved"), fyne.CurrentApp().Driver().AllWindows()[0])
				}
			}, fyne.CurrentApp().Driver().AllWindows()[0])

			dlg.SetFileName(filename)
			dlg.Show()
		})
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
		selector.OnChanged = onChange
		selector.SetSelected(lang.L("all_categories"))
		selector.Refresh()
	}

	// Daily Tab
	var selectedDay = time.Now()
	dailyLabel := widget.NewLabel("")
	var dailySelectedCategory = lang.L("all_categories")
	var dailySelectedProject = lang.L("all_projects")
	dailyFilterState := r.getFilterStateManager("daily")

	// Initialize selector with default option BEFORE container is built
	dailyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	dailyCategorySelector.SetSelected(lang.L("all_categories"))
	dailyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
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
		if dailySelectedProject != lang.L("all_projects") {
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
	}

	// Filter controls for daily tab
	dailyFilterControls := []fyne.CanvasObject{
		dailySearchEntry,
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_category")),
			dailyCategorySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_project")),
			dailyProjectSelector,
		),
		createExportButton(func() (time.Time, time.Time) {
			return selectedDay, selectedDay
		}, func() string {
			return service.GroupByNone
		}),
	}

	// Create responsive toolbar for daily tab
	dailyToolbarContainer := container.NewVBox()
	var rebuildDailyToolbar func()
	rebuildDailyToolbar = func() {
		canvas := fyne.CurrentApp().Driver().AllWindows()[0].Canvas()
		toolbar := r.createResponsiveToolbar(canvas, dailyNavControls, dailyFilterControls, dailyBadgeContainer, nil, dailyFilterState)
		dailyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		dailyToolbarContainer.Refresh()
	}

	dailyTab := container.NewBorder(
		dailyToolbarContainer,
		nil, nil, nil,
		dailyContent,
	)

	// Listen for window resize to rebuild toolbar
	go func() {
		// Initial build after a short delay to ensure window is ready
		time.Sleep(100 * time.Millisecond)
		rebuildDailyToolbar()
	}()

	// Weekly Tab
	getWeekStart := func(t time.Time) time.Time {
		offset := int(t.Weekday())
		if offset == 0 {
			offset = 7
		}
		return t.AddDate(0, 0, -offset+1)
	}
	var selectedWeekStart = getWeekStart(time.Now())
	weeklyLabel := widget.NewLabel("")
	weeklyGroupBy := service.GroupByNone
	var weeklySelectedCategory = lang.L("all_categories")
	var weeklySelectedProject = lang.L("all_projects")
	weeklyFilterState := r.getFilterStateManager("weekly")

	// Initialize selector with default option BEFORE container is built
	weeklyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	weeklyCategorySelector.SetSelected(lang.L("all_categories"))
	weeklyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
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
		if weeklySelectedProject != lang.L("all_projects") {
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
	}

	// Filter controls for weekly tab
	weeklyFilterControls := []fyne.CanvasObject{
		weeklySearchEntry,
		container.NewHBox(
			widget.NewLabel(lang.L("group_by")),
			weeklySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_category")),
			weeklyCategorySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_project")),
			weeklyProjectSelector,
		),
		createExportButton(func() (time.Time, time.Time) {
			return selectedWeekStart, selectedWeekStart.AddDate(0, 0, 6)
		}, func() string {
			return weeklyGroupBy
		}),
	}

	// Create responsive toolbar for weekly tab
	weeklyToolbarContainer := container.NewVBox()
	var rebuildWeeklyToolbar func()
	rebuildWeeklyToolbar = func() {
		canvas := fyne.CurrentApp().Driver().AllWindows()[0].Canvas()
		toolbar := r.createResponsiveToolbar(canvas, weeklyNavControls, weeklyFilterControls, weeklyBadgeContainer, nil, weeklyFilterState)
		weeklyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		weeklyToolbarContainer.Refresh()
	}

	weeklyTab := container.NewBorder(
		weeklyToolbarContainer,
		nil, nil, nil,
		weeklyContent,
	)

	// Listen for window resize to rebuild toolbar
	go func() {
		time.Sleep(100 * time.Millisecond)
		rebuildWeeklyToolbar()
	}()

	// Monthly Tab
	getMonthStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	var selectedMonth = getMonthStart(time.Now())
	monthlyLabel := widget.NewLabel("")
	monthlyGroupBy := service.GroupByNone
	var monthlySelectedCategory = lang.L("all_categories")
	var monthlySelectedProject = lang.L("all_projects")
	monthlyFilterState := r.getFilterStateManager("monthly")

	// Initialize selector with default option BEFORE container is built
	monthlyCategorySelector := widget.NewSelect([]string{lang.L("all_categories")}, nil)
	monthlyCategorySelector.SetSelected(lang.L("all_categories"))
	monthlyProjectSelector := widget.NewSelect(buildProjectOptions(), nil)
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
		if monthlySelectedProject != lang.L("all_projects") {
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
	}

	// Filter controls for monthly tab
	monthlyFilterControls := []fyne.CanvasObject{
		monthlySearchEntry,
		container.NewHBox(
			widget.NewLabel(lang.L("group_by")),
			monthlySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_category")),
			monthlyCategorySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_project")),
			monthlyProjectSelector,
		),
		createExportButton(func() (time.Time, time.Time) {
			return selectedMonth, selectedMonth.AddDate(0, 1, -1)
		}, func() string {
			return monthlyGroupBy
		}),
	}

	// Create responsive toolbar for monthly tab
	monthlyToolbarContainer := container.NewVBox()
	var rebuildMonthlyToolbar func()
	rebuildMonthlyToolbar = func() {
		canvas := fyne.CurrentApp().Driver().AllWindows()[0].Canvas()
		toolbar := r.createResponsiveToolbar(canvas, monthlyNavControls, monthlyFilterControls, monthlyBadgeContainer, nil, monthlyFilterState)
		monthlyToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		monthlyToolbarContainer.Refresh()
	}

	monthlyTab := container.NewBorder(
		monthlyToolbarContainer,
		nil, nil, nil,
		monthlyContent,
	)

	// Listen for window resize to rebuild toolbar
	go func() {
		time.Sleep(100 * time.Millisecond)
		rebuildMonthlyToolbar()
	}()

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
		if customSelectedProject != lang.L("all_projects") {
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
		var d dialog.Dialog
		cal := widget.NewCalendar(current, func(t time.Time) {
			onSelect(t)
			if d != nil {
				d.Hide()
			}
		})

		// We need to find the parent window
		wins := fyne.CurrentApp().Driver().AllWindows()
		if len(wins) > 0 {
			d = dialog.NewCustom(lang.L("select_date"), lang.L("cancel"), container.NewPadded(cal), wins[0])
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
	}

	// Filter controls for custom tab
	customFilterControls := []fyne.CanvasObject{
		customSearchEntry,
		container.NewHBox(
			widget.NewLabel(lang.L("group_by")),
			customSelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_category")),
			customCategorySelector,
		),
		container.NewHBox(
			widget.NewLabel(lang.L("filter_by_project")),
			customProjectSelector,
		),
		widget.NewButtonWithIcon(lang.L("refresh"), theme.ViewRefreshIcon(), func() {
			updateCustom()
		}),
		createExportButton(func() (time.Time, time.Time) {
			return startDate, endDate
		}, func() string {
			return customGroupBy
		}),
	}

	// Create responsive toolbar for custom tab
	customToolbarContainer := container.NewVBox()
	var rebuildCustomToolbar func()
	rebuildCustomToolbar = func() {
		canvas := fyne.CurrentApp().Driver().AllWindows()[0].Canvas()
		toolbar := r.createResponsiveToolbar(canvas, customNavControls, customFilterControls, customBadgeContainer, nil, customFilterState)
		customToolbarContainer.Objects = []fyne.CanvasObject{toolbar}
		customToolbarContainer.Refresh()
	}

	customTab := container.NewBorder(
		customToolbarContainer,
		nil, nil, nil,
		customContent,
	)

	// Listen for window resize to rebuild toolbar
	go func() {
		time.Sleep(100 * time.Millisecond)
		rebuildCustomToolbar()
	}()

	tabs := container.NewAppTabs(
		container.NewTabItem(lang.L("daily"), dailyTab),
		container.NewTabItem(lang.L("weekly"), weeklyTab),
		container.NewTabItem(lang.L("monthly"), monthlyTab),
		container.NewTabItem(lang.L("custom_range"), customTab),
	)

	tabs.OnSelected = func(item *container.TabItem) {
		switch item.Text {
		case lang.L("daily"):
			updateDaily()
		case lang.L("weekly"):
			updateWeekly()
		case lang.L("monthly"):
			updateMonthly()
		case lang.L("custom_range"):
			updateCustom()
		}
	}
	// Select initial tab to trigger data load
	tabs.SelectIndex(0)

	return tabs
}

type ListItem struct {
	IsHeader bool
	IsFooter bool
	Header   string
	Entry    models.TimeEntry
}

func (r *Reports) renderHistory(entries []models.TimeEntry, groupBy string, onRefresh func()) fyne.CanvasObject {
	if len(entries) == 0 {
		return widget.NewLabel(lang.L("no_entries"))
	}

	// Summary
	sums := make(map[string]time.Duration)
	categoryTotals := service.GetCategoryTotals(entries)
	var total time.Duration
	for _, e := range entries {
		dur := time.Duration(e.Duration) * time.Second
		if e.EndTime.IsZero() {
			dur = time.Since(e.StartTime)
		}
		sums[e.Description] += dur
		total += dur
	}

	summaryText := fmt.Sprintf(lang.L("total_time")+"%s\n", utils.FormatDuration(total))

	// Add project totals if grouping by project
	if groupBy == service.GroupByProject {
		projects, _ := r.storage.LoadProjects()
		projectTotals := service.GetProjectTotals(entries)
		if len(projectTotals) > 0 {
			summaryText += fmt.Sprintf("\n%s:\n", lang.L("by_project"))
			// Sort project IDs for consistent ordering
			var projectIDs []string
			for projID := range projectTotals {
				projectIDs = append(projectIDs, projID)
			}
			sort.Slice(projectIDs, func(i, j int) bool {
				namei := projectIDs[i]
				if namei != "unassigned" {
					for _, p := range projects {
						if p.ID == namei {
							namei = p.Name
							break
						}
					}
				} else {
					namei = lang.L("unassigned")
				}

				namej := projectIDs[j]
				if namej != "unassigned" {
					for _, p := range projects {
						if p.ID == namej {
							namej = p.Name
							break
						}
					}
				} else {
					namej = lang.L("unassigned")
				}
				return namei < namej
			})
			for _, projID := range projectIDs {
				projName := projID
				if projID != "unassigned" {
					for _, p := range projects {
						if p.ID == projID {
							projName = p.Name
							break
						}
					}
				} else {
					projName = lang.L("unassigned")
				}
				summaryText += fmt.Sprintf("  - %s: %s\n", projName, utils.FormatDuration(projectTotals[projID]))
			}
		}
	} else if len(categoryTotals) > 1 {
		// Add category breakdown if there are multiple categories
		summaryText += fmt.Sprintf("\n%s:\n", lang.L("by_category"))
		var categories []string
		for cat := range categoryTotals {
			categories = append(categories, cat)
		}
		sort.Strings(categories)
		for _, cat := range categories {
			summaryText += fmt.Sprintf("  - %s: %s\n", cat, utils.FormatDuration(categoryTotals[cat]))
		}
	}

	summaryText += "\n"
	for desc, dur := range sums {
		summaryText += fmt.Sprintf("- %s: %s\n", desc, utils.FormatDuration(dur))
	}
	summaryLabel := widget.NewLabel(summaryText)

	// Build List Items based on Grouping
	var listItems []ListItem

	if groupBy == service.GroupByNone {
		for i := len(entries) - 1; i >= 0; i-- {
			listItems = append(listItems, ListItem{IsHeader: false, Entry: entries[i]})
		}
	} else if groupBy == service.GroupByProject {
		// Group by project
		projects, _ := r.storage.LoadProjects()
		projectGroups := service.GroupByProjectID(entries)

		// Build project name to ID mapping
		projectNameMap := make(map[string]string)
		for _, p := range projects {
			projectNameMap[p.Name] = p.ID
		}

		// Get sorted project IDs
		var projectIDs []string
		for projID := range projectGroups {
			projectIDs = append(projectIDs, projID)
		}

		// Sort by project name
		sort.Slice(projectIDs, func(i, j int) bool {
			namei := projectIDs[i]
			if namei != "unassigned" {
				for _, p := range projects {
					if p.ID == namei {
						namei = p.Name
						break
					}
				}
			} else {
				namei = lang.L("unassigned")
			}

			namej := projectIDs[j]
			if namej != "unassigned" {
				for _, p := range projects {
					if p.ID == namej {
						namej = p.Name
						break
					}
				}
			} else {
				namej = lang.L("unassigned")
			}
			return namei < namej
		})

		for _, projID := range projectIDs {
			groupEntries := projectGroups[projID]

			// Calculate group total
			var groupTotal time.Duration
			for _, e := range groupEntries {
				dur := time.Duration(e.Duration) * time.Second
				if e.EndTime.IsZero() {
					dur = time.Since(e.StartTime)
				}
				groupTotal += dur
			}

			// Get project name for header
			projName := projID
			if projID != "unassigned" {
				for _, p := range projects {
					if p.ID == projID {
						projName = p.Name
						break
					}
				}
			} else {
				projName = lang.L("unassigned")
			}

			// Add Header
			listItems = append(listItems, ListItem{IsHeader: true, Header: projName})

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
				dur := time.Duration(e.Duration) * time.Second
				if e.EndTime.IsZero() {
					dur = time.Since(e.StartTime)
				}
				groupTotal += dur
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

			// Footer View (Subtotal)
			footerLabel := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true, Italic: true})

			// Task View
			taskContainer := container.NewBorder(nil, nil, nil,
				container.NewHBox(widget.NewLabel("00:00:00"), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)),
				container.NewVBox(
					widget.NewLabelWithStyle(lang.L("title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle(lang.L("date"), fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
					widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}), // Project label
				))

			return container.NewMax(headerLabel, footerLabel, taskContainer)
		},
		func(i int, o fyne.CanvasObject) {
			item := listItems[i]
			containerBox := o.(*fyne.Container)
			headerLabel := containerBox.Objects[0].(*widget.Label)
			footerLabel := containerBox.Objects[1].(*widget.Label)
			taskBox := containerBox.Objects[2].(*fyne.Container)

			if item.IsHeader {
				headerLabel.Show()
				footerLabel.Hide()
				taskBox.Hide()
				headerLabel.SetText(item.Header)
			} else if item.IsFooter {
				headerLabel.Hide()
				footerLabel.Show()
				taskBox.Hide()
				footerLabel.SetText(item.Header)
			} else {
				headerLabel.Hide()
				footerLabel.Hide()
				taskBox.Show()

				entry := item.Entry

				// Extract sub-widgets from taskBox
				rightBox := taskBox.Objects[1].(*fyne.Container)
				durLabel := rightBox.Objects[0].(*widget.Label)
				editBtn := rightBox.Objects[1].(*widget.Button)
				delBtn := rightBox.Objects[2].(*widget.Button)

				infoBox := taskBox.Objects[0].(*fyne.Container)
				titleLabel := infoBox.Objects[0].(*widget.Label)
				dateLabel := infoBox.Objects[1].(*widget.Label)
				projectLabel := infoBox.Objects[2].(*widget.Label)

				titleLabel.SetText(entry.Description)
				dateLabel.SetText(entry.StartTime.Format("Mon, 02 Jan 15:04"))

				// Display project name if assigned
				if entry.ProjectID != "" {
					projectName := ""
					for _, p := range r.projects {
						if p.ID == entry.ProjectID {
							projectName = p.Name
							break
						}
					}
					if projectName != "" {
						projectLabel.SetText(lang.L("project") + ": " + projectName)
						projectLabel.Show()
					} else {
						projectLabel.SetText("")
						projectLabel.Hide()
					}
				} else {
					projectLabel.SetText("")
					projectLabel.Hide()
				}

				dur := time.Duration(entry.Duration) * time.Second
				if entry.EndTime.IsZero() {
					dur = time.Since(entry.StartTime)
					durLabel.TextStyle = fyne.TextStyle{Italic: true}
					editBtn.Disable()
				} else {
					durLabel.TextStyle = fyne.TextStyle{}
					editBtn.Enable()
				}
				durLabel.SetText(utils.FormatDuration(dur))

				editBtn.OnTapped = func() {
					r.showEditDialog(entry, onRefresh)
				}
				delBtn.OnTapped = func() {
					parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
					dialog.ShowConfirm(lang.L("confirm_deletion"), lang.L("confirm_delete_task"), func(confirmed bool) {
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

	return container.NewBorder(
		container.NewVBox(summaryLabel, widget.NewSeparator()),
		nil, nil, nil,
		listView,
	)
}

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

	parentWindow := fyne.CurrentApp().Driver().AllWindows()[0]
	dlg := dialog.NewForm(lang.L("edit_task"), lang.L("save"), lang.L("cancel"), items, func(b bool) {
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
