# Requirements Document

## Introduction
Enhance the mcp-launch preflight TUI and launch flow to: (1) reliably surface MCP server status (no false ERR), (2) unify navigation across all screens, (3) add a pre-launch Config Collector so users no longer pass multiple --config flags, (4) provide robust tool-description editing (trim, truncate, multi-select bulk ops, direct edit, and accurate diff with wrap), (5) fix launch mode labeling and tunnel options, (6) deliver a dedicated, modern Final Results view, and (7) ensure tool-name accuracy in merged OpenAPI and allow/deny mapping.

## Alignment with Product Vision
This feature advances the Product Steering goals of transparency, safety, consistency, and terminal‑native polish. It reduces friction to launch, clarifies errors, and produces a high-confidence final summary suitable for sharing to ChatGPT Actions.

## Requirements

### R1 — Reliable Preflight Inspection
**User Story:** As a user, I want preflight to list every configured server with accurate status and discovered tools so that I can decide what to enable before launch.

#### Acceptance Criteria
1. WHEN preflight runs THEN each server row SHALL display status: OK, ERR, or HTTP.
2. IF a server supports tools/list pagination THEN inspector SHALL fetch all pages via nextCursor.
3. IF a server returns “Invalid request parameters” THEN inspector SHALL send minimal params {cursor:null} and surface error text (no crash).
4. WHEN a server errors THEN it SHALL still appear in the list with ERR and details available.

### R2 — Consistent Navigation & Input
**User Story:** As a keyboard-first user, I want all screens to use arrow keys + Enter, with optional numeric shortcuts, so that interaction feels predictable.

#### Acceptance Criteria
1. WHEN on any list/menu THEN ↑/↓ + Enter SHALL select; numbers remain optional.
2. WHEN toggling selections THEN Space SHALL toggle checkboxes consistently.
3. WHEN unsure THEN pressing ? SHALL show a concise help overlay for that screen.

### R3 — Config Collector (Pre-Phase)
**User Story:** As a user, I want to add and manage config files inside the TUI—with path completion and validation—so I don’t need multiple --config flags.

#### Acceptance Criteria
1. WHEN starting TUI with no configs or on “Edit configs” THEN a Config Collector screen SHALL let me add paths with Tab-completion and resolve ~ and env vars.
2. WHEN I add a path THEN the system SHALL convert it to an absolute, readable file or show a validation error.
3. WHEN configs are listed THEN I SHALL reorder and delete entries and set base ports, verbosity, tunnel type (None/Quick/Named), and named tunnel value when needed.

### R4 — Tool Description Management (Trim, Truncate, Edit, Bulk via Checkboxes)
**User Story:** As a user, I want to trim, truncate, bulk-apply to selected tools via checkboxes, or directly edit tool descriptions and preview diffs with wrapping so I can meet ≤300-char limits and understand changes.

#### Acceptance Criteria
1. WHEN selecting a tool THEN +/t SHALL create a ≤300 override with word-boundary trim; - SHALL clear override.
2. WHEN pressing e THEN a textarea editor SHALL open with character count and 300-limit indicator; Enter saves; only current tool is edited (never all).
3. WHEN pressing m THEN the manager SHALL enter multi-select mode with checkboxes; Space toggles per-tool selection; Enter opens an action palette for selected items.
4. WHEN choosing “Trim Selected” THEN only selected tools with RAW >300 SHALL get ≤300 overrides; choosing “Truncate Selected” SHALL hard-cut with ellipsis when word-boundary trimming cannot meet layout constraints.
5. WHEN pressing X THEN all >300 tools on this server SHALL get ≤300 overrides; Shift+X clears all overrides (confirm prompt).
6. WHEN pressing d THEN a diff view SHALL support unified and side-by-side modes with char-level highlights and a wrap toggle; unchanged lines SHALL not show +/- markers; identical content SHALL show “No changes”.
7. Default wrap mode in diff/edit views SHALL be enabled to avoid terminal cutoff; users can toggle off.

### R5 — Launch Mode & Tunnel Options (Correct Mapping)
**User Story:** As a user, I want launch options that do exactly what they say: Local (no tunnel), Cloudflare Quick (trycloudflare URL), or Cloudflare Named (use my tunnel).

#### Acceptance Criteria
1. WHEN choosing “Local” THEN only 127.0.0.1 URLs SHALL be shown; no Quick Tunnel is started.
2. WHEN choosing “Cloudflare Quick” THEN quick tunnels SHALL be started and a trycloudflare.com URL captured or a graceful warning displayed.
3. WHEN choosing “Cloudflare Named” AND I provide a tunnel name THEN named tunnel SHALL be started; base URL remains user-provided DNS; no Quick Tunnel starts.
4. The choice SHALL persist for the next session and be preselected.

### R6 — Final Results View (Modern Summary)
**User Story:** As a user, I want a dedicated results screen after launch showing core information clearly, with an optional logs box, so I can copy and share easily.

#### Acceptance Criteria
1. AFTER launch THEN a Final Results view SHALL show per-instance OpenAPI URL, masked API key, MCP server count, endpoints, and warnings (near/over 30), above a visually distinct bounded logs box region that is hidden by default.
2. WHEN in verbose/stream or pressing l THEN the bounded, scrollable logs box SHALL appear within a border; otherwise hidden.
3. WHEN pressing s THEN a compact copy block SHALL display; o attempts to open URL in default browser (where supported).

### R7 — OpenAPI Merge: Tool-Name Accuracy
**User Story:** As a developer, I want allow/deny filters and operation IDs to consistently reflect true tool names so I avoid mangled paths in the final OpenAPI.

#### Acceptance Criteria
1. WHEN merging THEN tool name SHALL be derived by first path segment; IF x-mcp-tool/x-tool exists THEN prefer it; ELSE IF operationId prefix matches <tool>__ THEN use it.
2. WHEN mapping fails THEN system SHALL warn and default to allowed rather than dropping operations.
3. OperationIds SHALL be <server>__<original or method_path> and $refs SHALL be namespaced only for local components discovered in the original spec.

## Non-Functional Requirements

### Architecture & Modularity
- Keep mcpclient inspector pure (protocol + pagination). TUI remains a pure overlay editor. Merger remains transport-agnostic.

### Performance
- Preflight inspection for 3 configs completes in ≤ 6s on a typical dev machine; TUI remains responsive under 100+ tools.

### Security
- Mask API keys in summaries and logs. No secrets written to disk beyond state files already used.

### Reliability
- Cancel/exit from TUI SHALL never start servers. On any TUI error, no processes launch. Clean shutdown order preserved.

### Usability
- Color fallback (NO_COLOR / dumb). Help overlay on every screen. Consistent keybindings.
