package lan

import (
	"encoding/json"
	cb "github.com/DukeAvi/cardboard"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

type seat struct {
	s          *Session
	token      string
	seq, round uint64
}

func (c *seat) action(action string, id uint64, ids []uint64) Response {
	c.seq++
	out := c.s.Handle(Request{Version: Version, Token: c.token, Action: action, Seq: c.seq, Round: c.round, ID: id, IDs: ids})
	c.round = out.Round
	return out
}
func (c *seat) poll() Response {
	return c.s.Handle(Request{Version: Version, Token: c.token, Action: "poll"})
}
func pair(t *testing.T, n int) (*Session, *seat, *seat) {
	t.Helper()
	s, err := New(game.Config{Cards: n, Wild: cb.Six})
	if err != nil {
		t.Fatal(err)
	}
	host := &seat{s: s, token: s.HostToken()}
	joined := s.Handle(Request{Version: Version, Token: s.JoinToken(), Action: "join"})
	if joined.Error != "" || joined.Player != 1 || joined.Token == "" {
		t.Fatal(joined)
	}
	guest := &seat{s: s, token: joined.Token}
	if r := host.action("ready", 0, nil); r.View != nil || r.Error != "" {
		t.Fatal("started without both ready", r)
	}
	if r := guest.action("ready", 0, nil); r.Error != "" || r.View == nil {
		t.Fatal(r)
	}
	return s, host, guest
}
func TestAdmissionPrivacyAndRestrictions(t *testing.T) {
	s, h, g := pair(t, 13)
	for _, req := range []Request{{Version: Version, Token: "wrong", Action: "join"}, {Version: Version, Token: s.JoinToken(), Action: "join"}, {Version: 99, Token: h.token, Action: "poll"}} {
		out := s.Handle(req)
		if out.Error == "" || out.View != nil {
			t.Fatal("unauthorized client got state", out)
		}
	}
	hv, gv := h.poll().View, g.poll().View
	if len(hv.Hand) != 13 || len(gv.Hand) != 13 {
		t.Fatal("bad hands")
	}
	for _, a := range hv.Hand {
		for _, b := range gv.Hand {
			if a.ID == b.ID {
				t.Fatal("opponent hand leaked")
			}
		}
	}
	if hv.Top == nil || hv.Stock != 81 {
		t.Fatal("bad public table")
	}
	before := g.poll().View
	if r := g.action("draw", 0, nil); r.Error == "" || !reflect.DeepEqual(before, r.View) {
		t.Fatal("opponent action mutated state")
	}
	if r := h.action("discard", hv.Hand[0].ID, nil); r.Error == "" {
		t.Fatal("discard before draw")
	}
	r := h.action("draw", 0, nil)
	if r.Error != "" || len(r.View.Hand) != 14 {
		t.Fatal(r)
	}
	if r = h.action("draw", 0, nil); r.Error == "" || len(r.View.Hand) != 14 {
		t.Fatal("double draw")
	}
	// Serialized snapshots contain only this seat's groups and cards.
	encoded, _ := json.Marshal(g.poll())
	if strings.Contains(string(encoded), "table") || strings.Contains(string(encoded), "common") {
		t.Fatal("engine internals serialized")
	}
	if r = h.action("group", 0, []uint64{hv.Hand[0].ID, hv.Hand[1].ID, hv.Hand[2].ID}); r.Error != "" {
		t.Fatal(r)
	}
	if len(g.poll().View.Groups) != 0 {
		t.Fatal("opponent groups leaked")
	}
	old := h.poll().View
	if r = h.action("declare", 999999, nil); r.Error == "" || !reflect.DeepEqual(old, r.View) {
		t.Fatal("invalid declaration changed state")
	}
}
func TestRetriesStaleRoundsAndPause(t *testing.T) {
	s, h, g := pair(t, 7)
	r := h.action("draw", 0, nil)
	dup := s.Handle(Request{Version: Version, Token: h.token, Action: "draw", Seq: h.seq, Round: h.round})
	if !reflect.DeepEqual(r.View, dup.View) {
		t.Fatal("retry drew twice")
	}
	stale := s.Handle(Request{Version: Version, Token: h.token, Action: "discard", Seq: h.seq - 1, Round: h.round, ID: r.View.Hand[0].ID})
	if stale.Error == "" {
		t.Fatal("stale action accepted")
	}
	old := r.View
	h.seq++
	bad := s.Handle(Request{Version: Version, Token: h.token, Action: "discard", Seq: h.seq, Round: 99, ID: old.Hand[0].ID})
	if bad.Error == "" || !reflect.DeepEqual(old, bad.View) {
		t.Fatal("stale round changed state")
	}
	s.lastSeen = time.Now().Add(-HeartbeatTimeout - time.Second)
	if r = h.action("discard", old.Hand[0].ID, nil); r.Error == "" || !reflect.DeepEqual(old, r.View) {
		t.Fatal("disconnect did not pause")
	}
	resumed := s.Handle(Request{Version: Version, Token: g.token, Action: "join"})
	if !resumed.Connected || resumed.Seq != g.seq {
		t.Fatal("rejoin failed", resumed)
	}
	if r = h.action("discard", old.Hand[0].ID, nil); r.Error != "" || r.View.Active != 1 || r.View.Phase != game.Draw {
		t.Fatal("rejoin did not resume", r)
	}
	s.lastSeen = time.Now().Add(-ResumeTTL - time.Second)
	if out := g.poll(); out.Error == "" || out.View != nil {
		t.Fatal("expired credential accepted")
	}
	fresh, _ := New(game.Config{Cards: 7, Wild: cb.Six})
	fresh.created = time.Now().Add(-11 * time.Minute)
	if r := fresh.Handle(Request{Version: Version, Token: fresh.join, Action: "join"}); r.Error == "" {
		t.Fatal("expired join code")
	}
}
func TestRecyclingAndOpenDrawAcrossTwoSeats(t *testing.T) {
	for _, n := range []int{7, 13} {
		s, h, g := pair(t, n)
		seats := []*seat{h, g}
		for turn := 0; turn < 200; turn++ {
			c := seats[turn%2]
			before := c.poll().View
			req := Request{Version: Version, Token: c.token, Action: "draw", Seq: c.seq + 1, Round: c.round, Open: turn%5 == 0}
			c.seq++
			out := s.Handle(req)
			if out.Error != "" || len(out.View.Hand) != n+1 {
				t.Fatal(out)
			}
			if before.Stock == 0 && !req.Open && out.View.Top.ID != before.Top.ID {
				t.Fatal("recycle lost visible top")
			}
			if r := c.action("discard", out.View.Hand[turn%len(out.View.Hand)].ID, nil); r.Error != "" {
				t.Fatal(r)
			}
			want := 54
			if n == 13 {
				want = 108
			}
			if s.g.TotalCards() != want {
				t.Fatal("card loss")
			}
		}
	}
}
func installWinningHand(t *testing.T, s *Session, n int) uint64 {
	t.Helper()
	h, err := s.g.Hand(0)
	if err != nil {
		t.Fatal(err)
	}
	sink, _ := cb.NewPile()
	for h.Len() > 0 {
		_ = cb.Transfer(h, 0, sink, 0)
	}
	cards := []cb.Card{{Rank: 4, Suit: cb.Hearts}, {Rank: 5, Suit: cb.Hearts}, {Rank: 6, Suit: cb.Hearts}, {Rank: 9, Suit: cb.Spades}, {Rank: 9, Suit: cb.Hearts}, {Rank: 9, Suit: cb.Diamonds}, {Rank: 9, Suit: cb.Clubs}}
	if n == 13 {
		cards = append(cards, cb.Card{Rank: 12, Suit: cb.Spades}, cb.Card{Rank: 12, Suit: cb.Hearts}, cb.Card{Rank: 12, Suit: cb.Clubs}, cb.Card{Rank: 2, Suit: cb.Spades}, cb.Card{Rank: 2, Suit: cb.Hearts}, cb.Card{Rank: 2, Suit: cb.Clubs})
	}
	cards = append(cards, cb.Card{Rank: 8, Suit: cb.Diamonds})
	for i := range cards {
		cards[i].ID = uint64(1000 + i)
	}
	src, _ := cb.NewPile(cards...)
	for src.Len() > 0 {
		_ = cb.Transfer(src, 0, h, h.Len())
	}
	return cards[n].ID
}
func TestWinningReplayAndExit(t *testing.T) {
	for _, n := range []int{7, 13} {
		s, h, g := pair(t, n)
		if r := h.action("draw", 0, nil); r.Error != "" {
			t.Fatal(r)
		}
		id := installWinningHand(t, s, n)
		won := h.action("declare", id, nil)
		if won.Error != "" || won.View.Winner != 0 || len(g.poll().View.Winning) < 2 {
			t.Fatal(won)
		}
		cfg := game.Config{Cards: 13, Wild: cb.King}
		h.seq++
		next := s.Handle(Request{Version: Version, Token: h.token, Action: "configure", Seq: h.seq, Config: cfg})
		if next.Error != "" || next.View != nil || next.Ready[0] || next.Ready[1] {
			t.Fatal("replay setup", next)
		}
		h.round = next.Round
		g.round = g.poll().Round
		if r := g.action("ready", 0, nil); r.Error != "" || r.View != nil {
			t.Fatal(r)
		}
		if r := h.action("ready", 0, nil); r.Error != "" || r.View.Config.Wild != cb.King {
			t.Fatal(r)
		}
		if r := g.action("leave", 0, nil); !r.Closed {
			t.Fatal("leave not delivered")
		}
		if r := h.poll(); !r.Closed || r.View != nil {
			t.Fatal("closed session exposed state")
		}
	}
}
func TestFramedTransport(t *testing.T) {
	s, _ := New(game.Config{Cards: 7, Wild: cb.Six})
	a, b := net.Pipe()
	go ServeConn(b, s)
	defer a.Close()
	if err := json.NewEncoder(a).Encode(Request{Version: Version, Token: s.host, Action: "poll"}); err != nil {
		t.Fatal(err)
	}
	var out Response
	if err := json.NewDecoder(a).Decode(&out); err != nil || out.Player != 0 {
		t.Fatal(out, err)
	}
	if _, _, err := ParseCode("192.168.137.1:45873/" + s.join); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"bad", "localhost/" + s.join, "localhost:45873/short"} {
		if _, _, err := ParseCode(code); err == nil {
			t.Fatal("bad code accepted")
		}
	}
}
