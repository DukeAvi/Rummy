package game

import (
	cb "github.com/DukeAvi/cardboard"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

func newGame(t *testing.T, n int) *Game {
	t.Helper()
	g, err := New(Config{n, cb.Six}, rand.New(rand.NewPCG(11, 22)))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func assertConserved(t *testing.T, g *Game) {
	t.Helper()
	want := 54
	if g.config.Cards == 13 {
		want = 108
	}
	seen := map[uint64]bool{}
	for _, p := range append(g.table.Hands(), g.table.Common(), g.discard) {
		for _, c := range p.Cards() {
			if c.ID == 0 || seen[c.ID] {
				t.Fatal("lost identity or duplicated card")
			}
			seen[c.ID] = true
		}
	}
	if len(seen) != want || g.TotalCards() != want {
		t.Fatalf("found %d physical cards, want %d", len(seen), want)
	}
}

func TestDeckDealWildAndIdentity(t *testing.T) {
	for _, n := range []int{7, 13} {
		cfg := Config{n, cb.Six}
		deck, err := Deck(cfg)
		if err != nil {
			t.Fatal(err)
		}
		copies := 1
		if n == 13 {
			copies = 2
		}
		if len(deck) != 54*copies {
			t.Fatal("wrong deck size")
		}
		jokers := 0
		faces := map[string]int{}
		table, err := cb.NewTableWithDeck(deck)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[uint64]bool{}
		for _, c := range table.Common().Cards() {
			if c.ID == 0 || ids[c.ID] {
				t.Fatal("duplicate physical ID")
			}
			ids[c.ID] = true
			faces[c.Short()]++
			if c.Suit == cb.Joker {
				jokers++
				if !IsWild(c, cb.Six) {
					t.Fatal("printed joker isn't wild")
				}
			}
		}
		if jokers != 2*copies {
			t.Fatal("wrong printed jokers")
		}
		for _, count := range faces {
			if count != copies {
				t.Fatal("wrong face multiplicity")
			}
		}
		g := newGame(t, n)
		if g.StockCount() != 54*copies-2*n-1 || g.discard.Len() != 1 || g.active != 0 || g.phase != Handoff {
			t.Fatal("wrong initial deal")
		}
		for _, h := range g.table.Hands() {
			if h.Len() != n || !h.FaceDown() {
				t.Fatal("wrong hand")
			}
		}
		assertConserved(t, g)
	}
	for rank := cb.Ace; rank <= cb.King; rank++ {
		g, err := New(Config{7, rank}, rand.New(rand.NewPCG(1, 2)))
		if err != nil || g.Config().Wild != rank {
			t.Fatal("rank selection failed")
		}
		for suit := cb.Spades; suit <= cb.Clubs; suit++ {
			for r := cb.Ace; r <= cb.King; r++ {
				if IsWild(cb.Card{Rank: r, Suit: suit}, rank) != (r == rank) {
					t.Fatal("wild classification")
				}
			}
		}
	}
	for _, cfg := range []Config{{8, cb.Six}, {7, 0}, {13, 14}} {
		if _, err := New(cfg, nil); err == nil {
			t.Fatal("invalid configuration")
		}
	}
}

func TestTurnPrivacyAndRestrictions(t *testing.T) {
	g := newGame(t, 7)
	if _, err := g.Hand(0); err == nil {
		t.Fatal("handoff leaked hand")
	}
	if err := g.DrawCard(0, false); err == nil {
		t.Fatal("draw during handoff")
	}
	if err := g.ConfirmHandoff(1); err == nil {
		t.Fatal("wrong player confirmed")
	}
	if err := g.ConfirmHandoff(0); err != nil {
		t.Fatal(err)
	}
	h, _ := g.Hand(0)
	c, _ := h.Top()
	if _, err := g.Hand(1); err == nil {
		t.Fatal("opponent hand leaked")
	}
	if err := g.DrawCard(1, false); err == nil {
		t.Fatal("opponent drew")
	}
	if err := g.DiscardCard(0, c.ID); err == nil {
		t.Fatal("discard before draw")
	}
	if err := g.Declare(0, c.ID); err == nil {
		t.Fatal("declare before draw")
	}
	if err := g.DrawCard(0, false); err != nil {
		t.Fatal(err)
	}
	if h.Len() != 8 {
		t.Fatal("draw did not add card")
	}
	if err := g.DrawCard(0, true); err == nil {
		t.Fatal("double draw")
	}
	if err := g.DiscardCard(1, c.ID); err == nil {
		t.Fatal("opponent discarded")
	}
	if err := g.DiscardCard(0, 999); err == nil || h.Len() != 8 {
		t.Fatal("absent discard changed state")
	}
	if err := g.DiscardCard(0, c.ID); err != nil {
		t.Fatal(err)
	}
	if g.phase != Handoff || g.active != 1 || !h.FaceDown() {
		t.Fatal("no privacy gate")
	}
	if _, err := g.Hand(0); err == nil {
		t.Fatal("outgoing hand visible")
	}
	if _, err := g.Hand(1); err == nil {
		t.Fatal("incoming hand visible before confirmation")
	}
	if _, err := g.Groups(0); err == nil {
		t.Fatal("outgoing groups visible")
	}
	if err := g.ConfirmHandoff(1); err != nil {
		t.Fatal(err)
	}
	if err := g.DrawCard(1, true); err != nil {
		t.Fatal(err)
	}
	other, _ := g.Hand(1)
	last, _ := other.At(other.Len() - 1)
	if last.ID != c.ID {
		t.Fatal("wrong open card drawn")
	}
	assertConserved(t, g)
}

func TestRecycleAndLongGameConservation(t *testing.T) {
	for _, n := range []int{7, 13} {
		g := newGame(t, n)
		for turn := 0; turn < 250; turn++ {
			p := g.Active()
			if err := g.ConfirmHandoff(p); err != nil {
				t.Fatal(err)
			}
			oldTop, _ := g.DiscardTop()
			empty := g.StockCount() == 0
			if err := g.DrawCard(p, false); err != nil {
				t.Fatal(err)
			}
			if empty {
				top, _ := g.DiscardTop()
				if top != oldTop || g.DiscardCount() != 1 {
					t.Fatal("recycling changed visible top")
				}
			}
			h, _ := g.Hand(p)
			c, _ := h.At(turn % h.Len())
			if err := g.DiscardCard(p, c.ID); err != nil {
				t.Fatal(err)
			}
			assertConserved(t, g)
		}
	}
}

func TestEmptyDrawIsAtomic(t *testing.T) {
	g := newGame(t, 7)
	_ = g.ConfirmHandoff(0)
	other, _ := g.table.Hand(1)
	for g.StockCount() > 0 {
		if err := cb.Transfer(g.table.Common(), 0, other, other.Len()); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(g)
	if err := g.DrawCard(0, false); err == nil || !reflect.DeepEqual(snapshot(g), before) {
		t.Fatal("unrecyclable stock draw changed state")
	}
	if err := g.DrawCard(0, true); err != nil {
		t.Fatal(err)
	}
	c, _ := g.table.Hand(0)
	top, _ := c.Top()
	if err := g.DiscardCard(0, top.ID); err != nil {
		t.Fatal(err)
	}
	_ = g.ConfirmHandoff(1)
	if err := cb.Transfer(g.discard, 0, g.table.Common(), 0); err != nil {
		t.Fatal(err)
	}
	before = snapshot(g)
	if err := g.DrawCard(1, true); err == nil || !reflect.DeepEqual(snapshot(g), before) {
		t.Fatal("empty open draw changed state")
	}
	assertConserved(t, g)
}

func snapshot(g *Game) any {
	type state struct {
		Piles   [][]cb.Card
		Hidden  []bool
		Groups  [2][][]uint64
		Active  int
		Phase   Phase
		Winner  int
		Winning []Meld
	}
	s := state{Groups: [2][][]uint64{cloneGroups(g.groups[0]), cloneGroups(g.groups[1])}, Active: g.active, Phase: g.phase, Winner: g.winner, Winning: g.Winning()}
	for _, p := range append(g.table.Hands(), g.table.Common(), g.discard) {
		s.Piles = append(s.Piles, p.Cards())
		s.Hidden = append(s.Hidden, p.FaceDown())
	}
	return s
}

// Build a complete real deck with a chosen first player's post-draw hand.
// Cards are still identified and dealt by Cardboard, including duplicates.
func winningFixture(t *testing.T, text string, n int) (*Game, uint64) {
	t.Helper()
	wanted := faces(t, text)
	deck, _ := Deck(Config{n, cb.Six})
	for _, c := range wanted {
		i := slices.IndexFunc(deck, func(other cb.Card) bool { return c.SameFace(other) })
		if i < 0 {
			t.Fatalf("fixture face %s is unavailable", c.Short())
		}
		deck = slices.Delete(deck, i, i+1)
	}
	var ordered []cb.Card
	for i := 0; i < n; i++ {
		ordered = append(ordered, wanted[i], deck[0])
		deck = deck[1:]
	}
	ordered = append(ordered, deck[0], wanted[n])
	ordered = append(ordered, deck[1:]...)
	table, err := cb.NewTableWithDeck(ordered)
	if err != nil {
		t.Fatal(err)
	}
	if err = table.Deal(2, n); err != nil {
		t.Fatal(err)
	}
	discard, _ := cb.NewPile()
	if err = cb.Transfer(table.Common(), 0, discard, 0); err != nil {
		t.Fatal(err)
	}
	g := &Game{config: Config{n, cb.Six}, table: table, discard: discard, phase: Draw, winner: -1}
	if err = g.DrawCard(0, false); err != nil {
		t.Fatal(err)
	}
	h, _ := g.Hand(0)
	final, _ := h.At(n)
	assertConserved(t, g)
	return g, final.ID
}

func TestGroupManagementAndReorderIdentity(t *testing.T) {
	g, _ := winningFixture(t, "4H 5H 6H 9H 9S 9D 9C 2D", 7)
	h, _ := g.Hand(0)
	cards := h.Cards()
	ids := []uint64{cards[0].ID, cards[1].ID, cards[2].ID}
	if err := g.AddGroup(0, ids); err != nil {
		t.Fatal(err)
	}
	before := snapshot(g)
	for _, bad := range [][]uint64{nil, {ids[0], ids[0]}, {ids[0], cards[4].ID}, {9999}} {
		if err := g.AddGroup(0, bad); err == nil || !reflect.DeepEqual(snapshot(g), before) {
			t.Fatal("invalid grouping changed state")
		}
	}
	if err := g.AddGroup(1, []uint64{cards[5].ID}); err == nil {
		t.Fatal("opponent grouping allowed")
	}
	ids[0] = 999
	if err := h.Move(0, 6); err != nil {
		t.Fatal(err)
	}
	groups, _ := g.Groups(0)
	if groups[0][0] != cards[0].ID {
		t.Fatal("group identity followed index")
	}
	groups[0][0] = 999
	again, _ := g.Groups(0)
	if again[0][0] != cards[0].ID {
		t.Fatal("groups leak mutable slices")
	}
	if err := g.Ungroup(0, cards[1].ID); err != nil {
		t.Fatal(err)
	}
	if groups, _ := g.Groups(0); len(groups) != 0 {
		t.Fatal("ungroup left membership")
	}
	// Drafts can be invalid and discarded members are removed automatically.
	if err := g.AddGroup(0, []uint64{cards[7].ID}); err != nil {
		t.Fatal(err)
	}
	if err := g.DiscardCard(0, cards[7].ID); err != nil {
		t.Fatal(err)
	}
	if len(g.groups[0]) != 0 {
		t.Fatal("discard retained empty group")
	}
}

func TestDeclarationTransactionAndWinner(t *testing.T) {
	for _, n := range []int{7, 13} {
		text := "4H 5H 6H 9H 9S 9D 9C 2D"
		if n == 13 {
			text = "4H 5H 6H 9H 9S 9D 9C QH QS QD 2S 2D 2C 8D"
		}
		for _, manual := range []bool{false, true} {
			g, final := winningFixture(t, text, n)
			h, _ := g.Hand(0)
			if manual {
				melds, err := FindPartition(h.Cards()[:n], g.config)
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range melds {
					var ids []uint64
					for _, c := range m.Cards {
						ids = append(ids, c.ID)
					}
					if err := g.AddGroup(0, ids); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := g.Declare(0, final); err != nil {
				t.Fatal(err)
			}
			if g.Winner() != 0 || g.Phase() != Finished || h.Len() != n {
				t.Fatal("winner not committed")
			}
			top, _ := g.DiscardTop()
			if top.ID != final {
				t.Fatal("final discard missing")
			}
			var ids [][]uint64
			for _, m := range g.Winning() {
				var group []uint64
				for _, c := range m.Cards {
					group = append(group, c.ID)
				}
				ids = append(ids, group)
			}
			if _, err := ValidateArrangement(h.Cards(), ids, g.config); err != nil {
				t.Fatal(err)
			}
			if err := g.DrawCard(0, false); err == nil {
				t.Fatal("draw after finish")
			}
			if err := g.DiscardCard(0, final); err == nil {
				t.Fatal("discard after finish")
			}
			if _, err := g.Hand(1); err == nil {
				t.Fatal("winner screen exposes opponent")
			}
			winner := g.Winning()
			winner[0].Cards[0].ID = 999
			if g.Winning()[0].Cards[0].ID == 999 {
				t.Fatal("winning arrangement not copied")
			}
			assertConserved(t, g)
		}
	}
	g, final := winningFixture(t, "4H 5H 6S 9H 9S 9D 9C 2D", 7)
	before := snapshot(g)
	if err := g.Declare(0, final); err == nil || !reflect.DeepEqual(snapshot(g), before) {
		t.Fatal("invalid declaration mutated state")
	}
	g, final = winningFixture(t, "4H 5H 6H 9H 9S 9D 9C 2D", 7)
	h, _ := g.Hand(0)
	cards := h.Cards()
	if err := g.AddGroup(0, []uint64{cards[0].ID, cards[1].ID, cards[2].ID}); err != nil {
		t.Fatal(err)
	}
	before = snapshot(g)
	if err := g.Declare(0, final); err == nil || !reflect.DeepEqual(snapshot(g), before) {
		t.Fatal("partial explicit declaration mutated state")
	}
	if err := g.ClearGroups(0); err != nil {
		t.Fatal(err)
	}
	if err := g.Declare(0, final); err != nil {
		t.Fatal(err)
	}
}
