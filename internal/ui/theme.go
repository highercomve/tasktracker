package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/spf13/viper"
)

// Theme preferences stored under the "theme" config key.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Palette shared with pvflasher so both apps look like one family.
var (
	colorPrimary = color.NRGBA{R: 0x3B, G: 0x82, B: 0xF6, A: 0xFF}
	colorSuccess = color.NRGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}
	colorWarning = color.NRGBA{R: 0xF5, G: 0x9E, B: 0x0B, A: 0xFF}
	colorError   = color.NRGBA{R: 0xEF, G: 0x44, B: 0x44, A: 0xFF}

	colorBackground    = color.NRGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}
	colorSurface       = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorBorder        = color.NRGBA{R: 0xD1, G: 0xD5, B: 0xDB, A: 0xFF}
	colorText          = color.NRGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}
	colorTextSecondary = color.NRGBA{R: 0x47, G: 0x55, B: 0x69, A: 0xFF}
	colorButton        = color.NRGBA{R: 0xE7, G: 0xEB, B: 0xF0, A: 0xFF}

	colorDarkBackground    = color.NRGBA{R: 0x0F, G: 0x14, B: 0x1C, A: 0xFF}
	colorDarkSurface       = color.NRGBA{R: 0x1E, G: 0x24, B: 0x2E, A: 0xFF}
	colorDarkElevated      = color.NRGBA{R: 0x2D, G: 0x35, B: 0x42, A: 0xFF}
	colorDarkBorder        = color.NRGBA{R: 0x33, G: 0x40, B: 0x4F, A: 0xFF}
	colorDarkText          = color.NRGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}
	colorDarkTextSecondary = color.NRGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}
)

// Custom theme names used by the components in this package.
const (
	colorNameSurface       fyne.ThemeColorName = "tasktracker-surface"
	colorNameSurfaceBorder fyne.ThemeColorName = "tasktracker-surface-border"
	colorNameSubtle        fyne.ThemeColorName = "tasktracker-subtle"

	sizeNameTimer fyne.ThemeSizeName = "tasktracker-timer"
	sizeNameStat  fyne.ThemeSizeName = "tasktracker-stat"
)

// AppTheme is the Task Tracker look: a neutral light or dark surface with a
// single blue accent, so only primary actions stand out.
type AppTheme struct {
	mu   sync.RWMutex
	dark bool
}

var appTheme = &AppTheme{dark: true}

// CurrentTheme returns the theme shared by the whole app.
func CurrentTheme() *AppTheme { return appTheme }

// IsDark reports whether the dark palette is active.
func (t *AppTheme) IsDark() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dark
}

func (t *AppTheme) setDark(dark bool) {
	t.mu.Lock()
	t.dark = dark
	t.mu.Unlock()
}

// ThemePreference returns the saved appearance preference.
func ThemePreference() string {
	switch pref := viper.GetString("theme"); pref {
	case ThemeLight, ThemeDark:
		return pref
	}
	return ThemeSystem
}

// ApplyTheme resolves the saved preference against the desktop setting and
// installs the theme. It is safe to call again whenever either changes.
func ApplyTheme(a fyne.App) {
	dark := false
	switch ThemePreference() {
	case ThemeDark:
		dark = true
	case ThemeLight:
	default:
		dark = a.Settings().ThemeVariant() == theme.VariantDark
	}
	if a.Settings().Theme() == appTheme && appTheme.IsDark() == dark {
		return
	}
	appTheme.setDark(dark)
	themeGeneration++
	a.Settings().SetTheme(appTheme)
}

// themeGeneration counts palette changes, and tabThemes records the
// generation each tab was last themed with.
var (
	themeGeneration int
	tabThemes       = map[*container.TabItem]int{}
)

// refreshTabTheme re-themes a tab that was hidden during a light/dark switch.
// Fyne only re-themes the selected tab of an AppTabs, so icons on the others
// keep the old colours until the theme is applied again.
func refreshTabTheme(item *container.TabItem) {
	if tabThemes[item] == themeGeneration {
		return
	}
	tabThemes[item] = themeGeneration
	fyne.CurrentApp().Settings().SetTheme(appTheme)
}

// Color implements fyne.Theme.
func (t *AppTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := t.IsDark()
	pick := func(light, darkC color.Color) color.Color {
		if dark {
			return darkC
		}
		return light
	}
	white := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	switch name {
	case theme.ColorNameBackground:
		return pick(colorBackground, colorDarkBackground)
	case colorNameSurface, theme.ColorNameInputBackground, theme.ColorNameMenuBackground:
		return pick(colorSurface, colorDarkSurface)
	case theme.ColorNameOverlayBackground:
		return pick(colorSurface, colorDarkSurface)
	case colorNameSurfaceBorder, theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return pick(colorBorder, colorDarkBorder)
	case colorNameSubtle:
		return pick(color.NRGBA{R: 0xF8, G: 0xFA, B: 0xFC, A: 0xFF}, color.NRGBA{R: 0x25, G: 0x2C, B: 0x37, A: 0xFF})
	case theme.ColorNameButton:
		return pick(colorButton, colorDarkElevated)
	case theme.ColorNameDisabledButton:
		return pick(color.NRGBA{R: 0xE5, G: 0xE7, B: 0xEB, A: 0xFF}, color.NRGBA{R: 0x27, G: 0x2E, B: 0x39, A: 0xFF})
	case theme.ColorNameDisabled:
		return pick(color.NRGBA{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}, color.NRGBA{R: 0x5B, G: 0x65, B: 0x73, A: 0xFF})
	case theme.ColorNameForeground:
		return pick(colorText, colorDarkText)
	case theme.ColorNamePlaceHolder:
		return pick(colorTextSecondary, colorDarkTextSecondary)
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnError:
		return white
	case theme.ColorNameForegroundOnWarning:
		return colorText
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return colorPrimary
	case theme.ColorNameSuccess:
		return colorSuccess
	case theme.ColorNameWarning:
		return colorWarning
	case theme.ColorNameError:
		return colorError
	case theme.ColorNameHover:
		return pick(color.NRGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0x0F}, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12})
	case theme.ColorNamePressed:
		return pick(color.NRGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0x1F}, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x22})
	case theme.ColorNameSelection:
		return pick(color.NRGBA{R: 0x3B, G: 0x82, B: 0xF6, A: 0x40}, color.NRGBA{R: 0x3B, G: 0x82, B: 0xF6, A: 0x60})
	case theme.ColorNameScrollBar:
		return pick(color.NRGBA{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}, color.NRGBA{R: 0x5F, G: 0x6B, B: 0x7A, A: 0xFF})
	case theme.ColorNameShadow:
		return pick(color.NRGBA{A: 0x22}, color.NRGBA{A: 0x66})
	case theme.ColorNameHeaderBackground:
		return pick(colorSurface, colorDarkSurface)
	}

	v := theme.VariantLight
	if dark {
		v = theme.VariantDark
	}
	return theme.DefaultTheme().Color(name, v)
}

// Font implements fyne.Theme.
func (t *AppTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon implements fyne.Theme.
func (t *AppTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size implements fyne.Theme.
func (t *AppTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case sizeNameTimer:
		return 44
	case sizeNameStat:
		return 22
	}
	return theme.DefaultTheme().Size(name)
}
