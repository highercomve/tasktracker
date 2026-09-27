package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/highercomve/tasktracker/internal/models"
	"github.com/highercomve/tasktracker/internal/service"
	"github.com/highercomve/tasktracker/internal/store"
	"github.com/highercomve/tasktracker/internal/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Projects struct {
	storage  *store.Storage
	projects []models.Project
	entries  []models.TimeEntry

	// stats caches the totals shown in each row, keyed by project ID
	stats map[string]service.ProjectStats

	// UI
	projectList *widget.List
	refreshList func()
}

func NewProjects(s *store.Storage) *Projects {
	return &Projects{
		storage: s,
	}
}

// MakeUI creates the project management interface
func (p *Projects) MakeUI() fyne.CanvasObject {
	// Load projects
	projects, err := p.storage.LoadProjects()
	if err == nil {
		p.projects = projects
	}

	// Create button
	createBtn := widget.NewButtonWithIcon(lang.L("new_project"), theme.ContentAddIcon(), nil)
	createBtn.Importance = widget.HighImportance

	// Search entry
	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = lang.L("search_projects")
	searchEntry.ActionItem = widget.NewIcon(theme.SearchIcon())

	// Toolbar
	toolbar := container.NewBorder(nil, nil, sectionTitle(lang.L("projects_tab")), createBtn, searchEntry)

	// Project List
	p.projectList = widget.NewList(
		func() int { return len(p.projects) },
		func() fyne.CanvasObject {
			return createProjectItemContainer()
		},
		func(i int, o fyne.CanvasObject) {
			if i >= len(p.projects) {
				return
			}
			p.updateProjectItem(o, p.projects[i])
		},
	)
	// Rows are not selectable; the buttons carry the actions.
	p.projectList.OnSelected = func(id widget.ListItemID) { p.projectList.UnselectAll() }

	noProjects := emptyState(theme.FolderIcon(), lang.L("no_projects"), lang.L("no_projects_hint"))
	noMatches := emptyState(theme.SearchIcon(), lang.L("no_matching_projects"), "")
	updateEmpty := func() {
		noProjects.Hide()
		noMatches.Hide()
		if len(p.projects) == 0 {
			if searchEntry.Text == "" {
				noProjects.Show()
			} else {
				noMatches.Show()
			}
		}
	}

	p.refreshList = func() {
		// Reload projects
		projects, err := p.storage.LoadProjects()
		if err == nil {
			p.projects = projects
		}

		// Re-filter if search text changed
		if searchEntry.Text != "" {
			filtered := p.filterProjects(p.projects, searchEntry.Text)
			p.projects = filtered
		}

		p.loadStats()
		p.projectList.Refresh()
		updateEmpty()
	}

	// Create button action
	createBtn.OnTapped = func() {
		p.showCreateProjectDialog(func() {
			p.refreshList()
		})
	}

	// Search action
	searchEntry.OnChanged = func(s string) {
		// Reload from storage and apply filter
		projects, err := p.storage.LoadProjects()
		if err == nil {
			if s == "" {
				p.projects = projects
			} else {
				p.projects = p.filterProjects(projects, s)
			}
		}
		p.projectList.Refresh()
		updateEmpty()
	}

	p.loadStats()
	updateEmpty()

	return Inset(12, container.NewBorder(
		Inset(2, toolbar),
		nil, nil, nil,
		newSurfaceWithInset(container.NewStack(p.projectList, noProjects, noMatches), 4),
	))
}

// Refresh reloads the projects and their totals, e.g. after tracking time.
func (p *Projects) Refresh() {
	if p.refreshList != nil {
		p.refreshList()
	}
}

// projectRow is one project in the list.
type projectRow struct {
	widget.BaseWidget

	swatch *Swatch
	name   *widget.Label
	desc   *widget.Label
	stats  *widget.Label
	edit   *widget.Button
	del    *widget.Button

	content fyne.CanvasObject
}

