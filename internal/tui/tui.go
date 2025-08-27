package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	mcpclient "mcp-launch/internal/mcpclient"
)

// Overlay is the edit overlay returned by the TUI.
// NOTE: Keys are *composite* server IDs, e.g. "<instance>/<server>".
// main.go translates these into per-instance overlays.
type Overlay struct {
	Disabled     map[string]bool              // "<inst>/<srv>" -> disabled?
	Allow        map[string]map[string]bool   // "<inst>/<srv>" -> tool -> true
	Deny         map[string]map[string]bool   // "<inst>/<srv>" -> tool -> true
	Descriptions map[string]map[string]string // "<inst>/<srv>" -> tool -> descOverride
}

// Run shows a lightweight TUI that lets the user edit server/tool filters,
// choose launch mode, and persist overlay state.
// - servers: composite key "<inst>/<srv>" -> tools
// - seed:    (optional) prior overlay to pre-populate selections
// Returns (overlay or nil if cancelled, launchMode "mcpo"|"raw").
func Run(servers map[string][]mcpclient.Tool, seed *Overlay) (*Overlay, string, error) {
	m := newModel(servers, seed)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return nil, "", err
	}
	return m.overlay, m.launchMode(), nil
}

/* ===== Model ===== */

type mode string

const (
	modeList   mode = "list"   // choose server
	modeMenu   mode = "menu"   // edit menu for a server
	modeAllow  mode = "allow"  // edit allowed tools
	modeDeny   mode = "deny"   // edit disallowed tools
	modeDesc   mode = "desc"   // manage description overrides
	modeDiff   mode = "diff"   // show before/after for current tool
	modeLaunch mode = "launch" // choose launch mode
)

type model struct {
	// data
	servers  map[string][]mcpclient.Tool     // "<inst>/<srv>" -> tools
	names    []string                        // ordered keys
	origDesc map[string]map[string]string    // "<inst>/<srv>" -> tool -> original description
	overlay  *Overlay                        // values to return (nil if cancelled)

	// ui state
	cursorServer int
	cursorTool   int
	curServer    string   // current composite server key
	curTools     []string // current server's tools (names)

	mode   mode
	launch string // "mcpo" (default) or "raw"

	// ephemeral selection state for allow/deny
	editSelect map[string]bool // tool -> selected?

	// last-diff cache
	diffBefore string
	diffAfter  string
}

func newModel(servers map[string][]mcpclient.Tool, seed *Overlay) model {
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

	ov := &Overlay{
		Disabled:     map[string]bool{},
		Allow:        map[string]map[string]bool{},
		Deny:         map[string]map[string]bool{},
		Descriptions: map[string]map[string]string{},
	}
	if seed != nil {
		// shallow copy is fine (maps rewritten in place by TUI)
		ov.Disabled = map[string]bool{}
		for k, v := range seed.Disabled {
			ov.Disabled[k] = v
		}
		ov.Allow = map[string]map[string]bool{}
		for k, m := range seed.Allow {
			ov.Allow[k] = map[string]bool{}
			for t, b := range m {
				ov.Allow[k][t] = b
			}
		}
		ov.Deny = map[string]map[string]bool{}
		for k, m := range seed.Deny {
			ov.Deny[k] = map[string]bool{}
			for t, b := range m {
				ov.Deny[k][t] = b
			}
		}
		ov.Descriptions = map[string]map[string]string{}
		for k, m := range seed.Descriptions {
			ov.Descriptions[k] = map[string]string{}
			for t, s := range m {
				ov.Descriptions[k][t] = s
			}
		}
	}

	return model{
		servers:  servers,
		names:    names,
		origDesc: orig,
		overlay:  ov,
		mode:     modeList,
		launch:   "mcpo",
	}
}

func (m model) Init() tea.Cmd { return nil }

/* ===== Update ===== */

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := strings.ToLower(msg.String())
		switch m.mode {
		case modeList:
			switch k {
			case "q", "ctrl+c":
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
			case "1", "a": // allowed tools
				m.prepareToolEditor(true)
				m.mode = modeAllow
				return m, nil
			case "2", "d": // disallowed tools
				m.prepareToolEditor(false)
				m.mode = modeDeny
				return m, nil
			case "3": // descriptions manager
				m.prepareToolEditor(true) // to populate curTools ordering
				m.mode = modeDesc
				return m, nil
			case "4": // toggle disable/enable server
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
					// Allow-all → clear explicit allow list
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
			case "b", "esc", "enter":
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
			case "+", "t": // trim to ≤300
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
			case "d": // show diff panel
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					raw := m.origDesc[m.curServer][t]
					oxr := ""
					if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
						oxr = mm[t]
					}
					m.diffBefore = strings.TrimSpace(raw)
					m.diffAfter = strings.TrimSpace(oxr)
					m.mode = modeDiff
					return m, nil
				}
			}
		case modeDiff:
			switch k {
			case "b", "esc", "enter", "q", "ctrl+c":
				// always return to desc view
				m.mode = modeDesc
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
			case "left", "right":
				if m.launch == "mcpo" {
					m.launch = "raw"
				} else {
					m.launch = "mcpo"
				}
				return m, nil
			case "1":
				m.launch = "mcpo"
				return m, nil
			case "2":
				m.launch = "raw"
				return m, nil
			case "enter":
				return m, tea.Quit
			}
		}
	default:
		return m, nil
	}
	return m, nil
}

