package descriptions

import (
    "mcp-launch/internal/tui/state"
    chips "mcp-launch/internal/tui/widgets/tagchips"
)

// RenderTags is a thin adapter over the TagChips widget for list items.
func RenderTags(tags []state.Tag, noColor bool) string {
    return chips.View(tags, noColor)
}

