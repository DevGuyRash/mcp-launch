# Requirements Document

## Introduction

This spec defines a set of TUI UX improvements for mcp-launch focused on: (1) tool description tagging and clarity, (2) keys legend/help presentation, (3) diff viewer alignment/highlighting/wrapping, (4) editor mode/mode-switch visibility, and (5) multi-select/description lists consistency. The goal is to make menus easier to scan, status more truthful (“Edited” vs automatic trims), and diffs readable across terminal widths.

## Alignment with Product Vision

- Consistent, discoverable, keyboard‑first TUI (Product: “Consistent TUI navigation and aesthetics”).
- Diff widget with robust modes and highlighting (Tech: “Diff view supports unified and side‑by‑side with char‑level highlights”).
- Progressive disclosure and terminal‑native beauty (Product Principles).
- Performance and responsiveness under large tool lists (Tech Constraints; Quality snapshot tests for diff rendering).

## Definitions

- Raw Description: Source text as discovered (pre‑transform), immutable snapshot for analysis.
- Modified/Override Description: The current working text after user edits and/or automatic shortening.
- Limit: Maximum allowed characters for a description. Default 300, configurable via settings/help.
- Edited: A state indicating the user has performed a manual textual change (configurable to include/exclude whitespace‑only edits; default includes any textual change).
- Trimmed: Word‑safe shortening operation applied to fit within Limit.
- Truncated: Hard cut operation applied to forcibly fit within Limit when trimming cannot.
- Over Limit (+N): Raw length exceeds Limit by N = raw_length − Limit.

Notes:
- Trimmed and Truncated are mutually exclusive; only one may apply at a time. Edited may co‑exist with either.
- Tags are computed from Raw and Modified states and update on save. They persist across sessions based on stored Modified text and metadata.

## Requirements

### R1: Description Status Tags (truthful and multi-tag)

User Story: As a user inspecting tool descriptions, I want accurate, color‑coded status chips to indicate the nature of changes and length constraints so that I can quickly assess whether a description is user‑edited vs automatically shortened.

Acceptance Criteria
1. WHEN the Descriptions list loads THEN the system SHALL compute and render the following independent tags (stable order: Edited, Trimmed, Truncated, Over Limit, Original Length, Modified Length; distinct colors):
   - Edited: Only when user has manually edited this tool’s description (tracked separately from auto transforms).
   - Trimmed: Word-safe shortening applied to fit within limit.
   - Truncated: Hard cut applied.
   - Over Limit (+N): Raw description exceeds limit by N characters.
   - Original Length (N): Raw length in characters.
   - Modified Length (M): Current working length shown.
2. IF content is only auto-shortened (Trimmed or Truncated) and never manually changed THEN Edited SHALL NOT appear.
3. Tags SHALL be rendered with distinct, consistent colors and ASCII fallbacks (e.g., [Edited], [Trimmed]) in NO_COLOR terminals.
4. Multi-select view SHALL show the same tags for each item.
5. Changes in edit view SHALL update tags upon save without requiring navigation away.

### R2: Keys Legend and Help UX

User Story: As a keyboard-first user, I want a clear, modern, grouped keys legend and discoverable help so I can understand available actions without parsing dense one-liners.

Acceptance Criteria
1. Keys help SHALL be multi-line, grouped (navigation, actions, view toggles), and consistently styled across screens; one-line run-ons SHALL be eliminated.
2. Pressing ? SHALL show a help overlay (non-blocking) listing keys, grouped and concise, with current mode indicated.
3. The server options menu SHALL remove numeric shortcuts 1–4; navigation remains via arrows/enter and listed keys only.
4. A status bar (or header) SHALL indicate current mode (e.g., [INSERT] or [CMD]) and key state toggles (e.g., Wrap: On/Off).

### R3: Diff Viewer Improvements (side-by-side, unified, wrapping)

User Story: As a user comparing RAW vs OVERRIDE descriptions, I need aligned columns, clear vertical separators, correct syntax highlighting, and a wrap toggle that actually works so I can read and understand changes at any terminal width.

Acceptance Criteria
1. Side-by-side view SHALL align columns with a stable vertical separator; column widths adapt to terminal size while preserving a minimal readable width per column. If width < (2×minCol + separator + gutters), the system SHALL fallback to unified view with a visible notice.
2. Unified view SHALL display proper +/- syntax highlighting; side-by-side SHALL show intraline highlights and clear diff colors (with readable NO_COLOR fallbacks). Headers (“RAW” and “OVERRIDE”) SHALL remain visible (sticky) during scroll.
3. Default Wrap: On. The Wrap toggle (w) SHALL function in both modes:
   - Wrap On: Soft-wrap lines within each column (side-by-side) or within the single pane (unified).
   - Wrap Off: Enable horizontal scrolling with ←/→ (and optional h/l) and a visible position indicator.
   The current Wrap state SHALL be displayed in the status bar and preserved across mode/view switches.
