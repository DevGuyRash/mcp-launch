# Tasks Document

- [x] 1. Create tag computation utility
  - File: internal/tui/util/tags.go
  - Implement ComputeTags(raw, modified string, limit int, edited bool) []Tag with stable order (Edited, Trimmed, Truncated, Over Limit, Orig Len, Mod Len). Ensure Trimmed vs Truncated exclusivity and Over Limit math.
  - Purpose: Provide truthful, consistent status tags for descriptions.
  - Leverage: Definitions in requirements; models below.
  - Requirements: R1, R7

- [x] 2. Add tag models and enums
  - File: internal/tui/state/tags.go
  - Define TagKind (EDITED, TRIMMED, TRUNCATED, OVER_LIMIT, ORIG_LEN, MOD_LEN) and Tag struct with optional Value.
  - Purpose: Standardize tag representation across widgets.
  - Requirements: R1, R7

- [x] 3. Implement TagChips widget
  - File: internal/tui/widgets/tagchips/tagchips.go
  - Render color-coded chips with ASCII fallback for NO_COLOR; expose View(tags []Tag, noColor bool) string.
  - Purpose: Display tags in lists and detail views.
  - Requirements: R1, R8

- [x] 4. Add StatusBar widget
  - File: internal/tui/widgets/statusbar/statusbar.go
  - Show [Mode], Wrap state, View mode (Unified/Side-by-side), width, and horizontal position indicator.
  - Purpose: Provide persistent UI state visibility.
  - Requirements: R2, R3, R4, R5

- [x] 5. Add HelpOverlay widget
  - File: internal/tui/widgets/helpoverlay/helpoverlay.go
  - Multi-line grouped keys (navigation, actions, view toggles, editor), with current mode highlighted; toggled by '?'.
  - Purpose: Replace dense one-line keys with discoverable overlay.
  - Requirements: R2

- [x] 6. Extend UI state models
  - File: internal/tui/state/models.go
  - Add fields: Mode (CMD/INSERT), Wrap (bool), View (Unified/SideBySide), Width, MinCol, ScrollHLeft/ScrollHRight, ScrollV, SyncScroll, Limit (default 300), Edited (bool), Notice.
  - Purpose: Support new interactions and deterministic fallback.
  - Requirements: R3, R4, R5, R7

- [x] 7. Implement reducers for toggles and scrolling
  - File: internal/tui/state/reducers.go
  - Add actions: ToggleWrap, ToggleMode, ToggleView, Resize (compute fallback threshold), ScrollLeft/Right(fast), ToggleSyncScroll, RecomputeTags.
  - Purpose: State-first, testable transitions; fix wrap toggle bug path.
  - Requirements: R3, R4, R5

- [x] 8. DiffView widget (unified/side-by-side)
  - File: internal/tui/widgets/diff/diff.go
  - Sticky headers (RAW/OVERRIDE), stable vertical separator, intraline highlighting; wrap on/off; horizontal scroll with indicators; narrow fallback notice.
  - Purpose: Readable diffs across widths; fix wrap defects.
  - Requirements: R3, R5, R8

- [x] 9. Editor widget updates
  - File: internal/tui/widgets/editor/editor.go
  - Clear CMD/INSERT indicators; mode-change toast; wrap persistence across modes; ensure cursor/viewport visible after toggles; show counters on save.
  - Purpose: Mode clarity; fix visibility bug.
  - Requirements: R4, R7

- [x] 10. Integrate TagChips into lists
  - File: internal/tui/views/descriptions/list.go (or equivalent list view)
  - Compute and render tags for each item in Descriptions and multi-select screens; update on save.
  - Purpose: Consistent tag display.
  - Requirements: R1, R6, R7

- [x] 11. Apply grouped keys legend to Allowed/Disallowed lists
  - File: internal/tui/views/tools/list.go (allowed/disallowed)
  - Replace one-line hints with HelpOverlay; keep space/enter behavior; preserve performance.
  - Purpose: Consistent help experience.
  - Requirements: R2, R6

- [x] 12. Remove numeric shortcuts 1–4 from server options menu
  - File: internal/tui/views/server/options.go
  - Remove 1–4 mappings; update help overlay accordingly.
  - Purpose: Align with modern UI guidance; reduce confusion.
  - Requirements: R2

- [x] 13. NO_COLOR fallbacks and color utility
  - File: internal/tui/util/color.go
  - Provide color palette and ASCII fallbacks for tags and diffs; never rely on color alone.
  - Purpose: Accessibility and portability across terminals.
  - Requirements: R1, R3, R8

- [x] 14. Unit tests: tags computation
  - File: internal/tui/util/tags_test.go
  - Cases: whitespace-only edit toggle, Trimmed vs Truncated exclusivity, Over Limit math, counters, stable order.
  - Purpose: Ensure correctness and prevent regressions.
  - Requirements: R1, R7

- [x] 15. Reducer tests for toggles/resize
  - File: internal/tui/state/reducers_test.go
  - Cases: ToggleWrap triggers redraw state; ToggleMode toast; Resize boundary across fallback threshold; ToggleView; ScrollLeft/Right; SyncScroll.
  - Purpose: Validate state transitions and wrap bug fix.
  - Requirements: R3, R4, R5

- [x] 16. Snapshot tests: DiffView
  - File: internal/tui/widgets/diff/diff_snapshot_test.go
  - Cases: unified and side-by-side; sticky headers; vertical separator alignment; NO_COLOR rendering; horizontal scroll indicators.
  - Purpose: Guard visual structure and alignment.
  - Requirements: R3, R5, R8

- [x] 17. Integration tests: lists + tags
  - File: internal/tui/views/descriptions/list_integration_test.go
  - Cases: recompute on save; multi-select performance; stability under large lists.
  - Purpose: Verify list-tag pipeline.
  - Requirements: R1, R6, R7

- [x] 18. Documentation updates
  - File: AGENTS.md (appendix) or .spec-workflow/README (if applicable)
  - Document keys, wrap defaults, scrolling keys, fallback threshold, NO_COLOR behavior.
  - Purpose: Make behavior discoverable beyond in-app help.
  - Requirements: R2, R3, R8

