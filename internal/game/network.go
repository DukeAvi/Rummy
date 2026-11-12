package game

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
)

// PrivateView is the only game state sent to a network player. It deliberately
// contains neither the opponent's cards/groups nor the closed stock order.
type PrivateView struct {
	Config                  Config
	Active                  int
	Phase                   Phase
	Hand                    []cb.Card
	Groups                  [][]uint64
	Stock, Discards, Winner int
	Top                     *cb.Card
	Winning                 []Meld
}

func (g *Game) ViewFor(player int) PrivateView {
	v := PrivateView{Config: g.config, Active: g.active, Phase: g.phase, Stock: g.StockCount(), Discards: g.DiscardCount(), Winner: g.winner, Winning: g.Winning()}
	if player >= 0 && player < 2 {
		h, _ := g.table.Hand(player)
		v.Hand = h.Cards()
		v.Groups = cloneGroups(g.groups[player])
	}
	if c, err := g.DiscardTop(); err == nil {
		v.Top = &c
	}
	return v
}

func (g *Game) Reorder(player int, ids []uint64) error {
	if err := g.check(player); err != nil {
		return err
	}
	h, _ := g.table.Hand(player)
	if len(ids) != h.Len() {
		return fmt.Errorf("order must include every card")
	}
	seen := map[uint64]bool{}
	for _, id := range ids {
		if seen[id] || indexOf(h, id) < 0 {
			return fmt.Errorf("invalid card order")
		}
		seen[id] = true
	}
	for i, id := range ids {
		if err := h.Move(indexOf(h, id), i); err != nil {
			return err
		}
	}
	return nil
}
