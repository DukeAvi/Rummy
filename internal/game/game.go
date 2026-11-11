package game

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"math/rand/v2"
	"slices"
)

type Phase string

const (
	Handoff  Phase = "pass control"
	Draw     Phase = "draw one card"
	Discard  Phase = "discard or declare"
	Finished Phase = "round finished"
)

type Game struct {
	config  Config
	table   *cb.Table
	discard *cb.Pile
	active  int
	phase   Phase
	groups  [2][][]uint64
	winner  int
	winning []Meld
	random  *rand.Rand
}

// New shuffles with a supplied deterministic source, or Cardboard's default
// random source when random is nil. Every round begins behind a privacy gate.
func New(cfg Config, random *rand.Rand) (*Game, error) {
	deck, err := Deck(cfg)
	if err != nil {
		return nil, err
	}
	table, err := cb.NewTableWithDeck(deck)
	if err != nil {
		return nil, err
	}
	if random == nil {
		table.Shuffle()
	} else if err = table.ShuffleWith(random); err != nil {
		return nil, err
	}
	if err = table.Deal(2, cfg.Cards); err != nil {
		return nil, err
	}
	discard, _ := cb.NewPile()
	if err = cb.Transfer(table.Common(), 0, discard, 0); err != nil {
		return nil, err
	}
	for _, h := range table.Hands() {
		h.SetFaceDown(true)
	}
	return &Game{config: cfg, table: table, discard: discard, phase: Handoff, winner: -1, random: random}, nil
}

func (g *Game) Config() Config               { return g.config }
func (g *Game) Active() int                  { return g.active }
func (g *Game) Phase() Phase                 { return g.phase }
func (g *Game) Winner() int                  { return g.winner }
func (g *Game) StockCount() int              { return g.table.Common().Len() }
func (g *Game) DiscardCount() int            { return g.discard.Len() }
func (g *Game) DiscardTop() (cb.Card, error) { return g.discard.Top() }
func (g *Game) TotalCards() int              { return g.table.TotalCards() + g.discard.Len() }

func (g *Game) check(player int) error {
	if player != g.active {
		return fmt.Errorf("it is the other player's turn")
	}
	if g.phase == Handoff {
		return fmt.Errorf("confirm the handoff before playing")
	}
	if g.phase == Finished {
		return fmt.Errorf("the round has finished")
	}
	return nil
}

func (g *Game) ConfirmHandoff(player int) error {
	if player != g.active || g.phase != Handoff {
		return fmt.Errorf("no handoff for this player")
	}
	h, _ := g.table.Hand(player)
	h.SetFaceDown(false)
	g.phase = Draw
	return nil
}

// Hand exposes only the active hand after the privacy gate. The TUI uses this
// pile with Cardboard.Selection; callers must not transfer cards themselves.
func (g *Game) Hand(player int) (*cb.Pile, error) {
	if err := g.check(player); err != nil {
		return nil, err
	}
	return g.table.Hand(player)
}

func (g *Game) DrawCard(player int, open bool) error {
	if err := g.check(player); err != nil {
		return err
	}
	if g.phase != Draw {
		return fmt.Errorf("already drew; discard exactly one card")
	}
	h, _ := g.table.Hand(player)
	if open {
		if g.discard.Len() == 0 {
			return fmt.Errorf("the open discard pile is empty")
		}
		if err := cb.Transfer(g.discard, 0, h, h.Len()); err != nil {
			return err
		}
	} else {
		if g.StockCount() == 0 {
			if g.discard.Len() < 2 {
				return fmt.Errorf("stock is empty; draw from the open pile")
			}
			// Index zero stays visible while all older discards become closed stock.
			for g.discard.Len() > 1 {
				if err := cb.Transfer(g.discard, 1, g.table.Common(), g.StockCount()); err != nil {
					return err
				}
			}
			if g.random == nil {
				g.table.Shuffle()
			} else if err := g.table.ShuffleWith(g.random); err != nil {
				return err
			}
		}
		if err := g.table.Draw(player, 1); err != nil {
			return err
		}
	}
	g.phase = Discard
	return nil
}

