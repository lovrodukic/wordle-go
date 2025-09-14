// Command solve runs the entropy-based solver against a specific answer.
// Usage:
//
//	go run ./cmd/solve -answer cabin
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/lovrodukic/wordle-go/lib"
)

func main() {
	var (
		answer      string
		answersFile string
		allowedFile string
		maxTurns    int
	)

	flag.StringVar(&answer, "answer", "", "5-letter lowercase answer to solve (required)")
	flag.StringVar(&answersFile, "answers-file", "data/answers.txt", "path to answers list")
	flag.StringVar(&allowedFile, "allowed-file", "data/allowed.txt", "path to allowed guesses")
	flag.IntVar(&maxTurns, "max-turns", 6, "maximum number of turns")
	flag.Parse()

	if answer == "" {
		log.Fatalf("-answer is required (e.g., -answer cabin)")
	}

	answers, err := lib.LoadWords(answersFile)
	if err != nil {
		log.Fatalf("load answers: %v", err)
	}
	allowed, err := lib.LoadWords(allowedFile)
	if err != nil {
		log.Fatalf("load allowed: %v", err)
	}

	won, turns, hist := lib.SolveOne(answer, answers, allowed, maxTurns)

	fmt.Printf("Target: %s\n", answer)
	for i, step := range hist {
		fmt.Printf("Turn %d: %-5s  %v  %s\n",
			i+1, step.Guess, step.Feedback, lib.TilesToEmoji(step.Feedback))
	}
	if won {
		fmt.Printf("Solved in %d turns ✅\n", turns)
	} else {
		fmt.Printf("Failed in %d turns ❌\n", turns)
	}
}
