package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Claude-style config: {"mcpServers": { "name": {"command": "...", "args": ["..."], ...}, ... }}
// We only need the names to know which subpaths mcpo exposes.
type Config struct {
	MCPServers map[string]any `json:"mcpServers"`
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
	return names
}