func indexOf(h *cb.Pile, id uint64) int {
	for i, c := range h.Cards() {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func (g *Game) DiscardCard(player int, id uint64) error {
	if err := g.check(player); err != nil {
		return err
	}
	if g.phase != Discard {
		return fmt.Errorf("draw before discarding")
	}
	h, _ := g.table.Hand(player)
	index := indexOf(h, id)
	if index < 0 {
		return fmt.Errorf("select a card in your hand")
	}
	if err := cb.Transfer(h, index, g.discard, 0); err != nil {
		return err
	}
	g.removeMembership(player, id)
	h.SetFaceDown(true)
	g.active = 1 - player
	g.phase = Handoff
	return nil
}

func (g *Game) Groups(player int) ([][]uint64, error) {
	if err := g.check(player); err != nil {
		return nil, err
	}
	return cloneGroups(g.groups[player]), nil
}

func cloneGroups(groups [][]uint64) [][]uint64 {
	out := make([][]uint64, len(groups))
	for i, ids := range groups {
		out[i] = slices.Clone(ids)
	}
	return out
}

// AddGroup permits invalid melds so players can inspect and edit drafts, but
// never permits absent cards, repeated identities, or overlapping groups.
func (g *Game) AddGroup(player int, ids []uint64) error {
	if err := g.check(player); err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("mark cards first")
	}
	h, _ := g.table.Hand(player)
	seen := map[uint64]bool{}
	for _, group := range g.groups[player] {
		for _, id := range group {
			seen[id] = true
		}
	}
	for _, id := range ids {
		if indexOf(h, id) < 0 {
			return fmt.Errorf("group contains a card outside your hand")
		}
		if seen[id] {
			return fmt.Errorf("groups cannot overlap or repeat a physical card; ungroup it first")
		}
		seen[id] = true
	}
	g.groups[player] = append(g.groups[player], slices.Clone(ids))
	return nil
}

func (g *Game) Ungroup(player int, id uint64) error {
	if err := g.check(player); err != nil {
		return err
	}
	for i, group := range g.groups[player] {
		if slices.Contains(group, id) {
			g.groups[player] = slices.Delete(g.groups[player], i, i+1)
			return nil
		}
	}
	return fmt.Errorf("selected card is not in a group")
}

func (g *Game) ClearGroups(player int) error {
	if err := g.check(player); err != nil {
		return err
	}
	g.groups[player] = nil
	return nil
}

func (g *Game) removeMembership(player int, id uint64) {
	for i, group := range g.groups[player] {
		if at := slices.Index(group, id); at >= 0 {
			g.groups[player][i] = slices.Delete(group, at, at+1)
			if len(g.groups[player][i]) == 0 {
				g.groups[player] = slices.Delete(g.groups[player], i, i+1)
			}
			return
		}
	}
}

// Declare is transactional: all validation precedes every state mutation.
// With no draft groups it searches every legal partition. Explicit groups
// must instead cover the entire remaining hand and satisfy the same rules.
func (g *Game) Declare(player int, discardID uint64) error {
	if err := g.check(player); err != nil {
		return err
	}
	if g.phase != Discard {
		return fmt.Errorf("draw before declaring with a final discard")
	}
	h, _ := g.table.Hand(player)
	index := indexOf(h, discardID)
	if index < 0 {
		return fmt.Errorf("select the final discard from your hand")
	}
	cards := h.Cards()
	cards = slices.Delete(cards, index, index+1)
	var melds []Meld
	var err error
	if len(g.groups[player]) == 0 {
		melds, err = FindPartition(cards, g.config)
	} else {
		melds, err = ValidateArrangement(cards, g.groups[player], g.config)
	}
	if err != nil {
		return err
	}
	if err = cb.Transfer(h, index, g.discard, 0); err != nil {
		return err
	}
	h.SetFaceDown(true)
	g.winner = player
	g.winning = melds
	g.phase = Finished
	return nil
}

func (g *Game) Winning() []Meld {
	if g.phase != Finished {
		return nil
	}
	out := make([]Meld, len(g.winning))
	for i, m := range g.winning {
		out[i] = Meld{Kind: m.Kind, Cards: slices.Clone(m.Cards), As: slices.Clone(m.As)}
	}
	return out
}
