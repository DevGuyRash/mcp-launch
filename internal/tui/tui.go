package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	mcpclient "mcp-launch/internal/mcpclient"
)

// Overlay is the edit overlay returned by the TUI.
// main.go copies these fields into its own overlay type.
type Overlay struct {
	Disabled     map[string]bool              // server -> disabled?
	Allow        map[string]map[string]bool   // server -> tool -> true
	Deny         map[string]map[string]bool   // server -> tool -> true
	Descriptions map[string]map[string]string // server -> tool -> descOverride
}

// Run shows a lightweight TUI that lets the user edit server/tool filters and choose a launch mode.
// It returns the overlay (or nil if cancelled), the launch mode ("mcpo" or "raw"), and an error.
func Run(servers map[string][]mcpclient.Tool) (*Overlay, string, error) {
	m := newModel(servers)
	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		return nil, "", err
	}
	return m.overlay, m.launchMode(), nil
}

// ===== Model =====

type mode string

const (
	modeList   mode = "list"   // choose server
	modeMenu   mode = "menu"   // edit menu for a server
	modeAllow  mode = "allow"  // edit allowed tools
	modeDeny   mode = "deny"   // edit disallowed tools
	modeDesc   mode = "desc"   // edit descriptions
	modeLaunch mode = "launch" // choose launch mode
)

type model struct {
	// data
	servers  map[string][]mcpclient.Tool
	names    []string
	origDesc map[string]map[string]string // server -> tool -> original description

	// overlay to return
	overlay *Overlay

	// ui state
	cursorServer int
	cursorTool   int
	curServer    string
	curTools     []string // current server's tools (names)

	mode   mode
	launch string // "mcpo" (default) or "raw"

	// ephemeral editor selection state for allow/deny
	editSelect map[string]bool // tool -> selected?
}

func newModel(servers map[string][]mcpclient.Tool) model {
	names := make([]string, 0, len(servers))
	orig := make(map[string]map[string]string, len(servers))
	for k, tools := range servers {
		names = append(names, k)
		dm := make(map[string]string, len(tools))
		for _, t := range tools {
			dm[t.Name] = t.Description
		}
		orig[k] = dm
	}
	sort.Strings(names)
	return model{
		servers:  servers,
		names:    names,
		origDesc: orig,
		overlay: &Overlay{
			Disabled:     map[string]bool{},
			Allow:        map[string]map[string]bool{},
			Deny:         map[string]map[string]bool{},
			Descriptions: map[string]map[string]string{},
		},
		mode:   modeList,
		launch: "mcpo",
	}
}

func (m model) Init() tea.Cmd { return nil }

