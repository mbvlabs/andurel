package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestModelSwitchTabsAndQuit(t *testing.T) {
	hub := NewHub()
	_, _ = hub.Writer(StreamApp).Write([]byte("http request\n"))
	_, _ = hub.Writer(StreamQueue).Write([]byte("job processed\n"))
	hub.SetStatus(StreamApp, StatusReady)

	m := newModel(hub, Header{Version: "test", ProxyURL: "http://localhost:3000"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mod := updated.(model)
	view := mod.render()
	if !strings.Contains(view, "http request") {
		t.Fatalf("expected app logs in default tab, got %q", view)
	}
	if !strings.Contains(view, "queue") || !strings.Contains(view, "build") {
		t.Fatalf("expected tab labels, got %q", view)
	}

	updated, _ = mod.Update(tea.KeyPressMsg{Text: "2", Code: '2'})
	mod = updated.(model)
	view = mod.render()
	if !strings.Contains(view, "job processed") {
		t.Fatalf("expected queue logs after selecting tab 2, got %q", view)
	}

	_, cmd := mod.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestTabClickSelectsQueue(t *testing.T) {
	hub := NewHub()
	m := newModel(hub, Header{Version: "test", ProxyURL: "http://localhost:3000"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mod := updated.(model)

	cursor := 0
	var x int
	for i, name := range mod.streams() {
		label := tabLabel(name, mod.hub.Status(name), i == mod.tab)
		w := lipgloss.Width(label) + 1
		if i == 1 {
			x = cursor + 1
			break
		}
		cursor += w
	}

	updated, _ = mod.Update(tea.MouseClickMsg{X: x, Y: 1, Button: tea.MouseLeft})
	mod = updated.(model)
	if mod.tab != 1 {
		t.Fatalf("tab = %d, want 1 (clicked x=%d)", mod.tab, x)
	}
}

func TestModelToolTabDigitAndFooter(t *testing.T) {
	hub := NewHub()
	hub.Enable("mailpit")
	_, _ = hub.Writer("mailpit").Write([]byte("smtp ready\n"))

	m := newModel(hub, Header{Version: "test", ProxyURL: "http://localhost:3000"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	mod := updated.(model)
	view := mod.render()
	if !strings.Contains(view, "mailpit") || !strings.Contains(view, "1–4") {
		t.Fatalf("expected mailpit tab and 1–4 footer, got %q", view)
	}

	updated, _ = mod.Update(tea.KeyPressMsg{Text: "4", Code: '4'})
	mod = updated.(model)
	view = mod.render()
	if !strings.Contains(view, "smtp ready") {
		t.Fatalf("expected mailpit logs after key 4, got %q", view)
	}
}
