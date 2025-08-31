package diff

import (
    "fmt"
    "strings"

    "mcp-launch/internal/tui/state"
)

type DiffView struct{}

func NewDiffView() DiffView { return DiffView{} }

// View renders a simplified diff view. For SideBySide it aligns two columns
// with a vertical separator. For Unified it prefixes lines with +/- markers.
// Note: This is a minimal implementation to support compilation and basic
// display needs; further enhancements can add intraline highlights and
// wrapping/scrolling polish.
func (DiffView) View(s state.UIState, raw, override string) string {
    if s.View == state.SideBySide {
        return sideBySide(raw, override, s)
    }
    return unified(raw, override)
}

func unified(raw, override string) string {
    var b strings.Builder
    b.WriteString("RAW vs OVERRIDE (Unified)\n")
    rawLines := strings.Split(raw, "\n")
    modLines := strings.Split(override, "\n")
    max := len(rawLines)
    if len(modLines) > max {
        max = len(modLines)
    }
    for i := 0; i < max; i++ {
        if i < len(rawLines) {
            fmt.Fprintf(&b, "- %s\n", rawLines[i])
        }
        if i < len(modLines) {
            fmt.Fprintf(&b, "+ %s\n", modLines[i])
        }
    }
    return b.String()
}

func sideBySide(raw, override string, s state.UIState) string {
    const sep = " │ "
    var b strings.Builder
    b.WriteString("RAW │ OVERRIDE\n")
    left := strings.Split(raw, "\n")
    right := strings.Split(override, "\n")
    max := len(left)
    if len(right) > max {
        max = len(right)
    }
    // Compute column width from total width if provided
    colWidth := 40
    if s.Width > 0 {
        // basic gutters + separator
        colWidth = (s.Width - len(sep)) / 2
        if colWidth < 10 {
            colWidth = 10
        }
    }
    for i := 0; i < max; i++ {
        l := ""
        r := ""
        if i < len(left) {
            l = left[i]
        }
        if i < len(right) {
            r = right[i]
        }
        l = clip(l, colWidth, s.ScrollHLeft)
        r = clip(r, colWidth, s.ScrollHRight)
        fmt.Fprintf(&b, "%s%s%s\n", pad(l, colWidth), sep, pad(r, colWidth))
    }
    return b.String()
}

func clip(s string, width int, start int) string {
    runes := []rune(s)
    if start < 0 {
        start = 0
    }
    if start >= len(runes) {
        return ""
    }
    end := start + width
    if end > len(runes) {
        end = len(runes)
    }
    return string(runes[start:end])
}

func pad(s string, width int) string {
    if w := len([]rune(s)); w < width {
        return s + strings.Repeat(" ", width-w)
    }
    return s
}