// createProjectItemContainer creates a template for a project list item
func createProjectItemContainer() fyne.CanvasObject {
	r := &projectRow{
		swatch: NewSwatch("", 16),
		name:   widget.NewLabelWithStyle("Project Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		desc:   captionLabel("Description"),
		stats:  widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true}),
		edit:   iconButton(theme.DocumentCreateIcon(), nil),
		del:    iconButton(theme.DeleteIcon(), nil),
	}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.desc.Truncation = fyne.TextTruncateEllipsis

	text := container.NewVBox(r.name, r.desc)
	text.Layout = &tightVBox{gap: -10}

	r.content = container.NewBorder(nil, nil,
		container.NewCenter(Inset(6, r.swatch)),
		container.NewHBox(container.NewCenter(r.stats), container.NewCenter(r.edit), container.NewCenter(r.del)),
		text,
	)
	r.ExtendBaseWidget(r)
	return r
}

func (r *projectRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

// updateProjectItem updates the display of a project list item
func (p *Projects) updateProjectItem(o fyne.CanvasObject, project models.Project) {
	row := o.(*projectRow)

	row.name.SetText(project.Name)
	row.swatch.SetHex(project.ColorHex)

	// Set description
	if project.Description != "" {
		row.desc.SetText(project.Description)
	} else {
		row.desc.SetText(lang.L("no_description"))
	}

	// Calculate and set stats
	stats := p.cachedProjectStats(project.ID)
	row.stats.SetText(fmt.Sprintf(lang.L("project_stats"), stats.EntryCount, utils.FormatDuration(stats.TotalTime)))

	// Edit button
	row.edit.OnTapped = func() {
		p.showEditProjectDialog(project, func() {
			p.refreshList()
		})
	}

	// Delete button
	row.del.OnTapped = func() {
		parentWindow := safeGetMainWindow()
		if parentWindow == nil {
			return
		}

		// Count tasks assigned to this project
		stats := p.calculateProjectStats(project.ID)
		taskCount := stats.EntryCount

		var message string
		if taskCount > 0 {
			message = fmt.Sprintf("Are you sure you want to delete project '%s'?\n\nThis project has %d task(s) assigned to it. The tasks will be unassigned but not deleted.", project.Name, taskCount)
		} else {
			message = fmt.Sprintf("Are you sure you want to delete project '%s'?", project.Name)
		}

		d := dialog.NewConfirm(
			lang.L("confirm_deletion"),
			message,
			func(confirmed bool) {
				if !confirmed {
					return
				}

				// Unassign tasks from this project
				if taskCount > 0 {
					entries, _ := p.storage.LoadEntriesForRange(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now())
					for _, e := range entries {
						if e.ProjectID == project.ID {
							e.ProjectID = ""
							p.storage.SaveEntry(e)
						}
					}
				}

				// Delete the project
				updatedProjects, deleted := service.DeleteProject(p.projects, project.ID)
				if deleted {
					p.projects = updatedProjects
					p.storage.SaveProjects(p.projects)
					p.refreshList()
				}
			},
			parentWindow,
		)
		d.SetConfirmImportance(widget.DangerImportance)
		d.Show()
	}
}

// loadStats computes every project's totals in one pass over the entries, so
// scrolling the list does not reload the whole history for each row.
func (p *Projects) loadStats() {
	entries, err := p.storage.LoadEntriesForRange(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now())
	if err != nil {
		entries = []models.TimeEntry{}
	}
	p.stats = make(map[string]service.ProjectStats)
	for _, s := range service.GetProjectsWithStats(p.projects, entries) {
		p.stats[s.ProjectID] = s
	}
}

// cachedProjectStats returns the totals computed by loadStats, falling back to
// computing them when the project is not known yet.
func (p *Projects) cachedProjectStats(projectID string) service.ProjectStats {
	if s, ok := p.stats[projectID]; ok {
		return s
	}
	return p.calculateProjectStats(projectID)
}

// calculateProjectStats calculates stats for a project
func (p *Projects) calculateProjectStats(projectID string) service.ProjectStats {
	// Load all entries to calculate stats
	entries, err := p.storage.LoadEntriesForRange(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now())
	if err != nil {
		entries = []models.TimeEntry{}
	}

	// Find the project
	var project *models.Project
	for i := range p.projects {
		if p.projects[i].ID == projectID {
			project = &p.projects[i]
			break
		}
	}

	if project == nil {
		return service.ProjectStats{}
	}

	// Get stats
	stats := service.GetProjectsWithStats([]models.Project{*project}, entries)
	if len(stats) > 0 {
		return stats[0]
	}

	return service.ProjectStats{
		ProjectID: projectID,
		Name:      project.Name,
	}
}

// showCreateProjectDialog shows dialog to create a new project
func (p *Projects) showCreateProjectDialog(onSave func()) {
	nameEntry := widget.NewEntry()
	nameEntry.PlaceHolder = "Project name"

	descEntry := widget.NewEntry()
	descEntry.PlaceHolder = "Project description (optional)"

	colorEntry := widget.NewEntry()
	colorEntry.PlaceHolder = "Color hex code (optional, e.g., #FF5733)"

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Color", colorPicker(colorEntry)),
	}

	parentWindow := safeGetMainWindow()
	if parentWindow == nil {
		return
	}
	dlg := dialog.NewForm(lang.L("create_project"), lang.L("create"), lang.L("cancel"), items, func(b bool) {
		if !b {
			return
		}

		name := nameEntry.Text
		if name == "" {
			dialog.ShowError(fmt.Errorf("project name is required"), parentWindow)
			return
		}

		// Check for duplicate names
		for _, proj := range p.projects {
			if proj.Name == name {
				dialog.ShowError(fmt.Errorf("a project with this name already exists"), parentWindow)
				return
			}
		}

		// Create new project
		newProject := service.CreateProject(name, descEntry.Text, colorEntry.Text)

		// Add to list and save
		p.projects = append(p.projects, newProject)
		if err := p.storage.SaveProjects(p.projects); err != nil {
			dialog.ShowError(err, parentWindow)
			return
		}

		onSave()
	}, parentWindow)

	dlg.Resize(fyne.NewSize(parentWindow.Canvas().Size().Width*3/4, dlg.MinSize().Height))
	dlg.Show()
}

