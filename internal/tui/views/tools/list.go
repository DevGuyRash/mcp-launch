package tools

import (
    help "mcp-launch/internal/tui/widgets/helpoverlay"
    "mcp-launch/internal/tui/state"
)

// RenderHelp returns the grouped keys overlay content for tools lists.
func RenderHelp(s state.UIState) string {
    h := help.NewHelpOverlay()
    return h.View(s)
}

