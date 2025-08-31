package state

import "testing"

func TestToggleWrap(t *testing.T) {
    s := UIState{Wrap: false}
    s = ToggleWrap(s)
    if !s.Wrap { t.Fatalf("expected Wrap to be true") }
}

func TestToggleModeSetsNotice(t *testing.T) {
    s := UIState{Mode: CMD}
    s = ToggleMode(s)
    if s.Mode != INSERT || s.Notice == "" { t.Fatalf("expected INSERT mode and notice") }
    s = ToggleMode(s)
    if s.Mode != CMD || s.Notice == "" { t.Fatalf("expected CMD mode and notice") }
}

func TestToggleView(t *testing.T) {
    s := UIState{View: Unified}
    s = ToggleView(s)
    if s.View != SideBySide { t.Fatalf("expected SideBySide view") }
}

func TestResizeFallbackToUnified(t *testing.T) {
    s := UIState{View: SideBySide, MinCol: 20}
    s = Resize(s, 30) // threshold = 2*20+3 = 43; 30 < 43 => unified
    if s.View != Unified { t.Fatalf("expected Unified after resize fallback") }
    if s.Notice == "" { t.Fatalf("expected fallback notice to be set") }
}

func TestScrolls(t *testing.T) {
    s := UIState{}
    s = ScrollRight(s, true, true)
    if s.ScrollHLeft == 0 { t.Fatalf("expected left scroll to increase") }
    s = ScrollLeft(s, true, true)
    if s.ScrollHLeft != 0 { t.Fatalf("expected left scroll to return to 0") }
}

func TestToggleSyncScroll(t *testing.T) {
    s := UIState{}
    s = ToggleSyncScroll(s)
    if !s.SyncScroll { t.Fatalf("expected SyncScroll to be true") }
}

