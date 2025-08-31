package state

// ToggleWrap flips the Wrap flag and returns a new state copy.
func ToggleWrap(s UIState) UIState {
    s.Wrap = !s.Wrap
    return s
}

// ToggleMode switches between CMD and INSERT modes and sets a brief notice.
func ToggleMode(s UIState) UIState {
    if s.Mode == CMD {
        s.Mode = INSERT
        s.Notice = "[INSERT]"
    } else {
        s.Mode = CMD
        s.Notice = "[CMD]"
    }
    return s
}

// ToggleView switches between Unified and SideBySide diff views.
func ToggleView(s UIState) UIState {
    if s.View == Unified {
        s.View = SideBySide
    } else {
        s.View = Unified
    }
    return s
}

// Resize updates width and sets a fallback notice if too narrow for side-by-side.
// Threshold heuristic: need at least 2*MinCol plus 3 chars for separator/gutters.
func Resize(s UIState, width int) UIState {
    s.Width = width
    threshold := 2*s.MinCol + 3
    if s.View == SideBySide && s.Width < threshold {
        s.View = Unified
        s.Notice = "Narrow width: using unified view"
    }
    return s
}

// ScrollLeft adjusts horizontal scroll for left or right column.
func ScrollLeft(s UIState, fast bool, leftColumn bool) UIState {
    delta := 1
    if fast {
        delta = 8
    }
    if leftColumn {
        if s.ScrollHLeft >= delta {
            s.ScrollHLeft -= delta
        } else {
            s.ScrollHLeft = 0
        }
    } else {
        if s.ScrollHRight >= delta {
            s.ScrollHRight -= delta
        } else {
            s.ScrollHRight = 0
        }
    }
    return s
}

// ScrollRight adjusts horizontal scroll for left or right column.
func ScrollRight(s UIState, fast bool, leftColumn bool) UIState {
    delta := 1
    if fast {
        delta = 8
    }
    if leftColumn {
        s.ScrollHLeft += delta
    } else {
        s.ScrollHRight += delta
    }
    return s
}

// ToggleSyncScroll toggles synchronized column scrolling.
func ToggleSyncScroll(s UIState) UIState {
    s.SyncScroll = !s.SyncScroll
    return s
}

// RecomputeTags is a placeholder for triggering tag recomputation flows.
func RecomputeTags(s UIState) UIState {
    return s
}

