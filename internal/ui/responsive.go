package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ScreenSize represents different screen size categories
type ScreenSize int

const (
	// ScreenCompact is for screens < 400px width
	ScreenCompact ScreenSize = iota
	// ScreenSmall is for screens 400-600px width
	ScreenSmall
	// ScreenMedium is for screens 600-800px width
	ScreenMedium
	// ScreenLarge is for screens > 800px width
	ScreenLarge
)

// Breakpoints for responsive layouts (in pixels)
const (
	BreakpointCompact = 400
	BreakpointSmall   = 600
	BreakpointMedium  = 800
)

// GetScreenSize returns the current screen size category based on canvas width
func GetScreenSize(canvas fyne.Canvas) ScreenSize {
	width := canvas.Size().Width
	switch {
	case width < BreakpointCompact:
		return ScreenCompact
	case width < BreakpointSmall:
		return ScreenSmall
	case width < BreakpointMedium:
		return ScreenMedium
	default:
		return ScreenLarge
	}
}

// IsCompactScreen returns true if the screen is compact or small
func IsCompactScreen(canvas fyne.Canvas) bool {
	size := GetScreenSize(canvas)
	return size == ScreenCompact || size == ScreenSmall
}

// CollapsiblePanel creates a collapsible panel with a header and content
type CollapsiblePanel struct {
	widget.BaseWidget
	Title     string
	Content   fyne.CanvasObject
	Expanded  bool
	OnToggle  func(expanded bool)
	header    *fyne.Container
	body      *fyne.Container
	toggleBtn *widget.Button
	container *fyne.Container
}

// NewCollapsiblePanel creates a new collapsible panel
func NewCollapsiblePanel(title string, content fyne.CanvasObject, expanded bool) *CollapsiblePanel {
	panel := &CollapsiblePanel{
		Title:    title,
		Content:  content,
		Expanded: expanded,
	}
	panel.ExtendBaseWidget(panel)
	return panel
}

// CreateRenderer implements fyne.Widget
func (p *CollapsiblePanel) CreateRenderer() fyne.WidgetRenderer {
	p.toggleBtn = widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() {
		p.Toggle()
	})

	titleLabel := widget.NewLabelWithStyle(p.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.header = container.NewBorder(nil, nil, p.toggleBtn, nil, titleLabel)

	p.body = container.NewStack(p.Content)
	if !p.Expanded {
		p.body.Hide()
		p.toggleBtn.SetIcon(theme.MenuDropUpIcon())
	}

	p.container = container.NewVBox(p.header, p.body)

	return widget.NewSimpleRenderer(p.container)
}

// Toggle toggles the expanded state of the panel
func (p *CollapsiblePanel) Toggle() {
	p.Expanded = !p.Expanded
	if p.Expanded {
		p.body.Show()
		p.toggleBtn.SetIcon(theme.MenuDropDownIcon())
	} else {
		p.body.Hide()
		p.toggleBtn.SetIcon(theme.MenuDropUpIcon())
	}
	if p.OnToggle != nil {
		p.OnToggle(p.Expanded)
	}
	p.Refresh()
}

// SetExpanded sets the expanded state
func (p *CollapsiblePanel) SetExpanded(expanded bool) {
	if p.Expanded != expanded {
		p.Toggle()
	}
}

// FilterBadge represents an active filter indicator
type FilterBadge struct {
	widget.BaseWidget
	Label     string
	OnClear   func()
	container *fyne.Container
}

// NewFilterBadge creates a new filter badge
func NewFilterBadge(label string, onClear func()) *FilterBadge {
	badge := &FilterBadge{
		Label:   label,
		OnClear: onClear,
	}
	badge.ExtendBaseWidget(badge)
	return badge
}

// CreateRenderer implements fyne.Widget
func (b *FilterBadge) CreateRenderer() fyne.WidgetRenderer {
	lbl := widget.NewLabel(b.Label)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	clearBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if b.OnClear != nil {
			b.OnClear()
		}
	})
	clearBtn.Importance = widget.LowImportance

	b.container = container.NewHBox(lbl, clearBtn)
	return widget.NewSimpleRenderer(b.container)
}

// SetLabel updates the badge label
func (b *FilterBadge) SetLabel(label string) {
	b.Label = label
	b.Refresh()
}

// FilterSummaryBar shows active filters with clear-all option
type FilterSummaryBar struct {
	widget.BaseWidget
	badges      []*FilterBadge
	clearAllBtn *widget.Button
	OnClearAll  func()
	container   *fyne.Container
}

// NewFilterSummaryBar creates a new filter summary bar
func NewFilterSummaryBar(onClearAll func()) *FilterSummaryBar {
	bar := &FilterSummaryBar{
		badges:     make([]*FilterBadge, 0),
		OnClearAll: onClearAll,
	}
	bar.ExtendBaseWidget(bar)
	return bar
}

