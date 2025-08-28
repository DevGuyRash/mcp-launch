# Requirements Document — TUI Preflight & UX Overhaul

## Introduction
This feature improves mcp-launch’s interactive preflight and end-to-end UX. It ensures every configured MCP server is visible with a clear status, introduces consistent keyboard navigation and a safer control flow (no launch on cancel/error), adds a direct description editor with a usable diff view, polishes the launch picker, and presents a final Results view that’s easier to read and share.

## Alignment with Product Vision
Aligns with product principles: Transparency, Safety first, Consistency, Progressive disclosure, and Terminal‑native beauty. It directly addresses user pain points and business objectives to reduce friction, improve reliability signals, and increase adoption.

## Requirements

### R1 — Preflight shows all configured servers with status
User Story: As a developer, I want preflight to list every server from my configs (even if inspection fails), so that I can understand issues and make informed choices.
Acceptance Criteria:
1. WHEN preflight runs THEN the list SHALL include all servers discovered in the configs.
2. IF a server’s `tools/list` fails THEN the server row SHALL display a visible error badge and a key to view details (e.g., “d” or Enter → details pane).
3. WHEN inspection succeeds THEN the row SHALL show the tool count and long‑description warnings (count) if applicable.

### R2 — Exiting/cancelling the TUI SHALL abort launch
User Story: As a developer, I want a safe way to exit the TUI without side effects, so that I don’t accidentally start servers.
Acceptance Criteria:
1. IF the user presses Esc or chooses Cancel/Back from the top level THEN the launcher SHALL exit without starting any processes.
2. IF the TUI panics or returns an error THEN the launcher SHALL abort and print a clear message; no servers/tunnels/proxies SHALL start.

### R3 — Consistent navigation model across all views
User Story: As a keyboard‑first user, I want uniform navigation, so that I don’t have to re‑learn controls per screen.
Acceptance Criteria:
1. All menus and pickers SHALL use Up/Down (and optional Vim keys j/k) to move; Enter to select/confirm; Space to toggle; Esc/b to go back.
2. The launch mode picker SHALL use Up/Down + Enter (not Left/Right).
3. Each view SHALL show a help footer with available keys.

### R4 — Description management with edit + readable diff
User Story: As a user, I want to trim long descriptions, directly edit overrides, and preview changes clearly, so that I can keep under model limits without losing meaning.
Acceptance Criteria:
1. The Description screen SHALL allow: (a) Trim to ≤300, (b) Clear override, (c) Direct edit of the override text in a textarea.
2. The Diff view SHALL show RAW vs OVERRIDE with line‑level highlighting and clear section headers.
3. Applying edits SHALL update the pending overlay without crashing or losing selection.

### R5 — Launch picker polish + memory
User Story: As a user, I want launch mode selection to feel consistent, so that I can confirm without confusion.
Acceptance Criteria:
1. The picker SHALL be navigable with Up/Down + Enter to confirm.
2. The last chosen launch mode SHALL pre‑select on the next run.

### R6 — Final Results view
User Story: As a user, I want a styled, scannable final summary before returning to the shell, so that I can copy/share essentials easily.
Acceptance Criteria:
1. AFTER launch succeeds THEN show a Results view with: per‑stack URL, API key, MCP server counts, OpenAPI operation totals, and warnings.
2. Provide a copy‑friendly block (mono, aligned) and colored variant in the TUI.
3. Provide an option to immediately regenerate and display detailed long‑description warnings when `-v/-vv` is active.

## Non-Functional Requirements
### Code Architecture and Modularity
- SRP: inspector, TUI, and merger remain separate; main.go handles launch lifecycle.
- Clear overlay contract: TUI returns either an overlay or nil to indicate “abort”.
- No hidden side effects in rendering logic; reducers control state changes.

### Performance
- Preflight with 3 configs SHALL complete in ≤ 5 seconds on a typical dev laptop.
- TUI rendering SHALL remain responsive with 100+ tools.

### Security
- Redact secrets (API keys) from logs and on-screen when not explicitly required.

### Reliability
- 0 crash‑launch incidents across regression runs.
- Graceful handling of inspection failures with actionable error messages.

### Usability
- Consistent keybinding model and visible help footer on all screens.
- Color tokens chosen for legibility in light/dark terminals; fallback when color is unavailable.
