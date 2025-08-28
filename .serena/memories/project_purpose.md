mcp-launch: a minimal supervisor/launcher for Model Context Protocol (MCP) servers that
- runs mcpo per config, merges exported OpenAPI specs behind a single front endpoint, and (optionally) exposes a Cloudflare Tunnel URL
- includes a Bubble Tea TUI preflight allowing per-server enable/disable, per-tool allow/deny, and tool description overrides (<=300 chars) before launching
- persists state/overrides and prints a concise “share with ChatGPT” report