# mcp-launch — one URL per config for many MCP servers (via **mcpo**)

> **Project scope & credits**  
> `mcp-launch` is a thin **wrapper** around existing, excellent projects:
>
> - **[`mcpo`](https://github.com/open-webui/mcpo)** — turns MCP servers into HTTP/OpenAPI with per‑tool routes and docs.
> - **MCP servers** such as **[`Serena`](https://github.com/oraios/serena)**.
> - **Cloudflare Tunnel** — to expose a stable public HTTPS URL without port‑forwarding.
>
> This tool simply **supervises processes**, **merges per‑tool OpenAPI specs** into a single `/openapi.json` **per stack**, and prints the URL(s) to paste into a Custom GPT.  
> It does **not** reimplement MCP or `mcpo`, and it aims to stay minimal and maintainable.
>
> _Not affiliated with or endorsed by OraiOS/Serena, Open WebUI/mcpo, Cloudflare, or OpenAI. All trademarks belong to their respective owners._

`mcp-launch`:
- Starts **mcpo** as a front door for one or more **MCP** servers
- Optionally publishes each stack via **Cloudflare Tunnel**
- Generates a **merged OpenAPI** per stack for a **Custom GPT** Action
- Exposes `/openapi.json` on the same public URL as the API routes per stack

**No wheel‑reinventing.** `mcpo` already supports multi‑server configs and per‑tool OpenAPI + docs; we supervise it, merge the specs, and publish the result.

---

## What’s new (this build)

- **Warn on long descriptions:** detects **tool descriptions > 300 chars**, summarized **per MCP server** (details only with `-v`/`-vv` or in `--log-file`).
- **Per‑server “30+ endpoints” warning:** you still get the overall count, and now also a warning **per server** once it exposes 30+ operations.
- **Optional TUI preflight** (Bubble Tea v1):
  - Edit **Allowed**/**Disallowed** tools (multi‑select)
  - Edit **Tool descriptions** (`+` sets trimmed ≤300; `-` clears)
  - **Disable/Enable** server
  - Choose **launch mode**:
    - **Via mcpo** (HTTP + OpenAPI; your current flow)
    - **Raw MCP** (stdio only; useful for running servers without mcpo)

The TUI inspects tools via MCP’s standard `initialize` → `tools/list` on **stdio**; we don’t reinvent discovery. It clones your configs to `.mcp-launch/tmp/<name>/mcp.config.json` and applies only safe edits (disabling servers) to the clones; description/allow/deny overrides are applied during merge when launching via `mcpo`.

---

## Prerequisites

- **Go 1.22+**
- **mcpo** (`pipx install mcpo` or `uv tool install mcpo`)
- **cloudflared**
- Optional: **uv** (`uvx`) and **node** (`npx`) if your MCP servers use them

---

## Install / Build

```bash
git clone <your fork> mcp-launch && cd mcp-launch
go mod tidy
go build -o mcp-launch
# Windows: go build -o mcp-launch.exe
```

---

## Quick start (dev)

```bash
# 1) Generate default config + state
./mcp-launch init

# 2) Bring everything up using an ephemeral Cloudflare Quick Tunnel
./mcp-launch up --tunnel quick

# 3) Copy the printed URL ending in /openapi.json and the API key
#    (key is also saved in .mcp-launch/state.json; shown by `mcp-launch status`)
```

Per‑tool docs (example):

```
https://<public>/serena/docs
https://<public>/time/docs
...
```

Stop with **Ctrl‑C** (tears down mcpo, spawned MCP servers, the proxy, and cloudflared), or:

```bash
./mcp-launch down
```

---

## TUI preflight (optional)

```bash
./mcp-launch up --tui --tunnel none   # discover→edit→choose launch mode
```

**Keys**
- Server list: `↑/↓`, `Enter` (edit), `c` (continue), `q` (quit)
- Allowed/Disallowed editors: `↑/↓`, `space` (toggle), `enter` (save), `b` (back)
- Descriptions: `↑/↓`, `+` set trimmed (≤300), `-` clear, `enter`/`b` back
- Launch chooser: `1` = mcpo, `2` = raw, `enter` = mcpo, `b` back

> In **raw** mode we launch stdio servers only (no mcpo/OpenAPI/tunnel).  
> In **mcpo** mode everything works as before, plus the new warnings.

---

## Add to a Custom GPT

1. ChatGPT → **Create a GPT** → **Configure** → **Actions**.
2. **Import from URL** → paste the schema URL printed by `up`, e.g.:
   ```
   https://gpt-code.example.com/openapi.json
   ```
3. Auth: **API Key** → header **`X-API-Key`** → paste the key printed by `up` (also in `mcp-launch status`).

---

## Command reference

- `init` — Scaffold `mcp.config.json` and default state at `./.mcp-launch/state.json`.

- `up` — Start one or more **stacks**: mcpo (default `:8800+i`), front proxy (default `:8000+i`), optional Cloudflare Tunnel, and generate merged OpenAPI.
  - Options:
    ```
    --config PATH        Path to Claude-style config (repeatable)
    --port N             Base front proxy port (default: 8000)
    --mcpo-port N        Base mcpo port (default: 8800)
    --api-key KEY        API key (used for all stacks with --shared-key)
    --shared-key         Use one API key for all stacks (default: per-stack random key)
    --tunnel MODE        quick | named | none (default: quick)
    --public-url URL     Public base URL (repeatable; align with --config or single for all)
    --tunnel-name NAME   cloudflared tunnel name (for --tunnel named)
    --tui                Run the TUI preflight (edit & launch mode)
    -v / -vv             Verbose / debug (prints long-description offenders)
    --log-file PATH      Tee all logs to file
    ```

- `status` — Show each stack’s ports, public URL, tools, and API key header.

- `openapi` — Regenerate merged OpenAPI for each running stack.
  - Options:
    ```
    --public-url URL     One per running stack (or one applied to all)
    ```

- `share` — Print `/openapi.json` URL(s) per stack.

- `down` — Stop cloudflared and **mcpo + its child MCP servers** for **all** stacks.

- `doctor` — Check required binaries.

### Output behavior

- **Default:** minimal summary + per‑server one‑liners (if any operation has `description > 300` and/or endpoints ≥ 30).  
  Add `-v`/`-vv` (or `--log-file`) to see **each** offending operation with method/path/tool/length.

---

## Security notes

- **API keys**: by default, **per‑stack random keys**. Use `--shared-key` to reuse one across stacks. All requests must include `X-API-Key: <value>`.
- **Tunnels**: Quick Tunnels are convenient but **ephemeral**; use Named Tunnels for stable URLs.

---

## Troubleshooting

- **Spec import fails** — Ensure the schema URL ends with `/openapi.json` and the spec’s `servers[0].url` is public. Re‑run:
  ```bash
  mcp-launch openapi --public-url https://your-host
  ```
- **Ctrl‑C didn’t stop everything** — We kill each stack’s **mcpo** process group (so its spawned MCP servers too) and cloudflared.

---

## License

MIT
