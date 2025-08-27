// Copyright
// SPDX-License-Identifier: MIT
// mcp-launch: minimal supervisor for mcpo + merged OpenAPI + Cloudflare tunnel
package main

const Version = "0.2.0"

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"crypto/rand"
)

const (
	defaultFrontPort = 8000
	defaultMcpoPort  = 8800
	stateDirName     = ".mcp-launch"
	stateFileName    = "state.json"
	defaultConfig    = "mcp.config.json"
)

type MCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Type    string            `json:"type,omitempty"` // sse | streamable-http
	URL     string            `json:"url,omitempty"`  // for sse/streamable-http
	Headers map[string]string `json:"headers,omitempty"`
}

type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

type State struct {
	APIKey        string   `json:"api_key"`
	ConfigPath    string   `json:"config_path"`
	FrontPort     int      `json:"front_port"`
	McpoPort      int      `json:"mcpo_port"`
	PublicURL     string   `json:"public_url"`
	TunnelMode    string   `json:"tunnel_mode"` // quick|named|none
	TunnelName    string   `json:"tunnel_name,omitempty"`
	CloudflaredPID int     `json:"cloudflared_pid"`
	McpoPID       int      `json:"mcpo_pid"`
	ToolNames     []string `json:"tool_names"`
	StartedAt     string   `json:"started_at"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd := os.Args[1]
	switch cmd {
	case "help", "-h", "--help":
		if len(os.Args) > 2 {
			helpTopic(os.Args[2])
		} else {
			usage()
		}
	case "version", "-v", "--version":
		fmt.Println("mcp-launch", Version)
		return
	case "init":
		cmdInit()
	case "doctor":
		cmdDoctor()
	case "up":
		cmdUp()
	case "status":
		cmdStatus()
	case "down":
		cmdDown()
	case "openapi":
		cmdOpenAPI()
	case "share":
		cmdShare()
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`mcp-launch ` + Version + `
One URL for many MCP servers (via mcpo). Serves /openapi.json and proxies everything else to mcpo.

USAGE
  mcp-launch <command> [options]

COMMANDS
  init         Scaffold mcp.config.json and default state
  up           Start mcpo + front proxy (+ optional Cloudflare Tunnel), then generate merged OpenAPI
  status       Show ports, URLs, detected tools, API key
  openapi      Regenerate merged OpenAPI (uses current/--public-url)
  share        Print the single URL you paste into ChatGPT (Custom GPT → Actions → Import from URL)
  down         Stop mcpo and cloudflared
  doctor       Check dependencies (mcpo, cloudflared, plus uvx/npx if referenced in config)
  help         Show help (try: mcp-launch help up)
  version      Print version

QUICK START
  mcp-launch init
  mcp-launch up --tunnel quick
  # Paste the printed https://.../openapi.json into Custom GPT → Actions → Import from URL

NOTES
  • Front proxy listens on --port (default 8000) and serves /openapi.json on the same host&port it proxies.
  • mcpo listens on --mcpo-port (default 8800).
  • The API key is required on all requests (header X-API-Key). It is generated if not provided.
  • State lives in ./.mcp-launch/state.json (ports, API key, public URL, etc).
`)
	}
	
func helpTopic(name string) {
	switch name {
	case "up":
		fmt.Println(`USAGE
  mcp-launch up [--config PATH] [--port N] [--mcpo-port N] [--api-key KEY]
                 [--tunnel quick|named|none] [--public-url URL] [--tunnel-name NAME]

OPTIONS
  --config PATH          Claude-style config (default: mcp.config.json)
  --port N               Front proxy port serving API + /openapi.json (default: 8000)
  --mcpo-port N          Internal mcpo port (default: 8800)
  --api-key KEY          API key for mcpo (generated if omitted)
  --tunnel MODE          quick | named | none (default: quick)
  --public-url URL       Public base URL used in the merged OpenAPI (recommended for named/none)
  --tunnel-name NAME     Named tunnel to run (cloudflared tunnel run NAME)

EXAMPLES
  # Dev (ephemeral URL):
  mcp-launch up --tunnel quick

  # Stable domain (named tunnel already configured):
  mcp-launch up --tunnel named --public-url https://gpt-tools.example.com --tunnel-name my-tunnel
`)
	case "openapi":
		fmt.Println(`USAGE
  mcp-launch openapi [--public-url URL]

DESCRIPTION
  Rebuild the merged OpenAPI document from the running mcpo per-tool specs,
  set its servers[0].url to the provided --public-url (or the current state/public URL),
  and store it for the front proxy to serve at /openapi.json.
`)
	default:
		usage()
	}
}
	
}

func cmdInit() {
	// Write default config if not exists
	if _, err := os.Stat(defaultConfig); errors.Is(err, os.ErrNotExist) {
		d := `{
  "mcpServers": {
    "serena": {
      "command": "uvx",
      "args": ["--from", "git+https://github.com/oraios/serena", "serena", "start-mcp-server", "--context", "ide-assistant"]
    },
    "time": {
      "command": "uvx",
      "args": ["mcp-server-time", "--local-timezone=America/Phoenix"]
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspaces/projects"]
    }
  }
}`
		_ = os.WriteFile(defaultConfig, []byte(d), 0644)
		fmt.Println("Wrote", defaultConfig)
	} else {
		fmt.Println(defaultConfig, "already exists; not overwriting")
	}
	ensureStateDir()
	st := State{
		APIKey:     randomKey(40),
		ConfigPath: defaultConfig,
		FrontPort:  defaultFrontPort,
		McpoPort:   defaultMcpoPort,
		TunnelMode: "quick",
		StartedAt:  time.Now().Format(time.RFC3339),
	}
	saveState(&st)
	fmt.Println("Initialized .mcp-launch/state.json with defaults")
}

func cmdDoctor() {
	st := loadState()
	// Check presence of mcpo and cloudflared
	checks := []string{"mcpo", "cloudflared"}
	// If config references uvx/npx, check those too
	cfg := readConfig(st.ConfigPath)
	need := map[string]bool{}
	for _, s := range cfg.MCPServers {
		if s.Command != "" {
			need[s.Command] = true
		}
	}
	for c := range need {
		checks = append(checks, c)
	}
	fmt.Println("Dependency checks:")
	ok := true
	for _, bin := range checks {
		if bin == "" { continue }
		_, err := exec.LookPath(bin)
		if err != nil {
			fmt.Printf("  ✗ %s not found in PATH\n", bin)
			ok = false
		} else {
			fmt.Printf("  ✓ %s found\n", bin)
		}
	}
	if ok {
		fmt.Println("All required executables found.")
	} else {
		fmt.Println("Missing executables detected. Install the items marked ✗ and retry.")
	}
}

func cmdUp() {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	fs.Usage = func() { helpTopic("up") }
	config := fs.String("config", defaultConfig, "Path to mcpo config file")
	port := fs.Int("port", defaultFrontPort, "Front HTTP port (serves /openapi.json and proxies to mcpo)")
	mcpoPort := fs.Int("mcpo-port", defaultMcpoPort, "Internal mcpo port")
	apiKey := fs.String("api-key", "", "API key for mcpo (optional; generated if empty)")
	tunnel := fs.String("tunnel", "quick", "Tunnel mode: quick|named|none")
	publicURL := fs.String("public-url", "", "Public base URL (required for named or none if you want merged spec to be correct)")
	tunnelName := fs.String("tunnel-name", "", "Named tunnel to run (optional; requires local cloudflared config)")
	_ = fs.Parse(os.Args[2:])

	ensureStateDir()
	st := loadState()
	if *apiKey == "" {
		if st.APIKey == "" {
			st.APIKey = randomKey(40)
		}
	} else {
		st.APIKey = *apiKey
	}
	st.ConfigPath = *config
	st.FrontPort = pickPort(*port)
	st.McpoPort = pickPort(*mcpoPort)
	st.TunnelMode = *tunnel
	if *publicURL != "" {
		st.PublicURL = strings.TrimRight(*publicURL, "/")
	}
	saveState(&st)

	// Start mcpo
	mcpoCmd := exec.Command(findBinary("mcpo"), "--port", fmt.Sprint(st.McpoPort), "--api-key", st.APIKey, "--config", st.ConfigPath, "--hot-reload")
	mcpoStdout, _ := mcpoCmd.StdoutPipe()
	mcpoStderr, _ := mcpoCmd.StderrPipe()
	if err := mcpoCmd.Start(); err != nil {
		fmt.Println("Failed to start mcpo:", err)
		return
	}
	st.McpoPID = mcpoCmd.Process.Pid
	saveState(&st)

	go streamPrefixed("mcpo", mcpoStdout)
	go streamPrefixed("mcpo", mcpoStderr)
	waitURL(fmt.Sprintf("http://127.0.0.1:%d/docs", st.McpoPort), 60*time.Second)

	// Determine tool names from config
	cfg := readConfig(st.ConfigPath)
	var toolNames []string
	for name := range cfg.MCPServers {
		toolNames = append(toolNames, name)
	}
	slices.Sort(toolNames)
	st.ToolNames = toolNames
	saveState(&st)

	// Start front proxy (serves /openapi.json, proxies to mcpo)
	proxy := newFrontProxy(st)
	go func() {
		if err := proxy.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("Front proxy error:", err)
		}
	}()

	// Start tunnel
	var quickURL string
	switch st.TunnelMode {
	case "quick":
		quickURL = startQuickTunnel(st.FrontPort)
		if quickURL == "" {
			fmt.Println("Quick Tunnel failed; continuing without a public URL.")
		} else {
			st.PublicURL = quickURL
			saveState(&st)
		}
	case "named":
		if st.PublicURL == "" {
			fmt.Println("Named tunnel selected but --public-url not provided. Please pass --public-url https://your.host")
		}
		startNamedTunnel(*tunnelName)
	case "none":
		// no-op
	default:
		fmt.Println("Unknown tunnel mode:", st.TunnelMode)
	}

	// Generate merged OpenAPI with known PublicURL (or localhost if still empty)
	baseURL := st.PublicURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", st.FrontPort)
	}
	spec, err := mergeOpenAPI(st, baseURL)
	if err != nil {
		fmt.Println("OpenAPI merge failed:", err)
	} else {
		proxy.SetOpenAPI(spec)
		fmt.Println("Merged OpenAPI available at:", fmt.Sprintf("http://127.0.0.1:%d/openapi.json", st.FrontPort))
		if st.PublicURL != "" {
			fmt.Println("Public OpenAPI URL:", st.PublicURL+"/openapi.json")
		}
	}

	fmt.Println()
	fmt.Println("=== SHARE THIS WITH CHATGPT (Actions → Import from URL) ===")
	if st.PublicURL != "" {
		fmt.Println(st.PublicURL + "/openapi.json")
	} else {
		fmt.Printf("http://127.0.0.1:%d/openapi.json (local only)\n", st.FrontPort)
	}
	fmt.Println("API key header: X-API-Key:", st.APIKey)
	fmt.Println()

	// Stream until interrupted
	fmt.Println("Press Ctrl+C to stop (or run `mcp-launch down` from another shell).")
	// Wait on mcpo; if it exits, we exit
	_ = mcpoCmd.Wait()
}

func cmdStatus() {
	st := loadState()
	fmt.Println("mcp-launch status:")
	fmt.Printf("- Front: http://127.0.0.1:%d  (serves /openapi.json; proxies to mcpo)\n", st.FrontPort)
	fmt.Printf("- mcpo:  http://127.0.0.1:%d\n", st.McpoPort)
	if st.PublicURL != "" {
		fmt.Println("- Public URL:", st.PublicURL)
	} else {
		fmt.Println("- Public URL: (none)")
	}
	fmt.Println("- Tunnel:", st.TunnelMode)
	fmt.Println("- Tools:", strings.Join(st.ToolNames, ", "))
	fmt.Println("- API key (X-API-Key):", st.APIKey)
}

func cmdShare() {
	st := loadState()
	if st.PublicURL == "" {
		fmt.Printf("No public URL known yet. Local: http://127.0.0.1:%d/openapi.json\n", st.FrontPort)
		return
	}
	fmt.Println(st.PublicURL + "/openapi.json")
}

func cmdDown() {
	st := loadState()
	// Kill cloudflared
	if st.CloudflaredPID > 0 {
		_ = killPID(st.CloudflaredPID)
		fmt.Println("Stopped cloudflared (pid", st.CloudflaredPID, ")")
		st.CloudflaredPID = 0
	}
	// Kill mcpo
	if st.McpoPID > 0 {
		_ = killPID(st.McpoPID)
		fmt.Println("Stopped mcpo (pid", st.McpoPID, ")")
		st.McpoPID = 0
	}
	saveState(&st)
}

func cmdOpenAPI() {
	fs := flag.NewFlagSet("openapi", flag.ExitOnError)
	fs.Usage = func() { helpTopic("openapi") }
	publicURL := fs.String("public-url", "", "Public base URL (optional override)")
	_ = fs.Parse(os.Args[2:])
	st := loadState()
	baseURL := st.PublicURL
	if *publicURL != "" {
		baseURL = strings.TrimRight(*publicURL, "/")
	}
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", st.FrontPort)
	}
	spec, err := mergeOpenAPI(st, baseURL)
	if err != nil {
		fmt.Println("OpenAPI merge failed:", err)
		return
	}
	// write to front proxy file path if running, otherwise dump to stdout
	out := filepath.Join(getStateDir(), "openapi.json")
	_ = os.WriteFile(out, spec, 0644)
	fmt.Println("Wrote merged OpenAPI to", out)
	fmt.Printf("Serve URL (if front proxy running): http://127.0.0.1:%d/openapi.json\n", st.FrontPort)
}

func readConfig(path string) MCPConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPConfig{}
	}
	var cfg MCPConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func ensureStateDir() string {
	dir := getStateDir()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
func getStateDir() string {
	return filepath.Join(".", stateDirName)
}
func statePath() string {
	return filepath.Join(getStateDir(), stateFileName)
}
func saveState(st *State) {
	_ = os.MkdirAll(getStateDir(), 0o755)
	data, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(statePath(), data, 0644)
}
func loadState() State {
	path := statePath()
	var st State
	data, err := os.ReadFile(path)
	if err != nil {
		// defaults
		st = State{
			APIKey:     randomKey(40),
			ConfigPath: defaultConfig,
			FrontPort:  defaultFrontPort,
			McpoPort:   defaultMcpoPort,
			TunnelMode: "quick",
			StartedAt:  time.Now().Format(time.RFC3339),
		}
		saveState(&st)
		return st
	}
	_ = json.Unmarshal(data, &st)
	return st
}

func findBinary(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	// Windows fallback with .exe
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		if p, err := exec.LookPath(name + ".exe"); err == nil {
			return p
		}
	}
	return name // let exec fail and print a clearer error
}

func streamPrefixed(tag string, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fmt.Printf("[%s] %s\n", tag, sc.Text())
	}
}

func randomKey(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// fallback to time-based seed if crypto fails (unlikely)
		for i := range buf {
			buf[i] = alphabet[int(time.Now().UnixNano()+int64(i))%len(alphabet)]
		}
		return string(buf)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

func pickPort(preferred int) int {
	if isFree(preferred) {
		return preferred
	}
	for p := preferred + 1; p < preferred+1000; p++ {
		if isFree(p) {
			return p
		}
	}
	return preferred
}
func isFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

type frontProxy struct {
	st     State
	srv    *http.Server
	proxy  *httputil.ReverseProxy
	mu     sync.RWMutex
	spec   []byte // merged openapi
}

func newFrontProxy(st State) *frontProxy {
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", st.McpoPort))
	p := httputil.NewSingleHostReverseProxy(target)
	fp := &frontProxy{st: st, proxy: p}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		fp.mu.RLock()
		defer fp.mu.RUnlock()
		if len(fp.spec) == 0 {
			http.Error(w, "spec not generated yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fp.spec)
	})
	// simple health
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// proxy everything else
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r)
	})
	fp.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", st.FrontPort),
		Handler: mux,
	}
	return fp
}

func (f *frontProxy) Serve() error {
	fmt.Printf("Front proxy listening on http://127.0.0.1:%d\n", f.st.FrontPort)
	return f.srv.ListenAndServe()
}
func (f *frontProxy) SetOpenAPI(spec []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spec = spec
}

func startQuickTunnel(frontPort int) string {
	bin := findBinary("cloudflared")
	cmd := exec.Command(bin, "tunnel", "--url", fmt.Sprintf("http://127.0.0.1:%d", frontPort))
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		fmt.Println("Failed to start cloudflared:", err)
		return ""
	}
	// save PID
	st := loadState()
	st.CloudflaredPID = cmd.Process.Pid
	saveState(&st)
	// Parse URL from output
	urlCh := make(chan string, 1)
	parse := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			fmt.Println("[cloudflared]", line)
			if strings.Contains(line, "trycloudflare.com") {
				u := findFirstURL(line)
				if u != "" {
					urlCh <- u
				}
			}
		}
	}
	go parse(stdout)
	go parse(stderr)

	select {
	case u := <-urlCh:
		return strings.TrimSuffix(u, "/")
	case <-time.After(20 * time.Second):
		return ""
	}
}

func startNamedTunnel(name string) {
	if name == "" {
		// run default named tunnel from config
		cmd := exec.Command(findBinary("cloudflared"), "tunnel", "run")
		_ = cmd.Start()
		st := loadState()
		st.CloudflaredPID = cmd.Process.Pid
		saveState(&st)
		return
	}
	cmd := exec.Command(findBinary("cloudflared"), "tunnel", "run", name)
	_ = cmd.Start()
	st := loadState()
	st.CloudflaredPID = cmd.Process.Pid
	saveState(&st)
}

func killPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// Windows does not support Kill with signals; Go abstracts it
	return process.Kill()
}

func waitURL(u string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			req, _ := http.NewRequest("GET", u, nil)
			_, err := http.DefaultClient.Do(req)
			if err == nil {
				return
			}
		}
	}
}

func findFirstURL(s string) string {
	// very small heuristic
	i := strings.Index(s, "http")
	if i == -1 {
		return ""
	}
	seg := s[i:]
	// cut at space
	if j := strings.IndexByte(seg, ' '); j != -1 {
		seg = seg[:j]
	}
	// cut trailing punctuation
	seg = strings.Trim(seg, "[]()<>\"'")
	return seg
}

// -------- OpenAPI merge --------

func mergeOpenAPI(st State, baseURL string) ([]byte, error) {
	cfg := readConfig(st.ConfigPath)
	if len(cfg.MCPServers) == 0 {
		return nil, fmt.Errorf("no mcpServers in %s", st.ConfigPath)
	}

	merged := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "MCP Tools via mcpo",
			"version": "1.0.0",
		},
		"servers": []any{
			map[string]any{"url": strings.TrimRight(baseURL, "/")},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"mcpoApiKey": map[string]any{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
			},
			"schemas":       map[string]any{},
			"parameters":    map[string]any{},
			"responses":     map[string]any{},
			"requestBodies": map[string]any{},
		},
		"security": []any{
			map[string]any{"mcpoApiKey": []any{}},
		},
		"paths": map[string]any{},
	}

	pathsOut := merged["paths"].(map[string]any)
	comp := merged["components"].(map[string]any)

	// iterate tool names deterministically
	names := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	slices.Sort(names)

	client := &http.Client{Timeout: 30 * time.Second}
	for _, name := range names {
		toolURL := fmt.Sprintf("http://127.0.0.1:%d/%s/openapi.json", st.McpoPort, name)
		req, _ := http.NewRequest("GET", toolURL, nil)
		// authorize if mcpo is gated
		if st.APIKey != "" {
			req.Header.Set("X-API-Key", st.APIKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", toolURL, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("fetch %s: status %s\n%s", toolURL, resp.Status, string(body))
		}
		var spec map[string]any
		if err := json.Unmarshal(body, &spec); err != nil {
			return nil, fmt.Errorf("parse %s: %w", toolURL, err)
		}

		// Prefix $refs for components we are going to rename
		localComp, _ := spec["components"].(map[string]any)
		sections := []string{"schemas", "parameters", "responses", "requestBodies"}
		// Track keys present so we only rewrite refs that exist
		localKeys := map[string]map[string]bool{}
		for _, sec := range sections {
			localKeys[sec] = map[string]bool{}
			if m, ok := localComp[sec].(map[string]any); ok {
				for k := range m {
					localKeys[sec][k] = true
				}
			}
		}
		// Deep copy before rewrite
		spec = deepCopy(spec).(map[string]any)
		rewriteRefs(spec, name, localKeys)

		// Move and rename components
		if localComp != nil {
			for _, sec := range sections {
				src, _ := localComp[sec].(map[string]any)
				dst, _ := comp[sec].(map[string]any)
				if src == nil || dst == nil {
					continue
				}
				for k, v := range src {
					dst[name+"__"+k] = v
				}
			}
		}

		// Merge paths with prefix
		if p, ok := spec["paths"].(map[string]any); ok {
			for rawPath, v := range p {
				newPath := "/" + strings.TrimLeft(name, "/") + ensureLeadingSlash(rawPath)
				// Namespace operationIds
				if m, ok := v.(map[string]any); ok {
					for method, op := range m {
						om, ok := op.(map[string]any)
						if !ok {
							continue
						}
						if oid, ok := om["operationId"].(string); ok && oid != "" {
							om["operationId"] = name + "__" + oid
						} else {
							// synthesize one
							om["operationId"] = name + "__" + strings.ToLower(method) + "_" + sanitizeForID(rawPath)
						}
					}
				}
				pathsOut[newPath] = v
			}
		}
	}

	out, _ := json.MarshalIndent(merged, "", "  ")
	return out, nil
}

func ensureLeadingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[0] != '/' {
		return "/" + s
	}
	return s
}

func sanitizeForID(p string) string {
	// replace non-alphanumerics with underscores
	var b strings.Builder
	for _, r := range p {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func deepCopy(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func rewriteRefs(v any, tool string, localKeys map[string]map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "$ref" {
				if s, ok := val.(string); ok {
					prefixes := []string{"#/components/schemas/", "#/components/parameters/", "#/components/responses/", "#/components/requestBodies/"}
					for _, pref := range prefixes {
						if strings.HasPrefix(s, pref) {
							name := strings.TrimPrefix(s, pref)
							sec := strings.Split(strings.TrimPrefix(pref, "#/components/"), "/")[0]
							if localKeys[sec][name] {
								t[k] = pref + tool + "__" + name
							}
							break
						}
					}
				}
			} else {
				rewriteRefs(val, tool, localKeys)
			}
		}
	case []any:
		for i := range t {
			rewriteRefs(t[i], tool, localKeys)
		}
	}
}
