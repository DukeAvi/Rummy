// Package tui binds the Rummy engine to a private, local Bubble Tea interface.
package tui

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"slices"
	"strings"
)

type screen uint8

const (
	menu screen = iota
	jokerMenu
	table
	victory
)

type Model struct {
	screen        screen
	width, height int
	modeChoice    int
	rank          cb.Rank
	game          board
	selection     *cb.Selection
	marked        map[uint64]bool
	layout        cb.Layout
	renderer      *cb.Renderer
	feedback      string
	help, quit    bool
	scroll        int
	inspect       bool
	winnerGroup   int
}

func New() *Model {
	return &Model{rank: cb.Six, marked: map[uint64]bool{}, renderer: cb.NewRenderer(cb.Vintage)}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case tea.KeyMsg:
		key := msg.String()
		if m.quit {
			switch key {
			case "y", "enter":
				return m, tea.Quit
			case "n", "esc", "q":
				m.quit = false
			}
			return m, nil
		}
		if key == "q" || key == "ctrl+c" {
			m.quit = true
			return m, nil
		}
		if key == "?" {
			m.help = !m.help
			m.scroll = 0
			return m, nil
		}
		if m.help {
			switch key {
			case "esc":
				m.help = false
			case "down", "j":
				m.scroll++
			case "up", "k":
				m.scroll = max(0, m.scroll-1)
			}
			return m, nil
		}
		if m.inspect {
			switch key {
			case "esc", "i":
				m.inspect = false
			case "down", "j":
				m.scroll++
			case "up", "k":
				m.scroll = max(0, m.scroll-1)
			}
			return m, nil
		}
		switch m.screen {
		case menu:
			switch key {
			case "up", "down", "left", "right", "h", "l", "j", "k", "tab":
				m.modeChoice = 1 - m.modeChoice
			case "7":
				m.modeChoice = 0
			case "1":
				m.modeChoice = 1
			case "enter", " ":
				m.screen = jokerMenu
			}
		case jokerMenu:
			switch key {
			case "left", "h", "up", "k":
				m.rank = cb.Rank((int(m.rank)+11)%13 + 1)
			case "right", "l", "down", "j", "tab":
				m.rank = cb.Rank(int(m.rank)%13 + 1)
			case "esc":
				m.screen = menu
			case "enter", " ":
				m.startRound()
			default:
				for rank := cb.Ace; rank <= cb.King; rank++ {
					if strings.EqualFold(key, rankLabel(rank)) {
						m.rank = rank
						break
					}
				}
				// 't' is a convenient single-key shortcut for ten.
				if key == "t" || key == "0" {
					m.rank = cb.Ten
				}
			}
		case table:
			if m.width > 0 && (m.width < 88 || m.height < 36) {
				return m, nil
			}
			if m.game.Phase() == game.Handoff {
				if key == "enter" {
					if err := m.game.ConfirmHandoff(m.game.Active()); err != nil {
						m.feedback = err.Error()
					} else {
						hand, _ := m.game.Hand(m.game.Active())
						m.selection = cb.NewSelection(hand)
						m.feedback = "Draw from the closed stock (d) or open pile (o)."
					}
				}
				return m, nil
			}
			m.play(key)
		case victory:
			switch key {
			case "enter", "p":
				// A new setup always offers both mode and joker-rank selection again.
				m.screen = menu
				m.game = nil
				m.selection = nil
				m.marked = map[uint64]bool{}
				m.feedback = ""
				m.winnerGroup = 0
				m.scroll = 0
			case "left", "h":
				m.winnerGroup = max(0, m.winnerGroup-1)
			case "right", "l":
				m.winnerGroup = min(len(m.game.Winning())-1, m.winnerGroup+1)
			case "up", "k":
				m.scroll = max(0, m.scroll-1)
			case "down", "j":
				m.scroll++
			}
		}
	}
	return m, nil
}

func (m *Model) startRound() {
	n := 7
	if m.modeChoice == 1 {
		n = 13
	}
	g, err := game.New(game.Config{Cards: n, Wild: m.rank}, nil)
	if err != nil {
		m.feedback = err.Error()
		return
	}
	m.game = g
	m.screen = table
	m.selection = nil
	m.marked = map[uint64]bool{}
	m.feedback = ""
	m.layout = cb.Overlap
}