// Update handles all TUI interactions.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := strings.ToLower(msg.String())

		switch m.mode {
		case modeList:
			switch k {
			case "q", "ctrl+c":
				// cancel: return nil overlay so caller can abort
				m.overlay = nil
				return m, tea.Quit
			case "up", "k":
				if m.cursorServer > 0 {
					m.cursorServer--
				}
			case "down", "j":
				if m.cursorServer < len(m.names)-1 {
					m.cursorServer++
				}
			case "enter":
				if len(m.names) == 0 {
					return m, nil
				}
				m.curServer = m.names[m.cursorServer]
				m.mode = modeMenu
				return m, nil
			case "c":
				m.mode = modeLaunch
				return m, nil
			}

		case modeMenu:
			switch k {
			case "q", "ctrl+c":
				m.overlay = nil
				return m, tea.Quit
			case "b", "esc":
				m.mode = modeList
				return m, nil
			case "1":
				// Allowed tools editor
				m.prepareToolEditor(true)
				m.mode = modeAllow
				return m, nil
			case "2":
				// Disallowed tools editor
				m.prepareToolEditor(false)
				m.mode = modeDeny
				return m, nil
			case "3":
				m.prepareToolEditor(true) // just for ordering
				m.mode = modeDesc
				return m, nil
			case "4":
				// Disable/Enable server toggle
				m.overlay.Disabled[m.curServer] = !m.overlay.Disabled[m.curServer]
				return m, nil
			case "c":
				m.mode = modeLaunch
				return m, nil
			}

		case modeAllow:
			switch k {
			case "q", "ctrl+c":
				m.overlay = nil
				return m, tea.Quit
			case "b", "esc":
				m.mode = modeMenu
				return m, nil
			case "up", "k":
				if m.cursorTool > 0 {
					m.cursorTool--
				}
			case "down", "j":
				if m.cursorTool < len(m.curTools)-1 {
					m.cursorTool++
				}
			case " ":
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					m.editSelect[t] = !m.editSelect[t]
				}
			case "enter":
				// commit: if all selected, clear explicit allow map
				if m.overlay.Allow[m.curServer] == nil {
					m.overlay.Allow[m.curServer] = map[string]bool{}
				}
				all := true
				for _, t := range m.curTools {
					if !m.editSelect[t] {
						all = false
						break
					}
				}
				if all {
					// means "allow all": clear explicit allow list
					delete(m.overlay.Allow, m.curServer)
				} else {
					am := map[string]bool{}
					for _, t := range m.curTools {
						if m.editSelect[t] {
							am[t] = true
						}
					}
					m.overlay.Allow[m.curServer] = am
				}
				m.mode = modeMenu
				return m, nil
			}

		case modeDeny:
			switch k {
			case "q", "ctrl+c":
				m.overlay = nil
				return m, tea.Quit
			case "b", "esc":
				m.mode = modeMenu
				return m, nil
			case "up", "k":
				if m.cursorTool > 0 {
					m.cursorTool--
				}
			case "down", "j":
				if m.cursorTool < len(m.curTools)-1 {
					m.cursorTool++
				}
			case " ":
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					m.editSelect[t] = !m.editSelect[t]
				}
			case "enter":
				// commit deny list
				if m.overlay.Deny[m.curServer] == nil {
					m.overlay.Deny[m.curServer] = map[string]bool{}
				}
				dm := map[string]bool{}
				for _, t := range m.curTools {
					if m.editSelect[t] {
						dm[t] = true
					}
				}
				if len(dm) == 0 {
					delete(m.overlay.Deny, m.curServer)
				} else {
					m.overlay.Deny[m.curServer] = dm
				}
				m.mode = modeMenu
				return m, nil
			}

		case modeDesc:
			switch k {
			case "q", "ctrl+c":
				m.overlay = nil
				return m, tea.Quit
			case "b", "esc":
				m.mode = modeMenu
				return m, nil
			case "up", "k":
				if m.cursorTool > 0 {
					m.cursorTool--
				}
			case "down", "j":
				if m.cursorTool < len(m.curTools)-1 {
					m.cursorTool++
				}
			case "+": // set trimmed override (<=300)
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					raw := m.origDesc[m.curServer][t]
					trim := trim300(raw)
					if m.overlay.Descriptions[m.curServer] == nil {
						m.overlay.Descriptions[m.curServer] = map[string]string{}
					}
					m.overlay.Descriptions[m.curServer][t] = trim
				}
			case "-": // clear override
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					if m.overlay.Descriptions[m.curServer] != nil {
						delete(m.overlay.Descriptions[m.curServer], t)
						if len(m.overlay.Descriptions[m.curServer]) == 0 {
							delete(m.overlay.Descriptions, m.curServer)
						}
					}
				}
			case "enter":
				m.mode = modeMenu
				return m, nil
			}

		case modeLaunch:
			switch k {
			case "q", "ctrl+c":
				m.overlay = nil
				return m, tea.Quit
			case "b", "esc":
				m.mode = modeList
				return m, nil
			case "1", "enter":
				m.launch = "mcpo"
				return m, tea.Quit
			case "2":
				m.launch = "raw"
				return m, tea.Quit
			}
		}

	default:
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case modeList:
		return m.viewList()
	case modeMenu:
		return m.viewMenu()
	case modeAllow:
		return m.viewSelect("Allowed tools (space to toggle, enter to save)")
	case modeDeny:
		return m.viewSelect("Disallowed tools (space to toggle, enter to save)")
	case modeDesc:
		return m.viewDesc()
	case modeLaunch:
		return m.viewLaunch()
	default:
		return ""
	}
}

