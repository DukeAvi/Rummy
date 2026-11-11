package game

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
	"math/bits"
)

type Kind string

const (
	Invalid  Kind = "invalid"
	Pure     Kind = "pure sequence"
	Sequence Kind = "impure sequence"
	Set      Kind = "set"
)

// Meld records physical cards, their assigned faces, and any substitutions.
// Cards and As have matching positions, sorted by sequence rank or set suit.
type Meld struct {
	Kind  Kind
	Cards []cb.Card
	As    []cb.Card
}

// Classify prefers a natural pure sequence, even when it includes rank-wild cards.
func Classify(cards []cb.Card, wild cb.Rank) (Meld, error) {
	if len(cards) < 3 {
		return Meld{}, fmt.Errorf("a group needs at least three cards")
	}
	seen := map[uint64]bool{}
	for _, c := range cards {
		if !c.Valid() || c.ID == 0 || seen[c.ID] {
			return Meld{}, fmt.Errorf("invalid or repeated physical card")
		}
		seen[c.ID] = true
	}
	if m, ok := sequence(cards, wild, true); ok {
		return m, nil
	}
	if m, ok := sequence(cards, wild, false); ok {
		return m, nil
	}
	if m, ok := set(cards, wild); ok {
		return m, nil
	}
	return Meld{}, fmt.Errorf("not consecutive in one suit, nor a 3/4-card set with distinct natural suits")
}

