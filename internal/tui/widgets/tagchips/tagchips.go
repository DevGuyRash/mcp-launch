package tagchips

import (
    "fmt"
    "os"
    "strings"

    "github.com/charmbracelet/lipgloss"
    "mcp-launch/internal/tui/state"
)

// View renders description tags in a stable order using colored chips when
// possible and ASCII fallbacks when color is disabled or not desired.
func View(tags []state.Tag, noColor bool) string {
    if len(tags) == 0 {
        return ""
    }
    // Honor NO_COLOR env var in addition to explicit param
    if !noColor && os.Getenv("NO_COLOR") != "" {
        noColor = true
    }

    parts := make([]string, 0, len(tags))
    for _, t := range tags {
        parts = append(parts, renderChip(t, noColor))
    }
    return strings.Join(parts, " ")
}

func renderChip(t state.Tag, noColor bool) string {
    label := chipLabel(t)
    if noColor {
        return fmt.Sprintf("[%s]", label)
    }
    style := chipStyle(t)
    return style.Render(" " + label + " ")
}

func chipLabel(t state.Tag) string {
    switch t.Kind {
    case state.EDITED:
        return "Edited"
    case state.TRIMMED:
        return "Trimmed"
    case state.TRUNCATED:
        return "Truncated"
    case state.OVER_LIMIT:
        return fmt.Sprintf("Over +%d", t.Value)
    case state.ORIG_LEN:
        return fmt.Sprintf("Orig %d", t.Value)
    case state.MOD_LEN:
        return fmt.Sprintf("Mod %d", t.Value)
    default:
        return "Tag"
    }
}

func chipStyle(t state.Tag) lipgloss.Style {
    // Simple readable palette; Task 13 may refactor to util/color with NO_COLOR fallbacks
    base := lipgloss.NewStyle().Padding(0, 1).Bold(true)
    switch t.Kind {
    case state.EDITED:
        return base.Background(lipgloss.Color("#3D6DFF")).Foreground(lipgloss.Color("#FFFFFF"))
    case state.TRIMMED:
        return base.Background(lipgloss.Color("#2AA876")).Foreground(lipgloss.Color("#FFFFFF"))
    case state.TRUNCATED:
        return base.Background(lipgloss.Color("#D9534F")).Foreground(lipgloss.Color("#FFFFFF"))
    case state.OVER_LIMIT:
        return base.Background(lipgloss.Color("#F0AD4E")).Foreground(lipgloss.Color("#111111"))
    case state.ORIG_LEN:
        return base.Background(lipgloss.Color("#6C757D")).Foreground(lipgloss.Color("#FFFFFF"))
    case state.MOD_LEN:
        return base.Background(lipgloss.Color("#5A5A5A")).Foreground(lipgloss.Color("#FFFFFF"))
    default:
        return base
    }
}