// showEditProjectDialog shows dialog to edit an existing project
func (p *Projects) showEditProjectDialog(project models.Project, onSave func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(project.Name)

	descEntry := widget.NewEntry()
	descEntry.SetText(project.Description)

	colorEntry := widget.NewEntry()
	colorEntry.SetText(project.ColorHex)

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Color", colorPicker(colorEntry)),
	}

	parentWindow := safeGetMainWindow()
	if parentWindow == nil {
		return
	}
	dlg := dialog.NewForm(lang.L("edit_project"), lang.L("save"), lang.L("cancel"), items, func(b bool) {
		if !b {
			return
		}

		newName := nameEntry.Text
		if newName == "" {
			dialog.ShowError(fmt.Errorf("project name is required"), parentWindow)
			return
		}

		// Check for duplicate names (excluding self)
		for _, proj := range p.projects {
			if proj.Name == newName && proj.ID != project.ID {
				dialog.ShowError(fmt.Errorf("a project with this name already exists"), parentWindow)
				return
			}
		}

		// Update project
		var updatedProject *models.Project
		for i := range p.projects {
			if p.projects[i].ID == project.ID {
				updatedProject = &p.projects[i]
				break
			}
		}

		if updatedProject == nil {
			return
		}

		service.UpdateProject(updatedProject, newName, descEntry.Text, colorEntry.Text)

		// Save
		if err := p.storage.SaveProjects(p.projects); err != nil {
			dialog.ShowError(err, parentWindow)
			return
		}

		onSave()
	}, parentWindow)

	dlg.Resize(fyne.NewSize(parentWindow.Canvas().Size().Width*3/4, dlg.MinSize().Height))
	dlg.Show()
}

// projectPalette offers a few ready-made project colours.
var projectPalette = []string{
	"#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6",
	"#EC4899", "#14B8A6", "#F97316", "#64748B",
}

