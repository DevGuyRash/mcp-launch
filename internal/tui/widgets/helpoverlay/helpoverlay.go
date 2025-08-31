package helpoverlay

import (
    "fmt"
    "strings"

    "mcp-launch/internal/tui/state"
)

type HelpOverlay struct{}

func NewHelpOverlay() HelpOverlay { return HelpOverlay{} }

// View returns grouped keys help with the current mode indicated.
func (HelpOverlay) View(s state.UIState) string {
    mode := "CMD"
    if s.Mode == state.INSERT {
        mode = "INSERT"
    }
    sections := []struct{
        title string
        keys  []string
    }{
        {"Navigation", []string{"↑/↓: move", "PgUp/PgDn: fast", "Home/End: edges"}},
        {"Actions", []string{"Enter: select", "Space: toggle", "s: save"}},
        {"View", []string{"v: toggle unified/side-by-side", "w: wrap on/off", "h/l or ←/→: scroll H"}},
        {"Editor", []string{"i: INSERT mode", "Esc: CMD mode"}},
    }
    var b strings.Builder
    fmt.Fprintf(&b, "Help (Mode: %s)\n", mode)
    for _, sec := range sections {
        fmt.Fprintf(&b, "\n%s:\n", sec.title)
        for _, k := range sec.keys {
            fmt.Fprintf(&b, "  %s\n", k)
        }
    }
    return b.String()
}

