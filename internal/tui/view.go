package tui

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F9E7D2")).Background(lipgloss.Color("#19474A")).Padding(0, 1)
	goldStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B58B51"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#92AAA5"))
)

func (m *Model) View() string {
	if m.width < 1 || m.height < 1 {
		return ""
	}
	if m.quit {
		return m.fit(titleStyle.Render("LEAVE THE TABLE?") + "\n\nQuit this game? Unsaved round progress will be lost.\n\ny / Enter: quit    n / Esc: return")
	}
	if m.help {
		return m.helpView()
	}
	if m.inspect {
		return m.inspectView()
	}
	switch m.screen {
	case menu:
		seven, thirteen := "  7-card Rummy", "  13-card Rummy"
		if m.modeChoice == 0 {
			seven = goldStyle.Render("› 7-card Rummy")
		} else {
			thirteen = goldStyle.Render("› 13-card Rummy")
		}
		return m.fit(titleStyle.Render("RUMMY · A TABLE FOR TWO") + "\n\nLocal hot-seat play · Cardboard Vintage\n\nChoose your round\n\n" + seven + "\n" + thirteen + "\n\nArrows: choose   Enter: continue\n?: rules & controls   q: quit")
	case jokerMenu:
		var ranks []string
		for rank := cb.Ace; rank <= cb.King; rank++ {
			label := " " + rankLabel(rank) + " "
			if rank == m.rank {
				label = goldStyle.Render("[" + rankLabel(rank) + "]")
			}
			ranks = append(ranks, label)
		}
		return m.fit(titleStyle.Render("CHOOSE THE WILD RANK") + "\n\n" + m.wrap(strings.Join(ranks, " ")) + "\n\n" + m.wrap("Every card of this rank is wild. Printed jokers are always wild. A rank-wild card can still be used naturally in a pure sequence.") + "\n\n" + m.wrap("Arrows: choose · A, 2–9, t/0, J, Q, K: shortcut · Enter: deal · Esc: back") + "\n\n" + m.feedback)
	case victory:
		return m.victoryView()
	default:
		return m.tableView()
	}
}

func (m *Model) heading() string {
	cfg := m.game.Config()
	return titleStyle.Render(fmt.Sprintf("RUMMY %d", cfg.Cards)) + "  " + goldStyle.Render("WILD: "+rankLabel(cfg.Wild)) + " + printed jokers"
}

func (m *Model) tableView() string {
	p := m.game.Active()
	if m.game.Phase() == game.Handoff {
		// Deliberately render no table, cards, groups, or outgoing action feedback.
		return m.fit(m.heading() + "\n\n" + goldStyle.Render(fmt.Sprintf("Pass control to Player %d", p+1)) + "\n\nBoth hands are hidden.\n\nPress Enter when you are ready to reveal your hand.\n\n?: help   q: quit")
	}
	hand, err := m.game.Hand(p)
	if err != nil {
		return m.fit(err.Error())
	}
	groups, _ := m.game.Groups(p)
	selected, _ := hand.At(m.selection.Selected())
	layout := "overlap"
	if m.layout == cb.Grid {
		layout = "grid"
	}
	header := m.heading() + "\n" + goldStyle.Render(fmt.Sprintf("PLAYER %d", p+1)) + fmt.Sprintf(" · %s · %d cards · %s", m.game.Phase(), hand.Len(), layout)
	details := fmt.Sprintf("Selected %d/%d: %s", m.selection.Selected()+1, hand.Len(), selected.Short())
	if m.selection.Carrying() {
		details += " · CARRYING"
	}
	var marks []string
	for i, c := range hand.Cards() {
		if m.marked[c.ID] {
			marks = append(marks, fmt.Sprint(i+1))
		}
	}
	if len(marks) > 0 {
		details += " · Marked positions: " + strings.Join(marks, ", ")
	}
	var groupLines []string
	byID := map[uint64]cb.Card{}
	for _, c := range hand.Cards() {
		byID[c.ID] = c
	}
	for i, ids := range groups {
		var cards []cb.Card
		for _, id := range ids {
			cards = append(cards, byID[id])
		}
		meld, err := game.Classify(cards, m.game.Config().Wild)
		label := string(meld.Kind)
		if err != nil {
			label = "INVALID: " + err.Error()
		}
		cursor := " "
		for _, id := range ids {
			if id == selected.ID {
				cursor = "›"
				break
			}
		}
		groupLines = append(groupLines, fmt.Sprintf("%s G%d [%s] %s", cursor, i+1, cardNames(cards), label))
	}
	if len(groups) == 0 {
		groupLines = []string{"No groups yet · m marks / g groups · v searches all partitions if no groups."}
	}
	footer := "←/→ h/l: select  Space: carry  m/g/u: groups  d/o: draw  x: discard  v: declare\na: auto-group  z: clear groups  i: inspect  c: layout  ?: help  q: quit"
	// Keep feedback and controls visible even when the artwork cannot fit.
	if m.width < 88 || m.height < 36 {
		return m.compactTable(header, details, hand, groupLines, footer)
	}
	groupCapacity := max(1, m.height-30)
	if len(groupLines) > groupCapacity {
		hidden := len(groupLines) - groupCapacity + 1
		groupLines = append(groupLines[:groupCapacity-1], fmt.Sprintf("+ %d more groups · i: inspect all groups", hidden))
	}
	leftWidth := m.width - 57
	bodyHeight := m.height - 11 - len(groupLines)
	bodyHeight = max(24, bodyHeight)
	art := m.renderer.Pile(hand, cb.View{Width: leftWidth, Height: bodyHeight, Layout: m.layout, Selected: m.selection.Selected(), Picking: m.selection.Carrying(), Picked: m.selection.Picked()})
	stock := m.renderer.Back()
	if m.game.StockCount() == 0 {
		stock = lipgloss.NewStyle().Width(cb.PixelWidth).Height(cb.PixelHeight).Render("Stock empty\nRecycles on next draw")
	}
	open := "Open pile empty"
	if c, err := m.game.DiscardTop(); err == nil {
		open = m.renderer.Card(c)
	}
	open = lipgloss.NewStyle().Width(cb.PixelWidth).Height(cb.PixelHeight).Render(open)
	rail := fmt.Sprintf("CLOSED: %-18d OPEN: %d\n", m.game.StockCount(), m.game.DiscardCount()) + lipgloss.JoinHorizontal(lipgloss.Top, stock, "  ", open) + "\n[d] draw closed              [o] draw open"
	body := lipgloss.JoinHorizontal(lipgloss.Top, art, "   ", rail)
	// At larger heights the grid can show additional rows; all text is bounded.
	return m.fit(header + "\n" + m.singleLine(details) + "\n" + body + "\n" + strings.Join(groupLines, "\n") + "\n" + goldStyle.Render(m.singleLine(m.feedback)) + "\n" + mutedStyle.Render(footer))
}