// ===== Views =====

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	itemStyle  = lipgloss.NewStyle()
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "205", Dark: "213"}).Bold(true)
	faintStyle = lipgloss.NewStyle().Faint(true)
)

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MCP servers") + "\n")
	if len(m.names) == 0 {
		b.WriteString("No servers found.\n")
	} else {
		for i, name := range m.names {
			line := fmt.Sprintf("  %s", name)
			if i == m.cursorServer {
				line = selStyle.Render("> " + name)
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\nEnter: edit server   c: continue   q: quit\n")
	return b.String()
}

func (m model) viewMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Server: %s", m.curServer)) + "\n\n")
	b.WriteString("1) Edit allowed tools\n")
	b.WriteString("2) Edit disallowed tools\n")
	b.WriteString("3) Edit tool descriptions (+: set trimmed to ≤300, -: clear)\n")
	toggle := "Enable"
	if m.overlay.Disabled[m.curServer] {
		toggle = "Enable (currently disabled)"
	}
	b.WriteString("4) Disable/Enable server — " + toggle + "\n\n")
	b.WriteString("b: back   c: choose launch   q: quit\n")
	return b.String()
}

func (m *model) prepareToolEditor(presetAll bool) {
	tools := m.servers[m.curServer]
	m.curTools = make([]string, 0, len(tools))
	for _, t := range tools {
		m.curTools = append(m.curTools, t.Name)
	}
	sort.Strings(m.curTools)
	m.cursorTool = 0
	m.editSelect = map[string]bool{}
	// default: all selected
	for _, t := range m.curTools {
		m.editSelect[t] = presetAll
	}
	// if there is an existing explicit list for this server, seed from it
	if m.mode == modeAllow {
		if am := m.overlay.Allow[m.curServer]; am != nil {
			for k := range m.editSelect {
				m.editSelect[k] = am[k]
			}
		}
	} else if m.mode == modeDeny {
		if dm := m.overlay.Deny[m.curServer]; dm != nil {
			for k := range m.editSelect {
				m.editSelect[k] = dm[k]
			}
		}
	}
}

func (m model) viewSelect(title string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s — %s", title, m.curServer)) + "\n")
	for i, t := range m.curTools {
		check := "[ ]"
		if m.editSelect[t] {
			check = "[x]"
		}
		line := fmt.Sprintf("  %s %s", check, t)
		if i == m.cursorTool {
			line = selStyle.Render("> " + line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nspace: toggle   enter: save   b: back   q: quit\n")
	return b.String()
}

func (m model) viewDesc() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Descriptions — %s", m.curServer)) + "\n")
	cur := m.curTools
	for i, t := range cur {
		override := ""
		if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
			override = mm[t]
		}
		raw := m.origDesc[m.curServer][t]
		rawNote := ""
		if len(raw) > 300 {
			rawNote = faintStyle.Render(fmt.Sprintf(" (raw %d>300)", len(raw)))
		}
		line := fmt.Sprintf("  %s%s", t, rawNote)
		if i == m.cursorTool {
			line = selStyle.Render("> " + line)
		}
		b.WriteString(line + "\n")
		if override != "" {
			b.WriteString(faintStyle.Render("     override: ") + override + "\n")
		}
	}
	b.WriteString("\n+: set trimmed (≤300)   -: clear   enter: back   b: back   q: quit\n")
	return b.String()
}

func (m model) viewLaunch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose how to launch servers") + "\n")
	cur1 := " "
	cur2 := " "
	if m.launch == "mcpo" {
		cur1 = ">"
	} else if m.launch == "raw" {
		cur2 = ">"
	}
	b.WriteString(fmt.Sprintf("%s 1) Via mcpo (HTTP + OpenAPI, recommended)\n", cur1))
	b.WriteString(fmt.Sprintf("%s 2) Normal MCP servers (stdio only)\n\n", cur2))
	b.WriteString("1/2: pick   enter: mcpo   b: back   q: quit\n")
	return b.String()
}

func (m *model) launchMode() string {
	if m.launch == "" {
		return "mcpo"
	}
	return m.launch
}

// ===== helpers =====

func trim300(s string) string {
	const lim = 300
	if len(s) <= lim {
		return s
	}
	// try to cut on a word boundary
	if idx := strings.LastIndex(s[:lim], " "); idx > 0 {
		return strings.TrimSpace(s[:idx]) + "…"
	}
	return strings.TrimSpace(s[:lim]) + "…"
}
