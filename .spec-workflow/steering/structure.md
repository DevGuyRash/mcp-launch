# Project Structure

## Directory Organization
```
project-root/
├── internal/
│   ├── cloudflare/         # Quick/named tunnel runners
│   ├── config/             # Claude-style config loader
│   ├── httpx/              # HTTP helpers (timeouts, readiness)
│   ├── mcpclient/          # MCP stdio/HTTP inspector (initialize, tools/list)
│   ├── merger/             # OpenAPI merger/normalizer
│   ├── ports/              # free-port discovery
│   ├── proc/               # process supervisor utilities
│   └── tui/                # Bubble Tea TUI (preflight)
├── main.go                 # CLI entry and commands (up/status/openapi/down)
├── mcp.config.json         # Example config
└── .spec-workflow/         # Steering/spec docs (generated)
```

## Naming Conventions
- Packages: lower_snake (Go standard: short, descriptive)
- Files: lower_snake.go (group by package responsibility)
- Exports: follow Go style (export only when needed)

## Import Patterns
1. Standard library first
2. Third‑party libraries
3. Internal packages (`mcp-launch/internal/...`)

## Code Structure Patterns
- Each internal subpackage owns a cohesive concern (SRP):
  - `mcpclient`: JSON‑RPC protocol specifics; no UI.
  - `tui`: UI state machine and rendering only; no process launch logic.
  - `merger`: pure OpenAPI transforms; no network except fetching specs.
  - `proc`: process orchestration primitives used by `main`.
- `main.go` wires the pieces together and handles CLI flags and lifecycle.

## Module Boundaries
- UI ↔ Core: `tui` returns a pure overlay value; `main` decides to launch or abort.
- Inspector ↔ TUI: inspector returns summaries/errors; TUI only displays and edits selections.
- Merger ↔ Network: fetch OpenAPI through front proxy; merger remains agnostic of transport details.

## Code Size Guidelines
- Files: target ≤ 400 lines where practical; split views/state if larger.
- Functions: aim ≤ 60 lines; extract helpers for clarity.
- Nesting: prefer early returns; avoid > 3 levels of nesting in TUI reducers.

## Documentation Standards
- Top‑level README: usage and flags kept current.
- Package doc comments for each internal package.
- Complex flows (e.g., OpenAPI ref rewriting) include short rationale comments.
