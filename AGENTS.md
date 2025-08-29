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

### 2.3. Configuration

The core configuration for the MCP servers managed by `mcp-launch` is defined in `mcp.config.json`. This file specifies how various MCP servers (e.g., `serena`, `mcp-server-time`, `@modelcontextprotocol/server-filesystem`) are invoked and their arguments. Understanding this file is key to comprehending the tools and services the `mcp-launch` application orchestrates.

## 3. LLM Interaction Context

This repository is designed with AI agent interaction in mind. The presence of:
- `.serena/` directory: Indicates integration with the Serena AI framework for semantic code intelligence and agent memory.
- `.spec-workflow/` directory: Suggests adherence to a structured specification workflow, which guides feature development and approvals.

Agents interacting with this repository should leverage these established structures for context, task management, and code modifications.
