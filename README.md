# mcp-launch — Inspect and Launch MCP Stacks (with TUI)

`mcp-launch` is a minimal supervisor for Model Context Protocol (MCP) stacks. It helps you:

- Inspect and edit MCP stacks in a friendly Bubble Tea TUI
- Launch stacks via `mcpo` (HTTP + OpenAPI) or RAW MCP (stdio only)
- Optionally publish stacks using Cloudflare Tunnels
- Share a merged `/openapi.json` per stack for Custom GPT Actions

The project is intentionally small and pragmatic: it does not re‑implement MCP or `mcpo`; it just orchestrates processes, merges OpenAPI, and provides a helpful preflight experience.

---

## Features

- TUI preflight (optional):
  - Edit Allowed/Disallowed tools (multi‑select)
  - Edit per‑tool descriptions with helpers to trim (≤300) or clear
  - Toggle servers enabled/disabled for a given run
  - Choose controller: `mcpo` (HTTP + OpenAPI) or `RAW` (stdio only)
- Results view with per‑stack summary, copyable env/URLs, and bounded logs pane
- Warnings for long tool descriptions and large endpoint counts
- Cloudflare Quick or Named tunnels for easy sharing

---

## Prerequisites

- Go 1.22+
- `mcpo` available on PATH (e.g., `pipx install mcpo` or `uv tool install mcpo`)
- `cloudflared` for public URLs (optional)
- Optional: `uvx`/`npx` if your MCP servers use them

---

## Quick Start

```bash
git clone <your fork> mcp-launch && cd mcp-launch
go mod tidy && go build -o mcp-launch

# Launch with the TUI (discover → edit → launch)
./mcp-launch up --tui [--config path ...] [-v|-vv]
```

Notes:
- If `--config` is omitted, the TUI includes a Config Collector. It supports `~` and `$ENV` paths and offers tab suggestions.
- `-v` prints additional preflight details; `-vv` includes debug logs.

### Pre‑made Configs (example)

```bash
go mod tidy && go build -o mcp-launch && ./mcp-launch up \
  --tui \
  --config "mcp_configs/mcp.chatgpt.spec-workflow.json" \
  --config "mcp_configs/mcp.chatgpt.utils.json" \
  --config "mcp_configs/mcp.serena.json"
```

### Non‑TUI Flow

```bash
./mcp-launch init                   # scaffold defaults
./mcp-launch up --tunnel quick      # start stacks + quick tunnel
./mcp-launch status                 # show URLs, tools, and API keys
./mcp-launch share                  # print per‑stack /openapi.json URL
./mcp-launch down                   # stop everything
```

---

## TUI Reference (Keys)

- Server list: `↑/↓` select, `Enter`/`d` details, `c` continue, `g` toggle controller (MCPO/RAW), `?` help
- Descriptions: `e` edit (uses `$VISUAL/$EDITOR` if set), `d` diff (unified/side‑by‑side), `w` wrap toggle, `m` multi‑select
  - In multi‑select: `t` Trim Selected (word‑boundary ≤300), `r` Truncate Selected (hard cut ≤300), `-` Clear Selected
- Results logs: `l` toggle, `j/k` scroll, `Shift+J/K` fast scroll, `w` wrap, `/` search, `n/N` next/prev, `S` save log file
- Tunnel picker: `Local (no tunnel)`, `Cloudflare Quick`, `Cloudflare Named`

---

## Security & Privacy

- API auth: Each stack uses an API key header `X-API-Key` (random per stack by default; `--shared-key` reuses one).
- Tunnels: Quick Tunnels are ephemeral. Use Named Tunnels for stable, shareable URLs.
- Input validation: Treat tool inputs and external data as untrusted; review tool allow/deny lists during preflight.

---

## Troubleshooting

- Spec import: Ensure the Custom GPT imports the schema URL ending in `/openapi.json`.
- Public base URL: If needed, regenerate with `mcp-launch openapi --public-url https://your-host` while stacks are running.
- Cleanup: `Ctrl‑C` tears down spawned processes; `./mcp-launch down` does a clean stop across stacks.

---

## Contributing

Contributions are welcome! Please keep changes focused and incremental. Open an issue to discuss larger ideas.

---

## License

MIT

