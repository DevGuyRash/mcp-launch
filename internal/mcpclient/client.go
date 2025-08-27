package mcpclient

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
    "time"

    cfg "mcp-launch/internal/config"
)

// Tool describes a single MCP tool discovered via tools/list.
type Tool struct {
    Name        string `json:"name"`
    Title       string `json:"title,omitempty"`
    Description string `json:"description,omitempty"`
}

// Summary is the result of inspecting one server.
type Summary struct {
    ServerName string
    Tools      []Tool
    Err        error
}

// InspectServer discovers tools for a single MCP server defined in a Claude-style config.
// It supports the stdio transport; streamable-http is a stub (future).
func InspectServer(ctx context.Context, name string, s cfg.MCPServer) (Summary, error) {
    if strings.ToLower(strings.TrimSpace(s.Type)) == "streamable-http" && s.URL != "" {
        return inspectHTTP(ctx, name, s)
    }
    return inspectStdio(ctx, name, s)
}

// --- stdio ---

func inspectStdio(ctx context.Context, name string, s cfg.MCPServer) (Summary, error) {
    if s.Command == "" {
        return Summary{ServerName: name}, fmt.Errorf("server %s missing command", name)
    }
    cmd := exec.CommandContext(ctx, s.Command, s.Args...)
    // Inherit parent env; if overrides present, replace keys rather than append duplicates.
    if len(s.Env) > 0 {
        em := map[string]string{}
        for _, kv := range os.Environ() {
            if i := strings.IndexByte(kv, '='); i >= 0 {
                em[kv[:i]] = kv[i+1:]
            }
        }
        for k, v := range s.Env { em[k] = v }
        env := make([]string, 0, len(em))
        for k, v := range em { env = append(env, fmt.Sprintf("%s=%s", k, v)) }
        cmd.Env = env
    }
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return Summary{ServerName: name}, err
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return Summary{ServerName: name}, err
    }
    stderr, _ := cmd.StderrPipe()
    if err := cmd.Start(); err != nil {
        return Summary{ServerName: name}, err
    }
    // Drain stderr in background to avoid blocking if server logs a lot.
    go io.Copy(io.Discard, stderr) //nolint:errcheck

    // Helper to send a JSON-RPC message (newline-delimited)
    send := func(v any) error {
        b, _ := json.Marshal(v)
        b = append(b, '\n')
        _, err := stdin.Write(b)
        return err
    }

    // Read loop for responses
    type resp struct {
        ID     any             `json:"id,omitempty"`
        Result json.RawMessage `json:"result,omitempty"`
        Error  *struct {
            Code    int    `json:"code"`
            Message string `json:"message"`
        } `json:"error,omitempty"`
        Headers map[string]any `json:"headers,omitempty"` // for streamable http; ignored here
    }
    rd := bufio.NewReader(stdout)

    // 1) initialize
    _ = send(map[string]any{
        "jsonrpc": "2.0",
        "id":      1,
        "method":  "initialize",
        "params": map[string]any{
            "protocolVersion": "2025-06-18",
            "capabilities":    map[string]any{},
            "clientInfo": map[string]any{
                "name":    "mcp-launch",
                "version": "0.0.0",
            },
        },
    })

    // wait for id=1
    ctxR, cancel := context.WithTimeout(ctx, 6*time.Second)
    defer cancel()
    var sawInit bool
    for !sawInit {
        line, err := readLine(ctxR, rd)
        if err != nil {
            _ = cmd.Process.Kill()
            return Summary{ServerName: name}, fmt.Errorf("init read: %w", err)
        }
        var r resp
        if json.Unmarshal([]byte(line), &r) == nil && r.ID != nil {
            switch r.ID.(type) {
            case float64:
                if int(r.ID.(float64)) == 1 {
                    if r.Error != nil {
                        _ = cmd.Process.Kill()
                        return Summary{ServerName: name}, fmt.Errorf("initialize failed: %s", r.Error.Message)
                    }
                    sawInit = true
                }
            case int:
                if r.ID.(int) == 1 {
                    sawInit = true
                }
            }
        }
    }
    // 2) initialized notification
    _ = send(map[string]any{
        "jsonrpc": "2.0",
        "method":  "initialized",
    })
    // 3) tools/list
    _ = send(map[string]any{
        "jsonrpc": "2.0",
        "id":      2,
        "method":  "tools/list",
        "params":  map[string]any{},
    })

    // read id=2
    var tools []Tool
    ctxL, cancelL := context.WithTimeout(ctx, 6*time.Second)
    defer cancelL()
    for {
        line, err := readLine(ctxL, rd)
        if err != nil {
            break
        }
        var r resp
        if json.Unmarshal([]byte(line), &r) == nil && r.ID != nil {
            if i, ok := r.ID.(float64); ok && int(i) == 2 {
                if r.Error != nil {
                    _ = cmd.Process.Kill()
                    return Summary{ServerName: name}, fmt.Errorf("tools/list failed: %s", r.Error.Message)
                }
                // result: { tools: [ {name, description, ...} ], nextCursor?: string }
                var wrapper struct {
                    Tools      []Tool `json:"tools"`
                    NextCursor string  `json:"nextCursor"`
                }
                if err := json.Unmarshal(r.Result, &wrapper); err == nil {
                    tools = append(tools, wrapper.Tools...)
                }
                break
            }
        }
    }
    // be nice and terminate
    _ = cmd.Process.Kill()
    return Summary{ServerName: name, Tools: tools}, nil
}

func readLine(ctx context.Context, rd *bufio.Reader) (string, error) {
    type res struct {
        s   string
        err error
    }
    ch := make(chan res, 1)
    go func() {
        line, err := rd.ReadString('\n')
        ch <- res{line, err}
    }()
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    case out := <-ch:
        return strings.TrimRight(out.s, "\r\n"), out.err
    }
}

// --- streamable-http (minimal, JSON-only) ---

func inspectHTTP(ctx context.Context, name string, s cfg.MCPServer) (Summary, error) {
    // For now, we skip implementation due to wide server variance (SSE vs JSON, sessions).
    // Fall back to stdio if command is present.
    if s.Command != "" {
        return inspectStdio(ctx, name, s)
    }
    return Summary{ServerName: name}, errors.New("streamable-http inspection not implemented for server without a stdio command fallback")
}
