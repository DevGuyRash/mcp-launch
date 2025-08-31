package util

import (
    "os"

    "github.com/charmbracelet/lipgloss"
)

// NoColor returns true if color output should be disabled.
func NoColor(explicit bool) bool {
    if explicit {
        return true
    }
    return os.Getenv("NO_COLOR") != ""
}

// Palette defines a small set of colors used across widgets.
type Palette struct {
    Primary   lipgloss.Color
    Success   lipgloss.Color
    Danger    lipgloss.Color
    Warning   lipgloss.Color
    Muted     lipgloss.Color
    MutedDark lipgloss.Color
}

// DefaultPalette returns the default palette.
func DefaultPalette() Palette {
    return Palette{
        Primary:   lipgloss.Color("#3D6DFF"),
        Success:   lipgloss.Color("#2AA876"),
        Danger:    lipgloss.Color("#D9534F"),
        Warning:   lipgloss.Color("#F0AD4E"),
        Muted:     lipgloss.Color("#6C757D"),
        MutedDark: lipgloss.Color("#5A5A5A"),
    }
}

