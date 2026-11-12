package tui

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"github.com/DukeAvi/rummy-tui/internal/lan"
	"slices"
)

// board allows local play and an authenticated network seat to use the same
// card controls and artwork. The network implementation never holds a Game.
type board interface {
	Config() game.Config
	Active() int
	Phase() game.Phase
	Winner() int
	StockCount() int
	DiscardCount() int
	DiscardTop() (cb.Card, error)
	Hand(int) (*cb.Pile, error)
	Groups(int) ([][]uint64, error)
	ConfirmHandoff(int) error
	DrawCard(int, bool) error
	DiscardCard(int, uint64) error
	Declare(int, uint64) error
	AddGroup(int, []uint64) error
	Ungroup(int, uint64) error
	ClearGroups(int) error
	Winning() []game.Meld
	TotalCards() int
}

type remote struct {
	address, token string
	player         int
	state          lan.Response
	pile           *cb.Pile
	send           func(lan.Request) (lan.Response, error)
	seq            uint64
	pending        *lan.Request
}

func (r *remote) accept(s lan.Response) {
	if s.Round != r.state.Round {
		r.pile = nil
	}
	r.state = s
	if s.View != nil {
		// Preserve local carry/reorder state when a heartbeat contains the same
		// physical cards. A draw/discard creates a new pile and selection instead.
		same := r.pile != nil && r.pile.Len() == len(s.View.Hand)
		if same {
			ids := map[uint64]bool{}
			for _, c := range r.pile.Cards() {
				ids[c.ID] = true
			}
			for _, c := range s.View.Hand {
				if !ids[c.ID] {
					same = false
				}
			}
		}
		if !same {
			r.pile, _ = cb.NewPile(s.View.Hand...)
		}
	}
}
func (r *remote) request(action string, id uint64, ids []uint64, open bool, cfg game.Config) error {
	if r.pending != nil {
		return fmt.Errorf("waiting to recover the previous action; please wait")
	}
	req := lan.Request{Version: lan.Version, Token: r.token, Action: action, Seq: r.seq + 1, Round: r.state.Round, ID: id, IDs: ids, Open: open, Config: cfg}
	r.pending = &req
	return r.retry()
}
func (r *remote) retry() error {
	if r.pending == nil {
		return nil
	}
	out, err := r.send(*r.pending)
	if err != nil {
		return err
	}
	if out.Closed {
		r.state.Closed = true
		r.pending = nil
		return fmt.Errorf("session ended")
	}
	if out.Player < 0 {
		return fmt.Errorf("%s", out.Error)
	}
	r.seq = out.Seq
	r.pending = nil
	r.accept(out)
	if out.Error != "" {
		return fmt.Errorf("%s", out.Error)
	}
	return nil
}
func (r *remote) Config() game.Config { return r.state.Config }
func (r *remote) Active() int         { return r.state.View.Active }
func (r *remote) Phase() game.Phase   { return r.state.View.Phase }
func (r *remote) Winner() int         { return r.state.View.Winner }
func (r *remote) StockCount() int     { return r.state.View.Stock }
func (r *remote) DiscardCount() int   { return r.state.View.Discards }
func (r *remote) DiscardTop() (cb.Card, error) {
	if r.state.View.Top == nil {
		return cb.Card{}, fmt.Errorf("open pile empty")
	}
	return *r.state.View.Top, nil
}
func (r *remote) Hand(p int) (*cb.Pile, error) {
	if p != r.player {
		return nil, fmt.Errorf("opponent's hand is private")
	}
	return r.pile, nil
}
func (r *remote) Groups(p int) ([][]uint64, error) {
	if p != r.player {
		return nil, fmt.Errorf("opponent's groups are private")
	}
	out := make([][]uint64, len(r.state.View.Groups))
	for i, g := range r.state.View.Groups {
		out[i] = slices.Clone(g)
	}
	return out, nil
}
func (r *remote) ConfirmHandoff(int) error { return nil }
func (r *remote) DrawCard(p int, open bool) error {
	return r.request("draw", 0, nil, open, game.Config{})
}
func (r *remote) DiscardCard(p int, id uint64) error {
	return r.request("discard", id, nil, false, game.Config{})
}
func (r *remote) Declare(p int, id uint64) error {
	return r.request("declare", id, nil, false, game.Config{})
}
func (r *remote) AddGroup(p int, ids []uint64) error {
	return r.request("group", 0, ids, false, game.Config{})
}
func (r *remote) Ungroup(p int, id uint64) error {
	return r.request("ungroup", id, nil, false, game.Config{})
}
func (r *remote) ClearGroups(p int) error { return r.request("clear", 0, nil, false, game.Config{}) }
func (r *remote) Winning() []game.Meld    { return r.state.View.Winning }
func (r *remote) TotalCards() int {
	if r.Config().Cards == 13 {
		return 108
	}
	return 54
}
