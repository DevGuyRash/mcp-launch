// Copyright
// SPDX-License-Identifier: MIT
// mcp-launch: minimal supervisor for mcpo + merged OpenAPI + Cloudflare tunnel
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Version = "0.4.2"

const (
	defaultFrontPort = 8000
	defaultMcpoPort  = 8800
	stateDirName     = ".mcp-launch"
	stateFileName    = "state.json"
	defaultConfig    = "mcp.config.json"
)

// ---------- config models ----------

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

// ---------- runtime / state ----------

type Instance struct {
	Name           string   `json:"name"` // derived from config filename
	ConfigPath     string   `json:"config_path"`
	FrontPort      int      `json:"front_port"`
	McpoPort       int      `json:"mcpo_port"`
	APIKey         string   `json:"api_key"`
	PublicURL      string   `json:"public_url"`
	TunnelMode     string   `json:"tunnel_mode"` // quick|named|none
	TunnelName     string   `json:"tunnel_name,omitempty"`
	CloudflaredPID int      `json:"cloudflared_pid"`
	McpoPID        int      `json:"mcpo_pid"`
	ToolNames      []string `json:"tool_names"`
	OperationCount int      `json:"operation_count"` // total OpenAPI operations after merge
}

type State struct {
	// Legacy single-instance fields (kept for backward compatibility)
	APIKey         string   `json:"api_key,omitempty"`
	ConfigPath     string   `json:"config_path,omitempty"`
	FrontPort      int      `json:"front_port,omitempty"`
	McpoPort       int      `json:"mcpo_port,omitempty"`
	PublicURL      string   `json:"public_url,omitempty"`
	TunnelMode     string   `json:"tunnel_mode,omitempty"`
	TunnelName     string   `json:"tunnel_name,omitempty"`
	CloudflaredPID int      `json:"cloudflared_pid,omitempty"`
	McpoPID        int      `json:"mcpo_pid,omitempty"`
	ToolNames      []string `json:"tool_names,omitempty"`

	// Multi-instance (preferred)
	Instances []Instance `json:"instances"`
	StartedAt string     `json:"started_at"`
}

