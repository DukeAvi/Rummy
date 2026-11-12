package tui

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"github.com/DukeAvi/rummy-tui/internal/lan"
	tea "github.com/charmbracelet/bubbletea"
	"net"
	"strings"
	"time"
)

type appScreen uint8

const (
	connectionMenu appScreen = iota
	networkMenu
	hostAddress
	joinCode
	setup
	lobby
	playing
	ended
)

type tickMsg struct{}
type joinedMsg struct {
	r   *remote
	err error
}
type hostMsg struct {
	s   *lan.Server
	err error
}

// App owns the installation-to-exit flow; Model owns card interaction.
// All UI state stays on Bubble Tea's event loop. Only the server uses goroutines.
type App struct {
	model                *Model
	screen               appScreen
	choice               int
	hosting, network     bool
	input, code, message string
	server               *lan.Server
	remote               *remote
	busy                 bool
	lastHand             *cb.Pile
}

func NewApp() *App           { return &App{model: New()} }
func (a *App) Init() tea.Cmd { return nil }
func heartbeat() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
func (a *App) Close() {
	if a.server != nil {
		a.server.Close()
	}
}
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m := a.model
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.Update(size)
		return a, nil
	}
	switch v := msg.(type) {
	case hostMsg:
		a.busy = false
		if v.err != nil {
			a.message = v.err.Error()
			a.screen = hostAddress
			return a, nil
		}
		a.server = v.s
		a.code = net.JoinHostPort(a.input, lan.Port) + "/" + v.s.Session.JoinToken()
		r := &remote{player: 0, token: v.s.Session.HostToken()}
		r.send = func(req lan.Request) (lan.Response, error) { return v.s.Session.Handle(req), nil }
		r.accept(v.s.Session.Handle(lan.Request{Version: lan.Version, Token: r.token, Action: "poll"}))
		a.remote = r
		a.screen = lobby
		a.message = "Share the full code. Both players press Enter when ready."
		return a, heartbeat()
	case joinedMsg:
		a.busy = false
		if v.err != nil {
			a.message = v.err.Error()
			return a, nil
		}
		a.remote = v.r
		a.screen = lobby
		a.message = "Connected. Review the round and press Enter when ready."
		return a, heartbeat()
	case tickMsg:
		if a.remote == nil || a.screen == ended {
			return a, nil
		}
		a.refresh()
		return a, heartbeat()
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	k := key.String()
	if m.quit {
		if k == "enter" || k == "y" {
			if a.remote != nil && a.remote.pending == nil {
				_ = a.remote.request("leave", 0, nil, false, game.Config{})
			}
			a.Close()
			return a, tea.Quit
		}
		m.Update(msg)
		return a, nil
	}
	// Code and address fields accept printable q as text; Ctrl+C still exits.
	if k == "ctrl+c" || (k == "q" && a.screen != joinCode && a.screen != hostAddress) {
		m.quit = true
		return a, nil
	}
	if a.busy {
		return a, nil
	}
	switch a.screen {
	case connectionMenu, networkMenu:
		switch k {
		case "up", "down", "left", "right", "j", "k", "tab":
			a.choice = 1 - a.choice
		case "enter", " ":
			if a.screen == connectionMenu {
				if a.choice == 0 {
					a.network = false
					a.screen = setup
					m.screen = menu
				} else {
					a.network = true
					a.screen = networkMenu
					a.choice = 0
				}
			} else {
				a.hosting = a.choice == 0
				if a.hosting {
					a.screen = hostAddress
					a.input = lan.LocalAddress()
				} else {
					a.screen = joinCode
					a.input = ""
				}
			}
		case "esc":
			a.screen = connectionMenu
			a.choice = 0
		}
	case hostAddress, joinCode:
		switch k {
		case "esc":
			a.screen = networkMenu
			a.message = ""
			a.choice = 0
		case "backspace", "ctrl+h":
			if len(a.input) > 0 {
				a.input = a.input[:len(a.input)-1]
			}
		case "ctrl+u":
			a.input = ""
		case "enter":
			if a.screen == hostAddress {
				a.input = strings.TrimSpace(a.input)
				if net.ParseIP(a.input) == nil && a.input != "localhost" {
					a.message = "Enter the laptop's reachable IP address."
					return a, nil
				}
				a.screen = setup
				m.screen = menu
			} else {
				address, token, err := lan.ParseCode(a.input)
				if err != nil {
					a.message = err.Error()
					return a, nil
				}
				a.busy = true
				a.message = "Connecting…"
				return a, func() tea.Msg {
					out, err := lan.Call(address, lan.Request{Version: lan.Version, Token: token, Action: "join"})
					if err == nil && out.Error != "" {
						err = fmt.Errorf("%s", out.Error)
					}
					if err != nil {
						return joinedMsg{err: err}
					}
					if out.Token != "" {
						token = out.Token
					}
					r := &remote{address: address, token: token, player: out.Player, seq: out.Seq}
					r.send = func(req lan.Request) (lan.Response, error) { return lan.Call(address, req) }
					r.accept(out)
					return joinedMsg{r: r}
				}
			}
		default:
			if key.Type == tea.KeyRunes {
				a.input += string(key.Runes)
			}
		}
	case setup:
		if k == "esc" && m.screen == menu {
			if a.network {
				a.screen = networkMenu
			} else {
				a.screen = connectionMenu
			}
			return a, nil
		}
		if a.network && m.screen == jokerMenu && (k == "enter" || k == " ") {
			n := 7
			if m.modeChoice == 1 {
				n = 13
			}
			cfg := game.Config{Cards: n, Wild: m.rank}
			if a.server == nil {
				a.busy = true
				return a, func() tea.Msg { s, err := lan.Listen(cfg); return hostMsg{s: s, err: err} }
			}
			if err := a.remote.request("configure", 0, nil, false, cfg); err != nil {
				a.message = err.Error()
				return a, nil
			}
			a.screen = lobby
			a.message = "New round proposed. Both players press Enter to ready."
			return a, nil
		}
		m.Update(msg)
		if m.screen == table {
			a.screen = playing
		}
	case lobby:
		if k == "enter" {
			a.message = "Ready. Waiting for the other player."
			if err := a.remote.request("ready", 0, nil, false, game.Config{}); err != nil {
				a.message = err.Error()
			}
			a.syncGame()
		}
		if k == "esc" && a.hosting && a.remote.state.View == nil {
			// Cancelling a lobby ends this session and invalidates its code.
			a.Close()
			a.server = nil
			a.remote = nil
			a.screen = networkMenu
			a.message = "Lobby cancelled; host again for a fresh code."
		}
	case playing:
		if !a.network {
			m.Update(msg)
			if m.screen == menu {
				a.screen = setup
			}
			return a, nil
		}
		r := a.remote
		if r.state.Closed {
			a.screen = ended
			return a, nil
		}
		if r.state.View != nil && r.state.View.Phase == game.Finished && (k == "p" || k == "enter") {
			if a.hosting {
				m.screen = menu
				a.screen = setup
			} else {
				a.screen = lobby
				a.message = "Waiting for the host to choose the next round."
			}
			return a, nil
		}
		if r.state.View != nil && r.state.View.Phase == game.Finished {
			m.Update(msg)
			return a, nil
		}
		if !r.state.Connected || r.pending != nil || r.state.View == nil || r.Active() != r.player {
			if k == "?" || m.help {
				m.Update(msg)
			}
			return a, nil
		}
		m.Update(msg)
		// Submit a placed order by physical ID, while carrying stays local.
		if (k == " " || k == "enter") && m.selection != nil && !m.selection.Carrying() {
			var ids []uint64
			for _, c := range r.pile.Cards() {
				ids = append(ids, c.ID)
			}
			if err := r.request("order", 0, ids, false, game.Config{}); err != nil {
				m.feedback = err.Error()
			}
		}
		a.syncGame()
	case ended:
		if k == "enter" || k == "esc" {
			a.Close()
			*a = *NewApp()
			a.model.width = m.width
			a.model.height = m.height
		}
	}
	return a, nil
}
func (a *App) refresh() {
	r := a.remote
	var err error
	if r.pending != nil {
		err = r.retry()
	} else {
		var out lan.Response
		out, err = r.send(lan.Request{Version: lan.Version, Token: r.token, Action: "poll"})
		if err == nil {
			if out.Closed {
				a.screen = ended
				a.message = "The other player ended the session."
				return
			}
			if out.Error != "" {
				err = fmt.Errorf("%s", out.Error)
			} else {
				r.accept(out)
			}
		}
	}
	if r.state.Closed {
		a.screen = ended
		a.message = "The session has ended."
		return
	}
	if err != nil {
		a.message = err.Error()
		r.state.Connected = false
		return
	}
	if r.state.Closed {
		a.screen = ended
		a.message = "The session has ended."
		return
	}
	a.syncGame()
}
func (a *App) syncGame() {
	r := a.remote
	m := a.model
	if a.screen == setup {
		return
	}
	if r.state.View == nil {
		a.screen = lobby
		return
	}
	v := r.state.View
	a.screen = playing
	m.game = r
	if v.Phase == game.Finished {
		m.screen = victory
		return
	}
	m.screen = table
	if m.selection == nil || a.handChanged() {
		a.lastHand = r.pile
		m.selection = cb.NewSelection(r.pile)
		_ = m.selection.Move(r.pile.Len())
		m.marked = map[uint64]bool{}
	}
}

