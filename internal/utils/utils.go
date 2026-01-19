package utils

import (
	"fmt"
	"image/color"
	"time"
)

// FormatDuration formats a time.Duration as HH:MM:SS.
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// ParseHexColor parses a hex color string and returns a color.Color.
// Accepts formats: #RRGGBB, RRGGBB, #RGB, RGB.
// Returns color.Transparent if the input is invalid.
func ParseHexColor(hex string) color.Color {
	// Remove # prefix if present
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}

	// Default to transparent if invalid
	if len(hex) != 6 && len(hex) != 3 {
		return color.Transparent
	}

	// Expand 3-char hex to 6-char
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	// Parse hex values
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.Transparent
	}

	return color.RGBA{R: r, G: g, B: b, A: 255}
}
