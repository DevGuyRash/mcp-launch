package tui

import (
    "fmt"
    "os"
    "sort"
    "strings"
    "unicode"
    "os/exec"
    "io/ioutil"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
    mcpclient "mcp-launch/internal/mcpclient"
)

// ServerStatus indicates inspection outcome for a server.
type ServerStatus int

const (
    StatusOK ServerStatus = iota
    StatusERR
    StatusHTTP // reserved for future streamable-http detection
)

// Overlay is the edit overlay returned by the TUI.
// NOTE: Keys are *composite* server IDs, e.g. "<instance>/<server>".
// main.go translates these into per-instance overlays.
type Overlay struct {
    Disabled     map[string]bool              // "<inst>/<srv>" -> disabled?
    Allow        map[string]map[string]bool   // "<inst>/<srv>" -> tool -> true
    Deny         map[string]map[string]bool   // "<inst>/<srv>" -> tool -> true
    Descriptions map[string]map[string]string // "<inst>/<srv>" -> tool -> descOverride
    LastLaunch   string                       `json:"last_launch,omitempty"`
}

// Run shows a lightweight TUI that lets the user edit server/tool filters,
// choose launch mode, and persist overlay state.
// - servers: composite key "<inst>/<srv>" -> tools
// - seed:    (optional) prior overlay to pre-populate selections
// - status:  per-server status for preflight badges
// - errs:    per-server error text (if any)
// Returns (overlay or nil if cancelled, launchMode "mcpo"|"raw").
func Run(servers map[string][]mcpclient.Tool, seed *Overlay, status map[string]ServerStatus, errs map[string]string) (*Overlay, string, error) {
    m := newModel(servers, seed, status, errs)
    p := tea.NewProgram(&m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        return nil, "", err
    }
    // persist last launch choice into overlay so callers can save it across runs
    if m.overlay != nil {
        m.overlay.LastLaunch = m.launchMode()
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
    modeDescEdit mode = "desc_edit" // direct edit description override
    modeDescMulti mode = "desc_multi" // multi-select actions for descriptions
)

type model struct {
    // data
    servers  map[string][]mcpclient.Tool     // "<inst>/<srv>" -> tools
    names    []string                        // ordered keys
    origDesc map[string]map[string]string    // "<inst>/<srv>" -> tool -> original description
    overlay  *Overlay                        // values to return (nil if cancelled)
    status   map[string]ServerStatus         // per-server status
    errors   map[string]string               // per-server error text

	// ui state
	cursorServer int
	cursorTool   int
	curServer    string   // current composite server key
	curTools     []string // current server's tools (names)

    mode   mode
    launch string // "mcpo" (default) or "raw"
    menuCursor int // selection for server menu entries
    launchSel int  // 0 none, 1 quick, 2 named

	// ephemeral selection state for allow/deny
	editSelect map[string]bool // tool -> selected?

    // last-diff cache
    diffBefore string
    diffAfter  string

    // editor buffer for description edit
    editBuffer string
    showHelp   bool

    // diff / view options
    diffUnified bool // true=unified, false=side-by-side
    wrapLines   bool  // soft wrap long lines in viewers

    controller  string // "mcpo" or "raw"
    termWidth   int
}

func newModel(servers map[string][]mcpclient.Tool, seed *Overlay, status map[string]ServerStatus, errs map[string]string) model {
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

    mdl := model{
        servers:  servers,
        names:    names,
        origDesc: orig,
        overlay:  ov,
        status:   status,
        errors:   errs,
        mode:     modeList,
        launch:   "mcpo",
        diffUnified: true,
        wrapLines:   true,
        controller:  "mcpo",
    }
    if seed != nil && strings.TrimSpace(seed.LastLaunch) != "" {
        mdl.launch = seed.LastLaunch
    }
    return mdl
}

func (m model) Init() tea.Cmd { return nil }

/* ===== Update ===== */

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        if msg.Width > 0 { m.termWidth = msg.Width }
        return m, nil
	case tea.KeyMsg:
		k := strings.ToLower(msg.String())
		switch m.mode {
            case modeList:
                switch k {
                case "q", "ctrl+c":
                    m.overlay = nil
                    return m, tea.Quit
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
			case "up", "k":
				if m.cursorServer > 0 {
					m.cursorServer--
				}
			case "down", "j":
				if m.cursorServer < len(m.names)-1 {
					m.cursorServer++
				}
                case "enter", "d":
                    if len(m.names) == 0 {
                        return m, nil
                    }
                    m.curServer = m.names[m.cursorServer]
                    m.mode = modeMenu
                    return m, nil
                case "c":
                    m.mode = modeLaunch
                    return m, nil
                case "g":
                    if m.controller == "mcpo" { m.controller = "raw" } else { m.controller = "mcpo" }
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
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
            case "up", "k":
                if m.menuCursor > 0 { m.menuCursor-- }
                return m, nil
            case "down", "j":
                if m.menuCursor < 3 { m.menuCursor++ }
                return m, nil
            case "enter":
                switch m.menuCursor {
                case 0:
                    m.prepareToolEditor(true)
                    m.mode = modeAllow
                case 1:
                    m.prepareToolEditor(false)
                    m.mode = modeDeny
                case 2:
                    m.prepareToolEditor(true)
                    m.mode = modeDesc
                case 3:
                    m.overlay.Disabled[m.curServer] = !m.overlay.Disabled[m.curServer]
                }
                return m, nil
            case "1", "a": // shortcuts
                m.prepareToolEditor(true)
                m.mode = modeAllow
                return m, nil
            case "2", "d":
                m.prepareToolEditor(false)
                m.mode = modeDeny
                return m, nil
            case "3":
                m.prepareToolEditor(true)
                m.mode = modeDesc
                return m, nil
            case "4":
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
                case "?":
                    m.showHelp = !m.showHelp
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
                case "?":
                    m.showHelp = !m.showHelp
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
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
                case "m":
                    // enter multi-select mode (start with no selection)
                    m.prepareToolEditor(false)
                    m.mode = modeDescMulti
                    return m, nil
            case "e": // edit description in simple textarea
                if len(m.curTools) > 0 {
                    t := m.curTools[m.cursorTool]
                    // Prefer external editor if available
                    raw := m.origDesc[m.curServer][t]
                    if edited, ok := tryExternalEditor(raw); ok {
                        if m.overlay.Descriptions[m.curServer] == nil { m.overlay.Descriptions[m.curServer] = map[string]string{} }
                        m.overlay.Descriptions[m.curServer][t] = strings.TrimSpace(edited)
                        return m, nil
                    }
                    oxr := ""
                    if mm := m.overlay.Descriptions[m.curServer]; mm != nil { oxr = mm[t] }
                    if oxr != "" { m.editBuffer = oxr } else { m.editBuffer = raw }
                    m.mode = modeDescEdit
                }
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
            case modeDescMulti:
                switch k {
                case "q", "ctrl+c":
                    m.overlay = nil
                    return m, tea.Quit
                case "b", "esc":
                    m.mode = modeDesc
                    return m, nil
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
                case "up", "k":
                    if m.cursorTool > 0 { m.cursorTool-- }
                case "down", "j":
                    if m.cursorTool < len(m.curTools)-1 { m.cursorTool++ }
                case " ":
                    if len(m.curTools) > 0 {
                        t := m.curTools[m.cursorTool]
                        m.editSelect[t] = !m.editSelect[t]
                    }
                case "t": // trim selected (operate on override if present; skip if ≤300)
                    if m.overlay.Descriptions[m.curServer] == nil { m.overlay.Descriptions[m.curServer] = map[string]string{} }
                    for _, t := range m.curTools {
                        if !m.editSelect[t] { continue }
                        src := m.origDesc[m.curServer][t]
                        if mm := m.overlay.Descriptions[m.curServer]; mm != nil && mm[t] != "" { src = mm[t] }
                        if len([]rune(src)) <= 300 { continue }
                        m.overlay.Descriptions[m.curServer][t] = trim300(src)
                    }
                    m.mode = modeDesc
                    return m, nil
                case "r": // truncate selected (operate on override if present; skip if ≤300)
                    if m.overlay.Descriptions[m.curServer] == nil { m.overlay.Descriptions[m.curServer] = map[string]string{} }
                    for _, t := range m.curTools {
                        if !m.editSelect[t] { continue }
                        src := m.origDesc[m.curServer][t]
                        if mm := m.overlay.Descriptions[m.curServer]; mm != nil && mm[t] != "" { src = mm[t] }
                        if len([]rune(src)) <= 300 { continue }
                        m.overlay.Descriptions[m.curServer][t] = truncate300(src)
                    }
                    m.mode = modeDesc
                    return m, nil
                case "-": // clear selected
                    if m.overlay.Descriptions[m.curServer] != nil {
                        for _, t := range m.curTools {
                            if m.editSelect[t] { delete(m.overlay.Descriptions[m.curServer], t) }
                        }
                        if len(m.overlay.Descriptions[m.curServer]) == 0 {
                            delete(m.overlay.Descriptions, m.curServer)
                        }
                    }
                    m.mode = modeDesc
                    return m, nil
                }
            case modeDescEdit:
                switch k {
                case "q", "ctrl+c":
                    m.overlay = nil
                    return m, tea.Quit
                case "b", "esc":
                    m.mode = modeDesc
                    return m, nil
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
            case "enter":
                if len(m.curTools) > 0 {
                    t := m.curTools[m.cursorTool]
                    if m.overlay.Descriptions[m.curServer] == nil {
                        m.overlay.Descriptions[m.curServer] = map[string]string{}
                    }
                    m.overlay.Descriptions[m.curServer][t] = strings.TrimSpace(m.editBuffer)
                }
                m.mode = modeDesc
                return m, nil
            default:
                // basic text input
                if msg.Type == tea.KeyBackspace || msg.Type == tea.KeyCtrlH {
                    if n := len([]rune(m.editBuffer)); n > 0 {
                        r := []rune(m.editBuffer)
                        m.editBuffer = string(r[:n-1])
                    }
                } else if msg.Type == tea.KeyRunes {
                    m.editBuffer += string(msg.Runes)
                }
                return m, nil
            }
            case modeDiff:
                switch k {
                case "b", "esc", "enter", "q", "ctrl+c", "d":
                    // always return to desc view
                    m.mode = modeDesc
                    return m, nil
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
                case "w":
                    m.wrapLines = !m.wrapLines
                    return m, nil
                case "u":
                    m.diffUnified = true
                    return m, nil
                case "s":
                    m.diffUnified = false
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
                case "?":
                    m.showHelp = !m.showHelp
                    return m, nil
                case "up", "k":
                    if m.launchSel > 0 { m.launchSel-- }
                    return m, nil
                case "down", "j":
                    if m.launchSel < 2 { m.launchSel++ }
                    return m, nil
                case "enter":
                    if m.overlay != nil {
                        switch m.launchSel {
                        case 0: m.overlay.LastLaunch = "none"
                        case 1: m.overlay.LastLaunch = "quick"
                        case 2: m.overlay.LastLaunch = "named"
                        }
                    }
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

func init() {
    // Basic no-color fallback for dumb terminals or NO_COLOR env
    if os.Getenv("NO_COLOR") != "" || strings.Contains(strings.ToLower(os.Getenv("TERM")), "dumb") {
        titleStyle = lipgloss.NewStyle()
        selStyle = lipgloss.NewStyle()
        faintStyle = lipgloss.NewStyle()
        tagStyle = lipgloss.NewStyle()
        tagWarnStyle = lipgloss.NewStyle()
    }
}

func (m model) View() string {
    if m.showHelp {
        return m.viewHelp()
    }
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
    case modeDescEdit:
        return m.viewDescEdit()
    case modeDescMulti:
        return m.viewDescMulti()
    case modeLaunch:
        return m.viewLaunch()
    default:
        return ""
    }
}

func (m model) viewList() string {
    var b strings.Builder
    if m.overlay == nil {
        return ""
    }
    hdr := "MCP servers (per config)"
    if m.controller == "raw" { hdr += "  [Controller: RAW]" } else { hdr += "  [Controller: MCPO]" }
    b.WriteString(titleStyle.Render(hdr) + "\n")
    if len(m.names) == 0 {
        b.WriteString("No servers found.\n")
    } else {
        for i, name := range m.names {
            // show disabled badge if set
            line := "  " + name
            if m.status[name] == StatusERR {
                line += " " + tagWarnStyle.Render("ERR")
            } else if m.status[name] == StatusHTTP {
                line += " " + tagStyle.Render("HTTP")
            }
            if m.overlay.Disabled[name] {
                line += " " + tagWarnStyle.Render("DISABLED")
            }
            if i == m.cursorServer {
                line = selStyle.Render("> " + line[2:])
            }
            b.WriteString(line + "\n")
        }
    }
    b.WriteString("\nEnter/d: details   c: tunnel   g: toggle controller   q: quit\n")
    return b.String()
}

func (m model) viewMenu() string {
    var b strings.Builder
    if m.overlay == nil {
        return ""
    }
    b.WriteString(titleStyle.Render(fmt.Sprintf("Server: %s", m.curServer)) + "\n\n")
    // show status & error details if present
    if st, ok := m.status[m.curServer]; ok {
        switch st {
        case StatusERR:
            b.WriteString(tagWarnStyle.Render("ERR") + "\n")
            if msg, ok2 := m.errors[m.curServer]; ok2 && strings.TrimSpace(msg) != "" {
                b.WriteString(faintStyle.Render(msg) + "\n\n")
            } else {
                b.WriteString(faintStyle.Render("No error details available") + "\n\n")
            }
        case StatusHTTP:
            b.WriteString(tagStyle.Render("HTTP (not yet implemented)") + "\n\n")
        case StatusOK:
            // no extra tag for OK
        }
    }
    items := []string{
        "Edit allowed tools",
        "Edit disallowed tools",
        "Edit tool descriptions",
    }
    toggle := "Disable server"
    if m.overlay.Disabled[m.curServer] { toggle = "Enable server (currently disabled)" }
    items = append(items, toggle)
    for i, it := range items {
        line := fmt.Sprintf("  %s", it)
        if i == m.menuCursor { line = selStyle.Render("> "+it) }
        b.WriteString(line+"\n")
    }
    b.WriteString("\n↑/↓ select   enter choose   1-4 shortcuts   b back   c tunnel   q quit\n")
    return b.String()
}

func (m model) viewHelp() string {
    var b strings.Builder
    b.WriteString(titleStyle.Render("Help — Key Bindings") + "\n\n")
    b.WriteString("Global: q/ctrl+c quit   b/esc back   ? toggle help\n\n")
    b.WriteString("List: ↑/k, ↓/j, enter/d details, c tunnel, g toggle controller\n")
    b.WriteString("Menu: ↑/↓ select, enter choose (1..4 shortcuts)\n")
    b.WriteString("Allow/Deny: ↑/k, ↓/j, space toggle, enter save\n")
    b.WriteString("Desc: e edit, + trim, - clear, d diff, m multi-select\n")
    b.WriteString("Multi: space toggle, t trim, r truncate, - clear, b back\n")
    b.WriteString("Diff: u unified, s side-by-side, w wrap, enter/b back\n")
    b.WriteString("Launch: ↑/↓ Select tunnel (Local/Quick/Named), enter confirm\n")
    b.WriteString("\n?: close help\n")
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
    if m.overlay == nil {
        return ""
    }
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
    if m.overlay == nil {
        return ""
    }
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
            // custom override; show TRIM/TRUNC if it exactly matches those transforms
            trimmed := trim300(raw)
            truncated := truncate300(raw)
            if oxr == trimmed {
                badges = append(badges, tagStyle.Render("OVR TRIM ≤300"))
            } else if oxr == truncated {
                badges = append(badges, tagStyle.Render("OVR TRUNC ≤300"))
            } else if len([]rune(oxr)) <= 300 {
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
    b.WriteString("\n+: trim ≤300   -: clear override   d: diff   e: edit ($EDITOR)   m: multi   enter/b: back   q: quit\n")
    return b.String()
}

func (m model) viewDescMulti() string {
    var b strings.Builder
    if m.overlay == nil { return "" }
    b.WriteString(titleStyle.Render(fmt.Sprintf("Descriptions — multi-select — %s", m.curServer)) + "\n")
    for i, t := range m.curTools {
        check := "[ ]"
        if m.editSelect[t] { check = "[x]" }
        line := fmt.Sprintf("  %s %s", check, t)
        if i == m.cursorTool { line = selStyle.Render("> "+line) }
        b.WriteString(line+"\n")
    }
    b.WriteString("\nspace toggle   t trim selected   r truncate selected   - clear selected   b back   q quit\n")
    return b.String()
}

func (m model) viewDescEdit() string {
    var b strings.Builder
    if m.overlay == nil {
        return ""
    }
    b.WriteString(titleStyle.Render(fmt.Sprintf("Edit Description — %s", m.curServer)) + "\n")
    if len(m.curTools) > 0 {
        t := m.curTools[m.cursorTool]
        b.WriteString(faintStyle.Render("Tool: ") + t + "\n\n")
    }
    if strings.TrimSpace(m.editBuffer) == "" {
        b.WriteString(faintStyle.Render("<type to edit>") + "\n")
    } else {
        b.WriteString(m.editBuffer + "\n")
    }
    b.WriteString("\nenter: save   b/esc: cancel   q: quit\n")
    return b.String()
}

func (m model) viewDiff() string {
    var b strings.Builder
    if m.overlay == nil {
        return ""
    }
    title := fmt.Sprintf("Diff — %s", m.curServer)
    b.WriteString(titleStyle.Render(title) + "\n\n")
    if strings.TrimSpace(m.diffAfter) == "" {
        b.WriteString("No override set. (Nothing to diff)\n\n")
        b.WriteString("enter/b back\n")
        return b.String()
    }
    if strings.TrimSpace(m.diffBefore) == strings.TrimSpace(m.diffAfter) {
        b.WriteString("No changes for this tool.\n\n")
        b.WriteString("u unified  s side-by-side  w wrap  enter/b back\n")
        return b.String()
    }
    if m.diffUnified {
        b.WriteString(renderUnifiedDiff(m.diffBefore, m.diffAfter, m.wrapLines))
    } else {
        width := m.termWidth
        if width <= 0 { width = 100 }
        col := (width - 6) / 2
        if col < 30 { col = 30 }
        b.WriteString(renderSideBySideDiff(m.diffBefore, m.diffAfter, col, m.wrapLines))
    }
    b.WriteString("\n[u] unified  [s] side-by-side  [w] wrap  [d] back  enter/b back\n")
    return b.String()
}

func (m model) viewLaunch() string {
    var b strings.Builder
    if m.overlay == nil {
        return ""
    }
    b.WriteString(titleStyle.Render("Tunnel Mode") + "\n")
    items := []string{"Local (no tunnel)", "Cloudflare Quick", "Cloudflare Named"}
    for i, it := range items {
        line := "  " + it
        if i == m.launchSel { line = selStyle.Render("> "+it) }
        b.WriteString(line+"\n")
    }
    b.WriteString("\n↑/↓ select   enter confirm   b back   q quit\n")
    return b.String()
}

func (m *model) launchMode() string {
    // Prefer controller toggle over legacy launch string
    if m.controller == "raw" { return "raw" }
    return "mcpo"
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
func truncate300(s string) string {
    const lim = 300
    r := []rune(strings.TrimSpace(s))
    if len(r) <= lim {
        return string(r)
    }
    return string(r[:lim]) + "…"
}

// tryExternalEditor opens $VISUAL or $EDITOR on a temp file and returns edited content.
func tryExternalEditor(initial string) (string, bool) {
    ed := os.Getenv("VISUAL")
    if strings.TrimSpace(ed) == "" { ed = os.Getenv("EDITOR") }
    if strings.TrimSpace(ed) == "" { return "", false }
    tf, err := ioutil.TempFile("", "mcp-launch-edit-*.txt")
    if err != nil { return "", false }
    path := tf.Name()
    _, _ = tf.WriteString(initial)
    _ = tf.Close()
    cmd := exec.Command(ed, path)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    if err := cmd.Run(); err != nil { os.Remove(path); return "", false }
    data, err := os.ReadFile(path)
    os.Remove(path)
    if err != nil { return "", false }
    return string(data), true
}
