# Project Purpose

`mcp-launch` is a minimal supervisor with a Bubble Tea TUI to:
- Inspect and launch Model Context Protocol (MCP) stacks using `mcpo`.
- Optionally publish launched stacks via Cloudflare.
- Provide preflight checks and an interactive TUI-driven launch experience.

The client speaks newline-delimited JSON over stdio to MCP servers, following strict guardrails to maximize compatibility and avoid regressions.
Recent resilience improvements include a fast–slow initialize wait strategy with an environment override for cold/slow starts.
