package ui

import (
	"image/color"

	"github.com/highercomve/tasktracker/internal/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// surfaceRadius is the corner radius used for panels on the main window.
const surfaceRadius = 10

// Surface is a rounded, bordered panel. Its colours are read from the active
// theme on every refresh, so switching light/dark mode needs no rebuild.
type Surface struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	inset   float32
}

// NewSurface wraps content in a themed panel.
func NewSurface(content fyne.CanvasObject) *Surface {
	return newSurfaceWithInset(content, 12)
}

func newSurfaceWithInset(content fyne.CanvasObject, inset float32) *Surface {
	s := &Surface{Content: content, inset: inset}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Surface) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = surfaceRadius
	bg.StrokeWidth = 1
	r := &surfaceRenderer{bg: bg, pad: Inset(s.inset, s.Content)}
	r.Refresh()
	return r
}

type surfaceRenderer struct {
	bg  *canvas.Rectangle
	pad *fyne.Container
}

func (r *surfaceRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.pad.Resize(size)
}

func (r *surfaceRenderer) MinSize() fyne.Size { return r.pad.MinSize() }

func (r *surfaceRenderer) Refresh() {
	r.bg.FillColor = theme.Color(colorNameSurface)
	r.bg.StrokeColor = theme.Color(colorNameSurfaceBorder)
	r.bg.Refresh()
	r.pad.Refresh()
}

func (r *surfaceRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.pad}
}

func (r *surfaceRenderer) Destroy() {}

// insetLayout pads its children by a fixed amount on every side.
type insetLayout struct{ inset float32 }

func (l *insetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.inset, l.inset))
		o.Resize(fyne.NewSize(size.Width-2*l.inset, size.Height-2*l.inset))
	}
}

func (l *insetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objs {
		min = min.Max(o.MinSize())
	}
	return min.Add(fyne.NewSize(2*l.inset, 2*l.inset))
}

// Inset pads content by a fixed amount on every side.
func Inset(inset float32, content fyne.CanvasObject) *fyne.Container {
	return container.New(&insetLayout{inset: inset}, content)
}

// Swatch is a small round colour marker, used for projects.
type Swatch struct {
	widget.BaseWidget
	hex  string
	size float32
}

// NewSwatch creates a swatch for a "#RRGGBB" colour. An empty colour shows a
// hollow ring so rows without a project still line up.
func NewSwatch(hex string, size float32) *Swatch {
	s := &Swatch{hex: hex, size: size}
	s.ExtendBaseWidget(s)
	return s
}

// SetHex changes the swatch colour.
func (s *Swatch) SetHex(hex string) {
	if s.hex == hex {
		return
	}
	s.hex = hex
	s.Refresh()
}

func (s *Swatch) MinSize() fyne.Size { return fyne.NewSize(s.size, s.size) }

func (s *Swatch) CreateRenderer() fyne.WidgetRenderer {
	c := canvas.NewCircle(color.Transparent)
	c.StrokeWidth = 1.5
	r := &swatchRenderer{s: s, circle: c}
	r.Refresh()
	return r
}

type swatchRenderer struct {
	s      *Swatch
	circle *canvas.Circle
}

func (r *swatchRenderer) Layout(size fyne.Size) {
	d := fyne.Min(size.Width, size.Height)
	d = fyne.Min(d, r.s.size)
	r.circle.Move(fyne.NewPos((size.Width-d)/2, (size.Height-d)/2))
	r.circle.Resize(fyne.NewSize(d, d))
}

func (r *swatchRenderer) MinSize() fyne.Size { return r.s.MinSize() }

func (r *swatchRenderer) Refresh() {
	if r.s.hex == "" {
		r.circle.FillColor = color.Transparent
		r.circle.StrokeColor = theme.Color(theme.ColorNameDisabled)
	} else {
		c := utils.ParseHexColor(r.s.hex)
		r.circle.FillColor = c
		r.circle.StrokeColor = c
	}
	r.circle.Refresh()
}

func (r *swatchRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.circle} }

func (r *swatchRenderer) Destroy() {}

// sectionTitle is the bold heading shown at the top of a panel.
func sectionTitle(text string) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	return l
}

// captionLabel is small, secondary text.
func captionLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.SizeName = theme.SizeNameCaptionText
	l.Importance = widget.LowImportance
	return l
}

// statTile shows a caption over a large value. The returned label updates the
// value.
func statTile(caption, value string) (*widget.Label, fyne.CanvasObject) {
	v := widget.NewLabelWithStyle(value, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.SizeName = sizeNameStat
	v.Truncation = fyne.TextTruncateEllipsis
	c := captionLabel(caption)
	box := container.NewVBox(c, v)
	box.Layout = &tightVBox{gap: -10}
	return v, newSurfaceWithInset(box, 6)
}

// emptyState is the centred placeholder shown when a list has nothing in it.
func emptyState(icon fyne.Resource, title, detail string) fyne.CanvasObject {
	img := widget.NewIcon(icon)
	iconBox := container.NewCenter(container.NewGridWrap(fyne.NewSize(48, 48), img))
	t := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	objs := []fyne.CanvasObject{iconBox, t}
	if detail != "" {
		d := widget.NewLabel(detail)
		d.Alignment = fyne.TextAlignCenter
		d.Importance = widget.LowImportance
		d.Wrapping = fyne.TextWrapWord
		objs = append(objs, d)
	}
	return container.NewCenter(container.NewVBox(objs...))
}

// filterChip is a rounded label with a clear button, used for active filters.
func filterChip(text string, onClear func()) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.SizeName = theme.SizeNameCaptionText
	clear := widget.NewButtonWithIcon("", theme.CancelIcon(), onClear)
	clear.Importance = widget.LowImportance
	return newSurfaceWithInset(container.NewHBox(l, clear), 0)
}

// minWidth forces a minimum width on content.
func minWidth(width float32, content fyne.CanvasObject) fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(width, 0))
	return container.NewStack(spacer, content)
}

// iconButton is a quiet, icon-only button used for row actions.
func iconButton(icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tapped)
	b.Importance = widget.LowImportance
	return b
}

// adaptiveColumns shows its two children side by side, the first one wider,
// when there is room and stacks them otherwise.
type adaptiveColumns struct {
	minWidth float32
	wide     bool
}

// newAdaptiveColumns lays out left and right as columns once the container is
// at least minWidth wide.
func newAdaptiveColumns(minWidth float32, left, right fyne.CanvasObject) *fyne.Container {
	return container.New(&adaptiveColumns{minWidth: minWidth}, left, right)
}

func (l *adaptiveColumns) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	pad := theme.Padding()
	l.wide = size.Width >= l.minWidth
	if l.wide && len(objs) == 2 {
		left := (size.Width - pad) * 0.58
		objs[0].Move(fyne.NewPos(0, 0))
		objs[0].Resize(fyne.NewSize(left, objs[0].MinSize().Height))
		objs[1].Move(fyne.NewPos(left+pad, 0))
		objs[1].Resize(fyne.NewSize(size.Width-left-pad, objs[1].MinSize().Height))
		return
	}
	y := float32(0)
	for _, o := range objs {
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + pad
	}
}

// MinSize follows the arrangement chosen by the last layout, so a scroll
// container around it only scrolls when the content really overflows.
func (l *adaptiveColumns) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objs {
		m := o.MinSize()
		w = fyne.Max(w, m.Width)
		if l.wide {
			h = fyne.Max(h, m.Height)
			continue
		}
		h += m.Height
		if i > 0 {
			h += theme.Padding()
		}
	}
	return fyne.NewSize(w, h)
}
