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
            // MCP protocol version (match current spec date)
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
    // 2) initialized notification (MCP v2025 style)
    _ = send(map[string]any{
        "jsonrpc": "2.0",
        "method":  "notifications/initialized",
    })
    // 3) tools/list with explicit null cursor and pagination
    var tools []Tool
    nextID := 2
    var cursorStr string // empty => first page
    for {
        // Build candidate param shapes for first page compatibility across servers
        type paramShape struct{ set bool; val map[string]any }
        var shapes []paramShape
        if strings.TrimSpace(cursorStr) != "" {
            shapes = []paramShape{{set: true, val: map[string]any{"cursor": cursorStr}}}
        } else {
            shapes = []paramShape{
                {set: true,  val: map[string]any{}},                 // params: {}
                {set: true,  val: map[string]any{"cursor": ""}},  // cursor as empty string
                {set: true,  val: map[string]any{"cursor": nil}},  // cursor: null
                {set: false, val: nil},                              // omit params entirely
            }
        }
        var lastErr error
        for attempt := 0; attempt < len(shapes); attempt++ {
            req := map[string]any{
                "jsonrpc": "2.0",
                "id":      nextID,
                "method":  "tools/list",
            }
            if shapes[attempt].set {
                req["params"] = shapes[attempt].val
            }
            _ = send(req)

            // read matching response
            ctxL, cancelL := context.WithTimeout(ctx, 6*time.Second)
            var r resp
            for {
                line, err := readLine(ctxL, rd)
                if err != nil {
                    cancelL()
                    _ = cmd.Process.Kill()
                    return Summary{ServerName: name}, fmt.Errorf("tools/list read: %w", err)
                }
                if json.Unmarshal([]byte(line), &r) == nil && r.ID != nil {
                    switch idv := r.ID.(type) {
                    case float64:
                        if int(idv) == nextID { goto GOT }
                    case int:
                        if idv == nextID { goto GOT }
                    }
                }
            }
        GOT:
            cancelL()
            if r.Error != nil {
                lastErr = fmt.Errorf("tools/list failed: %s", r.Error.Message)
                // try next shape if available (first page only)
                if strings.TrimSpace(cursorStr) == "" && attempt+1 < len(shapes) {
                    continue
                }
                _ = cmd.Process.Kill()
                return Summary{ServerName: name}, lastErr
            }
            // Success → process result and break attempts loop
            var wrapper struct {
                Tools      []Tool `json:"tools"`
                NextCursor string  `json:"nextCursor"`
            }
            if err := json.Unmarshal(r.Result, &wrapper); err == nil {
                if len(wrapper.Tools) > 0 {
                    tools = append(tools, wrapper.Tools...)
                }
            }
            if strings.TrimSpace(wrapper.NextCursor) == "" { // no more pages
                _ = cmd.Process.Kill()
                return Summary{ServerName: name, Tools: tools}, nil
            }
            cursorStr = wrapper.NextCursor
            break // proceed to next page
        }
        nextID++
    }
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