4. The vertical border SHALL remain visually consistent (not jagged or misaligned) during resize and navigation; column widths SHALL be recalculated on resize.
5. The previous defect (“wrap key does not work”) SHALL be fixed with a testable reducer path: toggling wrap triggers a redraw and state transition visible in tests.

### R4: Edit View Mode Clarity and Wrap/Mode Interop

User Story: As a user editing a description, I want an unmistakable mode indicator and predictable wrap behavior regardless of switching between CMD and INSERT modes.

Acceptance Criteria
1. The editor SHALL display a clear mode indicator: [INSERT] vs [CMD], each with distinct color and label. Switching modes SHALL briefly show a toast/overlay or status tick confirming the change.
2. Help area SHALL show how to switch modes and provide a concise summary of mode capabilities.
3. BUG: “If I unwrap in CMD mode, then switch back to edit, I can no longer see anything.” The system SHALL ensure the buffer remains visible; wrap state persists across modes; cursor remains within visible viewport after any toggle or mode change.
4. The editor SHALL expose current wrap state in the status bar and update it in real time when toggled in either mode.

### R5: Diff Screen Readability Enhancements

User Story: As a user reviewing long diffs, I want readable, non-cut-off views with proper alignment even when wrapping is off.

Acceptance Criteria
1. Side-by-side no-wrap SHALL show both columns with horizontal scrolling available: independently by default; synchronized mode is toggleable (documented in help overlay).
2. The vertical border SHALL remain straight and aligned across scrolled content; column widths SHALL be recalculated on resize.
3. Headers (“RAW | OVERRIDE”) SHALL remain visible/sticky or easily re-referenced.
4. Copy/yank shortcuts (optional) SHALL be discoverable, but no clipboard dependencies are required.

### R6: Lists Consistency (Allowed/Disallowed & Descriptions)

User Story: As a user, I want consistent list behavior and hints across menus.

Acceptance Criteria
1. Allowed/Disallowed tools lists remain functionally unchanged but adopt the same grouped, multi-line keys legend and shared status bar component.
2. Descriptions list SHALL support multi-select with tags as in R1, preserving performance on large lists.

### R7: Length Handling and Counters

User Story: As a user, I want clear visibility into description lengths and applied transformations.

Acceptance Criteria
1. When Raw length > Limit, Over Limit (+N) SHALL be shown, where N = Raw − Limit; this tag MAY remain visible after auto‑shortening to indicate original overage.
2. Original Length (N) and Modified Length (M) SHALL be displayed in the format “Orig N · Mod M”; values refresh after edits and trims/truncation.
3. Trimmed and Truncated SHALL be mutually exclusive; exactly one represents the applied operation. Edited MAY co‑exist with either.
4. Limit default is 300 characters and SHALL be surfaced in help; configuration changes update limits in real time.

### R8: Accessibility and Color Fallback

User Story: As a user in various terminals, I want the UI to remain readable without colors and under limited widths.

Acceptance Criteria
1. NO_COLOR and low-color terminals SHALL render readable markers (e.g., [Edited], [Trimmed]) and diff indicators (+/-).
2. Minimum width thresholds SHALL trigger automatic fallback to unified diff with a visible notice.

## Non-Functional Requirements

### Code Architecture and Modularity
- Apply SRP within internal/tui: reducers handle state; rendering in View* helpers; separate widgets for tags and diff.
- Avoid cross-coupling with process logic; UI remains pure overlay (per Structure doc).
- Clear interfaces for diff data and tag computation (utility functions with unit tests).

### Performance
- Rendering actions SHALL complete under 16ms average on typical terminals for common lists (<200 items); avoid excessive re-renders.
- Diff rendering SHALL paginate/virtualize for very long lines to keep UI responsive.

### Security
- No secrets or tokens rendered in diffs; sanitize logs.

### Reliability
- Unit tests for tag computation; reducer tests for wrap and mode switches; snapshot tests for diff alignment and wrap behavior.

### Usability
- Grouped keys help, consistent status bar, obvious mode indicators, and clear column headers/borders.
 - Accessibility: Use a high‑contrast palette for tags and diffs; provide NO_COLOR fallbacks. Aim for clear legibility in low‑color terminals; never rely on color alone to convey meaning.
