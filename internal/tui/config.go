package tui

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type collectModel struct {
    list       []string
    cursor     int
    inputMode  bool
    inputBuf   string
    suggest    []string
    showSuggest bool
    verb       int          // 0,1,2
    tunnel     string       // none|quick|named
    tunnelName string
    frontPort  int
    mcpoPort   int
    done       bool
    cancelled  bool
    msg        string
}

// CollectConfigs opens a simple TUI to gather config paths and basic settings.
func CollectConfigs(seed []string, baseFront, baseMcpo int, tunnel, tunnelName string, verbosity int) (configs []string, outFront int, outMcpo int, outTunnel string, outName string, outVerb int, ok bool, err error) {
    m := collectModel{list: append([]string{}, seed...), verb: verbosity, tunnel: tunnel, tunnelName: tunnelName, frontPort: baseFront, mcpoPort: baseMcpo}
    p := tea.NewProgram(&m, tea.WithAltScreen())
    _, rerr := p.Run()
    if rerr != nil { return nil, 0, 0, tunnel, tunnelName, verbosity, false, rerr }
    if m.cancelled { return nil, 0, 0, tunnel, tunnelName, verbosity, false, nil }
    return m.list, m.frontPort, m.mcpoPort, m.tunnel, m.tunnelName, m.verb, m.done, nil
}

func (m collectModel) Init() tea.Cmd { return nil }

func (m *collectModel) addPath(path string) {
    if strings.TrimSpace(path) == "" { return }
    p := expandPath(path)
    if _, err := os.Stat(p); err != nil {
        m.msg = fmt.Sprintf("! not found: %s", p)
        return
    }
    m.list = append(m.list, p)
    m.inputBuf = ""
}

func (m *collectModel) computeSuggestions() {
    // Provide simple directory-based suggestions for current input buffer
    in := m.inputBuf
    if strings.TrimSpace(in) == "" { m.suggest = nil; return }
    expanded := in
    if strings.HasPrefix(in, "~") { expanded = expandPath(in) }
    dir := expanded
    base := ""
    if fi, err := os.Stat(expanded); err == nil && fi.IsDir() {
        // ok
    } else {
        dir = filepath.Dir(expanded)
        base = filepath.Base(expanded)
    }
    entries, err := os.ReadDir(dir)
    if err != nil { m.suggest = nil; return }
    var out []string
    for _, e := range entries {
        name := e.Name()
        if base == "" || strings.Contains(strings.ToLower(name), strings.ToLower(base)) {
            cand := filepath.Join(dir, name)
            // Present with ~/ when within home
            if h, _ := os.UserHomeDir(); h != "" && strings.HasPrefix(cand, h) {
                cand = "~" + strings.TrimPrefix(cand, h)
            }
            out = append(out, cand)
        }
        if len(out) >= 8 { break }
    }
    m.suggest = out
}

func expandPath(p string) string {
    p = strings.TrimSpace(p)
    if strings.HasPrefix(p, "~/") {
        if h, err := os.UserHomeDir(); err == nil {
            p = filepath.Join(h, p[2:])
        }
    }
    p = os.ExpandEnv(p)
    if !filepath.IsAbs(p) {
        if abs, err := filepath.Abs(p); err == nil { p = abs }
    }
    return p
}

func (m collectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        k := strings.ToLower(msg.String())
        if m.inputMode {
            switch k {
            case "enter":
                m.inputMode = false
                m.addPath(m.inputBuf)
                return m, nil
            case "tab":
                if len(m.suggest) > 0 {
                    m.inputBuf = m.suggest[0]
                }
                return m, nil
            case "esc", "b":
                m.inputMode = false
                m.inputBuf = ""
                return m, nil
            default:
                if msg.Type == tea.KeyBackspace || msg.Type == tea.KeyCtrlH {
                    if n := len([]rune(m.inputBuf)); n > 0 {
                        r := []rune(m.inputBuf)
                        m.inputBuf = string(r[:n-1])
                    }
                } else if msg.Type == tea.KeyRunes {
                    m.inputBuf += string(msg.Runes)
                }
                m.computeSuggestions()
                return m, nil
            }
        }
        switch k {
        case "q", "ctrl+c":
            m.cancelled = true
            return m, tea.Quit
        case "a":
            m.inputMode = true
            m.inputBuf = ""
            return m, nil
        case "up", "k":
            if m.cursor > 0 { m.cursor-- }
            return m, nil
        case "down", "j":
            if m.cursor < len(m.list)-1 { m.cursor++ }
            return m, nil
        case "x", "delete":
            if len(m.list) > 0 && m.cursor >= 0 && m.cursor < len(m.list) {
                m.list = append(m.list[:m.cursor], m.list[m.cursor+1:]...)
                if m.cursor >= len(m.list) && m.cursor > 0 { m.cursor-- }
            }
            return m, nil
        case "v": // cycle verbosity 0->1->2
            m.verb = (m.verb + 1) % 3
            return m, nil
        case "t": // cycle tunnel
            switch m.tunnel {
            case "none": m.tunnel = "quick"
            case "quick": m.tunnel = "named"
            default: m.tunnel = "none"
            }
            return m, nil
        case "n": // edit tunnel name
            m.inputMode = true
            m.inputBuf = m.tunnelName
            return m, nil
        case "p": // edit base front/mcpo ports (simple bump)
            // small quality-of-life: up ports by +1 per press
            m.frontPort++
            m.mcpoPort++
            return m, nil
        case "enter":
            if m.inputMode { return m, nil }
            m.done = true
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m collectModel) View() string {
    var b strings.Builder
    title := lipgloss.NewStyle().Bold(true).Render("Config Collector")
    b.WriteString(title + "\n\n")
    if m.msg != "" { b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light:"160",Dark:"203"}).Render(m.msg) + "\n") }
    b.WriteString("Configs:\n")
    if len(m.list) == 0 { b.WriteString("  (none)\n") }
    for i, p := range m.list {
        line := "  " + p
        if i == m.cursor { line = selStyle.Render("> "+p) }
        b.WriteString(line + "\n")
    }
    b.WriteString("\nSettings:\n")
    b.WriteString(fmt.Sprintf("  Verbosity: %d  (v to cycle)\n", m.verb))
    b.WriteString(fmt.Sprintf("  Tunnel: %s  (t to cycle; n to edit name: %s)\n", m.tunnel, m.tunnelName))
    b.WriteString(fmt.Sprintf("  Base Ports: front=%d mcpo=%d  (p to bump)\n", m.frontPort, m.mcpoPort))
    if m.inputMode {
        b.WriteString("\nAdd Path: " + m.inputBuf + "\n")
        if m.showSuggest || true {
            for _, s := range m.suggest { b.WriteString(faint.Render("  • ")+s+"\n") }
        }
        b.WriteString("enter: add   tab: autocomplete   b/esc: cancel\n")
    } else {
        b.WriteString("\nKeys: a add  x delete  v verbosity  t tunnel  n name  p ports  enter continue  q quit\n")
    }
    return b.String()
}
