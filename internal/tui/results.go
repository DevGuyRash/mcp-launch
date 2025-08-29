package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ResultInstance struct {
	Name            string
	OpenAPI         string
	APIKeyMasked    string
	APIKeyRaw       string
	ServersCount    int
	Endpoints       int
	PerServerCounts map[string]int
	LongDescCounts  map[string]int
	Warnings        []string
	ConfigPath      string
}

type resultsModel struct {
	items     []ResultInstance
	showLogs  bool
	logs      []string
	ch        <-chan string
	logOffset int
	width     int
	wrapLogs  bool
	sel       int
	status    string
	frozen    bool
	frozenBuf []string
	// search state
	searching  bool
	searchBuf  string
	searchIdxs []int
	searchPos  int
	// live updates
	upch <-chan ResultUpdate
}

type logMsg string

func waitLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return nil
		}
		return logMsg(s)
	}
}

func ShowResults(items []ResultInstance, verbose bool, logCh <-chan string) error {
	m := resultsModel{items: items, showLogs: false, logs: []string{}, ch: logCh}
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m resultsModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.ch != nil {
		cmds = append(cmds, waitLog(m.ch))
	}
	if m.upch != nil {
		cmds = append(cmds, waitItem(m.upch))
	}
	return tea.Batch(cmds...)
}

func (m resultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		k := strings.ToLower(v.String())
		if m.searching {
			switch k {
			case "enter":
				m.searching = false
				m.computeSearch()
				m.jumpToResult(0)
				return m, nil
			case "esc", "b":
				m.searching = false
				m.searchBuf = ""
				m.searchIdxs = nil
				m.searchPos = 0
				return m, nil
			default:
				if v.Type == tea.KeyBackspace || v.Type == tea.KeyCtrlH {
					if n := len([]rune(m.searchBuf)); n > 0 {
						r := []rune(m.searchBuf)
						m.searchBuf = string(r[:n-1])
					}
				} else if v.Type == tea.KeyRunes {
					m.searchBuf += string(v.Runes)
				}
				return m, nil
			}
		}
		switch k {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "l":
			m.showLogs = !m.showLogs
			return m, nil
		case "j", "down":
			if m.showLogs {
				if m.logOffset < len(m.logs) {
					m.logOffset++
				}
			} else if m.sel < len(m.items)-1 {
				m.sel++
			}
			return m, nil
		case "k", "up":
			if m.showLogs {
				if m.logOffset > 0 {
					m.logOffset--
				}
			} else if m.sel > 0 {
				m.sel--
			}
			return m, nil
		case "J", "end":
			if m.showLogs {
				m.logOffset += 5
				if m.logOffset > len(m.logs) {
					m.logOffset = len(m.logs)
				}
			} else {
				m.sel = len(m.items) - 1
			}
			return m, nil
		case "K", "home":
			if m.showLogs {
				m.logOffset -= 5
				if m.logOffset < 0 {
					m.logOffset = 0
				}
			} else {
				m.sel = 0
			}
			return m, nil
		case "w":
			m.wrapLogs = !m.wrapLogs
			return m, nil
		case "/":
			m.searching = true
			m.searchBuf = ""
			m.showLogs = true
			return m, nil
		case "n":
			if len(m.searchIdxs) == 0 && strings.TrimSpace(m.searchBuf) != "" {
				m.computeSearch()
			}
			if len(m.searchIdxs) > 0 {
				m.jumpToResult(m.searchPos + 1)
			}
			return m, nil
		case "N":
			if len(m.searchIdxs) == 0 && strings.TrimSpace(m.searchBuf) != "" {
				m.computeSearch()
			}
			if len(m.searchIdxs) > 0 {
				m.jumpToResult(m.searchPos - 1)
			}
			return m, nil
		case "s":
			// save logs (lowercase s to avoid collision); also accept uppercase S
			fallthrough
		case "S":
			if path, err := m.saveLogs(); err == nil {
				m.status = "Saved logs to " + path
			} else {
				m.status = "Save failed: " + err.Error()
			}
			return m, nil
		case "f":
			m.frozen = !m.frozen
			if !m.frozen && len(m.frozenBuf) > 0 {
				m.logs = append(m.logs, m.frozenBuf...)
				m.frozenBuf = nil
			}
			if m.frozen {
				m.status = "Logs frozen"
			} else {
				m.status = "Logs resumed"
			}
			return m, nil
		case "r":
			if m.sel >= 0 && m.sel < len(m.items) {
				m.status = fmt.Sprintf("Restart requested: %s", m.items[m.sel].Name)
			}
			return m, nil
		case "R":
			m.status = "Restart all requested"
			return m, nil
		}
	case logMsg:
		if m.frozen {
			m.frozenBuf = append(m.frozenBuf, string(v))
		} else {
			m.logs = append(m.logs, string(v))
		}
		return m, waitLog(m.ch)
	case itemUpdateMsg:
		if v.Idx >= 0 && v.Idx < len(m.items) {
			it := m.items[v.Idx]
			// apply set-on-non-empty semantics
			if v.OpenAPI != "" {
				it.OpenAPI = v.OpenAPI
			}
			if v.APIKeyRaw != "" {
				it.APIKeyRaw = v.APIKeyRaw
			}
			if v.APIKeyMasked != "" {
				it.APIKeyMasked = v.APIKeyMasked
			}
			if v.ConfigPath != "" {
				it.ConfigPath = v.ConfigPath
			}
			if v.ServersCount >= 0 {
				it.ServersCount = v.ServersCount
			}
			if v.Endpoints >= 0 {
				it.Endpoints = v.Endpoints
			}
			if v.PerServerCounts != nil {
				it.PerServerCounts = v.PerServerCounts
			}
			if v.LongDescCounts != nil {
				it.LongDescCounts = v.LongDescCounts
			}
			m.items[v.Idx] = it
		}
		return m, waitItem(m.upch)
	case tea.WindowSizeMsg:
		if v.Width > 0 {
			m.width = v.Width
		}
		return m, nil
	}
	return m, nil
}

