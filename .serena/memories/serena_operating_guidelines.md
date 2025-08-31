# Serena Operating Guidelines (from serena_initial_instructions)

- Read minimally: Prefer symbol overviews and targeted symbol reads; avoid reading entire files unless strictly necessary.
- Avoid duplicate reading: Do not re-read the same content with symbolic tools after already reading an entire file.
- Discovery order: get_symbols_overview → find_symbol (depth as needed) → include_body only for necessary symbols. Use search_for_pattern when unsure of locations.
- Relationships: Use find_referencing_symbols to understand cross-symbol references.
- Scope searches with relative_path to limit analysis to relevant files/dirs.
- Modes: Planning + one-shot by default; complete tasks autonomously but keep token usage lean; only abort if critical info is missing and cannot be inferred.
- Non-code files: Use pattern search or targeted reads; reserve full-file reads for exceptional cases.
- Tooling context: Rely on internal tools and the provided toolset; don’t attempt to use excluded tools.
- Principle: Intelligent, resource-efficient reading—only what’s needed to solve the task.
