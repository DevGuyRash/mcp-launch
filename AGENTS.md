# AGENTS.md: Foundational Knowledge for LLM Interaction

This document provides foundational information for an AI agent (LLM) to understand and interact with this repository. The goal is to provide evergreen knowledge that remains relevant even if the repository undergoes significant changes.

## 1. Project Overview

This repository, named `mcp-launch`, serves as a minimal supervisor with a Bubble Tea Terminal User Interface (TUI). Its primary function is to:
- Inspect and launch Model Context Protocol (MCP) stacks using `mcpo`.
- Optionally publish these launched stacks over Cloudflare.

The project is developed using **Go**.

## 2. Setup and Launch

To get the `mcp-launch` application up and running, follow these steps:

### 2.1. Build the Application

The project is a Go application. To build the executable, use the standard Go build commands:

```bash
go mod tidy && go build -o mcp-launch
```

This command will resolve dependencies and compile the source code into an executable named `mcp-launch` in the project root directory.

### 2.2. Launch the Application (TUI)

The primary way to interact with `mcp-launch` is through its interactive TUI. After building, you can launch it using:

```bash
./mcp-launch up --tui [--config path ...] [-v|-vv]
```

- `--tui`: Activates the interactive TUI for preflight inspection and launch.
- `--config path ...`: (Optional) Specifies paths to `mcp.config.json` files. If omitted, the TUI will guide you through collecting configurations.
- `-v` or `-vv`: (Optional) Increases verbosity for logging (verbose info or debug logs).

#### 2.2.1. Launch with pre-made configs

To launch with a pre-defined set of configurations, you can use the following command:

```bash
go mod tidy && go build -o mcp-launch && ./mcp-launch up \
  --tui \
  --config "mcp_configs/mcp.chatgpt.spec-workflow.json" \
  --config "mcp_configs/mcp.chatgpt.utils.json" \
  --config "mcp_configs/mcp.serena.json"
```

### 2.3. Configuration

The core configuration for the MCP servers managed by `mcp-launch` is defined in `mcp.config.json`. This file specifies how various MCP servers (e.g., `serena`, `mcp-server-time`, `@modelcontextprotocol/server-filesystem`) are invoked and their arguments. Understanding this file is key to comprehending the tools and services the `mcp-launch` application orchestrates.

## 3. LLM Interaction Context

This repository is designed with AI agent interaction in mind. The presence of:
- `.serena/` directory: Indicates integration with the Serena AI framework for semantic code intelligence and agent memory.
- `.spec-workflow/` directory: Suggests adherence to a structured specification workflow, which guides feature development and approvals.

Agents interacting with this repository should leverage these established structures for context, task management, and code modifications.

## 4. MCP Client Incident Report (Handshake Regressions)

This section documents a regression that caused all MCP servers to fail during preflight and how it was resolved, to prevent recurrence.

- What failed: During preflight, multiple servers reported `init read: context deadline exceeded` and some `tools/list read: context deadline exceeded`. A panic `close of closed channel` was also observed in one iteration.

- Root causes:
  - Mixed framing: An experimental change alternated between LSP Content-Length framing and newline-delimited JSON on the same stdio stream. Some servers stalled or ignored requests in this mixed mode.
  - Over-ambitious reader changes: A multi-goroutine reader/pump and per-read goroutines introduced edge cases (double-closing a shared channel; potential contention), leading to a `close of closed channel` panic and timeouts.
  - Notification mismatch: Sending both legacy `initialized` and spec-conformant `notifications/initialized` confused some servers’ validation.
  - Response-loop break bug: Using an unlabeled `break` inside the `tools/list` response loop didn’t exit the loop and led to false timeouts.

- Known-good commit: `a770349562ab1994547f17b18515b7ccce954014`.

- Fix applied: Reverted `internal/mcpclient/client.go` to the handshake and pagination logic from the known-good commit. Key characteristics:
  - Newline-delimited JSON only (no LSP framing).
  - Initialize once with `protocolVersion: "2025-06-18"`; wait for `id:1` (6s timeout).
  - Send only `notifications/initialized`.
  - `tools/list` pagination uses first-page parameter fallbacks: `params:{}`, `cursor:""`, `cursor:null`, and omitting `params`.
  - Correct response matching using a labeled break (or `goto` label pattern) to exit the loop when the expected `id` is seen.

- How not to break it again (Guardrails):
  - Do not mix framing modes on stdio. Stick to newline-delimited JSON unless the project explicitly introduces a feature-flagged LSP pathway with full test coverage.
  - Keep the initialize + `notifications/initialized` sequence as-is unless updating to a newer MCP spec is coordinated and tested against multiple servers.
  - Preserve the `tools/list` first-page param-shape fallbacks to maximize server compatibility.
  - Avoid per-read goroutines or shared-channel closes from multiple goroutines; prefer the simple, blocking `ReadString` with a timeout wrapper as used in the known-good flow.
  - When modifying response loops, ensure the loop exits on the matched `id` (labeled break or equivalent). Add focused tests/logs when touching this area.

## 5. Troubleshooting: repomix (MCP)

If preflight shows `repomix: init read: EOF`, this indicates the process exited before responding to `initialize`. Common causes:

- Incorrect CLI invocation: Ensure the config uses `npx -y repomix@latest --mcp` so the CLI runs in MCP mode and binds stdio.
- Network/tooling prerequisites: `npx` may need network access the first time. Verify your environment permits this and that Node is available.
- Server writes to stderr only or prints logs that aren’t JSON-RPC: The current client expects newline-delimited JSON on stdout. If repomix changes behavior, revisit invocation or add a targeted compatibility switch per upstream docs.

To test repomix standalone:

```
# Inspect repomix CLI options
npx -y repomix@latest --help

# Quick handshake test (newline JSON):
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' \
| npx -y repomix@latest --mcp
```

Update the `mcp_configs/mcp.chatgpt.utils.json` entry for `repomix` only if upstream docs differ from the above (e.g., a different package name or subcommand).
