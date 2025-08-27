package cloudflare

import (
	"context"
	"fmt"
	"os/exec"

	"mcp-launch/internal/proc"
)

// RunQuickTunnel starts `cloudflared tunnel --url <local>` and returns the child.
// The caller should parse the public URL from logs (look for *.trycloudflare.com).
func RunQuickTunnel(ctx context.Context, sup *proc.Supervisor, localURL string) (*proc.Child, error) {
	args := []string{"tunnel", "--url", localURL}
	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	child, err := sup.Start("cloudflared", cmd)
	if err != nil {
		return nil, fmt.Errorf("start cloudflared: %w", err)
	}
	return child, nil
}
