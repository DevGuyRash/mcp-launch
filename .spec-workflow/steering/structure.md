# Structure Steering Document: mcp-launch

## Layout
```
internal/
  cloudflare/   # quick & named tunnel helpers
  config/       # Claude-style mcp.config loader
  httpx/        # HTTP helpers and readiness
  mcpclient/    # MCP inspector (stdio + HTTP placeholder)
  merger/       # OpenAPI merger/normalizer
  ports/        # free-port discovery
  proc/         # supervisor and process mgmt
  tui/          # Bubble Tea TUI preflight
main.go         # CLI commands: up/status/openapi/down
```

## Boundaries
- `tui` returns overlay only; no starting processes in UI
- `mcpclient` does protocol, timeouts, stderr draining; no UI
- `merger` is pure data transforms; fetch inputs via HTTP client provided in main

## File Size & Style
- Aim ≤ 400 LOC per file; extract helpers for rendering/diff logic
- Reducers (Update) keep small switch blocks per mode; push helpers
- Keep rendering in `View*` helpers; no side effects

## Testing Targets
- `mcpclient`: pagination, invalid-params regression
- `merger`: tool-name mapping & refs rewriting
- `tui`: selection reducers + diff widget snapshots

## Docs
- README usage remains authoritative for CLI
- Steering/spec docs under `.spec-workflow/`
