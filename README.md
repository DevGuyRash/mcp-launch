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

## Why multi‑config? (OpenAI ~30 tools per Action)

Custom GPT Actions currently handle roughly **30 tools**. If your MCP servers exceed that, **split** them across multiple config files. `mcp-launch` can boot **multiple stacks** at once (one per config), each with its own `/openapi.json` and API key, so you can import them into **separate GPTs/chats**.

Examples:

```bash
# Two stacks via Quick Tunnels (independent URLs + keys):
mcp-launch up \
  --config code.json \
  --config data.json \
  --tunnel quick

# Two named stacks with stable hosts and a single shared API key:
mcp-launch up \
  --config code.json --public-url https://gpt-code.example.com \
  --config data.json --public-url https://gpt-data.example.com \
  --tunnel named --tunnel-name my-tunnel \
  --api-key SECRET --shared-key
```

---

## How it works

```text
                 spawns/loads (stdio/SSE/HTTP)             HTTP (OpenAPI + tools)
+------------+  --------------------------------------->   +-------------+
| mcp-launch |                                             |  mcpo :8800 |
+------------+                                             +-------------+
      |
      | front proxy :8000.. :8000+N-1
      |   - serves /openapi.json (per stack)
      |   - proxies everything else to that stack's mcpo
      v
+-------------+            public HTTPS (per stack)
| cloudflared | ---------------------------------------> https://host-A
+-------------+                                          https://host-B
                                                         ...
mcpo connects to MCP servers defined in each mcp.config.json:

+-------------+   +--------------+   +-----------+    ...
|  Serena MCP |   |  Filesystem  |   |  Time MCP |
+-------------+   +--------------+   +-----------+
```

- `mcpo` exposes each MCP server at `/<name>` with docs at `/<name>/docs`.
- The front proxy **serves `/openapi.json`** on the **same host/port** it proxies to `mcpo`.
- With Cloudflare (Quick or Named), you get a public HTTPS URL **per stack** to share with ChatGPT.

---

## Prerequisites

- **Go 1.22+**
- **mcpo** (Python; `pipx install mcpo` or `uv tool install mcpo`)
- **cloudflared**
- Optional: **uv** (`uvx`) and **node** (`npx`) if your MCP servers use them

---

## Install / Build

```bash
git clone <your fork> mcp-launch && cd mcp-launch
go build -o mcp-launch
# Windows: go build -o mcp-launch.exe
```

---

## Quick start (dev)

```bash
# 1) Generate a default Claude-style config + state
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

## Stable setup (prod)

Use a **Named Tunnel** for a stable domain you own.

```bash
./mcp-launch up --tunnel named \
  --public-url https://gpt-tools.example.com \
  --tunnel-name <YOUR_TUNNEL_NAME>
```

Paste **`https://gpt-tools.example.com/openapi.json`** into ChatGPT.

---

## Add to a Custom GPT

1. ChatGPT → **Create a GPT** → **Configure** → **Actions**.
2. **Import from URL** → paste the schema URL printed by `up`, e.g.:
   ```
   https://gpt-code.example.com/openapi.json
   ```
3. Auth: **API Key** → header **`X-API-Key`** → paste the key printed by `up` (also in `mcp-launch status`).

Repeat for each stack if you split configs.

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
    -v                   Verbose (INFO) and stream subprocess logs
    -vv                  Debug (DEBUG) and stream subprocess logs
    --stream             Stream subprocess logs without changing verbosity
    --log-file PATH      Append logs to file (created if missing)
    ```

- `status` — Show each stack’s ports, public URL, tools, and API key header.

- `openapi` — Regenerate merged OpenAPI for each running stack.
  - Options:
    ```
    --public-url URL     Repeatable; one per running stack (or one applied to all)
    ```

- `share` — Print `/openapi.json` URL(s) per stack for easy copy/paste.

- `down` — Stop cloudflared and **mcpo + its child MCP servers** for **all** stacks.

- `doctor` — Check required binaries.

### Default output vs verbose

- **Default:** only essentials → per‑stack schema URL(s) and `X‑API‑Key` values. No log spam.
- **`-v` / `-vv`:** stream `mcpo`/`cloudflared` logs and print extra details like chosen ports.
- **`--log-file`:** always captures *everything* (our messages + subprocess output), regardless of verbosity.

---

## Configuration reference (`mcp.config.json`)

```json
{
  "mcpServers": {
    "serena": {
      "command": "uvx",
      "args": [
        "--from", "git+https://github.com/oraios/serena",
        "serena", "start-mcp-server",
        "--context", "ide-assistant"
      ]
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]
    },
    "time": {
      "command": "uvx",
      "args": ["mcp-server-time", "--local-timezone=America/Phoenix"]
    }
  }
}
```

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
- **Noisy console** — Only with `-v`/`-vv`/`--stream`. Default output is minimal. Use `--log-file` to capture details without cluttering the terminal.
- **Ctrl‑C didn’t stop everything** — Fixed: we kill each stack’s **mcpo** process group (so its spawned MCP servers too) and cloudflared.

---

## License

MIT
