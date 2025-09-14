package lib

import (
	"math"
)

type Step struct {
	Guess    string
	Feedback Feedback
}

// Returns only those targets that would produce the same feedback when scored
// against the provided guess.
func FilterCandidates(
	candidates []string,
	guess string,
	fb Feedback,
) []string {
	out := candidates[:0]
	for _, t := range candidates {
		got, err := Score(guess, t)
		if err != nil {
			continue
		}

		if got == fb {
			out = append(out, t)
		}
	}

	return append([]string(nil), out...)
}

// Computes the expected information (bits) if we were to play 'guess' against
// the current candidate set. It builds a distribution over all possible
// feedback patterns, then sums -p*log2(p).
func entropyForGuess(guess string, candidates []string) float64 {
	if len(candidates) == 0 {
		return 0
	}

	counts := make(map[Feedback]int, 64)
	for _, t := range candidates {
		fb, err := Score(guess, t)
		if err != nil {
			continue
		}
		counts[fb]++
	}

	n := float64(len(candidates))
	var h float64
	for _, c := range counts {
		p := float64(c) / n
		// log2(p) - ln(p)/ln(2)
		h -= p * (math.Log(p) / math.Log(2))
	}

	return h
}

// Picks the guess (from guessSet) with th emaximum entropy relative to the
// current candidates. Tie-break by preferring a guess that is itself still
// a possible target.
func ChooseNextGuess(
	guessSet,
	candidates []string,
) (
	best string,
	bestEntropy float64,
) {
	inC := make(map[string]bool, len(candidates))
	for _, w := range candidates {
		inC[w] = true
	}

	best = ""
	bestEntropy = -1.0
	for _, g := range guessSet {
		h := entropyForGuess(g, candidates)
		if h > bestEntropy || (almostEqual(h, bestEntropy) && inC[g]) {
			best, bestEntropy = g, h
		}
	}
	return
}

func almostEqual(a, b float64) bool {
	if a > b {
		return a-b < 1e-9
	}
	return b-a < 1e-9
}

// Plays a full game against 'answer' for up to maxTurns guesses, using the
// entropy to choose each guess. It returns whether we won, how many turns
// it took, and the move history.
func SolveOne(
	answer string, answers,
	guessSet []string,
	maxTurns int,
) (
	won bool, turns int,
	hist []Step,
) {
	candidates := append([]string(nil), answers...)
	guessed := make(map[string]bool, 16)

	for turn := 1; turn <= maxTurns; turn++ {
		pool := guessSet
		if turn >= 2 {
			pool = candidates
		}
		pool = excludeGuessed(pool, guessed)
		if len(pool) == 0 {
			// Fallback: if the pool is empty (shouldn’t happen),
			// try any candidate not guessed
			pool = excludeGuessed(candidates, guessed)
			if len(pool) == 0 {
				return false, turn, hist
			}
		}

		guess, _ := ChooseNextGuess(pool, candidates)
		if guess == "" {
			// No valid choice; bail safely.
			return false, turn, hist
		}
		guessed[guess] = true

		fb, err := Score(guess, answer)
		if err != nil {
			// With clean lists this shouldn't happen.
			return false, turn, hist
		}
		hist = append(hist, Step{Guess: guess, Feedback: fb})

		if AllGreen(fb) {
			return true, turn, hist
		}

		candidates = FilterCandidates(candidates, guess, fb)
	}
	return false, maxTurns, hist
}

func excludeGuessed(words []string, used map[string]bool) []string {
	out := make([]string, 0, len(words))
	seen := make(map[string]bool, len(words)) // avoid duplicates if backing arrays overlap
	for _, w := range words {
		if used[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}
