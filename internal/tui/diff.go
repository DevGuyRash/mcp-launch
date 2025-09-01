package tui

import (
	"github.com/charmbracelet/lipgloss"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"strings"
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
				if strings.TrimSpace(bl) == "" {
					continue
				}
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
		if wrap {
			for _, seg := range softWrap(l, 100) {
				sb.WriteString(diffDelLine.Render("- ") + seg + "\n")
			}
		} else {
			sb.WriteString(diffDelLine.Render("- ") + l + "\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(tagStyle.Render("OVERRIDE") + "\n")
	for _, l := range strings.Split(after, "\n") {
		if wrap {
			for _, seg := range softWrap(l, 100) {
				sb.WriteString(diffAddLine.Render("+ ") + seg + "\n")
			}
		} else {
			sb.WriteString(diffAddLine.Render("+ ") + l + "\n")
		}
	}
	return sb.String()
}

// renderSideBySideDiff renders a very simple side-by-side view.
// width is the max width of each column (best-effort).
func renderSideBySideDiff(before, after string, width int, wrap bool, leftStart, rightStart int) string {
	bLines := strings.Split(before, "\n")
	aLines := strings.Split(after, "\n")
	max := len(bLines)
	if len(aLines) > max {
		max = len(aLines)
	}
	pad := func(s string, n int) string {
		if len([]rune(s)) >= n {
			return s
		}
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
        if i < len(bLines) {
            bl = bLines[i]
        }
        if i < len(aLines) {
            al = aLines[i]
        }
        // produce display lines respecting wrap
        leftLines := []string{bl}
        rightLines := []string{al}
        if wrap {
            leftLines = softWrap(bl, width-2)
            rightLines = softWrap(al, width-2)
        }
		// expand to same number of visual lines
		rows := len(leftLines)
		if len(rightLines) > rows {
			rows = len(rightLines)
		}
		for r := 0; r < rows; r++ {
			ll := ""
			rl := ""
			if r < len(leftLines) {
				ll = leftLines[r]
			}
			if r < len(rightLines) {
				rl = rightLines[r]
			}
            if !wrap && bl != al && r == 0 {
                // char-level highlight only on first visual row when not wrapping
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
                ll = lbuf.String()
                rl = rbuf.String()
                // apply horizontal clipping when not wrapping
                if leftStart > 0 {
                    rr := []rune(ll)
                    if leftStart < len(rr) {
                        ll = string(rr[leftStart:])
                    } else { ll = "" }
                }
                if rightStart > 0 {
                    rr := []rune(rl)
                    if rightStart < len(rr) {
                        rl = string(rr[rightStart:])
                    } else { rl = "" }
                }
            } else if bl == al {
                ll = faint.Render(ll)
                rl = faint.Render(rl)
            }
            // apply clipping for non-first visual lines when not wrapping
            if !wrap && r > 0 {
                if leftStart > 0 {
                    rr := []rune(ll)
                    if leftStart < len(rr) { ll = string(rr[leftStart:]) } else { ll = "" }
                }
                if rightStart > 0 {
                    rr := []rune(rl)
                    if rightStart < len(rr) { rl = string(rr[rightStart:]) } else { rl = "" }
                }
            }
            left := pad(ll, width)
            right := rl
            sb.WriteString(left + "  |  " + right + "\n")
        }
    }
    return sb.String()
}

// softWrap breaks a string into lines of at most width columns, attempting to wrap at spaces.
func softWrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	r := []rune(s)
	for len(r) > width {
		cut := width
		for i := width; i > 0; i-- {
			if r[i-1] == ' ' || r[i-1] == '\t' {
				cut = i - 1
				break
			}
		}
		if cut <= 0 {
			cut = width
		}
		out = append(out, strings.TrimRight(string(r[:cut]), " \t"))
		r = r[cut:]
		for len(r) > 0 && (r[0] == ' ' || r[0] == '\t') {
			r = r[1:]
		}
	}
	out = append(out, string(r))
	return out
}
