package editor

import (
    "fmt"
    "strings"

    "mcp-launch/internal/tui/state"
)

type Editor struct{}

func NewEditor() Editor { return Editor{} }

// View renders a simple editable buffer preview along with mode and wrap state.
func (Editor) View(s state.UIState, buf string) string {
    header := "[CMD]"
    if s.Mode == state.INSERT {
        header = "[INSERT]"
    }
    wrap := "Wrap: Off"
    if s.Wrap {
        wrap = "Wrap: On"
    }
    // Counters surfaced in the status area on save can be added by callers
    var b strings.Builder
    fmt.Fprintf(&b, "%s  %s\n", header, wrap)
    fmt.Fprintf(&b, "%s\n", buf)
    return b.String()
}

