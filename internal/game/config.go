// Package game implements two-player Rummy independently of terminal input.
package game

import (
	"fmt"
	cb "github.com/DukeAvi/cardboard"
)

type Config struct {
	Cards int
	Wild  cb.Rank
}

func (c Config) Validate() error {
	if c.Cards != 7 && c.Cards != 13 {
		return fmt.Errorf("choose 7 or 13 cards")
	}
	if c.Wild < cb.Ace || c.Wild > cb.King {
		return fmt.Errorf("choose a wild rank from A through K")
	}
	return nil
}

// Deck returns faces; NewTableWithDeck assigns their physical identities.
func Deck(c Config) ([]cb.Card, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	decks := 1
	if c.Cards == 13 {
		decks = 2
	}
	var cards []cb.Card
	for range decks {
		cards = append(cards, cb.StandardDeck()...)
		cards = append(cards, cb.Card{Rank: cb.Ace, Suit: cb.Joker}, cb.Card{Rank: cb.Two, Suit: cb.Joker})
	}
	return cards, nil
}

func IsWild(c cb.Card, wild cb.Rank) bool { return c.Suit == cb.Joker || c.Rank == wild }
