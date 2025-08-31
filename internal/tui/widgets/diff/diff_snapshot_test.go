package diff

import (
    "strings"
    "testing"

    "mcp-launch/internal/tui/state"
)

func TestUnifiedSnapshot(t *testing.T) {
    v := NewDiffView()
    s := state.UIState{View: state.Unified}
    out := v.View(s, "a\nb", "a\nc")
    if !strings.Contains(out, "RAW vs OVERRIDE (Unified)") {
        t.Fatalf("missing unified header")
    }
    if !strings.Contains(out, "- a") || !strings.Contains(out, "+ c") {
        t.Fatalf("expected +/- lines in unified output")
    }
}

func TestSideBySideSnapshot(t *testing.T) {
    v := NewDiffView()
    s := state.UIState{View: state.SideBySide, Width: 60}
    out := v.View(s, "left", "right")
    if !strings.HasPrefix(out, "RAW │ OVERRIDE\n") {
        t.Fatalf("missing sbs header")
    }
    if !strings.Contains(out, " │ ") {
        t.Fatalf("missing separator")
    }
}

