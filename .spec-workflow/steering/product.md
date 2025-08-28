# Product Steering Document: mcp-launch

## Vision
mcp-launch provides a fast, developer‑friendly way to start, inspect, and share multiple MCP stacks. It should make preflight decisions safe and transparent, generate a clean merged OpenAPI per stack, and optionally publish via Cloudflare tunnels—all with a polished, consistent terminal UI.

## Goals
- Smooth preflight flow that always surfaces server status and errors.
- Consistent TUI navigation and aesthetics (keyboard-first, discoverable).
- Powerful yet simple tool description management to meet model token/limit constraints.
- Clear final summary with copy‑ready details for ChatGPT Actions import.
- Reliable process orchestration, clean shutdown, and safe defaults.

## Non‑Goals
- Replace mcpo or reimplement server protocols beyond the inspector.
- Full-blown file browser or external GUI; remain terminal-first.

## Users & Outcomes
- Power users orchestrating multiple configs quickly.
- Developers iterating on toolchains; want predictable OpenAPI and tunnel behavior.
- Outcome: Reduced friction and errors; faster path to a working, shareable stack.

## Success Metrics
- ≥95% accurate preflight server visibility.
- 0 implicit launches after cancel; explicit confirmation gates only.
- <30s median time to launch for 3‑config scenario.

## Principles
- Transparency, Safety First, Consistency, Progressive Disclosure, Terminal‑native Beauty.