/* ===== Views ===== */

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	selStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "205", Dark: "213"}).Bold(true)
	faintStyle   = lipgloss.NewStyle().Faint(true)
	tagStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "238", Dark: "245"}).Background(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Padding(0, 1)
	tagWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"}).Padding(0, 1)
)

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
	case modeDiff:
		return m.viewDiff()
	case modeLaunch:
		return m.viewLaunch()
	default:
		return ""
	}
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MCP servers (per config)") + "\n")
	if len(m.names) == 0 {
		b.WriteString("No servers found.\n")
	} else {
		for i, name := range m.names {
			// show disabled badge if set
			line := "  " + name
			if m.overlay.Disabled[name] {
				line += " " + tagWarnStyle.Render("DISABLED")
			}
			if i == m.cursorServer {
				line = selStyle.Render("> " + line[2:])
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
	b.WriteString("1) Edit allowed tools   (a)\n")
	b.WriteString("2) Edit disallowed tools (d)\n")
	b.WriteString("3) Edit tool descriptions (+ trim ≤300, - clear, d diff)\n")
	toggle := "Disable"
	if m.overlay.Disabled[m.curServer] {
		toggle = "Enable (currently disabled)"
	}
	b.WriteString("4) " + toggle + " server\n\n")
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
	// seed from existing allow/deny maps
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
		raw := m.origDesc[m.curServer][t]
		badges := make([]string, 0, 2)
		oxr := ""
		if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
			oxr = mm[t]
		}
		if oxr != "" {
			// custom override
			if len([]rune(oxr)) <= 300 {
				badges = append(badges, tagStyle.Render("OVR ≤300"))
			} else {
				badges = append(badges, tagStyle.Render("OVR"))
			}
		} else if len([]rune(raw)) > 300 {
			// raw too long (can trim)
			badges = append(badges, tagWarnStyle.Render(fmt.Sprintf("RAW %d>300", len(raw))))
		}
		line := "  " + t
		if len(badges) > 0 {
			line += "  " + strings.Join(badges, " ")
		}
		if i == m.cursorTool {
			line = selStyle.Render("> " + line[2:])
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n+: trim ≤300   -: clear override   d: view diff   enter/b: back   q: quit\n")
	return b.String()
}

func (m model) viewDiff() string {
	var b strings.Builder
	title := fmt.Sprintf("Diff — %s", m.curServer)
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if m.diffBefore == "" && m.diffAfter == "" {
		b.WriteString("No changes for this tool.\n")
		b.WriteString("\nenter/b: back\n")
		return b.String()
	}
	b.WriteString(tagStyle.Render("RAW") + "\n")
	if m.diffBefore == "" {
		b.WriteString(faintStyle.Render("<empty>") + "\n\n")
	} else {
		b.WriteString(m.diffBefore + "\n\n")
	}
	b.WriteString(tagStyle.Render("OVERRIDE") + "\n")
	if m.diffAfter == "" {
		b.WriteString(faintStyle.Render("<none set>") + "\n")
	} else {
		b.WriteString(m.diffAfter + "\n")
	}
	b.WriteString("\nenter/b: back\n")
	return b.String()
}

func (m model) viewLaunch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose how to launch servers") + "\n")
	cur1, cur2 := " ", " "
	if m.launch == "mcpo" {
		cur1 = ">"
	} else {
		cur2 = ">"
	}
	b.WriteString(fmt.Sprintf("%s mcpo (HTTP + OpenAPI, recommended)\n", cur1))
	b.WriteString(fmt.Sprintf("%s raw (stdio only)\n\n", cur2))
	b.WriteString("←/→ switch   enter: confirm   b: back   q: quit\n")
	return b.String()
}

func (m *model) launchMode() string {
	if m.launch == "" {
		return "mcpo"
	}
	return m.launch
}

/* ===== helpers ===== */

func trim300(s string) string {
	const lim = 300
	r := []rune(strings.TrimSpace(s))
	if len(r) <= lim {
		return string(r)
	}
	cut := lim
	for i := lim; i > 0; i-- {
		if unicode.IsSpace(r[i-1]) { cut = i - 1; break }
	}
	return strings.TrimSpace(string(r[:cut])) + "…"
}