func cardNames(cards []cb.Card) string {
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Short()
	}
	return strings.Join(names, " ")
}

func (m *Model) compactTable(header, details string, hand *cb.Pile, groups []string, _ string) string {
	top := "empty"
	if c, err := m.game.DiscardTop(); err == nil {
		top = c.Short()
	}
	public := fmt.Sprintf("Closed: %d · Open: %s (%d)", m.game.StockCount(), top, m.game.DiscardCount())
	var tokens []string
	for i, c := range hand.Cards() {
		token := fmt.Sprintf("%d:%s", i+1, c.Short())
		if m.marked[c.ID] {
			token += "*"
		}
		if i == m.selection.Selected() {
			token = "[" + token + "]"
		}
		tokens = append(tokens, token)
	}
	// Reserve the bottom of even a short screen for selected-card details,
	// feedback, and a short help hint. The middle is a scrolling information
	// area so every card and every group remains inspectable on small terminals.
	lines := strings.Split(m.wrap(public)+"\n"+m.wrap(strings.Join(tokens, "  "))+"\n"+m.wrap(strings.Join(groups, "\n")), "\n")
	available := max(1, m.height-7)
	// Focus the selected card's line while retaining manual group inspection in
	// the help-independent details modal (i).
	start := 0
	selectedToken := fmt.Sprintf("[%d:", m.selection.Selected()+1)
	for i, line := range lines {
		if strings.Contains(line, selectedToken) && i >= available {
			start = i - available + 1
			break
		}
	}
	middle := strings.Join(lines[start:min(start+available, len(lines))], "\n")
	footer := "d/o draw · x discard · v declare\n? help · i groups · q quit"
	return m.fit(header + "\n" + m.singleLine(details) + "\n" + middle + "\n" + goldStyle.Render(m.singleLine(m.feedback)) + "\n" + footer)
}

func (m *Model) victoryView() string {
	melds := m.game.Winning()
	lines := []string{m.heading(), goldStyle.Render(fmt.Sprintf("PLAYER %d WINS", m.game.Winner()+1)), "Every card accounted for · winning arrangement:"}
	for i, meld := range melds {
		var cards []string
		for j, c := range meld.Cards {
			name := c.Short()
			if !c.SameFace(meld.As[j]) {
				name += "→" + meld.As[j].Short()
			}
			cards = append(cards, name)
		}
		prefix := "  "
		if i == m.winnerGroup {
			prefix = "› "
		}
		lines = append(lines, fmt.Sprintf("%sG%d · %s · %s", prefix, i+1, meld.Kind, strings.Join(cards, " ")))
	}
	if m.width < 88 || m.height < 36 {
		content := strings.Split(m.wrap(strings.Join(lines[1:], "\n")), "\n")
		available := max(1, m.height-3)
		m.scroll = min(m.scroll, max(0, len(content)-available))
		return m.fit(lines[0] + "\n" + strings.Join(content[m.scroll:min(len(content), m.scroll+available)], "\n") + "\nj/k scroll · Enter/p play again · q quit")
	}
	if m.width >= 28 && m.height >= len(lines)+26 {
		pile, _ := cb.NewPile(melds[m.winnerGroup].Cards...)
		lines = append(lines, m.renderer.Pile(pile, cb.View{Width: m.width, Height: 24, Layout: cb.Overlap, Selected: -1}))
	}
	lines = append(lines, "←/→: inspect group · Enter / p: play again · q: quit")
	return m.fit(strings.Join(lines, "\n"))
}

