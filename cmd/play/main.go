package main

import (
	"fmt"
	"log"

	"github.com/lovrodukic/wordle-go/lib"
)

type caseScore struct {
	guess, target string
	expect        lib.Feedback
	label         string
}

func FB(a, b, c, d, e lib.Tile) lib.Feedback {
	return lib.Feedback{a, b, c, d, e}
}

func runScoringTests() (fails int) {
	tests := []caseScore{
		{
			guess:  "crane",
			target: "crane",
			expect: FB(lib.Green, lib.Green, lib.Green, lib.Green, lib.Green),
			label:  "all green",
		},
		{
			guess:  "abbey",
			target: "cabin",
			expect: FB(lib.Yellow, lib.Gray, lib.Green, lib.Gray, lib.Gray),
			label:  "duplicate handling",
		},
		{
			guess:  "arise",
			target: "raise",
			expect: FB(lib.Yellow, lib.Yellow, lib.Green, lib.Green, lib.Green),
			label:  "permutation",
		},
		{
			guess:  "stale",
			target: "estal",
			expect: FB(lib.Yellow, lib.Yellow, lib.Yellow, lib.Yellow, lib.Yellow),
			label:  "all yellow",
		},
	}

	for _, tc := range tests {
		got, err := lib.Score(tc.guess, tc.target)
		if err != nil {
			fmt.Printf("FAIL %-16s  %s vs %s  err=%v\n", tc.label, tc.guess, tc.target, err)
			fails++
			continue
		}
		if got != tc.expect {
			fmt.Printf("FAIL %-16s  %s vs %s  got %v %s  want %v %s\n",
				tc.label, tc.guess, tc.target, got, lib.TilesToEmoji(got), tc.expect, lib.TilesToEmoji(tc.expect))
			fails++
		} else {
			fmt.Printf("PASS %-16s  %s vs %s  %v %s\n",
				tc.label, tc.guess, tc.target, got, lib.TilesToEmoji(got))
		}
	}
	return
}

func runFullGame(answer string, guesses []string) (won bool, turns int, fail error) {
	g := lib.NewGame(answer)
	for _, guess := range guesses {
		fb, err := g.Guess(guess)
		if err != nil {
			return false, g.Turns, err
		}
		fmt.Printf("turn %d  guess=%s  %v %s\n", g.Turns, guess, fb, lib.TilesToEmoji(fb))
		if g.Won {
			return true, g.Turns, nil
		}
	}
	return false, g.Turns, nil
}

func main() {
	fmt.Println("=== scoring tests ===")
	if fails := runScoringTests(); fails > 0 {
		log.Fatalf("scoring tests failed: %d", fails)
	}
	fmt.Println("all scoring tests passed")

	fmt.Println("=== full game: answer=cabin ===")
	won, turns, err := runFullGame("cabin", []string{
		"abbey", "canal", "cabin",
	})
	if err != nil {
		log.Fatal(err)
	}
	status := "lost"
	if won {
		status = "won"
	}
	fmt.Printf("result: %s in %d turns\n", status, turns)
}