func (m *Model) result(err error, success string) {
	if err != nil {
		m.feedback = err.Error()
	} else {
		m.feedback = success
	}
}

func (m *Model) play(key string) {
	p := m.game.Active()
	hand, err := m.game.Hand(p)
	if err != nil {
		m.feedback = err.Error()
		return
	}
	selected, err := hand.At(m.selection.Selected())
	if err != nil {
		m.feedback = err.Error()
		return
	}
	// Carrying changes the actual pile. Complete or cancel it before any engine
	// operation can mutate membership or change the active hand.
	if m.selection.Carrying() && strings.Contains("doxvmguaz", key) && len(key) == 1 {
		m.feedback = "Place (Space/Enter) or cancel (Esc) the carried card first."
		return
	}
	switch key {
	case "i":
		m.inspect = true
		m.scroll = 0
	case "left", "h":
		m.result(m.selection.Move(-1), "")
	case "right", "l":
		m.result(m.selection.Move(1), "")
	case "enter", " ":
		if m.selection.Carrying() {
			m.result(m.selection.Place(), "Card placed.")
		} else {
			m.result(m.selection.Pick(), "Carrying: arrows move, Space places, Esc restores.")
		}
	case "esc":
		m.result(m.selection.Cancel(), "Carry cancelled.")
	case "c":
		if m.layout == cb.Grid {
			m.layout = cb.Overlap
		} else {
			m.layout = cb.Grid
		}
	case "d", "o":
		err := m.game.DrawCard(p, key == "o")
		m.result(err, "Drew one card. Arrange, then discard (x) or declare (v).")
		if err == nil {
			_ = m.selection.Move(hand.Len())
		}
	case "m":
		if m.marked[selected.ID] {
			delete(m.marked, selected.ID)
		} else {
			m.marked[selected.ID] = true
		}
		m.feedback = fmt.Sprintf("%d marked. Press g to create a group.", len(m.marked))
	case "g":
		var ids []uint64
		var cards []cb.Card
		for _, c := range hand.Cards() {
			if m.marked[c.ID] {
				ids = append(ids, c.ID)
				cards = append(cards, c)
			}
		}
		if err := m.game.AddGroup(p, ids); err != nil {
			m.feedback = err.Error()
		} else {
			m.marked = map[uint64]bool{}
			meld, err := game.Classify(cards, m.game.Config().Wild)
			if err != nil {
				m.feedback = "Draft group saved: " + err.Error()
			} else {
				m.feedback = "Group saved: " + string(meld.Kind) + "."
			}
		}
	case "u":
		m.result(m.game.Ungroup(p, selected.ID), "Group removed. Mark its cards to create an edited group.")
	case "z":
		m.result(m.game.ClearGroups(p), "Groups cleared. Declare now to search every legal arrangement.")
		m.marked = map[uint64]bool{}
	case "a":
		// Preview and store an exhaustive arrangement excluding the selected discard.
		if m.game.Phase() != game.Discard {
			m.feedback = "Draw first, then select a final discard for automatic grouping."
			return
		}
		cards := slices.Delete(hand.Cards(), m.selection.Selected(), m.selection.Selected()+1)
		melds, err := game.FindPartition(cards, m.game.Config())
		if err != nil {
			m.feedback = err.Error()
			return
		}
		_ = m.game.ClearGroups(p)
		for _, meld := range melds {
			var ids []uint64
			for _, c := range meld.Cards {
				ids = append(ids, c.ID)
			}
			if err := m.game.AddGroup(p, ids); err != nil {
				m.feedback = err.Error()
				return
			}
		}
		m.marked = map[uint64]bool{}
		m.feedback = "Complete arrangement found. Press v with this final discard selected."
	case "x":
		if err := m.game.DiscardCard(p, selected.ID); err != nil {
			m.feedback = err.Error()
		} else {
			m.selection = nil
			m.marked = map[uint64]bool{}
			m.feedback = ""
		}
	case "v":
		if err := m.game.Declare(p, selected.ID); err != nil {
			m.feedback = "Declaration rejected: " + err.Error()
		} else {
			m.screen = victory
			m.selection = nil
			m.marked = map[uint64]bool{}
			m.feedback = ""
			m.winnerGroup = 0
			m.scroll = 0
		}
	}
}

func rankLabel(rank cb.Rank) string {
	return strings.TrimSuffix((cb.Card{Rank: rank, Suit: cb.Spades}).Short(), "S")
}
