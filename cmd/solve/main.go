// Command solve runs the entropy-based solver against a specific answer.
// Usage:
//
//	go run ./cmd/solve -answer cabin [-start tales] [-answers-file data/answers.txt] [-allowed-file data/allowed.txt]
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/lovrodukic/wordle-go/lib"
)

func main() {
	var (
		answer      string
		answersFile string
		allowedFile string
		startWord   string
		maxTurns    int
	)

	flag.StringVar(&answer, "answer", "", "5-letter lowercase answer to solve (required)")
	flag.StringVar(&answersFile, "answers-file", "data/answers.txt", "path to answers list")
	flag.StringVar(&allowedFile, "allowed-file", "data/allowed.txt", "path to allowed guesses")
	flag.StringVar(&startWord, "start", lib.StartWord, "starting guess word")
	flag.IntVar(&maxTurns, "max-turns", 6, "maximum number of turns")
	flag.Parse()

	// Basic validation
	if len(answer) != 5 || strings.ToLower(answer) != answer {
		log.Fatalf("-answer must be a 5-letter lowercase word (e.g., -answer cabin)")
	}

	// Allow overriding the global start word (to match your bench behavior)
	lib.StartWord = startWord

	answers, err := lib.LoadWords(answersFile)
	if err != nil {
		log.Fatalf("load answers: %v", err)
	}
	allowed, err := lib.LoadWords(allowedFile)
	if err != nil {
		log.Fatalf("load allowed: %v", err)
	}

	// Build score table once for speed/consistency with the bench run
	scoreTable := lib.BuildScoreTable(allowed, answers)

	// Collect pretty board lines while solving
	board := make([]string, 0, maxTurns)
	obs := func(t lib.Trace) {
		board = append(board, fmt.Sprintf("%s  %s", lib.TilesToEmoji(t.Feedback), t.Guess))
	}

	// Run a single game against the provided answer
	won, turns, hist := lib.SolveOneWithObserver(answer, answers, allowed, maxTurns, obs, scoreTable)

	// Output
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

	// Also show the simple board block (same style as bench renderer)
	if len(board) > 0 {
		fmt.Println("\nBoard:")
		for _, ln := range board {
			fmt.Println(ln)
		}
	}
}
