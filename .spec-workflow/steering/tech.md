# Technology Stack

## Project Type
Terminal-first CLI + TUI supervisor for MCP servers that also serves a merged OpenAPI over HTTP and optionally a Cloudflare Tunnel. Single binary, no daemon.

## Core Technologies
### Primary Language(s)
- Language: Go (go 1.23 with toolchain 1.24.6)
- Build: `go build` (no CGO required)

### Key Dependencies/Libraries
- Bubble Tea (TUI framework): interactive state machine for preflight UI.
- Lipgloss (styles): consistent color and layout styling.
- net/http + ReverseProxy: front proxy to mcpo.
- Cloudflared (external process): optional public URL via “quick” or named tunnels.

### Application Architecture
- Process Orchestrator: starts mcpo, a front HTTP proxy, and (optionally) cloudflared per config stack.
- Preflight Inspector: for each config, probes servers via MCP JSON‑RPC (stdio) `initialize` → `tools/list` to enumerate tools; degrades gracefully on errors.
- TUI Preflight: a thin state machine around Bubble Tea rendering multiple views (list/menu/allow/deny/desc/diff/launch) and yielding an overlay (allow/deny/desc/disabled) or nil.
- OpenAPI Merger: fetch per‑server OpenAPI from mcpo, normalize, namespace, and join into a single spec with per‑tool description overrides and safety cleanups.

### External Integrations
- MCP servers launched downstream by mcpo.
- Cloudflared CLI (local install) for public URLs (Quick Tunnel or named).
- ChatGPT Custom GPT “Import from URL” expects a single OpenAPI with API‑key header.

## Development Environment
### Build & Development Tools
- Build: `go mod tidy && go build -o mcp-launch`
- Verbose modes: `-v` (INFO) and `-vv` (DEBUG) to stream subprocess logs.

### Code Quality Tools
- Standard Go tooling; no custom linters configured yet.
- Future: minimal unit tests for merger / mcpclient and TUI reducers.

### Version Control & Collaboration
- Git repository; conventional commits recommended for future changes.

## Deployment & Distribution
- Target Platforms: Linux/macOS/Windows terminals.
- Distribution: direct binary download; no installer required.

## Technical Requirements & Constraints
### Performance Requirements
- Preflight should complete in ≤ 5s for 3 configs on a typical developer machine.
- TUI must remain responsive under large tool lists (100+ tools total).

### Compatibility Requirements
- Terminals with ANSI color support (fallback to reduced styling if not available).
- No reliance on GPU/GUI; pure terminal.

### Security & Compliance
- Redact API keys in logs/console output.
- No external telemetry.

### Scalability & Reliability
- Multiple independent “stacks” (one per `--config`), each with isolated ports.
- Clean shutdown on Ctrl‑C: proxies then cloudflared then mcpo process groups.

## Technical Decisions & Rationale
1. Bubble Tea + Lipgloss: proven ecosystem for ergonomic TUIs in Go; portable across terminals.
2. “List everything” preflight: servers that error must still appear with status so the user can fix or choose to proceed.
3. Abort‑on‑cancel: TUI returns nil → launcher aborts; never launch on error or cancel.
4. Namespacing refs in OpenAPI: avoids component collisions across servers.
5. Operation counting and warnings: surface near‑30 and >30 endpoints to fit ChatGPT Action limits.

## Known Limitations
- Diff highlighting is line‑level; no semantic diff yet.
- No built‑in theme switching; styles are constants for now.
- Limited text editing (no multiline textarea) in current TUI; planned.
