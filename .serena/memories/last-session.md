# Last Session Summary

- Fixed build error by adding a local `mapsClone` helper used to duplicate JSON-RPC envelopes before setting `id`.
- Removed mixed framing; reverted MCP client handshake to newline-delimited JSON only. Single `initialize(id:1)`; send only `notifications/initialized`.
- Implemented fast–slow initialize wait: 6s fast window; on timeout only, a single fallback window controlled by `MCP_INIT_TIMEOUT_SEC` (default 20s). This addresses cold `npx` and dashboard-startup delays without server-specific tweaks.
- Updated `AGENTS.md`:
  - Removed repomix troubleshooting section.
  - Added “Startup and Timeout Policy (MCP)” and enhanced guardrails in the incident report to prevent regressions.
- Verified spec-workflow newline handshake works; noted TUI requires a TTY. Repomix issues traced to `npx` cache/environment, not client framing.
