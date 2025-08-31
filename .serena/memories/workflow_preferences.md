# Workflow Preferences

- Do not auto-commit changes. Always ask the user to commit at the end of each message.
- Keep changes minimal and focused on the task at hand.
- Prefer generic, server-agnostic behavior for MCP interactions (no server-specific customizations).
- When launches are slow (cold caches, parallel starts), prefer the fast–slow init strategy and allow manual override via MCP_INIT_TIMEOUT_SEC.
