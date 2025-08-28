# Build & Run
- go mod tidy && go build -o mcp-launch
- ./mcp-launch doctor
- ./mcp-launch up --tui --config <cfg1.json> --config <cfg2.json> -vv
- ./mcp-launch status
- ./mcp-launch openapi
- ./mcp-launch down

# Dev helpers
- rg -n "tools/list" internal/
- go test ./...  # (none yet)

# Cloudflared (external)
- cloudflared tunnel --url http://127.0.0.1:8000

# mcpo (external)
- mcpo --port 8800 --api-key <key> --config mcp.config.json