package tui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	mcpclient "mcp-launch/internal/mcpclient"
    // New TUI state and widgets
    tuiState "mcp-launch/internal/tui/state"
    chips "mcp-launch/internal/tui/widgets/tagchips"
    helpov "mcp-launch/internal/tui/widgets/helpoverlay"
    statusbar "mcp-launch/internal/tui/widgets/statusbar"
    util "mcp-launch/internal/tui/util"
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
	return m.overlay, m.launchMode(), nil
}

/* ===== Model ===== */

type mode string

const (
	modeList      mode = "list"       // choose server
	modeMenu      mode = "menu"       // edit menu for a server
	modeAllow     mode = "allow"      // edit allowed tools
	modeDeny      mode = "deny"       // edit disallowed tools
	modeDesc      mode = "desc"       // manage description overrides
	modeDiff      mode = "diff"       // show before/after for current tool
	modeLaunch    mode = "launch"     // choose launch mode
	modeDescEdit  mode = "desc_edit"  // direct edit description override
	modeDescMulti mode = "desc_multi" // multi-select actions for descriptions
)

type model struct {
	// data
	servers  map[string][]mcpclient.Tool  // "<inst>/<srv>" -> tools
	names    []string                     // ordered keys
	origDesc map[string]map[string]string // "<inst>/<srv>" -> tool -> original description
	overlay  *Overlay                     // values to return (nil if cancelled)
	status   map[string]ServerStatus      // per-server status
	errors   map[string]string            // per-server error text

	// ui state
	cursorServer int
	cursorTool   int
	curServer    string   // current composite server key
	curTools     []string // current server's tools (names)

	mode       mode
	launch     string // "mcpo" (default) or "raw"
	menuCursor int    // selection for server menu entries
	launchSel  int    // 0 none, 1 quick, 2 named

	// ephemeral selection state for allow/deny
	editSelect map[string]bool // tool -> selected?

	// last-diff cache
	diffBefore string
	diffAfter  string

	// editor buffer for description edit
	editBuffer string
	editInsert bool // true: insert mode (like Vim 'i'); false: command mode
	editWrap   bool // true: soft-wrap textarea to view width
	ta         textarea.Model
	statusMsg  string
	showHelp   bool

	// diff / view options
	diffUnified bool // true=unified, false=side-by-side
	wrapLines   bool // soft wrap long lines in viewers

	controller string // "mcpo" or "raw"
	termWidth  int
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

	ta := textarea.New()
	// Expand editing area a bit; width will be set on WindowSizeMsg
	ta.SetHeight(8)
	// Augment keymap: support ctrl+arrow word nav and ctrl+backspace/delete
	km := ta.KeyMap
	km.WordForward = key.NewBinding(key.WithKeys("alt+right", "alt+f", "ctrl+right"))
	km.WordBackward = key.NewBinding(key.WithKeys("alt+left", "alt+b", "ctrl+left"))
	km.DeleteWordBackward = key.NewBinding(key.WithKeys("alt+backspace", "ctrl+w", "ctrl+backspace"))
	km.DeleteWordForward = key.NewBinding(key.WithKeys("alt+delete", "alt+d", "ctrl+delete"))
	ta.KeyMap = km

	mdl := model{
		servers:     servers,
		names:       names,
		origDesc:    orig,
		overlay:     ov,
		status:      status,
		errors:      errs,
		mode:        modeList,
		launch:      "mcpo",
		diffUnified: true,
		wrapLines:   true,
		controller:  "mcpo",
		ta:          ta,
		editWrap:    true,
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
    if msg.Width > 0 {
        m.termWidth = msg.Width
        w := msg.Width - 4
        if w < 20 {
            w = 20
        }
        if m.editWrap {
            m.ta.SetWidth(w)
        } else {
            m.ta.SetWidth(10000)
        }
        // Deterministic narrow fallback for side-by-side diffs
        minCol := 30
        threshold := 2*minCol + 6 // two columns + separator/gutters
        if !m.diffUnified && m.termWidth < threshold {
            m.diffUnified = true
            m.statusMsg = "Narrow width: using unified view"
        }
    }
    return m, nil
	case tea.KeyMsg:
		rawKey := msg.String()
		k := strings.ToLower(rawKey)
		// If help overlay is showing, only handle closing it
		if m.showHelp {
			switch k {
			case "?", "esc", "b":
				m.showHelp = false
			}
			return m, nil
		}
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
				if m.controller == "mcpo" {
					m.controller = "raw"
				} else {
					m.controller = "mcpo"
				}
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
				if m.menuCursor > 0 {
					m.menuCursor--
				}
				return m, nil
			case "down", "j":
				if m.menuCursor < 3 {
					m.menuCursor++
				}
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
        case "a": // shortcut
            m.prepareToolEditor(true)
            m.mode = modeAllow
            return m, nil
        case "d":
            m.prepareToolEditor(false)
            m.mode = modeDeny
            return m, nil
        // Numeric shortcuts removed per UX guidance
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
			case "y": // copy effective text of current tool
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					txt := m.origDesc[m.curServer][t]
					if mm := m.overlay.Descriptions[m.curServer]; mm != nil && strings.TrimSpace(mm[t]) != "" {
						txt = mm[t]
					}
					_ = clipboard.WriteAll(txt)
					m.statusMsg = "Copied description"
				}
				return m, nil
			case "?":
				m.showHelp = !m.showHelp
				return m, nil
			case "m":
				// enter multi-select mode (start with no selection)
				m.prepareToolEditor(false)
				m.mode = modeDescMulti
				return m, nil
			case "e": // inline editor by default; external if uppercase 'E' or ctrl+e
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					raw := m.origDesc[m.curServer][t]
					// external editor path
					if rawKey == "E" || msg.Type == tea.KeyCtrlE {
						// prefer current override if present
						src := raw
						if mm := m.overlay.Descriptions[m.curServer]; mm != nil && strings.TrimSpace(mm[t]) != "" {
							src = mm[t]
						}
						if edited, ok := tryExternalEditor(src); ok {
							edited = strings.TrimSpace(edited)
							if strings.TrimSpace(src) == edited {
								// No change: clear existing override if same as raw; return to desc without diff
								if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
									if mm[t] == edited {
										delete(mm, t)
										if len(mm) == 0 {
											delete(m.overlay.Descriptions, m.curServer)
										}
									}
								}
								m.mode = modeDesc
								return m, nil
							}
							if m.overlay.Descriptions[m.curServer] == nil {
								m.overlay.Descriptions[m.curServer] = map[string]string{}
							}
							m.overlay.Descriptions[m.curServer][t] = edited
							// show diff
							m.diffBefore = strings.TrimSpace(src)
							m.diffAfter = edited
							m.mode = modeDiff
							return m, nil
						}
					}
					// fallback to inline editor powered by textarea
					oxr := ""
					if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
						oxr = mm[t]
					}
					if oxr != "" {
						m.editBuffer = oxr
					} else {
						m.editBuffer = raw
					}
					m.ta.SetValue(m.editBuffer)
					m.ta.Focus()
					if m.termWidth > 0 {
						w := m.termWidth - 4
						if w < 20 {
							w = 20
						}
						m.ta.SetWidth(w)
					}
					m.mode = modeDescEdit
					m.editInsert = true
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
			case "+", "t": // trim to ≤limit
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					raw := m.origDesc[m.curServer][t]
					lim := descLimit()
					// only apply if needed
					if len([]rune(strings.TrimSpace(raw))) <= lim {
						return m, nil
					}
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
			case "b", "esc", "enter", "m":
				m.mode = modeDesc
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
			case "a": // select all
				for _, t := range m.curTools {
					m.editSelect[t] = true
				}
			case "u": // deselect all
				for _, t := range m.curTools {
					m.editSelect[t] = false
				}
			case "t": // trim selected (compute from raw; skip if ≤limit)
				if m.overlay.Descriptions[m.curServer] == nil {
					m.overlay.Descriptions[m.curServer] = map[string]string{}
				}
				for _, t := range m.curTools {
					if !m.editSelect[t] {
						continue
					}
					src := m.origDesc[m.curServer][t]
					if len([]rune(src)) <= descLimit() {
						continue
					}
					m.overlay.Descriptions[m.curServer][t] = trim300(src)
				}
				return m, nil
			case "r": // truncate selected (compute from raw; skip if ≤limit)
				if m.overlay.Descriptions[m.curServer] == nil {
					m.overlay.Descriptions[m.curServer] = map[string]string{}
				}
				for _, t := range m.curTools {
					if !m.editSelect[t] {
						continue
					}
					src := m.origDesc[m.curServer][t]
					if len([]rune(src)) <= descLimit() {
						continue
					}
					m.overlay.Descriptions[m.curServer][t] = truncate300(src)
				}
				return m, nil
			case "-": // clear selected
				if m.overlay.Descriptions[m.curServer] != nil {
					for _, t := range m.curTools {
						if m.editSelect[t] {
							delete(m.overlay.Descriptions[m.curServer], t)
						}
					}
					if len(m.overlay.Descriptions[m.curServer]) == 0 {
						delete(m.overlay.Descriptions, m.curServer)
					}
				}
				return m, nil
			}
		case modeDescEdit:
			if m.editInsert {
				// Insert mode: forward to textarea; ESC leaves insert; Ctrl+S saves; Ctrl+Y copies
				if msg.Type == tea.KeyEsc {
					m.editInsert = false
					m.ta.Blur()
					return m, nil
				}
				if k == "ctrl+s" {
					if len(m.curTools) > 0 {
						t := m.curTools[m.cursorTool]
						if m.overlay.Descriptions[m.curServer] == nil {
							m.overlay.Descriptions[m.curServer] = map[string]string{}
						}
						m.overlay.Descriptions[m.curServer][t] = strings.TrimSpace(m.ta.Value())
					}
					m.mode = modeDesc
					return m, nil
				}
				if k == "ctrl+y" {
					_ = clipboard.WriteAll(m.ta.Value())
					m.statusMsg = "Copied current text to clipboard"
					return m, nil
				}
				var cmd tea.Cmd
				m.ta, cmd = m.ta.Update(msg)
				return m, cmd
			}
			// Command mode
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
			case "w": // toggle wrapping for textarea view
				m.editWrap = !m.editWrap
				w := m.termWidth - 4
				if w < 20 {
					w = 20
				}
				if m.editWrap {
					m.ta.SetWidth(w)
				} else {
					m.ta.SetWidth(10000)
				}
				if m.editWrap {
					m.statusMsg = "Wrap: on"
				} else {
					m.statusMsg = "Wrap: off"
				}
				return m, nil
			case "y": // copy current buffer to clipboard
				_ = clipboard.WriteAll(m.ta.Value())
				m.statusMsg = "Copied edited text"
				return m, nil
			case "E": // open external editor with current edited text
				cur := m.ta.Value()
				if edited, ok := tryExternalEditor(cur); ok {
					edited = strings.TrimSpace(edited)
					if edited == strings.TrimSpace(cur) {
						m.statusMsg = "No changes"
						return m, nil
					}
					if len(m.curTools) > 0 {
						t := m.curTools[m.cursorTool]
						if m.overlay.Descriptions[m.curServer] == nil {
							m.overlay.Descriptions[m.curServer] = map[string]string{}
						}
						m.overlay.Descriptions[m.curServer][t] = edited
					}
					m.diffBefore = strings.TrimSpace(cur)
					m.diffAfter = edited
					m.mode = modeDiff
					return m, nil
				}
				return m, nil
			case "i":
				m.editInsert = true
				m.ta.Focus()
				return m, nil
			case "enter":
				if len(m.curTools) > 0 {
					t := m.curTools[m.cursorTool]
					if m.overlay.Descriptions[m.curServer] == nil {
						m.overlay.Descriptions[m.curServer] = map[string]string{}
					}
					m.overlay.Descriptions[m.curServer][t] = strings.TrimSpace(m.ta.Value())
				}
				m.mode = modeDesc
				return m, nil
			}
			return m, nil
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
				if m.launchSel > 0 {
					m.launchSel--
				}
				return m, nil
			case "down", "j":
				if m.launchSel < 2 {
					m.launchSel++
				}
				return m, nil
			case "enter":
				if m.overlay != nil {
					switch m.launchSel {
					case 0:
						m.overlay.LastLaunch = "none"
					case 1:
						m.overlay.LastLaunch = "quick"
					case 2:
						m.overlay.LastLaunch = "named"
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
    var out string
    switch m.mode {
    case modeList:
        out = m.viewList()
    case modeMenu:
        out = m.viewMenu()
    case modeAllow:
        out = m.viewSelect("Allowed tools (space to toggle, enter to save)")
    case modeDeny:
        out = m.viewSelect("Disallowed tools (space to toggle, enter to save)")
    case modeDesc:
        out = m.viewDesc()
    case modeDiff:
        out = m.viewDiff()
    case modeDescEdit:
        out = m.viewDescEdit()
    case modeDescMulti:
        out = m.viewDescMulti()
    case modeLaunch:
        out = m.viewLaunch()
    default:
        out = ""
    }
    // Append Help overlay (non-blocking) if toggled
    if m.showHelp {
        hov := helpov.NewHelpOverlay()
        out += "\n" + hov.View(m.uiState()) + "\n"
    }
    // Append Status Bar for consistent state visibility
    sb := statusbar.NewStatusBar()
    out += "\n" + sb.View(m.uiState()) + "\n"
    return out
}

// uiState maps the current model fields into shared UI state for widgets.
func (m model) uiState() tuiState.UIState {
    s := tuiState.UIState{}
    if m.editInsert {
        s.Mode = tuiState.INSERT
    } else {
        s.Mode = tuiState.CMD
    }
    // Wrap depends on context
    if m.mode == modeDescEdit {
        s.Wrap = m.editWrap
    } else {
        s.Wrap = m.wrapLines
    }
    if m.diffUnified {
        s.View = tuiState.Unified
    } else {
        s.View = tuiState.SideBySide
    }
    s.Width = m.termWidth
    s.MinCol = 30
    s.ScrollHLeft = 0
    s.ScrollHRight = 0
    s.ScrollV = 0
    s.SyncScroll = false
    s.Limit = descLimit()
    s.Edited = false
    s.Notice = strings.TrimSpace(m.statusMsg)
    return s
}

func (m model) viewList() string {
	var b strings.Builder
	if m.overlay == nil {
		return ""
	}
	hdr := "MCP servers (per config)"
	if m.controller == "raw" {
		hdr += "  [Controller: RAW]"
	} else {
		hdr += "  [Controller: MCPO]"
	}
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
    b.WriteString("\nEnter/d: details   c: tunnel   g: toggle controller   q: quit   ?: help\n")
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
	if m.overlay.Disabled[m.curServer] {
		toggle = "Enable server (currently disabled)"
	}
	items = append(items, toggle)
	for i, it := range items {
		line := fmt.Sprintf("  %s", it)
		if i == m.menuCursor {
			line = selStyle.Render("> " + it)
		}
		b.WriteString(line + "\n")
	}
    b.WriteString("\n↑/↓ select   enter choose   b back   c tunnel   q quit   ?: help\n")
	return b.String()
}

func (m model) viewHelp() string {
	var b strings.Builder
	lim := descLimit()
	b.WriteString(titleStyle.Render("Help — Key Bindings") + "\n\n")
	b.WriteString("Global: q/ctrl+c quit   b/esc back   ? toggle help\n\n")
	b.WriteString("List: ↑/k, ↓/j, enter/d details, c tunnel, g toggle controller\n")
	b.WriteString("Menu: ↑/↓ select, enter choose (1..4 shortcuts)\n")
	b.WriteString("Allow/Deny: ↑/k, ↓/j, space toggle, enter save\n")
	b.WriteString(fmt.Sprintf("Desc: e edit inline, E edit in $EDITOR, +/t trim ≤%d, - clear, d diff, m multi-select\n", lim))
	b.WriteString(fmt.Sprintf("Multi: space toggle, a all, u none, t trim ≤%d, r truncate ≤%d, - clear, b back\n", lim, lim))
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
        oxr := ""
        if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
            oxr = mm[t]
        }
        // Compute tags with auto-detection of Trimmed/Truncated; mark EDITED when override
        // differs from raw and is not an auto-shortening.
        base := util.ComputeTags(raw, oxr, descLimit(), false)
        hasTrim, hasTrunc := false, false
        for _, tg := range base {
            if tg.Kind == tuiState.TRIMMED { hasTrim = true }
            if tg.Kind == tuiState.TRUNCATED { hasTrunc = true }
        }
        edited := strings.TrimSpace(oxr) != "" && strings.TrimSpace(oxr) != strings.TrimSpace(raw) && !(hasTrim || hasTrunc)
        tags := make([]tuiState.Tag, 0, len(base)+1)
        if edited { tags = append(tags, tuiState.Tag{Kind: tuiState.EDITED}) }
        tags = append(tags, base...)
        chipsLine := chips.View(tags, util.NoColor(false))
        line := "  " + t
        if strings.TrimSpace(chipsLine) != "" {
            line += "  " + chipsLine
        }
        if i == m.cursorTool {
            line = selStyle.Render("> " + line[2:])
        }
        b.WriteString(line + "\n")
    }
    b.WriteString("\nKeys: + trim   - clear   d diff   e edit   E $EDITOR   m multi   enter/b back   q quit   y copy   ?: help\n")
    return b.String()
}