const helpText = `CONTROLS
Left/Right or h/l  Select a card; move it while carrying
Space / Enter     Pick up or place the selected card
Esc               Restore a carried card / close overlay
m                 Mark or unmark the selected card
g                 Create a draft group from marked cards
u                 Ungroup the selected card's entire group
z                 Clear all groups and marks
i                 Inspect groups (scroll with arrows/j/k)
d / o             Draw closed / open (once per turn)
x                 Discard selected card and pass control
v                 Declare with selected card as final discard
a                 Auto-group all except the selected discard
c                 Toggle overlap / paged grid layout
?                 Open / close this help; arrows/j/k scroll
q / Ctrl+C        Quit with confirmation

TURN AND PRIVACY
Draw one, arrange, then discard one. Each handoff hides both
hands until the next player presses Enter. Group drafts persist
between your turns. Place/cancel a carried card before actions.

DECLARING
Hold 8/14 cards and select the final discard. With no draft
groups, v searches ALL partitions of the remaining 7/13 cards.
With draft groups, every remaining card must be grouped exactly
once; ungroup the final discard first. Invalid declarations leave
all cards and groups unchanged. a previews an automatic winning
arrangement; z clears drafts to restore exhaustive declaration.

HOUSE RULES
7 cards: exactly TWO groups, 3 + 4; at least one PURE sequence.
13 cards: cover every card; at least one PURE sequence.
No compulsory second sequence in either mode.
Sequences: 3+ consecutive cards in the same suit. A-2-3 and
Q-K-A work; K-A-2 does not. Printed and rank-wild jokers can
substitute in impure sequences and 3/4-card sets. Natural set
cards need distinct suits, even across duplicate decks.
A wild-rank card used as its OWN face is allowed in a PURE
sequence (4H 5H 6H with 6 wild). Substituting 6S for 6H makes
that sequence impure. Printed jokers never make a pure sequence.

STOCK AND DISPLAY
An empty stock recycles older discards, keeping the open top.
Small terminals use compact ranks/suits; resize to 88x36 for
Cardboard Vintage art. Grid pages follow the selected card.
Marks appear as positions in art mode and * in compact mode.

Esc / ?: close · Up/Down or j/k: scroll`

func (m *Model) helpView() string {
	return m.scrollView(helpText, "RUMMY · RULES & CONTROLS")
}

func (m *Model) scrollView(content, title string) string {
	lines := strings.Split(m.wrap(content), "\n")
	available := max(1, m.height-2)
	m.scroll = min(m.scroll, max(0, len(lines)-available))
	return m.fit(titleStyle.Render(title) + "\n" + strings.Join(lines[m.scroll:min(m.scroll+available, len(lines))], "\n") + "\n↑/↓ scroll · Esc close")
}

func (m *Model) wrap(text string) string       { return ansi.Wrap(text, max(1, m.width), " ") }
func (m *Model) singleLine(text string) string { return ansi.Truncate(text, max(1, m.width), "…") }
func (m *Model) fit(text string) string {
	lines := strings.Split(text, "\n")
	lines = lines[:min(len(lines), max(1, m.height))]
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, max(1, m.width), "")
	}
	return strings.Join(lines, "\n")
}

func (m *Model) inspectView() string {
	if m.game == nil || m.game.Phase() == game.Handoff {
		return m.fit("Confirm the handoff first.")
	}
	p := m.game.Active()
	hand, err := m.game.Hand(p)
	if err != nil {
		return m.fit(err.Error())
	}
	groups, _ := m.game.Groups(p)
	byID := map[uint64]cb.Card{}
	for _, c := range hand.Cards() {
		byID[c.ID] = c
	}
	var lines []string
	for i, ids := range groups {
		var cards []cb.Card
		for _, id := range ids {
			cards = append(cards, byID[id])
		}
		lines = append(lines, fmt.Sprintf("G%d: %s", i+1, cardNames(cards)))
		meld, err := game.Classify(cards, m.game.Config().Wild)
		if err != nil {
			lines = append(lines, "INVALID: "+err.Error())
		} else {
			lines = append(lines, string(meld.Kind))
			for j, c := range meld.Cards {
				if !c.SameFace(meld.As[j]) {
					lines = append(lines, c.Short()+" substitutes for "+meld.As[j].Short())
				}
			}
		}
		lines = append(lines, "")
	}
	if len(groups) == 0 {
		lines = append(lines, "No draft groups. Mark cards with m, then press g.")
	}
	lines = append(lines, "Close with Esc / i. Select a card and press u to remove its group; mark cards and g to rebuild it.")
	return m.scrollView(strings.Join(lines, "\n"), "YOUR MELD GROUPS")
}
