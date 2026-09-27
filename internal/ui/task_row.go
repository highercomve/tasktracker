package ui

import (
	"strings"
	"time"

	"github.com/highercomve/tasktracker/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// taskRow is one time entry in a list: a project swatch, the description with
// a detail line under it, the duration and the row actions.
type taskRow struct {
	widget.BaseWidget

	swatch *Swatch
	title  *widget.Label
	meta   *widget.Label
	dur    *widget.Label
	edit   *widget.Button
	del    *widget.Button

	content fyne.CanvasObject
}

func newTaskRow() *taskRow {
	r := &taskRow{
		swatch: NewSwatch("", 12),
		title:  widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		meta:   captionLabel(""),
		dur:    widget.NewLabelWithStyle("00:00:00", fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true}),
		edit:   iconButton(theme.DocumentCreateIcon(), nil),
		del:    iconButton(theme.DeleteIcon(), nil),
	}
	r.title.Truncation = fyne.TextTruncateEllipsis
	r.meta.Truncation = fyne.TextTruncateEllipsis

	// Pull the two lines together so the row reads as one item.
	text := container.NewVBox(r.title, r.meta)
	text.Layout = &tightVBox{gap: -10}

	r.content = container.NewBorder(nil, nil,
		container.NewCenter(Inset(4, r.swatch)),
		container.NewHBox(container.NewCenter(r.dur), container.NewCenter(r.edit), container.NewCenter(r.del)),
		text,
	)
	r.ExtendBaseWidget(r)
	return r
}

func (r *taskRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

// setDuration shows d, in italics while the entry is still open.
func (r *taskRow) setDuration(text string, live bool) {
	style := fyne.TextStyle{Monospace: true, Italic: live}
	if r.dur.TextStyle != style {
		r.dur.TextStyle = style
		r.dur.Refresh()
	}
	r.dur.SetText(text)
}

// entryMeta builds the detail line of an entry: project, categories and when
// it happened.
func entryMeta(entry models.TimeEntry, projectName, timeFormat string) string {
	var parts []string
	if projectName != "" {
		parts = append(parts, projectName)
	}
	var tags []string
	for _, t := range entry.Tags {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	if len(tags) > 0 {
		parts = append(parts, strings.Join(tags, ", "))
	}
	when := entry.StartTime.Format(timeFormat)
	if !entry.EndTime.IsZero() {
		end := entry.EndTime.Format("15:04")
		if entry.EndTime.Format("2006-01-02") != entry.StartTime.Format("2006-01-02") {
			end = entry.EndTime.Format(timeFormat)
		}
		when += " – " + end
	} else {
		when += " –"
	}
	parts = append(parts, when)
	return strings.Join(parts, "  ·  ")
}

// entryDuration is the tracked time of a finished entry, or the elapsed time
// of one that is still open.
func entryDuration(e models.TimeEntry) time.Duration {
	if e.EndTime.IsZero() {
		return time.Since(e.StartTime)
	}
	return time.Duration(e.Duration) * time.Second
}

// tightVBox stacks objects vertically with a fixed (possibly negative) gap,
// so label padding does not push lines apart.
type tightVBox struct{ gap float32 }

func (l *tightVBox) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	min := l.MinSize(objs)
	y := (size.Height - min.Height) / 2
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + l.gap
	}
}

func (l *tightVBox) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = fyne.Max(w, m.Width)
		h += m.Height
		n++
	}
	if n > 1 {
		h += l.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}
