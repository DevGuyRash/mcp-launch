# Product Overview

## Product Purpose
mcp-launch is a fast, developer‑friendly launcher for Model Context Protocol (MCP) stacks. It discovers and supervises MCP servers (via mcpo), merges their OpenAPI specs, optionally publishes a temporary public URL (Cloudflare Tunnel), and prints a clean “share with ChatGPT” report. A lightweight Bubble Tea TUI runs before launch (“preflight”) to let users tailor which servers/tools are exposed and adjust tool descriptions to stay within model limits.

This initiative elevates the preflight and UX: make status transparent, navigation consistent, editing powerful, and outcomes predictable. The product must feel reliable and delightful for power‑users running multiple configs.

## Target Users
- Power users and agents who orchestrate multiple MCP servers locally.
- Developers iterating quickly on toolchains for ChatGPT Custom GPT Actions/Agents.
- Security‑aware users who want explicit control over exposed tools and descriptions.

Pain points today:
- Silent or unclear preflight failures (e.g., tools/list error) hide servers entirely.
- Exiting/cancelling TUI can still start stacks; crashes can leave processes running.
- Inconsistent navigation (numbers vs arrows; left/right vs up/down).
- No direct editing of tool descriptions; diff view is hard to parse.
- Final terminal report is hard to scan; styling inconsistent across views.

## Key Features
1. Preflight diagnostics: list all configured servers with per‑server status (OK/Failed/HTTP), error details on demand, and counts (tools, long descriptions).
2. Safe control flow: exit/cancel always aborts launch; TUI errors never start processes; clear confirmation before continuing.
3. Consistent navigation: arrows + Enter selection everywhere; Space to toggle; Vim keys (hjkl) as secondary.
4. Rich description management: trim to ≤300, diff preview with highlighting, and a direct editor for overrides.
5. Uniform theming: top status bar, consistent help/hints footer, sectioned layouts, and color tokens for states.
6. Launch picker polish: intuitive up/down selection with descriptions; remembers last choice.
7. Final Results view: a dedicated, styled summary (and copy‑friendly variant) before returning to shell.
8. Observability and logs: verbose modes stream categorized logs; clear error surfaces; redaction of secrets in output.

## Business Objectives
- Reduce launch friction: make preflight decisions fast and safe.
- Improve reliability signals: clear outcomes and actionable errors.
- Increase adoption: an attractive, consistent TUI that feels first‑class, not a bolt‑on.

## Success Metrics
- ≥95% of runs surface all configured servers (even on errors) with accurate status.
- 0 crash‑launch incidents (no stacks start when TUI errors/cancels).
- Task completion time: users reach “Launch” with confidence in < 30 seconds for 3‑config scenarios.
- ≥3 UX issues from the list demonstrably resolved (navigation, edit, diff, summary).

## Product Principles
1. Transparency: never hide a server; show status and let users decide.
2. Safety first: no implicit launch after cancel; explicit confirmation gates.
3. Consistency: one navigation model across all views with discoverable help.
4. Progressive disclosure: show badges at a glance, details on demand.
5. Terminal‑native beauty: aesthetics without sacrificing speed and portability.

## Monitoring & Visibility
- Dashboard type: CLI TUI with a final summary screen.
- Real‑time updates: live streaming of subprocess logs in verbose modes.
- Key metrics displayed: servers detected, tools per server, long‑desc count, public URLs, OpenAPI operation totals, warnings.
- Sharing: rendered “Import to ChatGPT” block and a copy‑stripped plaintext variant.

## Future Vision
- Theming profiles (high‑contrast, dim‑light) and dynamic width handling.
- Optional persistent profiles for per‑server allow/deny and descriptions.
- Extensible panel system (e.g., a log/preview pane).
- Keyboard‑first discoverability (F1 help overlay, “?” anywhere).