func (m resultsModel) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render("Results (Share with ChatGPT)")
	b.WriteString(title + "\n")
	for i, it := range m.items {
		hdr := fmt.Sprintf("%d) %s", i+1, it.OpenAPI)
		if i == m.sel {
			hdr = selStyle.Render(hdr)
		}
		b.WriteString(hdr + "\n")
		b.WriteString(fmt.Sprintf("    %-16s %s\n", "X-API-Key:", it.APIKeyMasked))
		b.WriteString(fmt.Sprintf("    %-16s %s\n", "Config:", it.ConfigPath))
		warn := ""
		if it.Endpoints > 30 {
			warn = "  ⚠ OVER 30-limit"
		} else if it.Endpoints >= 28 {
			warn = "  ⚠ near 30"
		}
		b.WriteString(fmt.Sprintf("    %-16s %d\n", "MCP servers:", it.ServersCount))
		b.WriteString(fmt.Sprintf("    %-16s %d%s\n", "Endpoints:", it.Endpoints, warn))
		if len(it.PerServerCounts) > 0 {
			// single-line per server
			for name, cnt := range it.PerServerCounts {
				long := 0
				if it.LongDescCounts != nil {
					long = it.LongDescCounts[name]
				}
				suffix := ""
				if long > 0 {
					suffix = "  ⚠ tool descriptions >300"
				}
				b.WriteString(fmt.Sprintf("     - %s: %d tools%s\n", name, cnt, suffix))
			}
		}
		b.WriteString("    --- copy ---\n")
		b.WriteString(fmt.Sprintf("    OPENAPI=%s\n", it.OpenAPI))
		b.WriteString(fmt.Sprintf("    API_KEY=%s\n\n", it.APIKeyRaw))
	}
	status := ""
	if m.searching {
		status = fmt.Sprintf("  /%s", m.searchBuf)
	} else if len(m.searchIdxs) > 0 {
		status = fmt.Sprintf("  [%d/%d]", m.searchPos+1, len(m.searchIdxs))
	}
	b.WriteString("(j/k select) (r) restart (R) all (l) logs (/) search (n/N) next (S) save (f) freeze (q) quit" + status + "\n")
	if strings.TrimSpace(m.status) != "" {
		b.WriteString(faintStyle.Render(m.status) + "\n")
	}
	b.WriteString("\n")
	if m.showLogs {
		if m.searching {
			// search bar above logs
			sb := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
			b.WriteString(sb.Render("Search: "+m.searchBuf) + "\n")
		}
		border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).MarginTop(1)
		// Show last N lines (e.g., 12) with offset
		const maxLines = 12
		start := len(m.logs) - maxLines - m.logOffset
		if start < 0 {
			start = 0
		}
		end := start + maxLines
		if end > len(m.logs) {
			end = len(m.logs)
		}
		// truncate lines to fit width
		avail := m.width
		if avail <= 0 {
			avail = 100
		}
		// account for border padding ~2
		avail -= 4
		if avail < 20 {
			avail = 20
		}
		lines := make([]string, 0, end-start)
		for i, ln := range m.logs[start:end] {
			idx := start + i
			if m.wrapLogs {
				lines = append(lines, m.highlight(ln, idx))
			} else {
				r := []rune(ln)
				if len(r) > avail {
					ln = string(r[:avail-1]) + "…"
				}
				lines = append(lines, m.highlight(ln, idx))
			}
		}
		content := strings.Join(lines, "\n")
		b.WriteString(border.Render(content))
	}
	return b.String()
}

