# Suggested Commands

- Build and run TUI:
  - `go mod tidy && go build -o mcp-launch`
  - `./mcp-launch up --tui -vv --config "mcp_configs/mcp.chatgpt.spec-workflow.json" --config "mcp_configs/mcp.chatgpt.utils.json" --config "mcp_configs/mcp.serena.json"`

- Adjust init timeout (slow environments / cold npx caches):
  - `export MCP_INIT_TIMEOUT_SEC=30`

- Quick newline-JSON handshake test (spec-workflow):
  - `printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' | npx -y @pimzino/spec-workflow-mcp@latest . --AutoStartDashboard`

- Quick newline-JSON handshake test (repomix):
  - `printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' | npx -y repomix@latest --mcp`

- Notes:
  - Run in an interactive terminal to avoid `/dev/tty` errors from the TUI.
  - If npx cache issues occur, clear `~/.npm/_npx` or use isolated cache: `npx --cache /tmp/npx-$$ ...`.
