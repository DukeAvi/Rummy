package tui

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func press(m *Model, key string) tea.Cmd {
	special := map[string]tea.KeyType{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "left": tea.KeyLeft, "right": tea.KeyRight, "up": tea.KeyUp, "down": tea.KeyDown, "ctrl+c": tea.KeyCtrlC}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if kind, ok := special[key]; ok {
		msg = tea.KeyMsg{Type: kind}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func tableModel(t *testing.T, n int) *Model {
	t.Helper()
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	g, err := game.New(game.Config{Cards: n, Wild: cb.Six}, rand.New(rand.NewPCG(5, 9)))
	if err != nil {
		t.Fatal(err)
	}
	m.game = g
	m.screen = table
	press(m, "enter")
	return m
}

func hand(t *testing.T, m *Model) *cb.Pile {
	t.Helper()
	h, err := m.game.Hand(m.game.Active())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSetupAndJokerSelection(t *testing.T) {
	for _, n := range []int{7, 13} {
		m := New()
		if m.Init() != nil || m.View() != "" {
			t.Fatal("unexpected initial output")
		}
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		if !strings.Contains(m.View(), "A TABLE FOR TWO") {
			t.Fatal("missing main menu")
		}
		if n == 13 {
			press(m, "down")
		}
		press(m, "enter")
		if m.screen != jokerMenu {
			t.Fatal("no joker menu")
		}
		press(m, "A")
		if m.rank != cb.Ace {
			t.Fatal("ace shortcut")
		}
		press(m, "left")
		if m.rank != cb.King {
			t.Fatal("rank does not wrap")
		}
		press(m, "right")
		if m.rank != cb.Ace {
			t.Fatal("rank does not wrap")
		}
		for _, r := range []struct {
			key  string
			rank cb.Rank
		}{{"6", cb.Six}, {"t", cb.Ten}, {"J", cb.Jack}, {"Q", cb.Queen}, {"K", cb.King}} {
			press(m, r.key)
			if m.rank != r.rank {
				t.Fatalf("shortcut %s got rank %v", r.key, m.rank)
			}
		}
		press(m, "esc")
		if m.screen != menu {
			t.Fatal("cannot return to modes")
		}
		press(m, "enter")
		press(m, "6")
		press(m, "enter")
		if m.screen != table || m.game.Phase() != game.Handoff || m.game.Config().Cards != n || m.game.Config().Wild != cb.Six {
			t.Fatal("setup configuration lost")
		}
		if !strings.Contains(m.View(), "Pass control to Player 1") || strings.Contains(m.View(), "▀") {
			t.Fatal("initial privacy gate")
		}
		press(m, " ")
		press(m, "d")
		if m.game.Phase() != game.Handoff {
			t.Fatal("handoff needs Enter")
		}
		press(m, "enter")
		if hand(t, m).Len() != n || m.game.Phase() != game.Draw {
			t.Fatal("hand not revealed")
		}
	}
}

func TestCarryMarksAndGroupEditing(t *testing.T) {
	m := tableModel(t, 7)
	h := hand(t, m)
	original := h.Cards()
	id := original[0].ID
	press(m, "m")
	press(m, " ")
	press(m, "right")
	press(m, "right")
	c, _ := h.At(2)
	if c.ID != id || !m.selection.Carrying() || !m.marked[id] {
		t.Fatal("carry or identity failed")
	}
	before := h.Cards()
	stock := m.game.StockCount()
	for _, key := range []string{"d", "o", "x", "v", "a", "z", "g", "u", "m"} {
		press(m, key)
		if !reflect.DeepEqual(h.Cards(), before) || m.game.StockCount() != stock || !m.selection.Carrying() {
			t.Fatalf("%s mutated carrying state", key)
		}
	}
	press(m, "esc")
	if !reflect.DeepEqual(h.Cards(), original) {
		t.Fatal("cancel changed order")
	}
	press(m, " ")
	press(m, "right")
	press(m, "enter")
	if m.selection.Carrying() {
		t.Fatal("place kept carry")
	}
	press(m, "right")
	press(m, "m")
	press(m, "g")
	groups, err := m.game.Groups(0)
	if err != nil || len(groups) != 1 || len(groups[0]) != 2 || len(m.marked) != 0 {
		t.Fatal("marks did not form draft group")
	}
	if !strings.Contains(m.feedback, "at least three") {
		t.Fatal("invalid group reason absent")
	}
	press(m, "i")
	if !strings.Contains(m.View(), "INVALID") {
		t.Fatal("group inspector missing reason")
	}
	press(m, "esc")
	press(m, "u")
	groups, _ = m.game.Groups(0)
	if len(groups) != 0 {
		t.Fatal("ungroup failed")
	}
	press(m, "m")
	press(m, "g")
	press(m, "z")
	groups, _ = m.game.Groups(0)
	if len(groups) != 0 || len(m.marked) != 0 {
		t.Fatal("clear groups failed")
	}
	press(m, "c")
	if m.layout != cb.Grid {
		t.Fatal("grid toggle")
	}
	press(m, "c")
	if m.layout != cb.Overlap {
		t.Fatal("overlap toggle")
	}
}

func TestDrawDiscardPrivacyAndOpenDraw(t *testing.T) {
	for _, n := range []int{7, 13} {
		m := tableModel(t, n)
		h := hand(t, m)
		before := h.Cards()
		press(m, "x")
		if !reflect.DeepEqual(before, h.Cards()) || m.game.Phase() != game.Draw {
			t.Fatal("discard before draw")
		}
		stock := m.game.StockCount()
		press(m, "d")
		if h.Len() != n+1 || m.game.StockCount() != stock-1 || m.game.Phase() != game.Discard {
			t.Fatal("closed draw")
		}
		before = h.Cards()
		press(m, "d")
		press(m, "o")
		if !reflect.DeepEqual(before, h.Cards()) {
			t.Fatal("double draw")
		}
		selected, _ := h.At(m.selection.Selected())
		press(m, "m")
		m.feedback = "SECRET FROM PLAYER ONE"
		press(m, "x")
		if m.game.Phase() != game.Handoff || m.game.Active() != 1 || m.selection != nil || len(m.marked) != 0 {
			t.Fatal("turn handoff state")
		}
		hidden := m.View()
		if !strings.Contains(hidden, "Pass control to Player 2") || strings.Contains(hidden, "SECRET") || strings.Contains(hidden, "▀") || strings.Contains(hidden, "Selected") || strings.Contains(hidden, "G1") {
			t.Fatal("handoff leaked previous view")
		}
		for _, key := range []string{"d", "o", "x", "v", "m", "g", "u", "i", "h", " "} {
			press(m, key)
			if m.View() != hidden {
				t.Fatalf("%s bypassed privacy gate", key)
			}
		}
		press(m, "enter")
		next := hand(t, m)
		baseline := m.View()
		if err := h.Move(0, h.Len()-1); err != nil {
			t.Fatal(err)
		}
		if m.View() != baseline {
			t.Fatal("inactive hand influences rendering")
		}
		press(m, "o")
		last, _ := next.At(next.Len() - 1)
		if last.ID != selected.ID || next.Len() != n+1 {
			t.Fatal("did not draw the visible open card")
		}
		press(m, "x")
		if !strings.Contains(m.View(), "Pass control to Player 1") {
			t.Fatal("reverse privacy gate")
		}
		want := 54
		if n == 13 {
			want = 108
		}
		if m.game.TotalCards() != want {
			t.Fatal("TUI lost cards")
		}
	}
}

// Install an isolated UI hand fixture using Cardboard transfers. Engine tests
// independently verify whole-deck identity conservation with actual deals.
func fixtureHand(t *testing.T, m *Model, n int) {
	t.Helper()
	h := hand(t, m)
	scratch, _ := cb.NewPile()
	for h.Len() > 0 {
		if err := cb.Transfer(h, 0, scratch, 0); err != nil {
			t.Fatal(err)
		}
	}
	cards := []cb.Card{{Rank: cb.Four, Suit: cb.Hearts}, {Rank: cb.Five, Suit: cb.Hearts}, {Rank: cb.Six, Suit: cb.Hearts}, {Rank: cb.Nine, Suit: cb.Hearts}, {Rank: cb.Nine, Suit: cb.Spades}, {Rank: cb.Nine, Suit: cb.Diamonds}, {Rank: cb.Nine, Suit: cb.Clubs}}
	if n == 13 {
		cards = append(cards, cb.Card{Rank: cb.Queen, Suit: cb.Hearts}, cb.Card{Rank: cb.Queen, Suit: cb.Spades}, cb.Card{Rank: cb.Queen, Suit: cb.Diamonds}, cb.Card{Rank: cb.Two, Suit: cb.Spades}, cb.Card{Rank: cb.Two, Suit: cb.Diamonds}, cb.Card{Rank: cb.Two, Suit: cb.Clubs})
	}
	cards = append(cards, cb.Card{Rank: cb.Eight, Suit: cb.Diamonds})
	for i := range cards {
		cards[i].ID = uint64(1000 + i)
	}
	source, err := cb.NewPile(cards...)
	if err != nil {
		t.Fatal(err)
	}
	for source.Len() > 0 {
		if err := cb.Transfer(source, 0, h, h.Len()); err != nil {
			t.Fatal(err)
		}
	}
	m.selection = cb.NewSelection(h)
	_ = m.selection.Move(n)
}

func TestDeclarationVictoryAndPlayAgain(t *testing.T) {
	for _, n := range []int{7, 13} {
		for _, automatic := range []bool{false, true} {
			m := tableModel(t, n)
			press(m, "d")
			fixtureHand(t, m, n)
			if automatic {
				press(m, "a")
				groups, _ := m.game.Groups(0)
				if len(groups) < 2 {
					t.Fatal("automatic grouping missing")
				}
			}
			press(m, "v")
			if m.screen != victory || m.game.Winner() != 0 || handCount(m.game.Winning()) != n {
				t.Fatalf("victory missing: %s", m.feedback)
			}
			output := m.View()
			if !strings.Contains(output, "PLAYER 1 WINS") || !strings.Contains(output, "pure sequence") || !strings.Contains(output, "9C") {
				t.Fatal("incomplete winning arrangement")
			}
			press(m, "right")
			if m.winnerGroup != 1 {
				t.Fatal("winner group navigation")
			}
			press(m, "p")
			if m.screen != menu || m.game != nil {
				t.Fatal("play again did not reset")
			}
			press(m, "enter")
			press(m, "3")
			press(m, "enter")
			if m.game.Config().Wild != cb.Three {
				t.Fatal("cannot select new joker for next round")
			}
		}
	}
}

func handCount(melds []game.Meld) int {
	n := 0
	for _, m := range melds {
		n += len(m.Cards)
	}
	return n
}

func TestFailedDeclarationAndPartialGrouping(t *testing.T) {
	m := tableModel(t, 7)
	press(m, "d")
	fixtureHand(t, m, 7)
	_ = m.selection.Move(-7)
	press(m, "m")
	press(m, "right")
	press(m, "m")
	press(m, "right")
	press(m, "m")
	press(m, "g")
	_ = m.selection.Move(7)
	before := hand(t, m).Cards()
	groups, _ := m.game.Groups(0)
	press(m, "v")
	afterGroups, _ := m.game.Groups(0)
	if m.screen != table || !reflect.DeepEqual(before, hand(t, m).Cards()) || !reflect.DeepEqual(groups, afterGroups) || !strings.Contains(m.feedback, "ungrouped") {
		t.Fatal("invalid declaration not recoverable")
	}
	press(m, "z")
	press(m, "v")
	if m.screen != victory {
		t.Fatal("clear did not restore exhaustive validation")
	}
	// A wrong final discard leaves all cards, marks, and groups in place.
	m = tableModel(t, 7)
	press(m, "d")
	fixtureHand(t, m, 7)
	_ = m.selection.Move(-7)
	press(m, "m")
	before = hand(t, m).Cards()
	press(m, "v")
	if m.screen != table || m.game.Phase() != game.Discard || !reflect.DeepEqual(before, hand(t, m).Cards()) || len(m.marked) != 1 || !strings.Contains(m.feedback, "rejected") {
		t.Fatal("failed declaration changed hand")
	}
}

func TestHelpAndQuitAreModal(t *testing.T) {
	m := tableModel(t, 7)
	before := hand(t, m).Cards()
	press(m, "?")
	press(m, "d")
	if !m.help || m.game.Phase() != game.Draw || !reflect.DeepEqual(before, hand(t, m).Cards()) {
		t.Fatal("help allowed actions")
	}
	for range 90 {
		press(m, "down")
		_ = m.View()
	}
	if !strings.Contains(m.View(), "Esc / ?") {
		t.Fatal("help not scrollable")
	}
	press(m, "esc")
	if m.help {
		t.Fatal("help did not close")
	}
	press(m, "q")
	if !m.quit || strings.Contains(m.View(), "▀") {
		t.Fatal("quit not opaque")
	}
	press(m, "d")
	if m.game.Phase() != game.Draw {
		t.Fatal("quit allowed draw")
	}
	press(m, "n")
	if m.quit {
		t.Fatal("quit cancellation")
	}
	press(m, "ctrl+c")
	cmd := press(m, "y")
	if cmd == nil {
		t.Fatal("quit has no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("did not quit")
	}
}

func TestResponsiveArtworkAndBounds(t *testing.T) {
	m := tableModel(t, 13)
	press(m, "d")
	if !strings.Contains(m.View(), "▀") || !strings.Contains(m.View(), "CLOSED:") || !strings.Contains(m.View(), "OPEN:") {
		t.Fatal("missing Cardboard artwork/piles")
	}
	for _, size := range [][2]int{{1, 1}, {12, 5}, {28, 12}, {40, 18}, {80, 24}, {87, 35}, {88, 36}, {100, 40}, {160, 70}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, layout := range []cb.Layout{cb.Overlap, cb.Grid} {
				m.layout = layout
				for _, screen := range []screen{table, menu, jokerMenu} {
					m.screen = screen
					assertBounds(t, m.View(), size[0], size[1])
				}
			}
			m.help = true
			assertBounds(t, m.View(), size[0], size[1])
			m.help = false
			m.screen = table
		})
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if output := m.View(); !strings.Contains(output, "Zoom out") || strings.Contains(output, "▀") {
		t.Fatal("small terminal must request resizing")
	}
}

func assertBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("%d rows exceeds %d", len(lines), height)
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > width {
			t.Fatalf("line exceeds %d columns: %q", width, line)
		}
	}
}

func TestManyDraftGroupsKeepActionsVisible(t *testing.T) {
	m := tableModel(t, 13)
	press(m, "d")
	h := hand(t, m)
	for _, c := range h.Cards() {
		if err := m.game.AddGroup(0, []uint64{c.ID}); err != nil {
			t.Fatal(err)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 88, Height: 36})
	m.feedback = "Drafts need editing."
	view := m.View()
	assertBounds(t, view, 88, 36)
	for _, text := range []string{"more groups", "Drafts need editing.", "q: quit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("many groups hide %q", text)
		}
	}
	press(m, "i")
	for range 100 {
		press(m, "down")
		_ = m.View()
	}
	if !strings.Contains(m.View(), "G14:") {
		t.Fatal("last draft inaccessible in inspector")
	}
}

func TestSmallWinnerRequestsResize(t *testing.T) {
	m := tableModel(t, 13)
	press(m, "d")
	fixtureHand(t, m, 13)
	press(m, "v")
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	if view := m.View(); !strings.Contains(view, "Zoom out") || strings.Contains(view, "4H") {
		t.Fatal("winner must request resize, not replace art with text")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if !strings.Contains(m.View(), "▀") {
		t.Fatal("winner artwork did not recover after resize")
	}
}