func (m model) viewDescMulti() string {
	var b strings.Builder
	if m.overlay == nil {
		return ""
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Descriptions — multi-select — %s", m.curServer)) + "\n")
    for i, t := range m.curTools {
        check := "[ ]"
        if m.editSelect[t] {
            check = "[x]"
        }
        raw := m.origDesc[m.curServer][t]
        oxr := ""
        if mm := m.overlay.Descriptions[m.curServer]; mm != nil {
            oxr = mm[t]
        }
        base := util.ComputeTags(raw, oxr, descLimit(), false)
        hasTrim, hasTrunc := false, false
        for _, tg := range base {
            if tg.Kind == tuiState.TRIMMED { hasTrim = true }
            if tg.Kind == tuiState.TRUNCATED { hasTrunc = true }
        }
        edited := strings.TrimSpace(oxr) != "" && strings.TrimSpace(oxr) != strings.TrimSpace(raw) && !(hasTrim || hasTrunc)
        tags := make([]tuiState.Tag, 0, len(base)+1)
        if edited { tags = append(tags, tuiState.Tag{Kind: tuiState.EDITED}) }
        tags = append(tags, base...)
        chipsLine := chips.View(tags, util.NoColor(false))
        line := fmt.Sprintf("  %s %s", check, t)
        if strings.TrimSpace(chipsLine) != "" {
            line += "  " + chipsLine
        }
        if i == m.cursorTool {
            line = selStyle.Render("> " + line)
        }
        b.WriteString(line + "\n")
    }
    b.WriteString("\nspace toggle   a all   u none   t trim   r truncate   - clear   b back   q quit   ?: help\n")
    return b.String()
}

func (m model) viewDescEdit() string {
	var b strings.Builder
	if m.overlay == nil {
		return ""
	}
	modeLabel := "[CMD]"
	if m.editInsert {
		modeLabel = "[INSERT]"
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Edit Description — %s %s", m.curServer, modeLabel)) + "\n")
	if len(m.curTools) > 0 {
		t := m.curTools[m.cursorTool]
		b.WriteString(faintStyle.Render("Tool: ") + t + "\n\n")
	}
	b.WriteString(m.ta.View())
	b.WriteString("\n")
	if m.editInsert {
		b.WriteString("Esc: cmd  Ctrl+S: save\n")
	} else {
		b.WriteString("i: insert  enter: save  b/esc: back  q: quit  y: copy  w: wrap\n")
	}
	if strings.TrimSpace(m.statusMsg) != "" {
		b.WriteString(faintStyle.Render(m.statusMsg) + "\n")
	}
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
		if width <= 0 {
			width = 100
		}
		col := (width - 6) / 2
		if col < 30 {
			col = 30
		}
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
		if i == m.launchSel {
			line = selStyle.Render("> " + it)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n↑/↓ select   enter confirm   b back   q quit\n")
	return b.String()
}

func (m *model) launchMode() string {
	// Prefer controller toggle over legacy launch string
	if m.controller == "raw" {
		return "raw"
	}
	return "mcpo"
}

/* ===== helpers ===== */

func descLimit() int {
	// Allow override via MCP_LAUNCH_DESC_LIMIT; default 300
	if v := strings.TrimSpace(os.Getenv("MCP_LAUNCH_DESC_LIMIT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 300
}

func trim300(s string) string {
	lim := descLimit()
	r := []rune(strings.TrimSpace(s))
	if len(r) <= lim {
		return string(r)
	}
	// Reserve 1 for ellipsis so total <= lim
	if lim <= 1 {
		return "…"
	}
	cut := lim - 1
	for i := cut; i > 0; i-- {
		if unicode.IsSpace(r[i-1]) {
			cut = i - 1
			break
		}
	}
	out := strings.TrimSpace(string(r[:cut])) + "…"
	rr := []rune(out)
	if len(rr) > lim {
		out = string(rr[:lim])
	}
	return out
}
func truncate300(s string) string {
	lim := descLimit()
	r := []rune(strings.TrimSpace(s))
	if len(r) <= lim {
		return string(r)
	}
	if lim <= 1 {
		return "…"
	}
	out := string(r[:lim-1]) + "…"
	rr := []rune(out)
	if len(rr) > lim {
		out = string(rr[:lim])
	}
	return out
}

// tryExternalEditor opens $VISUAL or $EDITOR on a temp file and returns edited content.
func tryExternalEditor(initial string) (string, bool) {
	ed := os.Getenv("VISUAL")
	if strings.TrimSpace(ed) == "" {
		ed = os.Getenv("EDITOR")
	}
	if strings.TrimSpace(ed) == "" {
		return "", false
	}
	tf, err := ioutil.TempFile("", "mcp-launch-edit-*.txt")
	if err != nil {
		return "", false
	}
	path := tf.Name()
	_, _ = tf.WriteString(initial)
	_ = tf.Close()
	cmd := exec.Command(ed, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return "", false
	}
	data, err := os.ReadFile(path)
	os.Remove(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}
