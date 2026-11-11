package game

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"strconv"
	"strings"
	"testing"
)

func faces(t *testing.T, text string) []cb.Card {
	t.Helper()
	var out []cb.Card
	for _, s := range strings.Fields(text) {
		c := cb.Card{ID: uint64(len(out) + 1)}
		if s == "JB" || s == "JR" {
			c.Suit = cb.Joker
			c.Rank = cb.Ace
			if s == "JR" {
				c.Rank = cb.Two
			}
		} else {
			suit := strings.IndexByte("SHDC", s[len(s)-1])
			if suit < 0 {
				t.Fatal(s)
			}
			c.Suit = cb.Suit(suit)
			rank := s[:len(s)-1]
			switch rank {
			case "A":
				c.Rank = cb.Ace
			case "J":
				c.Rank = cb.Jack
			case "Q":
				c.Rank = cb.Queen
			case "K":
				c.Rank = cb.King
			default:
				n, err := strconv.Atoi(rank)
				if err != nil {
					t.Fatal(err)
				}
				c.Rank = cb.Rank(n)
			}
		}
		out = append(out, c)
	}
	return out
}

func TestMeldRules(t *testing.T) {
	tests := []struct {
		name, cards string
		wild        cb.Rank
		want        Kind
	}{
		{"pure", "4H 5H 7H 6H", cb.Six, Pure},
		{"natural wild at end", "4H 5H 6H", cb.Six, Pure},
		{"wild substitution", "4H 5H 6S 7H", cb.Six, Sequence},
		{"printed substitution", "4H JB 6H", cb.King, Sequence},
		{"several jokers", "4H JB JR 6S", cb.Six, Sequence},
		{"low ace", "AH 2H 3H", cb.Six, Pure},
		{"high ace", "QH KH AH", cb.Six, Pure},
		{"high ace missing king", "QH JB AH", cb.Six, Sequence},
		{"low ace missing two", "AH JR 3H", cb.Six, Sequence},
		{"wraparound", "KH AH 2H", cb.Six, Invalid},
		{"long wraparound", "QH KH AH 2H", cb.Six, Invalid},
		{"mixed suit", "4H 5S 6H", cb.King, Invalid},
		{"gap", "4H 5H 7H", cb.King, Invalid},
		{"three kind", "9H 9S 9D", cb.Six, Set},
		{"four kind", "9H 9S 9D 9C", cb.Six, Set},
		{"printed set", "9H 9S JB JR", cb.Six, Set},
		{"rank wild set", "9H 9S 6C", cb.Six, Set},
		{"duplicate natural suits", "9H 9H 9S", cb.Six, Invalid},
		{"duplicate plus joker", "9H 9H JB JR", cb.Six, Invalid},
		{"five kind", "9H 9S 9D 9C JB", cb.Six, Invalid},
		{"two cards", "9H JB", cb.Six, Invalid},
		{"duplicate sequence face", "4H 4H 5H", cb.Six, Invalid},
		{"all jokers", "JB JR 6H", cb.Six, Sequence},
		{"wild natural suits can substitute", "6H 6H 6S", cb.Six, Sequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards := faces(t, tt.cards)
			m, err := Classify(cards, tt.wild)
			if tt.want == Invalid {
				if err == nil {
					t.Fatalf("accepted %s as %s", tt.cards, m.Kind)
				}
				return
			}
			if err != nil || m.Kind != tt.want {
				t.Fatalf("got %s / %v, want %s", m.Kind, err, tt.want)
			}
			// Every interpreted face must either be natural or backed by a joker.
			used := map[uint64]bool{}
			for i, c := range m.Cards {
				if used[c.ID] || (!c.SameFace(m.As[i]) && !IsWild(c, tt.wild)) {
					t.Fatal("invalid interpretation")
				}
				if m.Kind == Pure && (c.Suit == cb.Joker || !c.SameFace(m.As[i])) {
					t.Fatal("substitution in pure sequence")
				}
				used[c.ID] = true
			}
		})
	}
	cards := faces(t, "4H 5H 6H")
	cards[1].ID = cards[0].ID
	if _, err := Classify(cards, cb.King); err == nil {
		t.Fatal("reused physical card accepted")
	}
}

