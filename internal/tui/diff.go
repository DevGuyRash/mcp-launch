package tui

import (
    "strings"
    dmp "github.com/sergi/go-diff/diffmatchpatch"
    "github.com/charmbracelet/lipgloss"
)

var (
    diffDelLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"})
    diffAddLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"})
    diffDelChar = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"}).Underline(true)
    diffAddChar = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"}).Underline(true)
    faint       = lipgloss.NewStyle().Faint(true)
)

// renderUnifiedDiff renders a simple unified diff with line- and char-level highlights.
func renderUnifiedDiff(before, after string, wrap bool) string {
    if before == after {
        return "No changes\n"
    }
    // Heuristic: if line counts match, do per-line char highlight; otherwise show raw blocks.
    bLines := strings.Split(before, "\n")
    aLines := strings.Split(after, "\n")
    var sb strings.Builder
    if len(bLines) == len(aLines) && len(bLines) > 0 {
        for i := 0; i < len(bLines); i++ {
            bl := bLines[i]
            al := aLines[i]
            if bl == al {
                if strings.TrimSpace(bl) == "" { continue }
                sb.WriteString("  ")
                sb.WriteString(faint.Render(bl))
                sb.WriteString("\n")
                continue
            }
            // char-level on pair
            d := dmp.New()
            diffs := d.DiffMain(bl, al, false)
            d.DiffCleanupSemantic(diffs)
            // deleted line (whole-line color with embedded char spans)
            sb.WriteString(diffDelLine.Render("- "))
            for _, df := range diffs {
                switch df.Type {
                case dmp.DiffDelete:
                    sb.WriteString(diffDelChar.Render(df.Text))
                case dmp.DiffEqual:
                    sb.WriteString(diffDelLine.Render(df.Text))
                }
            }
            sb.WriteString("\n")
            // added line
            sb.WriteString(diffAddLine.Render("+ "))
            for _, df := range diffs {
                switch df.Type {
                case dmp.DiffInsert:
                    sb.WriteString(diffAddChar.Render(df.Text))
                case dmp.DiffEqual:
                    sb.WriteString(diffAddLine.Render(df.Text))
                }
            }
            sb.WriteString("\n")
        }
        return sb.String()
    }
    // Fallback: show raw blocks
    sb.WriteString(tagStyle.Render("RAW") + "\n")
    for _, l := range strings.Split(before, "\n") {
        sb.WriteString(diffDelLine.Render("- ") + l + "\n")
    }
    sb.WriteString("\n")
    sb.WriteString(tagStyle.Render("OVERRIDE") + "\n")
    for _, l := range strings.Split(after, "\n") {
        sb.WriteString(diffAddLine.Render("+ ") + l + "\n")
    }
    return sb.String()
}

// renderSideBySideDiff renders a very simple side-by-side view.
// width is the max width of each column (best-effort).
func renderSideBySideDiff(before, after string, width int, wrap bool) string {
    bLines := strings.Split(before, "\n")
    aLines := strings.Split(after, "\n")
    max := len(bLines)
    if len(aLines) > max { max = len(aLines) }
    pad := func(s string, n int) string {
        if len([]rune(s)) >= n { return s }
        return s + strings.Repeat(" ", n-len([]rune(s)))
    }
    var sb strings.Builder
    header := lipgloss.JoinHorizontal(lipgloss.Top,
        tagWarnStyle.Render("RAW"),
        strings.Repeat(" ", width-3),
        tagStyle.Render("OVERRIDE"),
    )
    sb.WriteString(header + "\n")
    for i := 0; i < max; i++ {
        var bl, al string
        if i < len(bLines) { bl = bLines[i] }
        if i < len(aLines) { al = aLines[i] }
        if bl == al {
            left := pad(faint.Render(bl), width)
            right := faint.Render(al)
            sb.WriteString(left + "  |  " + right + "\n")
            continue
        }
        // char-level spans side-by-side
        d := dmp.New()
        diffs := d.DiffMain(bl, al, false)
        d.DiffCleanupSemantic(diffs)
        var lbuf, rbuf strings.Builder
        for _, df := range diffs {
            switch df.Type {
            case dmp.DiffDelete:
                lbuf.WriteString(diffDelChar.Render(df.Text))
            case dmp.DiffInsert:
                rbuf.WriteString(diffAddChar.Render(df.Text))
            case dmp.DiffEqual:
                lbuf.WriteString(diffDelLine.Render(df.Text))
                rbuf.WriteString(diffAddLine.Render(df.Text))
            }
        }
        left := pad(diffDelLine.Render("- ")+lbuf.String(), width)
        right := diffAddLine.Render("+ ")+rbuf.String()
        sb.WriteString(left + "  |  " + right + "\n")
    }
    return sb.String()
}
