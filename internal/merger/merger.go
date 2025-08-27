package merger

import (
    "encoding/json"
    "fmt"
    "sort"
)

// Merge multiple OpenAPI JSON documents (as raw bytes), prefixing paths by server name,
// namespacing operationIds, and merging components/schemas conservatively.
func Merge(serverToSpec map[string][]byte) ([]byte, error) {
    type OA struct {
        OpenAPI  string           `json:"openapi,omitempty"`
        Info     map[string]any   `json:"info,omitempty"`
        Servers  []map[string]any `json:"servers,omitempty"`
        Paths    map[string]any   `json:"paths"`
        Comps    map[string]any   `json:"components,omitempty"`
        Security []map[string]any `json:"security,omitempty"`
    }
    final := OA{
        OpenAPI:  "3.1.0",
        Info:     map[string]any{"title": "MCP Tools via mcpo", "version": "1.0.0"},
        Paths:    map[string]any{},
        Comps:    map[string]any{"schemas": map[string]any{}, "securitySchemes": map[string]any{}},
        Security: []map[string]any{{"mcpoApiKey": []any{}}},
    }

    // Inject our apiKey header
    final.Comps["securitySchemes"].(map[string]any)["mcpoApiKey"] = map[string]any{
        "type": "apiKey", "in": "header", "name": "X-API-Key",
    }

    ordered := make([]string, 0, len(serverToSpec))
    for k := range serverToSpec {
        ordered = append(ordered, k)
    }
    sort.Strings(ordered)
    for _, name := range ordered {
        raw := serverToSpec[name]
        var spec map[string]any
        if err := json.Unmarshal(raw, &spec); err != nil {
            return nil, fmt.Errorf("parse %s spec: %w", name, err)
        }
        paths, _ := spec["paths"].(map[string]any)
        if paths == nil {
            return nil, fmt.Errorf("%s spec has no .paths", name)
        }
        for p, entry := range paths {
            // Prefix path with /<name>
            newPath := fmt.Sprintf("/%s%s", name, p)
            // Patch operationIds to `<name>__<opId>` if present
            if m, ok := entry.(map[string]any); ok {
                for method, v := range m {
                    if op, ok := v.(map[string]any); ok {
                        if opID, ok2 := op["operationId"].(string); ok2 && opID != "" {
                            op["operationId"] = fmt.Sprintf("%s__%s", name, opID)
                        }
                        m[method] = op
                    }
                }
                final.Paths[newPath] = m
            } else {
                final.Paths[newPath] = entry
            }
        }
        // Merge components.schemas if present
        if comp, ok := spec["components"].(map[string]any); ok {
            if schemas, ok := comp["schemas"].(map[string]any); ok {
                dst := final.Comps["schemas"].(map[string]any)
                for k, v := range schemas {
                    ns := fmt.Sprintf("%s__%s", name, k)
                    dst[ns] = v
                }
            }
            // Merge securitySchemes (ignore—ours is at top-level)
        }
    }

    // Servers will be filled in by callers (the public baseUrl) if desired
    out := map[string]any{
        "openapi":    final.OpenAPI,
        "info":       final.Info,
        "servers":    []map[string]any{}, // set by writer
        "paths":      final.Paths,
        "components": final.Comps,
        "security":   final.Security,
    }
    return json.MarshalIndent(out, "", "  ")
}
