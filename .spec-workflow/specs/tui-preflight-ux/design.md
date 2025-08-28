# Design Document — TUI Preflight & UX Overhaul

## Overview
Elevate the preflight and run UX of mcp-launch. The TUI must always surface every configured MCP server with an explicit status (OK/ERR/HTTP), provide consistent navigation (Up/Down/Enter/Space/Esc), enable description trimming and direct editing with a clearer diff, make the launch mode picker consistent, and present a final Results view before returning to the shell. Safety is paramount: cancelling or errors must never start processes.

## Steering Document Alignment
### Technical Standards (tech.md)
- Keep SRP boundaries: `tui` (UI state/render) stays pure and returns overlay or nil; `main` owns lifecycle decisions; `mcpclient`/`merger` remain headless.
- Namespaced OpenAPI refs and op-count warnings remain unchanged; we only adjust UX.
- Abort‑on‑cancel becomes a hard invariant; redact API keys everywhere.

### Project Structure (structure.md)
- Preserve package boundaries: no process control in `tui`; no UI code in `mcpclient`/`merger`.
- Split large views if needed to keep files readable; use helpers for key handling.

## Code Reuse Analysis
### Existing Components to Leverage
- `internal/mcpclient`: continue to perform `initialize` → `tools/list` probing; capture error text if any.
- `internal/tui`: reuse model structure, styles, and list/select views as a base.
- `main.go`: reuse signal handling, state persistence, and merge pipeline.

### Integration Points
- Preflight passes a complete map `<inst>/<srv> → []Tool` PLUS an error map for failed inspections so the TUI can display all servers regardless of tool discovery.
- TUI returns overlay or nil; `cmdUp` treats nil as abort and prints a single line “Cancelled — no servers launched”.

## Architecture
- Add a preflight aggregation struct:
  - `ByComposite map[string][]Tool` (existing)
  - `Status map[string]ServerStatus` (new)
  - `Error map[string]string` (new)
- `ServerStatus` enum: `OK`, `ERR`, `HTTP` (for future streamable-http detection) — default `OK` when tools were listed.
- TUI consumes these maps to render status badges; details view reads from `Error`.
- Editor subsystem: a simple textarea buffer (single-line wrap with scrolling) used only in `modeDescEdit`; diff view shows RAW vs OVERRIDE with minimal highlight (prefix +/- markers and faint style for unchanged context).
- Results view: a transient screen rendered by `cmdUp` after starting stacks; not a Bubble Tea program (keeps responsibilities clear) but styled, aligned output.

### State Machine Additions (tui)
- Views: `modeList`, `modeMenu`, `modeAllow`, `modeDeny`, `modeDesc`, `modeDiff`, `modeLaunch`, `modeDescEdit` (new).
- Keymap (all views): Up/Down (j/k), Enter (confirm), Space (toggle where applicable), Esc/b (back), `?` (help overlay, optional), `d` (diff), `e` (edit description where applicable).
- Launch picker: Up/Down + Enter; remembers previous selection in overlay seed (persisted externally in `.mcp-launch/overrides.json`).

## Components and Interfaces
### Preflight Aggregator (main)
- Purpose: enumerate config servers deterministically, attempt inspection, record status and error text without filtering out failures.
- Interfaces: returns `servers map[string][]Tool`, `status map[string]ServerStatus`, `errs map[string]string` to TUI.
- Dependencies: `config.Load`, `mcpclient.InspectServer`.

### TUI Model Enhancements (`internal/tui`)
- Purpose: render lists with status badges, offer details, allow/deny selection, description edit, diff, and launch mode selection.
- Interfaces: `Run(servers, seed, status, errs) (*Overlay, string, error)` — return overlay or nil and launch mode.
- Dependencies: lipgloss styles; no process or file IO.

### Description Editor
- Purpose: allow direct override editing for a selected tool.
- Interfaces: transitions from `modeDesc` via `e`; maintains an internal buffer; `Enter` saves to `overlay.Descriptions[curServer][tool]`; `Esc` discards.
- Dependencies: none beyond Bubble Tea.

### Results View (main)
- Purpose: present a styled summary after stacks are up (URL, API key, tool counts, op totals, warnings). Provide a copy‑friendly block.
- Interfaces: inline renderer before waiting for signals; respects `-v/-vv` for detailed long‑description warnings.

## Data Models
- `type ServerStatus int { OK, ERR, HTTP }` where `ERR` triggers a tag and details key.
- Extend TUI model with:
  - `status map[string]ServerStatus`
  - `errors map[string]string`
  - `lastLaunch string`
  - `editBuffer string` for `modeDescEdit`.

## Error Handling
### Scenario 1: `tools/list` fails
- Handling: mark server `ERR`, store error text, show badge and allow viewing details; tool list may be empty.
- User Impact: server remains visible; user can proceed (launch) or disable.

### Scenario 2: TUI panic/error
- Handling: `Run` returns error or overlay=nil; `cmdUp` aborts launch and prints a single‑line message.
- User Impact: safe abort; nothing starts.

### Scenario 3: Quick tunnel fails
- Handling: show warning in final Results; continue without public URL.
- User Impact: local URLs remain; optionally rerun with named/none.

## Testing Strategy
### Unit Testing
- `toolNameFromRawPath`, `countHTTPMethods`, and ref rewriter stay covered; add tests for `trim300` behavior on boundaries.
- Add small reducer tests for TUI selection toggling and `modeDescEdit` save/cancel transitions (headless). 

### Integration Testing
- Simulate preflight with one OK and one ERR server → verify TUI renders both (status badges present).
- Verify overlay translation preserves allow/deny/desc and that nil aborts launch.

### End-to-End
- Manual run with 3 configs under `-vv`: confirm consistent navigation and Results view readability.
- Validate that cancel from top-level list ends without launching any processes.

## Appendix A — Research Notes: just‑every/code TUI (inspiration)
- Persistent header and status hints: The fork highlights a simple, always‑visible interface with a clear header and consistent hints and panels (images and feature list). We’ll mirror a small top status bar plus a consistent footer with context‑sensitive key help. Source: README features and screenshots (Simple interface; Multi‑Agent panels; Theme system). [License: Apache‑2.0].
- Unified diffs with syntax highlighting: Code showcases a side‑by‑side diff viewer. We won’t implement a full sxs diff now, but we’ll improve our single‑column diff readability (± markers, color accents) and leave sxs as a future item. Source: README “Diff Viewer”.
- Theme system and accessibility: Code exposes a theme command with live preview and an explicit accessibility focus. We’ll define color tokens with good contrast and a no‑color fallback; full theme switching can be a later iteration. Source: README “Theme System”.
- Multi‑pane/agent panels: The UI organizes complex tasks into panels (e.g., /plan, /solve, /code). For mcp-launch we’ll keep preflight single‑focused but allow an optional details pane (error details, long‑desc list) activated on demand. Source: README “Multi‑Agent Commands”.
- Browser integration panels: While not needed here, the principle of auxiliary panels for external context is useful. We can optionally surface a right‑hand details pane for logs/warnings without leaving the list. Source: README “Browser Integration”.
- Safety/approvals affordances: The fork emphasizes safety modes and approvals; we align by making cancel/abort explicit and preventing accidental launch after errors. Source: README “Safety modes”.

Guardrails:
- Pattern‑level inspiration only; no code copying. just‑every/code is Apache‑2.0; our work remains original and Go/Bubble Tea‑native.
- We keep `tui` pure and avoid coupling to process control; focus on layout, keymaps, and readability.

References:
- just‑every/code README (features, screenshots, theme and panel concepts).