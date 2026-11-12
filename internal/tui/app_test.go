package tui

import (
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"github.com/DukeAvi/rummy-tui/internal/lan"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

func appKey(a *App, k string) tea.Cmd {
	special := map[string]tea.KeyType{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight, "backspace": tea.KeyBackspace}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	if v, ok := special[k]; ok {
		msg = tea.KeyMsg{Type: v}
	}
	_, cmd := a.Update(msg)
	return cmd
}
func TestConnectionChoicesAndCodeErrors(t *testing.T) {
	a := NewApp()
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if !strings.Contains(a.View(), "One PC") || !strings.Contains(a.View(), "OTA") {
		t.Fatal("missing first choice")
	}
	appKey(a, "enter")
	if a.screen != setup {
		t.Fatal("local setup")
	}
	appKey(a, "esc")
	appKey(a, "down")
	appKey(a, "enter")
	if a.screen != networkMenu {
		t.Fatal("network selection")
	}
	appKey(a, "down")
	appKey(a, "enter")
	if a.screen != joinCode {
		t.Fatal("join menu")
	}
	appKey(a, "invalid")
	appKey(a, "enter")
	if a.busy || !strings.Contains(a.message, "entire") {
		t.Fatal("bad code error")
	}
	appKey(a, "esc")
	appKey(a, "enter")
	if a.screen != hostAddress {
		t.Fatal("host menu")
	}
}
func networkApp(s *lan.Session, token string, p int) *App {
	r := &remote{token: token, player: p}
	r.send = func(req lan.Request) (lan.Response, error) { return s.Handle(req), nil }
	r.accept(s.Handle(lan.Request{Version: lan.Version, Token: token, Action: "poll"}))
	a := NewApp()
	a.model.width = 120
	a.model.height = 40
	a.remote = r
	a.network = true
	a.hosting = p == 0
	a.screen = lobby
	return a
}
func TestNetworkReadyTurnsPrivacyAndRetry(t *testing.T) {
	s, _ := lan.New(game.Config{Cards: 7, Wild: cb.Six})
	joined := s.Handle(lan.Request{Version: lan.Version, Token: s.JoinToken(), Action: "join"})
	h, g := networkApp(s, s.HostToken(), 0), networkApp(s, joined.Token, 1)
	appKey(h, "enter")
	if h.screen != lobby {
		t.Fatal("premature start")
	}
	appKey(g, "enter")
	h.refresh()
	if h.screen != playing || g.screen != playing {
		t.Fatal("not started")
	}
	if !strings.Contains(h.View(), "▀") || !strings.Contains(g.View(), "Waiting for Player 1") {
		t.Fatal("turn views")
	}
	appKey(g, "d")
	if len(g.remote.pile.Cards()) != 7 {
		t.Fatal("waiting player drew")
	}
	appKey(h, "d")
	if h.remote.pile.Len() != 8 {
		t.Fatal("draw")
	}
	appKey(h, "m")
	appKey(h, "g")
	if len(h.remote.state.View.Groups) != 1 {
		t.Fatal("network grouping")
	}
	appKey(h, " ")
	appKey(h, "left")
	appKey(h, "enter")
	if h.model.selection.Carrying() {
		t.Fatal("carry not placed")
	}
	appKey(h, "x")
	g.refresh()
	if !strings.Contains(h.View(), "Waiting for Player 2") || !strings.Contains(g.View(), "▀") || len(g.remote.state.View.Groups) != 0 {
		t.Fatal("handoff/privacy")
	}
	appKey(g, "o")
	if g.remote.pile.Len() != 8 {
		t.Fatal("open draw")
	}
	appKey(g, "x")
	h.refresh()
	if h.remote.Active() != 0 {
		t.Fatal("reverse turn")
	}
	// Losing a response must retry the identical action rather than consume a
	// second draw. The transport already delivered it to the host in this test.
	original := h.remote.send
	lost := true
	h.remote.send = func(req lan.Request) (lan.Response, error) {
		out, err := original(req)
		if lost {
			lost = false
			return lan.Response{}, tea.ErrProgramKilled
		}
		return out, err
	}
	appKey(h, "d")
	if h.remote.pending == nil {
		t.Fatal("missing retry")
	}
	h.refresh()
	if h.remote.pending != nil || h.remote.pile.Len() != 8 {
		t.Fatal("retry duplicated draw")
	}
	appKey(g, "q")
	cmd := appKey(g, "y")
	if cmd == nil {
		t.Fatal("no quit")
	}
	h.refresh()
	if h.screen != ended {
		t.Fatal("peer exit not surfaced")
	}
}