func TestDeclarations(t *testing.T) {
	tests := []struct {
		name, cards string
		n           int
		wild        cb.Rank
		valid       bool
	}{
		{"seven pure3 set4", "4H 5H 6H 9H 9S 9D 9C", 7, cb.King, true},
		{"seven pure4 set3", "4H 5H 6H 7H 9S 9D 9C", 7, cb.King, true},
		{"seven pure3 impure4", "4H 5H 6H 9S JS QS JB", 7, cb.King, true},
		{"seven pure4 pure3", "4H 5H 6H 7H 9S 10S JS", 7, cb.King, true},
		{"seven long sequence is not two groups", "AH 2H 3H 4H 5H 6H 7H", 7, cb.King, true},
		{"seven no pure", "4H 5H JB 9H 9S 9D 9C", 7, cb.King, false},
		{"thirteen one sequence three sets", "4H 5H 6H 9H 9S 9D 9C QH QS QD 2S 2D 2C", 13, cb.King, true},
		{"thirteen two sequences", "AH 2H 3H QS KS AS 9H 9S 9D 5H 5S 5D 5C", 13, cb.Six, true},
		{"thirteen no pure", "4H 5H JB 9H 9S 9D 9C QH QS QD 2S 2D 2C", 13, cb.King, false},
		{"thirteen leftover", "4H 5H 6H 9H 9S 9D 9C QH QS QD 2S 2D 8C", 13, cb.King, false},
		{"wild natural compulsory pure", "4H 5H 6H 9H 9S 9D 9C", 7, cb.Six, true},
		{"substituted wild not pure", "4H 5H 6S 9H 9S 9D 9C", 7, cb.Six, false},
		{"ambiguous joker allocation", "3H 4H 5H 6H 5S 5D JB", 7, cb.King, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards := faces(t, tt.cards)
			cfg := Config{tt.n, tt.wild}
			melds, err := FindPartition(cards, cfg)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, got %v (%v)", tt.valid, err, melds)
			}
			if err != nil {
				return
			}
			var ids [][]uint64
			for _, m := range melds {
				var group []uint64
				for _, c := range m.Cards {
					group = append(group, c.ID)
				}
				ids = append(ids, group)
			}
			if _, err := ValidateArrangement(cards, ids, cfg); err != nil {
				t.Fatal(err)
			}
			// Order of the hand must never determine whether a partition exists.
			for i, j := 0, len(cards)-1; i < j; i, j = i+1, j-1 {
				cards[i], cards[j] = cards[j], cards[i]
			}
			if _, err := FindPartition(cards, cfg); err != nil {
				t.Fatal("order-dependent partition", err)
			}
		})
	}
}

func TestExplicitArrangementFailures(t *testing.T) {
	cards := faces(t, "4H 5H 6H 9H 9S 9D 9C")
	cfg := Config{7, cb.King}
	for name, groups := range map[string][][]uint64{
		"overlap":       {{1, 2, 3}, {3, 4, 5, 6}},
		"leftover":      {{1, 2, 3}, {4, 5, 6}},
		"absent":        {{1, 2, 3}, {4, 5, 6, 99}},
		"empty":         {},
		"invalid group": {{1, 2, 4}, {3, 5, 6, 7}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateArrangement(cards, groups, cfg); err == nil {
				t.Fatal("invalid arrangement accepted")
			}
		})
	}
	long := faces(t, "AH 2H 3H 4H 5H 6H 7H")
	if _, err := ValidateArrangement(long, [][]uint64{{1, 2, 3, 4, 5, 6, 7}}, cfg); err == nil {
		t.Fatal("one seven-card group accepted")
	}
	cards[1].ID = cards[0].ID
	if _, err := FindPartition(cards, cfg); err == nil {
		t.Fatal("overlapping identities accepted")
	}
	for _, n := range []int{0, 6, 8, 12, 14} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			if _, err := FindPartition(make([]cb.Card, n), cfg); err == nil {
				t.Fatal("bad card count")
			}
		})
	}
}
