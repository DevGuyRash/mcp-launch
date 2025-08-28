# Technical Steering Document: mcp-launch

## Stack
- Language: Go 1.23 (toolchain 1.24.6)
- UI: Bubble Tea + Lipgloss
- HTTP: net/http + ReverseProxy
- Process: exec and pgid control (Unix), Windows taskkill fallback
- Optional: Cloudflared CLI for Quick/Named tunnels

## Architecture
- main: CLI & lifecycle orchestration
- internal/mcpclient: MCP stdio inspector (initialize → tools/list with pagination)
- internal/tui: preflight editor, pure overlay output
- internal/merger: OpenAPI merge with component namespacing and path prefixing
- internal/proc: process supervision and logging

## Conventions
- SRP by package; no cross-coupling of UI and process logic
- Terminal color fallback via NO_COLOR or dumb terminals
- Secrets (API keys) masked in logs and summaries

## Key Decisions
- Inspector must list tools with cursor pagination and never block stderr
- TUI uses arrow/enter/space consistently; “?” shows help everywhere
- Diff view supports unified and side‑by‑side with char‑level highlights
- Merge step derives tool name by first path segment, or x-mcp-tool, or operationId prefix
- Tunnel modes: local/quick/named; labels must match behavior

## Constraints
- No CGO; fast startup; minimal dependencies
- Keep terminal responsiveness under large tool lists

## Quality
- Unit tests for inspector pagination and mapping helpers
- Snapshot tests for TUI reducers and diff rendering
- Graceful shutdown ordering: proxy → cloudflared → mcpo group
