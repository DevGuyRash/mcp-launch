package config

import (
    "encoding/json"
    "fmt"
    "os"
    "sort"
)

// Claude-style config: {"mcpServers": { "name": {"command": "...", "args": ["..."], ...}, ... }}
// We only need the names to know which subpaths mcpo exposes.
type Config struct {
    MCPServers map[string]MCPServer `json:"mcpServers"`
}

// MCPServer mirrors common client configs (Claude Desktop, Cursor, etc.).
// Only fields used by this program are modeled.
type MCPServer struct {
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Type    string            `json:"type,omitempty"`    // "stdio" (default) | "streamable-http"
    URL     string            `json:"url,omitempty"`     // for streamable-http
    Headers map[string]string `json:"headers,omitempty"` // for streamable-http
    Env     map[string]string `json:"env,omitempty"`     // environment variables
    // Extra is retained to round-trip unknown fields when we clone configs.
    Extra   map[string]any    `json:"-"`                 // not serialized
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    var c Config
    if err := json.Unmarshal(data, &c); err != nil {
        return nil, fmt.Errorf("parse config JSON: %w", err)
    }
    if len(c.MCPServers) == 0 {
        return nil, fmt.Errorf("config has no mcpServers")
    }
    return &c, nil
}

func ServerNames(c *Config) []string {
    names := make([]string, 0, len(c.MCPServers))
    for k := range c.MCPServers {
        names = append(names, k)
    }
    sort.Strings(names)
    return names
}

// Shallow copy config (sufficient for our clone/patch use).
func Clone(c *Config) *Config {
    out := &Config{MCPServers: make(map[string]MCPServer, len(c.MCPServers))}
    for k, v := range c.MCPServers {
        cp := v
        if v.Args != nil {
            cp.Args = append([]string(nil), v.Args...)
        }
        if v.Headers != nil {
            cp.Headers = map[string]string{}
            for hk, hv := range v.Headers {
                cp.Headers[hk] = hv
            }
        }
        if v.Env != nil {
            cp.Env = map[string]string{}
            for ek, ev := range v.Env {
                cp.Env[ek] = ev
            }
        }
        out.MCPServers[k] = cp
    }
    return out
}

func Save(path string, c *Config) error {
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