// Selection has no pile accessor; retaining the current pointer separately
// lets a draw replace the hand without invalidating an unrelated carry.
func (a *App) handChanged() bool   { return a.modelHand() != a.remote.pile }
func (a *App) modelHand() *cb.Pile { return a.lastHand }

func (a *App) View() string {
	m := a.model
	if m.quit {
		return m.View()
	}
	var body string
	switch a.screen {
	case connectionMenu, networkMenu:
		title := "HOW WILL YOU PLAY?"
		choices := []string{"One PC · pass the keyboard", "OTA / local network · two computers"}
		if a.screen == networkMenu {
			title = "NETWORK PLAY"
			choices = []string{"Host a game", "Join a game"}
		}
		body = title + "\n\n"
		for i, c := range choices {
			prefix := "  "
			if a.choice == i {
				prefix = "› "
			}
			body += prefix + c + "\n"
		}
		body += "\n↑/↓ choose · Enter continue · Esc back · q quit"
	case hostAddress:
		body = "HOST · REACHABLE ADDRESS\n\n" + m.wrap("Enter the laptop IP that your friend can reach over the hotspot. The server listens on TCP port "+lan.Port+". In WSL, this may be your Windows hotspot adapter address; see README.") + "\n\n> " + a.input + "\n\nCtrl+U clear · Enter continue · Esc back · Ctrl+C quit\n\n" + m.wrap(a.message)
	case joinCode:
		body = "JOIN YOUR FRIEND\n\nConnect to the host's hotspot first.\nPaste the ENTIRE game or rejoin code, then press Enter.\n\n" + m.wrap("> "+a.input) + "\n\nCtrl+U clear · Esc back · Ctrl+C quit\n\n" + m.wrap(a.message)
	case setup:
		return m.View()
	case lobby:
		r := a.remote
		body = fmt.Sprintf("NETWORK LOBBY · Player %d\n\n%d-card Rummy · wild %s\n\n", r.player+1, r.state.Config.Cards, rankLabel(r.state.Config.Wild))
		if a.hosting {
			body += "Share this whole join code:\n" + m.wrap(a.code) + "\n\n"
		} else {
			body += "Keep this private rejoin code (valid for 5 minutes offline):\n" + m.wrap(r.address+"/"+r.token) + "\n\n"
		}
		body += fmt.Sprintf("Player 1 ready: %t · Player 2 ready: %t\n\n", r.state.Ready[0], r.state.Ready[1]) + m.wrap(a.message) + "\n\nEnter: ready · q: quit"
		if a.hosting {
			body += " · Esc: cancel lobby"
		}
	case playing:
		if !a.network {
			return m.View()
		}
		r := a.remote
		if !r.state.Connected {
			body = "CONNECTION PAUSED\n\n" + m.wrap(a.message) + "\n\nWaiting for your friend / retrying connection.\nNo game actions are allowed while disconnected.\n\nq: leave session"
		} else if r.pending != nil {
			body = "Recovering the previous action…\nPlease wait. q: leave"
		} else if m.help {
			return m.View()
		} else if r.state.View.Phase == game.Finished {
			return m.View()
		} else if r.Active() != r.player {
			body = fmt.Sprintf("RUMMY %d · wild %s\n\nYou are Player %d.\nWaiting for Player %d to finish their turn.\n\nYour hand and groups remain private.\nq: leave session", r.Config().Cards, rankLabel(r.Config().Wild), r.player+1, r.Active()+1)
		} else {
			return m.View()
		}
	case ended:
		body = "SESSION ENDED\n\n" + m.wrap(a.message) + "\n\nEnter: main menu · q: quit"
	}
	return m.fit(body)
}