// ---------- CLI ----------

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	switch os.Args[1] {
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
One URL per config for many MCP servers (via mcpo). Serves /openapi.json per stack and proxies everything else to its mcpo.

USAGE
  mcp-launch <command> [options]

COMMANDS
  init         Scaffold mcp.config.json and default state
  up           Start one or more stacks (mcpo + proxy + optional Cloudflare) and generate merged OpenAPI per stack
  status       Show ports, URLs, tools, API keys
  openapi      Regenerate merged OpenAPI for running stacks (uses current/--public-url)
  share        Print the URL(s) you paste into ChatGPT (Custom GPT → Actions → Import from URL)
  down         Stop all stacks (mcpo trees and cloudflared)
  doctor       Check dependencies (mcpo, cloudflared, plus uvx/npx if referenced in config)
  help         Show help (try: mcp-launch help up)
  version      Print version

NOTES
  • Ctrl-C on 'up' will stop all started stacks: mcpo (+ spawned MCP servers), front proxies, and cloudflared.
  • Default output is minimal; use -v or -vv to stream detailed logs. Use --log-file to tee logs to a file.
`)
}

func helpTopic(name string) {
	switch name {
	case "up":
		fmt.Println(`USAGE
  mcp-launch up [--config PATH ...] [--port N] [--mcpo-port N] [--api-key KEY] [--shared-key]
                 [--tunnel quick|named|none] [--public-url URL ...] [--tunnel-name NAME]
                 [-v | -vv] [--stream] [--log-file PATH]

DESCRIPTION
  Starts one or more independent "stacks" (one per --config):
    stack = mcpo(:<mcpo-port+i>) + front proxy(:<port+i>) + optional cloudflared tunnel
  Each stack gets its own merged /openapi.json, URL, and API key (unless --shared-key is used).

OPTIONS
  --config PATH          Repeatable. Claude-style config(s). Default: mcp.config.json if omitted.
  --port N               Base front proxy port (default: 8000). Subsequent stacks use N+1, N+2, ...
  --mcpo-port N          Base internal mcpo port (default: 8800). Subsequent stacks use N+1, N+2, ...
  --api-key KEY          API key. With --shared-key this is used for all stacks; otherwise keys are generated per stack.
  --shared-key           Use a single API key for all stacks (safer default is per-stack keys).
  --tunnel MODE          quick | named | none (default: quick)
  --public-url URL       Repeatable. For named/none, provide one per --config (or one applied to all).
  --tunnel-name NAME     Named tunnel to run (cloudflared tunnel run NAME) for each stack
  -v                     Verbose INFO logs and stream subprocess output
  -vv                    DEBUG logs (also streams subprocess output)
  --stream               Stream subprocess logs without changing verbosity
  --log-file PATH        Append logs to file (created if missing)

WHY MULTI-CONFIG?
  OpenAI Custom GPTs currently support ~30 tools per Action. Split your servers into multiple configs,
  run multiple stacks, and import each /openapi.json in a different GPT/chat.

EXAMPLES
  # Two stacks via Quick Tunnels (independent URLs + keys):
  mcp-launch up \
    --config code.json \
    --config data.json \
    --tunnel quick

  # Two named stacks with stable hosts and a single shared API key:
  mcp-launch up \
    --config code.json --public-url https://gpt-code.example.com \
    --config data.json --public-url https://gpt-data.example.com \
    --tunnel named --tunnel-name my-tunnel \
    --api-key SECRET --shared-key
`)
	case "openapi":
		fmt.Println(`USAGE
  mcp-launch openapi [--public-url URL ...]

DESCRIPTION
  Rebuild the merged OpenAPI document for each running stack from its per-tool specs,
  and set servers[0].url to the provided --public-url(s) (one per stack or one for all)
  or the instance's current public URL if not provided.
`)
	default:
		usage()
	}
}

// ---------- commands ----------

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
		StartedAt: time.Now().Format(time.RFC3339),
		Instances: nil,
	}
	saveState(&st)
	fmt.Println("Initialized .mcp-launch/state.json with defaults")
}

func cmdDoctor() {
	// Read *a* config (if present) to suggest uvx/npx checks; otherwise just mcpo/cloudflared.
	st := loadState()
	cfg := MCPConfig{}
	if len(st.Instances) > 0 {
		cfg = readConfig(st.Instances[0].ConfigPath)
	} else if st.ConfigPath != "" {
		cfg = readConfig(st.ConfigPath)
	} else {
		cfg = readConfig(defaultConfig)
	}
	checks := []string{"mcpo", "cloudflared"}
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
		if bin == "" {
			continue
		}
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

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func cmdUp() {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	fs.Usage = func() { helpTopic("up") }
	var configs stringSlice
	var publicURLs stringSlice
	fs.Var(&configs, "config", "Path to mcpo config file (repeatable)")
	port := fs.Int("port", defaultFrontPort, "Base front HTTP port (serves /openapi.json and proxies to mcpo)")
	mcpoPort := fs.Int("mcpo-port", defaultMcpoPort, "Base internal mcpo port")
	apiKey := fs.String("api-key", "", "API key (used for all stacks if --shared-key)")
	sharedKey := fs.Bool("shared-key", false, "Use a single API key for all stacks")
	fs.Var(&publicURLs, "public-url", "Public base URL (repeatable; align with --config or single for all)")
	tunnelName := fs.String("tunnel-name", "", "Named tunnel to run (optional; requires local cloudflared config)")
	tunnel := fs.String("tunnel", "quick", "Tunnel mode: quick|named|none")
	verbose := fs.Bool("v", false, "Verbose logs (INFO)")
	debug := fs.Bool("vv", false, "Debug logs (DEBUG)")
	stream := fs.Bool("stream", false, "Stream subprocess logs without changing verbosity")
	logPath := fs.String("log-file", "", "Append logs to file (created if missing)")
	_ = fs.Parse(os.Args[2:])

	ensureStateDir()
	st := loadState()

	// Which configs?
	if len(configs) == 0 {
		configs = append(configs, defaultConfig)
	}

	verbosity := 0
	if *debug {
		verbosity = 2
	} else if *verbose {
		verbosity = 1
	}

	streamProcs := *stream || *verbose || *debug

	// Open log file (optional)
	lf, err := openLogFile(*logPath)
	if err != nil {
		fmt.Println("Could not open log file:", err)
	}
	defer func() {
		if lf != nil {
			_ = lf.Close()
		}
	}()

	// Determine API key(s)
	shared := ""
	if *sharedKey {
		if *apiKey != "" {
			shared = *apiKey
		} else if st.APIKey != "" {
			shared = st.APIKey
		} else {
			shared = randomKey(40)
		}
		// Store on legacy field for convenience
		st.APIKey = shared
	}

	// Build instance plans (reserve unique ports per stack)
	instances := make([]Instance, 0, len(configs))
	takenFront := map[int]bool{}
	takenMcpo := map[int]bool{}
	for i, cfgPath := range configs {
		name := nameFromPath(cfgPath, i)
		front := reservePort(*port+i, takenFront)
		mcpoP := reservePort(*mcpoPort+i, takenMcpo)
		inst := Instance{
			Name:       name,
			ConfigPath: cfgPath,
			FrontPort:  front,
			McpoPort:   mcpoP,
			TunnelMode: *tunnel,
			TunnelName: *tunnelName,
		}
		if *sharedKey {
			inst.APIKey = shared
		} else {
			inst.APIKey = randomKey(40)
		}
		// Pre-set public URL if provided
		if len(publicURLs) == 1 {
			inst.PublicURL = strings.TrimRight(publicURLs[0], "/")
		} else if len(publicURLs) > i {
			inst.PublicURL = strings.TrimRight(publicURLs[i], "/")
		}
		instances = append(instances, inst)
	}

	// Start stacks one by one
	type running struct {
		inst   *Instance
		proxy  *frontProxy
		mcpo   *exec.Cmd
		tunnel *exec.Cmd
	}
	runs := make([]*running, 0, len(instances))

	for i := range instances {
		inst := &instances[i]

		// Start mcpo
		mcpoCmd := exec.Command(findBinary("mcpo"),
			"--port", fmt.Sprint(inst.McpoPort),
			"--api-key", inst.APIKey,
			"--config", inst.ConfigPath,
			"--hot-reload",
		)
		if runtime.GOOS != "windows" {
			mcpoCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		}
		stdout, _ := mcpoCmd.StdoutPipe()
		stderr, _ := mcpoCmd.StderrPipe()
		if err := mcpoCmd.Start(); err != nil {
			fmt.Printf("Failed to start mcpo for %s: %v\n", inst.Name, err)
			continue
		}
		inst.McpoPID = mcpoCmd.Process.Pid
		saveStateMulti(&st, instances)

		tag := "mcpo#" + inst.Name
		go scanAndMaybeStream(tag, stdout, streamProcs, lf, nil)
		go scanAndMaybeStream(tag, stderr, streamProcs, lf, nil)

		waitURL(fmt.Sprintf("http://127.0.0.1:%d/docs", inst.McpoPort), 60*time.Second)

		// Record MCP server names from config
		cfg := readConfig(inst.ConfigPath)
		var toolNames []string
		for name := range cfg.MCPServers {
			toolNames = append(toolNames, name)
		}
		slices.Sort(toolNames)
		inst.ToolNames = toolNames
		saveStateMulti(&st, instances)

		// Front proxy
		proxy := newFrontProxy(inst.FrontPort, inst.McpoPort)
		go func(name string, fp *frontProxy) {
			if err := fp.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) && streamProcs {
				fmt.Printf("[front#%s] error: %v\n", name, err)
			}
		}(inst.Name, proxy)
		if verbosity > 0 {
			fmt.Printf("[front#%s] http://127.0.0.1:%d\n", inst.Name, inst.FrontPort)
		}

		// Cloudflare tunnel
		switch inst.TunnelMode {
		case "quick":
			u := startQuickTunnel("cloudflared#"+inst.Name, inst.FrontPort, streamProcs, lf)
			if u == "" {
				if verbosity > 0 {
					fmt.Printf("[tunnel#%s] Quick Tunnel failed; continuing without a public URL.\n", inst.Name)
				}
			} else {
				inst.PublicURL = u
				saveStateMulti(&st, instances)
			}
		case "named":
			if inst.PublicURL == "" && verbosity > 0 {
				fmt.Printf("[tunnel#%s] Named tunnel selected but --public-url not provided. Please pass --public-url https://your.host\n", inst.Name)
			}
			startNamedTunnel("cloudflared#"+inst.Name, inst.TunnelName, streamProcs, lf)
		case "none":
			// no-op
		default:
			if verbosity > 0 {
				fmt.Printf("[tunnel#%s] Unknown tunnel mode: %s\n", inst.Name, inst.TunnelMode)
			}
		}

		// Merge OpenAPI for this instance
		baseURL := inst.PublicURL
		if baseURL == "" {
			baseURL = fmt.Sprintf("http://127.0.0.1:%d", inst.FrontPort)
		}
		spec, err := mergeOpenAPI(*inst, baseURL)
		if err != nil {
			fmt.Printf("[openapi#%s] merge failed: %v\n", inst.Name, err)
		} else {
			inst.OperationCount = countOperations(spec)
			saveStateMulti(&st, instances)
			// quick sanity check: any dangling component refs?
			if warns := findDanglingComponentRefs(spec); len(warns) > 0 {
				fmt.Printf("[openapi#%s] WARNING: unresolved $ref targets detected:\n", inst.Name)
				max := warns
				if len(max) > 8 {
					max = max[:8]
				}
				for _, w := range max {
					fmt.Println("  -", w)
				}
				if len(warns) > len(max) {
					fmt.Printf("  … and %d more\n", len(warns)-len(max))
				}
			}
			proxy.SetOpenAPI(spec)
		}
		runs = append(runs, &running{inst: inst, proxy: proxy, mcpo: mcpoCmd})
	}

	// Minimal “important” output
	fmt.Println()
	if len(runs) == 0 {
		fmt.Println("No stacks started.")
		return
	}
	fmt.Println("=== SHARE THESE WITH CHATGPT (Actions → Import from URL) ===")
	for idx, r := range runs {
		inst := r.inst
		url := inst.PublicURL
		if url == "" {
			url = fmt.Sprintf("http://127.0.0.1:%d", inst.FrontPort)
		}
		fmt.Printf("%d) %s/openapi.json  (config: %s)\n", idx+1, url, filepath.Base(inst.ConfigPath))
		fmt.Printf("   X-API-Key: %s\n", inst.APIKey)
		warn := ""
		switch {
		case inst.OperationCount > 30:
			warn = "  ⚠ OVER 30-limit"
		case inst.OperationCount >= 28:
			warn = "  ⚠ near 30"
		}
		fmt.Printf("   MCP servers: %d\n", len(inst.ToolNames))
		fmt.Printf("   Endpoints (OpenAPI operations): %d%s\n", inst.OperationCount, warn)
	}
	fmt.Println()

	// Handle signals and mcpo exits
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	doneCh := make(chan string, len(runs))
	for _, r := range runs {
		go func(name string, cmd *exec.Cmd) {
			_ = cmd.Wait()
			doneCh <- name
		}(r.inst.Name, r.mcpo)
	}

	if verbosity > 0 {
		fmt.Println("Press Ctrl+C to stop (or run `mcp-launch down` from another shell).")
	}

	cleanup := func() {
		// stop proxies first
		for _, r := range runs {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = r.proxy.Close(ctx)
			cancel()
		}
		// stop cloudflared and mcpo trees
		for i := range instances {
			inst := &instances[i]
			if inst.CloudflaredPID > 0 {
				_ = killPID(inst.CloudflaredPID)
				inst.CloudflaredPID = 0
			}
			if inst.McpoPID > 0 {
				_ = killProcessGroup(inst.McpoPID)
				inst.McpoPID = 0
			}
		}
		saveStateMulti(&st, instances)
	}

	select {
	case <-sigCh:
		fmt.Println("\nReceived signal, shutting down…")
		cleanup()
	case name := <-doneCh:
		fmt.Println("\nmcpo exited for stack:", name)
		cleanup()
	}
}

func cmdStatus() {
	st := loadState()
	if len(st.Instances) == 0 {
		// Legacy single-instance print (if present)
		fmt.Println("mcp-launch status (legacy):")
		fmt.Printf("- Front: http://127.0.0.1:%d\n", st.FrontPort)
		fmt.Printf("- mcpo:  http://127.0.0.1:%d\n", st.McpoPort)
		if st.PublicURL != "" {
			fmt.Println("- Public URL:", st.PublicURL)
		} else {
			fmt.Println("- Public URL: (none)")
		}
		fmt.Println("- Tunnel:", st.TunnelMode)
		if len(st.ToolNames) > 0 {
			fmt.Println("- Tools:", strings.Join(st.ToolNames, ", "))
		}
		if st.APIKey != "" {
			fmt.Println("- API key (X-API-Key):", st.APIKey)
		}
		return
	}

	fmt.Println("mcp-launch status (multi):")
	for i, inst := range st.Instances {
		base := inst.PublicURL
		if base == "" {
			base = fmt.Sprintf("http://127.0.0.1:%d", inst.FrontPort)
		}
		fmt.Printf("[%d] %s\n", i+1, inst.Name)
		fmt.Printf("    Front: %s/openapi.json\n", base)
		fmt.Printf("    mcpo:  http://127.0.0.1:%d\n", inst.McpoPort)
		fmt.Printf("    Tunnel: %s\n", inst.TunnelMode)
		if len(inst.ToolNames) > 0 {
			fmt.Printf("    MCP servers: %d\n", len(inst.ToolNames))
		}
		warn := ""
		switch {
		case inst.OperationCount > 30:
			warn = "  ⚠ OVER 30-limit"
		case inst.OperationCount >= 28:
			warn = "  ⚠ near 30"
		}
		if inst.OperationCount > 0 {
			fmt.Printf("    Endpoints (OpenAPI operations): %d%s\n", inst.OperationCount, warn)
		}
		fmt.Printf("    X-API-Key: %s\n", inst.APIKey)
	}
}

func cmdShare() {
	st := loadState()
	if len(st.Instances) == 0 {
		if st.PublicURL == "" {
			fmt.Printf("Local: http://127.0.0.1:%d/openapi.json\n", st.FrontPort)
			return
		}
		fmt.Println(st.PublicURL + "/openapi.json")
		return
	}
	for _, inst := range st.Instances {
		base := inst.PublicURL
		if base == "" {
			base = fmt.Sprintf("http://127.0.0.1:%d", inst.FrontPort)
		}
		fmt.Printf("%s: %s/openapi.json\n", inst.Name, base)
	}
}

func cmdDown() {
	st := loadState()
	// Prefer multi-instance
	if len(st.Instances) > 0 {
		for i := range st.Instances {
			inst := &st.Instances[i]
			if inst.CloudflaredPID > 0 {
				_ = killPID(inst.CloudflaredPID)
				fmt.Println("Stopped cloudflared (pid", inst.CloudflaredPID, ") for", inst.Name)
				inst.CloudflaredPID = 0
			}
			if inst.McpoPID > 0 {
				_ = killProcessGroup(inst.McpoPID)
				fmt.Println("Stopped mcpo (pid", inst.McpoPID, ") and its child MCP servers for", inst.Name)
				inst.McpoPID = 0
			}
		}
		saveState(&st)
		return
	}
	// Fallback legacy
	if st.CloudflaredPID > 0 {
		_ = killPID(st.CloudflaredPID)
		fmt.Println("Stopped cloudflared (pid", st.CloudflaredPID, ")")
		st.CloudflaredPID = 0
	}
	if st.McpoPID > 0 {
		_ = killProcessGroup(st.McpoPID)
		fmt.Println("Stopped mcpo (pid", st.McpoPID, ") and its child MCP servers")
		st.McpoPID = 0
	}
	saveState(&st)
}

func cmdOpenAPI() {
	fs := flag.NewFlagSet("openapi", flag.ExitOnError)
	fs.Usage = func() { helpTopic("openapi") }
	var publicURLs stringSlice
	fs.Var(&publicURLs, "public-url", "Public base URL (repeatable; align with running stacks or one for all)")
	_ = fs.Parse(os.Args[2:])

	st := loadState()
	if len(st.Instances) == 0 {
		fmt.Println("No running stacks found in state.")
		return
	}

	for i := range st.Instances {
		inst := &st.Instances[i]
		baseURL := inst.PublicURL
		if len(publicURLs) == 1 {
			baseURL = strings.TrimRight(publicURLs[0], "/")
		} else if len(publicURLs) > i {
			baseURL = strings.TrimRight(publicURLs[i], "/")
		}
		if baseURL == "" {
			baseURL = fmt.Sprintf("http://127.0.0.1:%d", inst.FrontPort)
		}
		spec, err := mergeOpenAPI(*inst, baseURL)
		if err != nil {
			fmt.Printf("[openapi#%s] merge failed: %v\n", inst.Name, err)
			continue
		}
		// warn if dangling refs
		if warns := findDanglingComponentRefs(spec); len(warns) > 0 {
			fmt.Printf("[openapi#%s] WARNING: unresolved $ref targets detected:\n", inst.Name)
			for _, w := range warns {
				fmt.Println("  -", w)
			}
		}
		out := filepath.Join(getStateDir(), fmt.Sprintf("openapi_%s.json", inst.Name))
		_ = os.WriteFile(out, spec, 0644)
		fmt.Printf("Wrote merged OpenAPI for %s to %s\n", inst.Name, out)
		fmt.Printf("Serve URL (if front proxy running): http://127.0.0.1:%d/openapi.json\n", inst.FrontPort)
	}
}

// ---------- helpers ----------

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

func getStateDir() string { return filepath.Join(".", stateDirName) }
func statePath() string   { return filepath.Join(getStateDir(), stateFileName) }

func saveState(st *State) {
	_ = os.MkdirAll(getStateDir(), 0o755)
	data, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(statePath(), data, 0644)
}

func saveStateMulti(st *State, instances []Instance) {
	st.Instances = instances
	saveState(st)
}

func loadState() State {
	path := statePath()
	var st State
	data, err := os.ReadFile(path)
	if err != nil {
		// default empty state
		st = State{
			StartedAt: time.Now().Format(time.RFC3339),
			Instances: nil,
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

var logFileMu sync.Mutex

func openLogFile(path string) (*os.File, error) {
	if path == "" {
		return nil, nil
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "=== mcp-launch %s started at %s ===\n", Version, time.Now().Format(time.RFC3339))
	return f, nil
}

func scanAndMaybeStream(tag string, r io.Reader, stream bool, logFile *os.File, onLine func(string)) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if stream {
			fmt.Printf("[%s] %s\n", tag, line)
		}
		if logFile != nil {
			logFileMu.Lock()
			_, _ = fmt.Fprintf(logFile, "[%s] %s\n", tag, line)
			logFileMu.Unlock()
		}
		if onLine != nil {
			onLine(line)
		}
	}
}

func randomKey(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
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

// Reserve an actually-free TCP port >= start that hasn't already been picked for another stack in this run.
func reservePort(start int, taken map[int]bool) int {
	p := start
	for tries := 0; tries < 4096; tries++ {
		if !taken[p] && isFree(p) {
			taken[p] = true
			return p
		}
		p++
	}
	taken[start] = true
	return start
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
	srv   *http.Server
	proxy *httputil.ReverseProxy
	mu    sync.RWMutex
	spec  []byte // merged openapi
}

func newFrontProxy(frontPort, mcpoPort int) *frontProxy {
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", mcpoPort))
	p := httputil.NewSingleHostReverseProxy(target)
	fp := &frontProxy{proxy: p}

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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r)
	})

	fp.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", frontPort),
		Handler: mux,
	}
	return fp
}

func (f *frontProxy) Serve() error { return f.srv.ListenAndServe() }

func (f *frontProxy) SetOpenAPI(spec []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spec = spec
}

func (f *frontProxy) Close(ctx context.Context) error { return f.srv.Shutdown(ctx) }

func startQuickTunnel(tag string, frontPort int, stream bool, logFile *os.File) string {
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
	for i := range st.Instances {
		if st.Instances[i].FrontPort == frontPort {
			st.Instances[i].CloudflaredPID = cmd.Process.Pid
			break
		}
	}
	saveState(&st)

	// Parse URL from output
	urlCh := make(chan string, 1)
	parse := func(line string) {
		if strings.Contains(line, "trycloudflare.com") {
			u := findFirstURL(line)
			if u != "" {
				urlCh <- u
			}
		}
	}
	go scanAndMaybeStream(tag, stdout, stream, logFile, parse)
	go scanAndMaybeStream(tag, stderr, stream, logFile, parse)

	select {
	case u := <-urlCh:
		return strings.TrimSuffix(u, "/")
	case <-time.After(25 * time.Second):
		return ""
	}
}

func startNamedTunnel(tag, name string, stream bool, logFile *os.File) {
	args := []string{"tunnel", "run"}
	if name != "" {
		args = append(args, name)
	}
	cmd := exec.Command(findBinary("cloudflared"), args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	_ = cmd.Start()
	go scanAndMaybeStream(tag, stdout, stream, logFile, nil)
	go scanAndMaybeStream(tag, stderr, stream, logFile, nil)

	// save PID
	st := loadState()
	for i := range st.Instances {
		if st.Instances[i].CloudflaredPID == 0 {
			st.Instances[i].CloudflaredPID = cmd.Process.Pid
			break
		}
	}
	saveState(&st)
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
	}
	pr, err := os.FindProcess(pid)
	if err == nil {
		_ = pr.Signal(syscall.SIGTERM)
		time.Sleep(300 * time.Millisecond)
	}
	return nil
}

// killProcessGroup kills a process and (on Unix) its entire process group.
func killProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
	}
	// Send SIGTERM to process group (-pid)
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	time.Sleep(800 * time.Millisecond)
	// If still alive, SIGKILL the group
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return nil
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
	i := strings.Index(s, "http")
	if i == -1 {
		return ""
	}
	seg := s[i:]
	if j := strings.IndexByte(seg, ' '); j != -1 {
		seg = seg[:j]
	}
	seg = strings.Trim(seg, "[]()<>\"'")
	return seg
}

func nameFromPath(p string, i int) string {
	base := filepath.Base(p)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		return fmt.Sprintf("group%d", i+1)
	}
	// keep simple characters
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, base)
	return base
}

// -------- OpenAPI merge --------

func mergeOpenAPI(inst Instance, baseURL string) ([]byte, error) {
	cfg := readConfig(inst.ConfigPath)
	if len(cfg.MCPServers) == 0 {
		return nil, fmt.Errorf("no mcpServers in %s", inst.ConfigPath)
	}

	merged := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "MCP Tools via mcpo (" + inst.Name + ")",
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
		toolURL := fmt.Sprintf("http://127.0.0.1:%d/%s/openapi.json", inst.McpoPort, name)
		req, _ := http.NewRequest("GET", toolURL, nil)
		if inst.APIKey != "" {
			req.Header.Set("X-API-Key", inst.APIKey)
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

		// Collect local component keys from the ORIGINAL to know which refs are local.
		origComp, _ := spec["components"].(map[string]any)
		sections := []string{"schemas", "parameters", "responses", "requestBodies"}
		localKeys := map[string]map[string]bool{}
		for _, sec := range sections {
			localKeys[sec] = map[string]bool{}
			if m, ok := origComp[sec].(map[string]any); ok {
				for k := range m {
					localKeys[sec][k] = true
				}
			}
		}

		// Work on a deep copy and rewrite all $refs to namespaced form BEFORE moving anything.
		spec = deepCopy(spec).(map[string]any)
		rewriteRefs(spec, name, localKeys)

		// Move components from the rewritten copy (so nested $refs stay namespaced).
		localComp, _ := spec["components"].(map[string]any)
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

		// Merge paths with prefix; remove per-op security (rely on top-level).
		if p, ok := spec["paths"].(map[string]any); ok {
			for rawPath, v := range p {
				newPath := "/" + strings.TrimLeft(name, "/") + ensureLeadingSlash(rawPath)
				if m, ok := v.(map[string]any); ok {
					for method, op := range m {
						om, ok := op.(map[string]any)
						if !ok {
							continue
						}
						if oid, ok := om["operationId"].(string); ok && oid != "" {
							om["operationId"] = name + "__" + oid
						} else {
							om["operationId"] = name + "__" + strings.ToLower(method) + "_" + sanitizeForID(rawPath)
						}
						// Cleanup: remove any per-operation security (duplicate of top-level).
						delete(om, "security")
					}
				}
				pathsOut[newPath] = v
			}
		}
	}

	// Global cleanups:
	//  - tighten empty response schemas
	//  - coerce obvious integer-like number types
	if paths, ok := merged["paths"].(map[string]any); ok {
		tightenResponses(paths)
	}
	coerceIntegerTypes(merged)

	out, _ := json.MarshalIndent(merged, "", "  ")
	return out, nil
}

// rewriteRefs recursively rewrites local component $refs to namespaced form "<tool>__Name".
func rewriteRefs(v any, tool string, localKeys map[string]map[string]bool) {
	switch node := v.(type) {
	case map[string]any:
		if ref, ok := node["$ref"].(string); ok {
			if newRef := rewriteRefString(ref, tool, localKeys); newRef != ref {
				node["$ref"] = newRef
			}
		}
		for k, child := range node {
			if k == "$ref" {
				continue
			}
			rewriteRefs(child, tool, localKeys)
		}
	case []any:
		for i := range node {
			rewriteRefs(node[i], tool, localKeys)
		}
	}
}

func rewriteRefString(ref, tool string, localKeys map[string]map[string]bool) string {
	const base = "#/components/"
	if !strings.HasPrefix(ref, base) {
		return ref
	}
	rest := strings.TrimPrefix(ref, base) // e.g., "schemas/ValidationError"
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return ref
	}
	section, name := parts[0], parts[1]
	if secs, ok := localKeys[section]; ok && secs[name] {
		return base + section + "/" + tool + "__" + name
	}
	return ref
}

// Count total OpenAPI operations under .paths
func countOperations(spec []byte) int {
	var m map[string]any
	if err := json.Unmarshal(spec, &m); err != nil {
		return 0
	}
	paths, _ := m["paths"].(map[string]any)
	if paths == nil {
		return 0
	}
	methods := map[string]struct{}{
		"get": {}, "post": {}, "put": {}, "delete": {}, "patch": {},
		"options": {}, "head": {}, "trace": {}, "connect": {},
	}
	count := 0
	for _, v := range paths {
		if mm, ok := v.(map[string]any); ok {
			for mk := range mm {
				if _, ok := methods[strings.ToLower(mk)]; ok {
					count++
				}
			}
		}
	}
	return count
}

// Find unresolved component $ref targets for quick diagnostics.
func findDanglingComponentRefs(spec []byte) []string {
	var m map[string]any
	if err := json.Unmarshal(spec, &m); err != nil {
		return nil
	}
	comp, _ := m["components"].(map[string]any)
	if comp == nil {
		return nil
	}
	sections := []string{"schemas", "parameters", "responses", "requestBodies"}
	have := map[string]map[string]bool{}
	for _, sec := range sections {
		have[sec] = map[string]bool{}
		if mm, ok := comp[sec].(map[string]any); ok {
			for k := range mm {
				have[sec][k] = true
			}
		}
	}
	var warns []string
	seen := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			if ref, ok := n["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/") {
				rest := strings.TrimPrefix(ref, "#/components/")
				parts := strings.SplitN(rest, "/", 2)
				if len(parts) == 2 {
					sec, name := parts[0], parts[1]
					if sect, ok := have[sec]; ok {
						if !sect[name] {
							msg := fmt.Sprintf("%s not found (section=%s)", ref, sec)
							if !seen[msg] {
								seen[msg] = true
								warns = append(warns, msg)
							}
						}
					}
				}
			}
			for k, child := range n {
				if k == "$ref" {
					continue
				}
				walk(child)
			}
		case []any:
			for i := range n {
				walk(n[i])
			}
		}
	}
	walk(m)
	return warns
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

// ---------- OpenAPI Cleanups (agnostic) ----------

// tightenResponses removes empty schemas ({}), collapses anyOf that include {},
// and deletes empty content blocks. It does not change status codes.
func tightenResponses(paths map[string]any) {
	for _, v := range paths {
		ops, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for _, ov := range ops {
			op, ok := ov.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := op["responses"].(map[string]any)
			if !ok {
				continue
			}
			for code, rv := range responses {
				rmap, ok := rv.(map[string]any)
				if !ok {
					continue
				}
				// Ensure there's at least a description.
				if _, ok := rmap["description"]; !ok {
					rmap["description"] = "Successful Response"
				}
				content, ok := rmap["content"].(map[string]any)
				if !ok || len(content) == 0 {
					responses[code] = rmap
					continue
				}
				for ctype, cv := range content {
					cm, ok := cv.(map[string]any)
					if !ok {
						continue
					}
					if schema, ok := cm["schema"]; ok {
						clean := cleanupSchemaNode(schema)
						if clean == nil {
							// remove this media type entirely
							delete(content, ctype)
						} else {
							cm["schema"] = clean
						}
					}
				}
				if len(content) == 0 {
					delete(rmap, "content")
				}
				responses[code] = rmap
			}
		}
	}
}

// cleanupSchemaNode returns a cleaned schema node or nil if it becomes empty.
func cleanupSchemaNode(schema any) any {
	switch n := schema.(type) {
	case map[string]any:
		// Empty object {} → nil
		if len(n) == 0 {
			return nil
		}
		// anyOf with {} entries → drop empty ones; collapse to single if only one remains
		if anyOf, ok := n["anyOf"].([]any); ok {
			filtered := make([]any, 0, len(anyOf))
			for _, it := range anyOf {
				if mm, ok := it.(map[string]any); ok && len(mm) == 0 {
					continue
				}
				filtered = append(filtered, it)
			}
			if len(filtered) == 0 {
				return nil
			}
			if len(filtered) == 1 {
				return filtered[0]
			}
			n["anyOf"] = filtered
			return n
		}
		// Recurse into children
		for k, v := range n {
			if k == "$ref" {
				continue
			}
			if cleaned := cleanupSchemaNode(v); cleaned == nil {
				// If a child becomes nil and was a compositional key, handle lightly; otherwise keep structure.
				// We won't remove arbitrary keys to avoid over-aggressive pruning.
			} else {
				n[k] = cleaned
			}
		}
		return n
	case []any:
		out := make([]any, 0, len(n))
		for _, it := range n {
			if cleaned := cleanupSchemaNode(it); cleaned != nil {
				out = append(out, cleaned)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return n
	}
}

// coerceIntegerTypes walks the entire document and converts obvious integer-like
// schemas from "number" → "integer" when safe (integral default/enum/multipleOf).
func coerceIntegerTypes(root any) {
	switch node := root.(type) {
	case map[string]any:
		// Detect and coerce this schema if applicable.
		if t, ok := node["type"].(string); ok && t == "number" {
			if shouldBeInteger(node) {
				node["type"] = "integer"
				// intentionally no format guess (keeps it generic)
			}
		}
		for k, v := range node {
			if k == "$ref" {
				continue
			}
			coerceIntegerTypes(v)
		}
	case []any:
		for i := range node {
			coerceIntegerTypes(node[i])
		}
	}
}

func shouldBeInteger(schema map[string]any) bool {
	// default integral?
	if dv, ok := schema["default"]; ok {
		if isIntegralNumber(dv) {
			return true
		}
	}
	// enum all integral?
	if ev, ok := schema["enum"].([]any); ok && len(ev) > 0 {
		allInt := true
		for _, e := range ev {
			if !isIntegralNumber(e) {
				allInt = false
				break
			}
		}
		if allInt {
			return true
		}
	}
	// multipleOf integral?
	if mv, ok := schema["multipleOf"]; ok && isIntegralNumber(mv) {
		return true
	}
	return false
}

func isIntegralNumber(v any) bool {
	switch n := v.(type) {
	case float64:
		return math.Trunc(n) == n
	case int, int32, int64:
		return true
	default:
		return false
	}
}
