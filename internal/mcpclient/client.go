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
    debug := os.Getenv("MCP_LAUNCH_DEBUG_MCPCLIENT") == "1"
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
		for k, v := range s.Env {
			em[k] = v
		}
		env := make([]string, 0, len(em))
		for k, v := range em {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
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
    if debug { fmt.Println("[mcpclient] spawn:", s.Command, strings.Join(s.Args, " ")) }
    if err := cmd.Start(); err != nil {
        return Summary{ServerName: name}, err
    }
    // Parse both stdout and stderr as some servers (incorrectly) write JSON-RPC to stderr.

    // Two sending strategies: LSP framing and newline-delimited JSON.
    sendLSP := func(v any) error {
        b, _ := json.Marshal(v)
        header := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n", len(b))
        frame := append([]byte(header), b...)
        _, err := stdin.Write(frame)
        if debug && err != nil { fmt.Println("[mcpclient] write(LSP) err:", err) }
        return err
    }
    sendLine := func(v any) error {
        b, _ := json.Marshal(v)
        b = append(b, '\n')
        _, err := stdin.Write(b)
        if debug && err != nil { fmt.Println("[mcpclient] write(line) err:", err) }
        return err
    }

	// Read loop helpers (support both newline-JSON and Content-Length framing)
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
    rdErr := bufio.NewReader(stderr)
    // Single reader goroutines pumping full JSON messages into a shared channel.
    messages := make(chan string, 256)
    pump := func(tag string, r *bufio.Reader) {
        defer close(messages)
        for {
            line, err := r.ReadString('\n')
            if err != nil {
                if debug { fmt.Println("[mcpclient]", tag, "closed:", err) }
                return
            }
            lt := strings.TrimSpace(line)
            if lt == "" {
                continue
            }
            low := strings.ToLower(lt)
            if strings.HasPrefix(low, "content-length:") {
                parts := strings.SplitN(lt, ":", 2)
                if len(parts) != 2 { continue }
                nstr := strings.TrimSpace(parts[1])
                // drain remaining headers until blank line
                for {
                    hdr, e2 := r.ReadString('\n')
                    if e2 != nil { return }
                    if strings.TrimSpace(hdr) == "" { break }
                }
                var n int
                fmt.Sscanf(nstr, "%d", &n)
                if n <= 0 { continue }
                buf := make([]byte, n)
                if _, e := io.ReadFull(r, buf); e != nil { if debug { fmt.Println("[mcpclient] read body err:", e) }; return }
                msg := strings.TrimSpace(string(buf))
                if msg != "" {
                    if debug { fmt.Println("[mcpclient] <<", tag, msg) }
                    select { case messages <- msg: default: }
                }
                continue
            }
            if strings.HasPrefix(lt, "{") || strings.HasPrefix(lt, "[") {
                if debug { fmt.Println("[mcpclient] <<", tag, lt) }
                select { case messages <- lt: default: }
                continue
            }
            // ignore noise lines
            if debug { fmt.Println("[mcpclient] (noise)", tag, lt) }
        }
    }
    go pump("stdout", rd)
    go pump("stderr", rdErr)

    // 1) initialize (auto-detect framing): send both with distinct ids; accept whichever returns first.
    // Initialize param shape variants for broad compatibility
    initParams := []map[string]any{
        {
            "protocolVersion": "2025-06-18",
            "capabilities":    map[string]any{},
            "clientInfo": map[string]any{"name": "mcp-launch", "version": "0.0.0"},
        },
        {
            "protocolVersion": "2024-11-05",
            "capabilities":    map[string]any{},
            "clientInfo": map[string]any{"name": "mcp-launch", "version": "0.0.0"},
        },
        {
            // omit protocolVersion completely
            "capabilities": map[string]any{},
            "clientInfo":   map[string]any{"name": "mcp-launch", "version": "0.0.0"},
        },
    }
    makeInit := func(params map[string]any) map[string]any {
        return map[string]any{
            "jsonrpc": "2.0",
            "method":  "initialize",
            "params":  params,
        }
    }
    // id=1 for LSP, id=2 for line
    req1 := makeInit(initParams[0])
    req1["id"] = 1
    req2 := makeInit(initParams[0])
    req2["id"] = 2
    framing := "auto"

    // Robust handshake: wrappers like npx/uvx may consume early stdin before exec.
    // Re-send initialize periodically until we see a proper response or timeout.
    initDeadline := 60 * time.Second
    // Heuristic: when using package runners that may install on first run, allow more time
    if strings.Contains(strings.ToLower(s.Command), "npx") ||
        strings.Contains(strings.ToLower(s.Command), "uvx") ||
        strings.Contains(strings.ToLower(s.Command), "pipx") ||
        strings.Contains(strings.ToLower(s.Command), "bunx") {
        initDeadline = 120 * time.Second
    }
    start := time.Now()
    // Attempt initialize with alternating framings and param shapes until success or timeout
    attempt := 0
    for {
        // select framing and params for this attempt (prefer LSP, retry, then line)
        shape := initParams[(attempt/2)%len(initParams)]
        // IDs remain constant; choose which one to send this time
        if attempt%4 < 3 {
            req1 = makeInit(shape); req1["id"] = 1
            if debug { fmt.Println("[mcpclient] >> initialize (LSP)") }
            _ = sendLSP(req1)
        } else {
            req2 = makeInit(shape); req2["id"] = 2
            if debug { fmt.Println("[mcpclient] >> initialize (line)") }
            _ = sendLine(req2)
        }

        // small read window for each attempt
        readCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
        var got bool
        for !got {
            var line string
            select {
            case <-readCtx.Done():
                // timeout for this attempt
                break
            case line, _ = <-messages:
                if line == "" { // channel closed
                    cancel()
                    _ = cmd.Process.Kill()
                    return Summary{ServerName: name}, fmt.Errorf("init read: %w", io.EOF)
                }
            }
            if line == "" { break }
            var r resp
            if json.Unmarshal([]byte(line), &r) == nil && r.ID != nil {
                switch idv := r.ID.(type) {
                case float64:
                    if int(idv) == 1 || int(idv) == 2 {
                        if int(idv) == 1 {
                            framing = "lsp"
                        } else {
                            framing = "line"
                        }
                        if r.Error != nil {
                            cancel()
                            _ = cmd.Process.Kill()
                            return Summary{ServerName: name}, fmt.Errorf("initialize failed: %s", r.Error.Message)
                        }
                        got = true
                    }
                case int:
                    if idv == 1 || idv == 2 {
                        if idv == 1 { framing = "lsp" } else { framing = "line" }
                        got = true
                    }
                }
            }
        }
        cancel()
        if got {
            break
        }
        if time.Since(start) >= initDeadline {
            _ = cmd.Process.Kill()
            return Summary{ServerName: name}, fmt.Errorf("init read: %w", context.DeadlineExceeded)
        }
        attempt++
    }
	// 2) initialized notifications (send both variants for compatibility)
	sender := sendLSP
	if framing == "line" {
		sender = sendLine
	}
	_ = sender(map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialized",
	})
	_ = sender(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	// 3) tools/list with explicit null cursor and pagination
    var tools []Tool
    nextID := 3
	var cursorStr string // empty => first page
	for {
		// Build candidate param shapes for first page compatibility across servers
		type paramShape struct {
			set bool
			val map[string]any
		}
		var shapes []paramShape
		if strings.TrimSpace(cursorStr) != "" {
			shapes = []paramShape{{set: true, val: map[string]any{"cursor": cursorStr}}}
		} else {
			shapes = []paramShape{
				{set: true, val: map[string]any{}},              // params: {}
				{set: true, val: map[string]any{"cursor": ""}},  // cursor as empty string
				{set: true, val: map[string]any{"cursor": nil}}, // cursor: null
				{set: false, val: nil},                          // omit params entirely
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
			if framing == "line" {
				_ = sendLine(req)
			} else {
				_ = sendLSP(req)
			}

        // read matching response
        ctxL, cancelL := context.WithTimeout(ctx, 12*time.Second)
        var r resp
        for {
            var line string
            select {
            case <-ctxL.Done():
                cancelL()
                _ = cmd.Process.Kill()
                return Summary{ServerName: name}, fmt.Errorf("tools/list read: %w", context.DeadlineExceeded)
            case line, _ = <-messages:
                if line == "" { // channel closed
                    cancelL()
                    _ = cmd.Process.Kill()
                    return Summary{ServerName: name}, fmt.Errorf("tools/list read: %w", io.EOF)
                }
            }
            if json.Unmarshal([]byte(line), &r) == nil && r.ID != nil {
                switch idv := r.ID.(type) {
                case float64:
                    if int(idv) == nextID { break }
                case int:
                    if idv == nextID { break }
                }
            }
        }
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
				NextCursor string `json:"nextCursor"`
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

// (removed readLine: message pump handles reading)

// --- streamable-http (minimal, JSON-only) ---

func inspectHTTP(ctx context.Context, name string, s cfg.MCPServer) (Summary, error) {
	// For now, we skip implementation due to wide server variance (SSE vs JSON, sessions).
	// Fall back to stdio if command is present.
	if s.Command != "" {
		return inspectStdio(ctx, name, s)
	}
	return Summary{ServerName: name}, errors.New("streamable-http inspection not implemented for server without a stdio command fallback")
}

// mapsClone creates a shallow clone of a map[string]any, including nested params.
func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	if p, ok := in["params"].(map[string]any); ok {
		cp := make(map[string]any, len(p))
		for k, v := range p {
			cp[k] = v
		}
		out["params"] = cp
	}
	return out
}