// colorPicker pairs the hex entry with a live preview and preset swatches.
func colorPicker(entry *widget.Entry) fyne.CanvasObject {
	preview := NewSwatch(entry.Text, 20)
	onChanged := entry.OnChanged
	entry.OnChanged = func(s string) {
		preview.SetHex(strings.TrimSpace(s))
		if onChanged != nil {
			onChanged(s)
		}
	}

	presets := container.NewHBox()
	for _, hex := range projectPalette {
		hex := hex
		btn := widget.NewButton("", func() { entry.SetText(hex) })
		btn.Importance = widget.LowImportance
		presets.Add(container.NewStack(btn, container.NewCenter(NewSwatch(hex, 18))))
	}

	return container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewCenter(Inset(4, preview)), entry),
		presets,
	)
}

// filterProjects filters projects by name or description
func (p *Projects) filterProjects(projects []models.Project, query string) []models.Project {
	if query == "" {
		return projects
	}

	var filtered []models.Project
	for _, proj := range projects {
		// Simple substring match on name and description
		if matchesQuery(proj.Name, query) || matchesQuery(proj.Description, query) {
			filtered = append(filtered, proj)
		}
	}

	return filtered
}

// matchesQuery checks if text contains query (case-insensitive)
func matchesQuery(text, query string) bool {
	// Simple substring match - could be enhanced with fuzzy search
	for i := 0; i <= len(text)-len(query); i++ {
		match := true
		for j := 0; j < len(query); j++ {
			t := text[i+j]
			q := query[j]
			if t >= 'A' && t <= 'Z' {
				t = t - 'A' + 'a'
			}
			if q >= 'A' && q <= 'Z' {
				q = q - 'A' + 'a'
			}
			if t != q {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ShowProjectDetails shows a detailed view of a project with statistics and task list
func (p *Projects) ShowProjectDetails(projectID string) fyne.CanvasObject {
	// Find project
	var project *models.Project
	for i := range p.projects {
		if p.projects[i].ID == projectID {
			project = &p.projects[i]
			break
		}
	}

	if project == nil {
		return container.NewVBox(
			widget.NewLabel("Project not found"),
		)
	}

	// Load entries
	entries, err := p.storage.LoadEntriesForRange(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now())
	if err != nil {
		entries = []models.TimeEntry{}
	}

	// Filter entries for this project
	projectEntries := service.FilterByProject(entries, projectID)

	// Calculate stats
	stats := p.calculateProjectStats(projectID)

	// Header with project info
	nameLabel := canvas.NewText(project.Name, theme.ForegroundColor())
	nameLabel.TextSize = 24
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	descLabel := widget.NewLabel(project.Description)
	if project.Description == "" {
		descLabel.SetText("No description")
		descLabel.TextStyle = fyne.TextStyle{Italic: true}
	}

	createdLabel := widget.NewLabel(fmt.Sprintf("Created: %s", project.CreatedAt.Format("2006-01-02")))
	updatedLabel := widget.NewLabel(fmt.Sprintf("Updated: %s", project.UpdatedAt.Format("2006-01-02")))

	// Stats
	statsBox := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Total Entries: %d", stats.EntryCount)),
		widget.NewLabel(fmt.Sprintf("Total Time: %s", utils.FormatDuration(stats.TotalTime))),
	)

	// Task list for this project
	taskList := widget.NewList(
		func() int { return len(projectEntries) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				widget.NewLabel("00:00"),
				widget.NewLabel("Task"),
			)
		},
		func(i int, o fyne.CanvasObject) {
			if i >= len(projectEntries) {
				return
			}
			entry := projectEntries[i]
			box := o.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			dur := box.Objects[1].(*widget.Label)

			title.SetText(entry.Description)

			if entry.Duration > 0 {
				dur.SetText(utils.FormatDuration(time.Duration(entry.Duration) * time.Second))
			} else if !entry.EndTime.IsZero() {
				dur.SetText(utils.FormatDuration(entry.EndTime.Sub(entry.StartTime)))
			} else {
				dur.SetText("Running")
				dur.TextStyle = fyne.TextStyle{Italic: true}
			}
		},
	)

	return container.NewBorder(
		container.NewVBox(
			nameLabel,
			descLabel,
			container.NewHBox(createdLabel, layout.NewSpacer(), updatedLabel),
			widget.NewSeparator(),
			statsBox,
			widget.NewSeparator(),
			widget.NewLabel("Tasks:"),
		),
		nil, nil, nil,
		taskList,
	)
}
