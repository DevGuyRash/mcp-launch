# Tasks Document

- [ ] 1. Implement tools/list pagination and minimal params in inspector
  - File: internal/mcpclient/client.go
  - Change: send `tools/list` with `{cursor:null}`; if `result.nextCursor` is non-empty, loop with `{cursor: nextCursor}`; accumulate `Tools`.
  - Preserve stderr draining goroutine; keep 6s timeouts for init/list phases.
  - Purpose: resolve “Invalid request parameters” and retrieve full tool lists.
  - Requirements: R1

- [ ] 2. Preflight status propagation
  - File: main.go (preflight loop)
  - Change: keep server in composite map even on error (already done); ensure `StatusERR` + error text flows into TUI `errs`.
  - Purpose: show all servers with badges OK/ERR/HTTP.
  - Requirements: R1

- [ ] 3. TUI — unify navigation across all screens
  - File: internal/tui/tui.go
  - Change: replace numeric-only menus with list selection items; ensure ↑/↓ + Enter everywhere; Space toggles checkboxes; `?` opens help overlay in each mode.
  - Purpose: consistent input UX.
  - Requirements: R2

- [ ] 4. TUI — Config Collector screen
  - File: internal/tui/tui.go (new mode `config`)
  - Add: path input with Tab completion (fuzzy suggestions), `~` and env var resolution, validation; list with delete/reorder; settings for base ports, verbosity, tunnel type (`none|quick|named`), and named tunnel value.
  - Purpose: eliminate need for multiple `--config` flags; gather settings pre-launch.
  - Requirements: R3

- [ ] 5. TUI — Description manager per-tool editor and badges
  - File: internal/tui/tui.go
  - Add: `e` multiline textarea with char count and ≤300 indicator; `+`/`t` trim current; `-` clear; badges RAW n>300 / OVR ≤300 / OVR.
  - Purpose: manage and visualize per-tool overrides.
  - Requirements: R4

- [ ] 6. TUI — Descriptions multi-select mode + bulk operations
  - File: internal/tui/tui.go
  - Add: press `m` to enter multi-select (checkboxes on tools); Space toggles; `Enter` opens an action palette with: Trim Selected, Truncate Selected, Clear Overrides Selected (with confirm for clear). Implement both safe trim (word boundary) and hard truncate with ellipsis.
  - Purpose: mass trimming/truncating for selected tools; ensure `e` continues to edit only the active tool.
  - Requirements: R4

- [ ] 7. TUI — DiffWidget (unified/side-by-side with char-level; wrap toggle, default wrap on)
  - File: internal/tui/diff.go (new)
  - Add: pure functions `renderUnified` and `renderSideBySide` using diffmatchpatch; `w` toggles wrapping; show +/- only on changed lines; unchanged faint; identical → “No changes”.
  - Purpose: accurate, readable diffs.
  - Requirements: R4

- [ ] 8. Launch Mode remap & persistence
  - File: internal/tui/tui.go (launch picker) and main.go
  - TUI: radio selects `none|quick|named`; persist to `overlay.LastLaunch`.
  - main.go: honor `none` (no tunnel), `quick` (startQuickTunnel), `named` (startNamedTunnel only); ensure labels and behavior match.
  - Purpose: fix inverted options and allow named tunnel UX.
  - Requirements: R5

- [ ] 9. Final Results view with optional bounded logs box
  - File: internal/tui/tui.go (new mode `results`)
  - Add: modern, sectioned summary; copy block (`s`); toggle logs box (`l`) with clear border; keep stdout summary for non-TTY.
  - Purpose: polished end-state with core info and optional logs area.
  - Requirements: R6

- [ ] 10. OpenAPI merge — tool-name heuristic and warnings
  - File: internal/merger/merger.go (add helper)
  - Add: tool derivation order: first path segment → x-mcp-tool/x-tool → operationId prefix `<tool>__`; warn and allow if unresolved; keep `<server>__` prefix for operationId.
  - Purpose: stable allow/deny and operation naming.
  - Requirements: R7

- [ ] 11. Tests — inspector pagination and regression for invalid params
  - Files: internal/mcpclient/client_test.go (new)
  - Add: table-driven tests simulating nextCursor; ensure accumulation and error surfacing; cover minimal params.
  - Purpose: guard against regressions.
  - Requirements: R1

- [ ] 12. Tests — merger mapping
  - Files: internal/merger/merger_test.go (new)
  - Add: cases for first-segment, x-mcp-tool, operationId prefix, and unresolved mapping path.
  - Purpose: verify tool-name accuracy.
  - Requirements: R7

- [ ] 13. Tests — TUI reducers & diff snapshots
  - Files: internal/tui/tui_reducer_test.go, internal/tui/diff_test.go (new)
  - Add: golden snapshots for diff rendering (unified/sxs, wrap on/off); reducer tests for multi-select bulk trim/truncate and navigation consistency.
  - Purpose: prevent UI regressions.
  - Requirements: R2, R4, R6

- [ ] 14. Docs — README updates (Config Collector, Results view, tunnel modes)
  - File: README.md
  - Add: section on Config Collector and Results screen; clarify tunnel modes and mapping.
  - Purpose: align docs with UX.
  - Requirements: R3, R5, R6

- [ ] 15. Polishing — color tokens & NO_COLOR fallback
  - File: internal/tui/tui.go
  - Ensure consistent styles (badges, borders, selections) across screens; verify fallback in help, diff, results.
  - Purpose: usable in all terminals.
  - Requirements: NFR Usability
