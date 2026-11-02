package tui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Header is the static top-of-screen runner metadata.
type Header struct {
	Version  string
	ProxyURL string
}

type model struct {
	hub    *Hub
	header Header
	lines  <-chan Line
	unsub  func()
	tab    int
	width  int
	height int
	vp     viewport.Model
	follow bool
	ready  bool
}

func newModel(hub *Hub, header Header) model {
	ch, unsub := hub.Subscribe()
	m := model{
		hub:    hub,
		header: header,
		lines:  ch,
		unsub:  unsub,
		follow: true,
		vp:     viewport.New(viewport.WithWidth(80), viewport.WithHeight(20)),
	}
	m.vp.SoftWrap = true
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitHub(m.lines), tickStatus())
}

type statusTickMsg struct{}

func tickStatus() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

func waitHub(ch <-chan Line) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return line
	}
}

func (m model) streams() []string {
	return m.hub.StreamNames()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layoutViewport()
		m.refreshViewport()
		return m, nil
	case Line:
		m.refreshViewport()
		return m, waitHub(m.lines)
	case statusTickMsg:
		m.refreshViewport()
		return m, tickStatus()
	case tea.MouseClickMsg:
		if m.hitTab(msg.X, msg.Y) {
			m.follow = true
			m.refreshViewport()
		}
		return m, nil
	case tea.MouseWheelMsg:
		m.vp, _ = m.vp.Update(msg)
		m.follow = m.vp.AtBottom()
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "q", "ctrl+c":
			if m.unsub != nil {
				m.unsub()
				m.unsub = nil
			}
			return m, tea.Quit
		case "tab", "right":
			streams := m.streams()
			if len(streams) > 0 {
				m.tab = (m.tab + 1) % len(streams)
			}
			m.follow = true
			m.refreshViewport()
			return m, nil
		case "left", "shift+tab":
			streams := m.streams()
			if len(streams) > 0 {
				m.tab = (m.tab + len(streams) - 1) % len(streams)
			}
			m.follow = true
			m.refreshViewport()
			return m, nil
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				idx := int(key[0] - '1')
				streams := m.streams()
				if idx < len(streams) {
					m.tab = idx
					m.follow = true
					m.refreshViewport()
					return m, nil
				}
			}
			m.vp, _ = m.vp.Update(msg)
			m.follow = m.vp.AtBottom()
			return m, nil
		}
	}
	return m, nil
}

func (m *model) hitTab(x, y int) bool {
	if y != 1 {
		return false
	}
	cursor := 0
	for i, name := range m.streams() {
		label := tabLabel(name, m.hub.Status(name), i == m.tab)
		w := lipgloss.Width(label) + 1
		if x >= cursor && x < cursor+w {
			m.tab = i
			return true
		}
		cursor += w
	}
	return false
}

func (m *model) layoutViewport() {
	chrome := 4 // header, tabs, footer, separator
	h := max(m.height-chrome, 1)
	w := max(m.width, 1)
	m.vp.SetWidth(w)
	m.vp.SetHeight(h)
}

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	streams := m.streams()
	if len(streams) == 0 {
		return
	}
	if m.tab >= len(streams) {
		m.tab = len(streams) - 1
	}
	atBottom := m.vp.AtBottom()
	content := HighlightLines(m.hub.Lines(streams[m.tab]))
	m.vp.SetContent(content)
	if m.follow || atBottom {
		m.vp.GotoBottom()
		m.follow = true
	}
}

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) render() string {
	if !m.ready {
		return "starting andurel run..."
	}
	header := fmt.Sprintf("andurel run %s  %s", m.header.Version, m.header.ProxyURL)
	header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(header)

	streams := m.streams()
	var tabs []string
	for i, name := range streams {
		tabs = append(tabs, tabLabel(name, m.hub.Status(name), i == m.tab))
	}
	tabBar := strings.Join(tabs, " ")
	footerHint := "1/2/3"
	if n := len(streams); n > 3 {
		footerHint = fmt.Sprintf("1–%d", min(9, n))
	} else if n > 0 && n < 3 {
		footerHint = fmt.Sprintf("1–%d", n)
	}
	footer := lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("click tab · %s · ←/→ · ↑↓ scroll · q quit", footerHint),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, tabBar, m.vp.View(), footer)
}

func tabLabel(name string, status Status, active bool) string {
	dot := statusDot(status)
	label := fmt.Sprintf(" %s %s ", name, dot)
	style := lipgloss.NewStyle().Foreground(tabColor(name))
	if active {
		style = style.Bold(true).Underline(true)
	} else {
		style = style.Faint(true)
	}
	return style.Render(label)
}

func tabColor(name string) color.Color {
	switch name {
	case StreamApp:
		return lipgloss.Color("10")
	case StreamQueue:
		return lipgloss.Color("13")
	case "mailpit":
		return lipgloss.Color("214") // warm amber, matches plan mock
	default:
		return lipgloss.Color("11")
	}
}

func statusDot(status Status) string {
	switch status {
	case StatusReady:
		return "●"
	case StatusStarting:
		return "◐"
	case StatusError:
		return "✖"
	case StatusExited:
		return "○"
	default:
		return "·"
	}
}

// Run starts the alt-screen TUI until quit. Call Hub.Dump after it returns.
func Run(ctx context.Context, hub *Hub, header Header) error {
	p := tea.NewProgram(newModel(hub, header), tea.WithContext(ctx))
	_, err := p.Run()
	if err == tea.ErrInterrupted || err == context.Canceled {
		return nil
	}
	return err
}
