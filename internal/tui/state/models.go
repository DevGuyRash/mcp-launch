package state

// EditorMode represents the editor's current input mode.
type EditorMode int

const (
    CMD EditorMode = iota
    INSERT
)

// DiffMode controls how the diff is rendered.
type DiffMode int

const (
    Unified DiffMode = iota
    SideBySide
)

// UIState holds cross-widget UI state used by status bar, diff, and editor.
type UIState struct {
    // Mode & View
    Mode EditorMode
    Wrap bool
    View DiffMode

    // Layout & scrolling
    Width   int
    MinCol  int
    ScrollHLeft  int
    ScrollHRight int
    ScrollV int
    SyncScroll bool

    // Description constraints & flags
    Limit  int  // default 300 at runtime if zero
    Edited bool // user-initiated edits

    // Notices and ephemeral messages
    Notice string
}