// A sequence is a matching of physical cards to a contiguous rank window.
// Non-wild cards must match exactly. Wild cards can fill any remaining slot.
func sequence(cards []cb.Card, wild cb.Rank, pure bool) (Meld, bool) {
	n := len(cards)
	if n > 13 {
		return Meld{}, false
	}
	for suit := cb.Spades; suit <= cb.Clubs; suit++ {
		for start := 1; start+n-1 <= 14; start++ {
			// 2..A is the longest high-Ace window; A may occur only at an endpoint.
			if start == 1 && n > 13 {
				continue
			}
			slots := make([]cb.Card, n)
			for i := range slots {
				rank := start + i
				if rank == 14 {
					rank = 1
				}
				slots[i] = cb.Card{Rank: cb.Rank(rank), Suit: suit}
			}
			placed := make([]cb.Card, n)
			used := make([]bool, n)
			ok := true
			// For impure sequences place the fixed cards first, reserving natural
			// positions for rank-wilds whenever available in the second pass.
			for _, c := range cards {
				if !pure && IsWild(c, wild) {
					continue
				}
				found := false
				for i, s := range slots {
					if !used[i] && c.SameFace(s) {
						placed[i] = c
						used[i] = true
						found = true
						break
					}
				}
				if !found {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			if !pure {
				var pending []cb.Card
				for _, c := range cards {
					if !IsWild(c, wild) {
						continue
					}
					found := false
					for i, s := range slots {
						if !used[i] && c.Suit != cb.Joker && c.SameFace(s) {
							placed[i] = c
							used[i] = true
							found = true
							break
						}
					}
					if !found {
						pending = append(pending, c)
					}
				}
				for _, c := range pending {
					for i := range slots {
						if !used[i] {
							placed[i] = c
							used[i] = true
							break
						}
					}
				}
			}
			kind := Sequence
			if pure {
				kind = Pure
			}
			return Meld{Kind: kind, Cards: placed, As: slots}, true
		}
	}
	return Meld{}, false
}

func set(cards []cb.Card, wild cb.Rank) (Meld, bool) {
	if len(cards) != 3 && len(cards) != 4 {
		return Meld{}, false
	}
	for rank := cb.Ace; rank <= cb.King; rank++ {
		var suits [4]bool
		var naturals, jokers []cb.Card
		ok := true
		for _, c := range cards {
			if IsWild(c, wild) {
				jokers = append(jokers, c)
				continue
			}
			if c.Rank != rank || suits[c.Suit] {
				ok = false
				break
			}
			suits[c.Suit] = true
			naturals = append(naturals, c)
		}
		if !ok {
			continue
		}
		m := Meld{Kind: Set, Cards: append([]cb.Card(nil), naturals...)}
		for _, c := range naturals {
			m.As = append(m.As, cb.Card{Rank: rank, Suit: c.Suit})
		}
		for _, c := range jokers {
			for suit := cb.Spades; suit <= cb.Clubs; suit++ {
				if !suits[suit] {
					suits[suit] = true
					m.Cards = append(m.Cards, c)
					m.As = append(m.As, cb.Card{Rank: rank, Suit: suit})
					break
				}
			}
		}
		return m, true
	}
	return Meld{}, false
}

func validateCards(cards []cb.Card, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if len(cards) != cfg.Cards {
		return fmt.Errorf("declaration needs exactly %d cards after discarding", cfg.Cards)
	}
	seen := map[uint64]bool{}
	for _, c := range cards {
		if !c.Valid() || c.ID == 0 || seen[c.ID] {
			return fmt.Errorf("every physical card must have a distinct nonzero ID")
		}
		seen[c.ID] = true
	}
	return nil
}

// ValidateArrangement requires explicit groups to cover every card exactly once.
func ValidateArrangement(cards []cb.Card, groups [][]uint64, cfg Config) ([]Meld, error) {
	if err := validateCards(cards, cfg); err != nil {
		return nil, err
	}
	byID := map[uint64]cb.Card{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	seen := map[uint64]bool{}
	var melds []Meld
	pure := false
	for i, ids := range groups {
		var group []cb.Card
		for _, id := range ids {
			c, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("group %d contains a card outside the remaining hand", i+1)
			}
			if seen[id] {
				return nil, fmt.Errorf("groups overlap: one physical card is used twice")
			}
			seen[id] = true
			group = append(group, c)
		}
		m, err := Classify(group, cfg.Wild)
		if err != nil {
			return nil, fmt.Errorf("group %d: %w", i+1, err)
		}
		pure = pure || m.Kind == Pure
		melds = append(melds, m)
	}
	if len(seen) != len(cards) {
		return nil, fmt.Errorf("%d cards are ungrouped; group every card or clear groups for automatic search", len(cards)-len(seen))
	}
	if !pure {
		return nil, fmt.Errorf("at least one pure sequence is required")
	}
	if cfg.Cards == 7 && len(melds) != 2 {
		return nil, fmt.Errorf("7-card Rummy needs exactly two groups of sizes 3 and 4")
	}
	return melds, nil
}

// FindPartition exhaustively searches disjoint valid subsets, anchoring each
// step on the first remaining card. Memoization bounds search to 2^13 states
// per pure-sequence flag and group count; no hand ordering heuristic is used.
func FindPartition(cards []cb.Card, cfg Config) ([]Meld, error) {
	if err := validateCards(cards, cfg); err != nil {
		return nil, err
	}
	type candidate struct {
		mask int
		meld Meld
	}
	candidates := make([][]candidate, len(cards))
	full := (1 << len(cards)) - 1
	for mask := 1; mask <= full; mask++ {
		n := bits.OnesCount(uint(mask))
		if n < 3 || (cfg.Cards == 7 && n > 4) {
			continue
		}
		var subset []cb.Card
		for i, c := range cards {
			if mask&(1<<i) != 0 {
				subset = append(subset, c)
			}
		}
		m, err := Classify(subset, cfg.Wild)
		if err == nil {
			first := bits.TrailingZeros(uint(mask))
			candidates[first] = append(candidates[first], candidate{mask, m})
		}
	}
	type state struct {
		remaining, count int
		pure             bool
	}
	failed := map[state]bool{}
	var search func(int, int, bool) ([]Meld, bool)
	search = func(left, count int, pure bool) ([]Meld, bool) {
		if left == 0 {
			return nil, pure && (cfg.Cards != 7 || count == 2)
		}
		s := state{left, count, pure}
		if failed[s] || (cfg.Cards == 7 && count >= 2) {
			return nil, false
		}
		first := bits.TrailingZeros(uint(left))
		for _, c := range candidates[first] {
			if left&c.mask != c.mask {
				continue
			}
			if tail, ok := search(left^c.mask, count+1, pure || c.meld.Kind == Pure); ok {
				return append([]Meld{c.meld}, tail...), true
			}
		}
		failed[s] = true
		return nil, false
	}
	if melds, ok := search(full, 0, false); ok {
		return melds, nil
	}
	return nil, fmt.Errorf("no complete arrangement with a pure sequence%s", map[bool]string{true: " and exactly two groups of 3 + 4", false: ""}[cfg.Cards == 7])
}