// computeSearch builds indexes of lines containing searchBuf (case-insensitive)
func (m *resultsModel) computeSearch() {
	m.searchIdxs = nil
	m.searchPos = 0
	q := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if q == "" {
		return
	}
	for i, ln := range m.logs {
		if strings.Contains(strings.ToLower(ln), q) {
			m.searchIdxs = append(m.searchIdxs, i)
		}
	}
}

func (m *resultsModel) jumpToResult(pos int) {
	if len(m.searchIdxs) == 0 {
		return
	}
	if pos < 0 {
		pos = len(m.searchIdxs) - 1
	}
	if pos >= len(m.searchIdxs) {
		pos = 0
	}
	m.searchPos = pos
	// show the line near bottom of panel
	const maxLines = 12
	idx := m.searchIdxs[m.searchPos]
	// set offset so that end = idx+1; start=end-maxLines
	end := idx + 1
	start := end - maxLines
	if start < 0 {
		start = 0
	}
	m.logOffset = len(m.logs) - end
	if m.logOffset < 0 {
		m.logOffset = 0
	}
}

var highlightStyle = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "228", Dark: "94"}).Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "0"})

func (m *resultsModel) highlight(s string, idx int) string {
	q := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if q == "" || len(m.searchIdxs) == 0 {
		return s
	}
	if !containsIndex(m.searchIdxs, idx) {
		return s
	}
	// naive multi-match highlight
	lower := strings.ToLower(s)
	out := s
	off := 0
	for {
		p := strings.Index(lower[off:], q)
		if p < 0 {
			break
		}
		p += off
		// apply highlight to rune ranges; for simplicity, operate on bytes (safe for ASCII logs)
		out = out[:p] + highlightStyle.Render(out[p:p+len(q)]) + out[p+len(q):]
		off = p + len(highlightStyle.Render(out[p:p+len(q)]))
		if off >= len(out) {
			break
		}
	}
	return out
}

func containsIndex(a []int, x int) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

func (m *resultsModel) saveLogs() (string, error) {
	dir := filepath.Join(".mcp-launch", "logs")
	_ = os.MkdirAll(dir, 0o755)
	name := time.Now().Format("20060102_150405") + ".log"
	path := filepath.Join(dir, name)
	data := strings.Join(m.logs, "\n")
	return path, os.WriteFile(path, []byte(data), 0644)
}

func mask(s string) string { return s }

// Live updates support
type ResultUpdate struct {
	Idx             int
	OpenAPI         string
	APIKeyMasked    string
	APIKeyRaw       string
	ServersCount    int
	Endpoints       int
	PerServerCounts map[string]int
	LongDescCounts  map[string]int
	ConfigPath      string
}

type itemUpdateMsg ResultUpdate

func waitItem(ch <-chan ResultUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return nil
		}
		return itemUpdateMsg(u)
	}
}

// ShowResultsLive starts the Results TUI immediately and applies updates and logs as they stream in.
func ShowResultsLive(items []ResultInstance, logCh <-chan string, updates <-chan ResultUpdate) error {
	m := resultsModel{items: items, ch: logCh, upch: updates}
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
