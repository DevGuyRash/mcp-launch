# Tasks — TUI Preflight & UX Overhaul

- [x] 1. Preflight aggregation: capture server status and errors
  - Files: main.go (cmdUp preflight loop)
  - Add maps: `status map[string]ServerStatus`, `errs map[string]string` keyed by `<inst>/<srv>`
  - Rule: if `InspectServer` returns error, set `status[key]=ERR`, `errs[key]=err.Error()`, still include `key` in servers map (possibly with empty tools)
  - Purpose: R1 visibility of all servers, even when `tools/list` fails
  - Leverage: internal/mcpclient.InspectServer, internal/config
  - Requirements: R1

- [x] 2. Pass status/errs into TUI
  - Files: internal/tui/tui.go (signature), main.go (appTUI.Run call site)
  - Change signature to `Run(servers map[string][]Tool, seed *Overlay, status map[string]ServerStatus, errs map[string]string) (*Overlay, string, error)`
  - Purpose: TUI can render badges and details
  - Requirements: R1

- [x] 3. Status badges + details view
  - Files: internal/tui/tui.go (model fields, viewList, viewMenu)
  - Add model fields: `status map[string]ServerStatus`, `errors map[string]string`
  - Show tag `ERR` (warn color) when `status[key]==ERR`; press `d` or `enter` on a server to view details pane showing captured error text
  - Purpose: Visible, actionable preflight errors
  - Requirements: R1

- [x] 4. Safe abort on cancel/error
  - Files: internal/tui/tui.go (Update returns), main.go (cmdUp)
  - Ensure `q/ctrl+c` and top-level `esc/b` set `overlay=nil` and quit
  - In cmdUp: if `ovComposite==nil` then print "Cancelled — no servers launched" and return early (no stacks started)
  - Purpose: R2 safety
  - Requirements: R2

- [x] 5. Unify navigation across all views
  - Files: internal/tui/tui.go (key handling in all modes)
  - Up/Down and j/k to move; Enter confirm; Space toggle; Esc/b back; `?` opens key help overlay
  - Launch picker: switch to Up/Down + Enter (remove Left/Right dependency)
  - Update all view footers to reflect unified keys
  - Purpose: R3 consistency
  - Requirements: R3

- [x] 6. Description edit (textarea)
  - Files: internal/tui/tui.go
  - New mode: `modeDescEdit`; new field `editBuffer string`
  - From `modeDesc`, key `e` opens editor with current override (or raw text trimmed to 300 as initial suggestion)
  - Keys: text input, Enter=save to `overlay.Descriptions[curServer][tool]`, Esc=cancel; preserve selection
  - Optional: use a minimal in-house text input; later consider `bubbles/textinput` if desired
  - Purpose: R4 direct editing
  - Requirements: R4

- [x] 7. Diff view readability
  - Files: internal/tui/tui.go (viewDiff)
  - Add simple line-level markers: prefix removed with `- ` (warn color), added with `+ ` (accent), unchanged faint; show RAW then OVERRIDE with clear headers
  - Purpose: R4 readable diff
  - Requirements: R4

- [x] 8. Launch picker persistence
  - Files: internal/tui/tui.go (model field lastLaunch), .mcp-launch/overrides.json write path already exists in main.go
  - Remember last chosen `launch` in overlay seed or separate small state; pre-select on next run
  - Purpose: R5 memory/polish
  - Requirements: R5

- [x] 9. Final Results view (styled summary)
  - Files: main.go (cmdUp results print section)
  - Replace plain console block with an aligned, styled summary and a copy-friendly mono block; include per-stack URL, API key, server counts, op totals, long-desc warnings
  - When `-v/-vv`, include per-server long-desc tool names
  - Purpose: R6 readability and copy/share
  - Requirements: R6

- [x] 10. Help overlay and status bar
  - Files: internal/tui/tui.go
  - Add `?` to toggle a help overlay listing key bindings; add a persistent top status bar (title + selected instance/server + hint area)
  - Purpose: Discoverability and parity with modern TUI UX
  - Requirements: R3 (extended)

- [x] 11. Panic hardening
  - Files: internal/tui/tui.go
  - Audit dereferences in views to ensure `overlay` and `curTools` are non-nil; add guards in `prepareToolEditor` and menu views; add default return paths
  - Purpose: avoid nil deref seen in stack trace
  - Requirements: Reliability NF

- [x] 12. Color/contrast fallback
  - Files: internal/tui/tui.go (styles)
  - Ensure readable colors for light/dark; add fallback (no-color) path when term lacks color support
  - Purpose: Usability NF

- [x] 13. Redact secrets everywhere
  - Files: main.go (summary), state printing
  - Ensure API key only printed where explicitly required; never log keys in verbose subprocess streams; mask in summaries if copied
  - Purpose: Security NF

- [x] 14. Manual test matrix
  - Files: docs (optional) or as checklist in PR
  - Scenarios: (a) 3 configs: one OK, one ERR, one long-desc warnings; (b) Cancel from list; (c) Edit/trim/clear description; (d) Launch picker recall; (e) Quick tunnel failure path
  - Purpose: validate acceptance criteria

- [x] 15. Inspiration parity from just‑every/code TUI (pattern‑level, no code copy)
  - Files: internal/tui/tui.go (layout only), docs/INSPIRATION.md (optional)
  - Adopt patterns explicitly inspired by just‑every/code:
    - Persistent top header with session/status info and bottom footer with context‑sensitive hints (aligns with screenshots showing consistent chrome and hints)
    - Readable diff view; side‑by‑side is future work, for now improve single‑column clarity
    - Theme tokens with strong contrast and an optional no‑color mode; future: theme toggle
    - Optional right‑hand details pane (toggle) to show error details or long‑desc lists without leaving list view
    - Consistent keymap (`?` help overlay), and clear focus/selection styling
  - Guardrails: Do not copy code; patterns only. just‑every/code is Apache‑2.0, our implementation stays original and Go/Bubble Tea‑native
  - Requirements: R3, R4, R6
