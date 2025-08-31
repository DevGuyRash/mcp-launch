package statusbar

import (
    "fmt"
    "strings"

    "mcp-launch/internal/tui/state"
)

type StatusBar struct{}

func NewStatusBar() StatusBar { return StatusBar{} }

// View composes a concise status line reflecting key UI state.
func (StatusBar) View(s state.UIState) string {
    mode := "[CMD]"
    if s.Mode == state.INSERT {
        mode = "[INSERT]"
    }
    wrap := "Wrap: Off"
    if s.Wrap {
        wrap = "Wrap: On"
    }
    view := "Unified"
    if s.View == state.SideBySide {
        view = "Side-by-side"
    }
    pos := fmt.Sprintf("H:%d|%d V:%d", s.ScrollHLeft, s.ScrollHRight, s.ScrollV)
    width := fmt.Sprintf("W:%d", s.Width)

    parts := []string{mode, wrap, view, pos, width}
    if s.Notice != "" {
        parts = append(parts, s.Notice)
    }
    return strings.Join(parts, "  ")
}

