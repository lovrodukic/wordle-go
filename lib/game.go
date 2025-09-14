// This file implements the Wordle scoring rules (greens/yellows/grays with
// correct duplcate-letter handling).
package lib

import (
	"errors"
	"unicode"
)

type Tile uint8

const (
	Gray   Tile = 0 // Letter not present
	Yellow Tile = 1 // Letter in word but wrong spot
	Green  Tile = 2 // Letter exactly correct
)

type Feedback [5]Tile

func Score(guess, target string) (Feedback, error) {
	var result Feedback

	if err := validateWord(guess); err != nil {
		return result, err
	}
	if err := validateWord(target); err != nil {
		return result, err
	}

	// Remaining copies per letter (a..z)
	available := letterCounts(target)

	// Mark exact matches (greens) first
	for i := range guess {
		if guess[i] == target[i] {
			result[i] = Green
			available[guess[i]-'a']--
		}
	}

	// Mark inexact matches (yellows) if still available
	for i := range guess {
		if result[i] == Green {
			continue
		}
		li := guess[i] - 'a'
		if available[li] > 0 {
			result[i] = Yellow
			available[li]--
		} else {
			result[i] = Gray
		}
	}

	return result, nil
}

func AllGreen(t [5]Tile) bool {
	for i := range t {
		if t[i] != Green {
			return false
		}
	}
	return true
}

func TilesToEmoji(t [5]Tile) string {
	out := ""
	for i := range t {
		switch t[i] {
		case Green:
			out += "🟩"
		case Yellow:
			out += "🟨"
		case Gray:
			out += "🔳"
		}
	}
	return out
}

type Game struct {
	Answer string
	Turns  int
	Won    bool
}

func NewGame(answer string) *Game {
	return &Game{Answer: answer, Turns: 0, Won: false}
}

func (g *Game) Guess(guess string) ([5]Tile, error) {
	g.Turns++
	tiles, err := Score(guess, g.Answer)
	if err != nil {
		return tiles, err
	}

	if AllGreen(tiles) {
		g.Won = true
	}

	return tiles, nil
}

// --- Helpers ---

func validateWord(s string) error {
	if len(s) != 5 {
		return errors.New("word must be exactly 5 letters")
	}

	for _, r := range s {
		if r < 'a' || r > 'z' || !unicode.IsLetter(r) {
			return errors.New("word must be lowercase a..z letters only")
		}
	}

	return nil
}

func letterCounts(s string) [26]int {
	var c [26]int
	for i := range s {
		c[s[i]-'a']++
	}

	return c
}