// CreateRenderer implements fyne.Widget
func (f *FilterSummaryBar) CreateRenderer() fyne.WidgetRenderer {
	f.clearAllBtn = widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		if f.OnClearAll != nil {
			f.OnClearAll()
		}
	})
	f.clearAllBtn.Importance = widget.LowImportance

	f.container = container.NewHBox()
	f.updateContainer()

	return widget.NewSimpleRenderer(f.container)
}

// updateContainer rebuilds the container with current badges
func (f *FilterSummaryBar) updateContainer() {
	objects := make([]fyne.CanvasObject, 0)
	for _, badge := range f.badges {
		objects = append(objects, badge)
	}
	if len(f.badges) > 0 {
		objects = append(objects, f.clearAllBtn)
	}
	f.container.Objects = objects
	f.container.Refresh()
}

// SetBadges sets the active filter badges
func (f *FilterSummaryBar) SetBadges(badges []*FilterBadge) {
	f.badges = badges
	f.updateContainer()
}

// AddBadge adds a filter badge
func (f *FilterSummaryBar) AddBadge(badge *FilterBadge) {
	f.badges = append(f.badges, badge)
	f.updateContainer()
}

// ClearBadges removes all badges
func (f *FilterSummaryBar) ClearBadges() {
	f.badges = make([]*FilterBadge, 0)
	f.updateContainer()
}

// HasBadges returns true if there are active filter badges
func (f *FilterSummaryBar) HasBadges() bool {
	return len(f.badges) > 0
}

// ResponsiveToolbar creates a toolbar that adapts to screen size
type ResponsiveToolbar struct {
	widget.BaseWidget
	PrimaryItems   []fyne.CanvasObject // Always visible items
	SecondaryItems []fyne.CanvasObject // Hidden in collapsed panel on small screens
	Canvas         fyne.Canvas
	container      *fyne.Container
	panel          *CollapsiblePanel
	expanded       bool
}

// NewResponsiveToolbar creates a new responsive toolbar
func NewResponsiveToolbar(canvas fyne.Canvas, primary, secondary []fyne.CanvasObject) *ResponsiveToolbar {
	toolbar := &ResponsiveToolbar{
		PrimaryItems:   primary,
		SecondaryItems: secondary,
		Canvas:         canvas,
		expanded:       false,
	}
	toolbar.ExtendBaseWidget(toolbar)
	return toolbar
}

// CreateRenderer implements fyne.Widget
func (t *ResponsiveToolbar) CreateRenderer() fyne.WidgetRenderer {
	t.container = container.NewVBox()
	t.rebuildLayout()
	return widget.NewSimpleRenderer(t.container)
}

// rebuildLayout rebuilds the toolbar layout based on screen size
func (t *ResponsiveToolbar) rebuildLayout() {
	if t.container == nil {
		return
	}

	isCompact := IsCompactScreen(t.Canvas)

	if isCompact {
		// Compact layout: primary items + expandable panel for secondary
		primaryBox := container.NewHBox(t.PrimaryItems...)

		if len(t.SecondaryItems) > 0 {
			secondaryBox := container.NewVBox()
			for _, item := range t.SecondaryItems {
				secondaryBox.Add(item)
			}
			t.panel = NewCollapsiblePanel("Filters", secondaryBox, t.expanded)
			t.panel.OnToggle = func(expanded bool) {
				t.expanded = expanded
			}
			t.container.Objects = []fyne.CanvasObject{primaryBox, t.panel}
		} else {
			t.container.Objects = []fyne.CanvasObject{primaryBox}
		}
	} else {
		// Large layout: all items in a horizontal box
		allItems := make([]fyne.CanvasObject, 0)
		allItems = append(allItems, t.PrimaryItems...)
		if len(t.SecondaryItems) > 0 {
			allItems = append(allItems, layout.NewSpacer())
			allItems = append(allItems, t.SecondaryItems...)
		}
		t.container.Objects = []fyne.CanvasObject{container.NewHBox(allItems...)}
	}
	t.container.Refresh()
}

// Refresh rebuilds the layout and refreshes the widget
func (t *ResponsiveToolbar) Refresh() {
	t.rebuildLayout()
	t.BaseWidget.Refresh()
}

// CompactToolbarRow creates a row of controls suitable for compact screens
func CompactToolbarRow(items ...fyne.CanvasObject) *fyne.Container {
	return container.NewHBox(items...)
}

// ResponsiveGrid creates a grid that adapts columns based on screen width
func ResponsiveGrid(canvas fyne.Canvas, items []fyne.CanvasObject) *fyne.Container {
	size := GetScreenSize(canvas)
	var cols int
	switch size {
	case ScreenCompact:
		cols = 1
	case ScreenSmall:
		cols = 2
	case ScreenMedium:
		cols = 3
	default:
		cols = 4
	}
	return container.NewGridWithColumns(cols, items...)
}
