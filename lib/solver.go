package lib

import (
	"math"
)

var StartWord = "tales"

type Step struct {
	Guess    string
	Feedback Feedback
}

// Trace holds information about one turn, used for real-time observers.
type Trace struct {
	Turn              int
	Guess             string
	Feedback          Feedback
	CandidatesBefore  int
	CandidatesAfter   int
	PoolSize          int
	ChosenEntropyBits float64
}

type ScoreTable struct {
	// Table holds Feedback for (guessIdx * len(answers) + answerIdx)
	Table       []Feedback
	GuessIndex  map[string]int // guess -> row index (built from union(allowed, answers))
	AnswerIndex map[string]int // answer -> column index (built from answers list)
	AnswersLen  int
	Guesses     []string
	Answers     []string
}

// Precomputes Feedback for every (guess, answer), where This guarantees we have a row even
// if you use an answer as a guess.
func BuildScoreTable(allowed, answers []string) *ScoreTable {
	seen := make(map[string]bool, len(allowed)+len(answers))
	guesses := make([]string, 0, len(allowed)+len(answers))

	for _, g := range allowed {
		if !seen[g] {
			seen[g] = true
			guesses = append(guesses, g)
		}
	}

	for _, a := range answers {
		if !seen[a] {
			seen[a] = true
			guesses = append(guesses, a)
		}
	}

	gi := make(map[string]int, len(guesses))
	ai := make(map[string]int, len(answers))
	for i, g := range guesses {
		gi[g] = i
	}
	for j, a := range answers {
		ai[a] = j
	}

	tbl := make([]Feedback, len(guesses)*len(answers))
	for i, g := range guesses {
		row := i * len(answers)
		for j, a := range answers {
			fb, _ := Score(g, a)
			tbl[row+j] = fb
		}
	}

	return &ScoreTable{
		Table:       tbl,
		GuessIndex:  gi,
		AnswerIndex: ai,
		AnswersLen:  len(answers),
		Guesses:     guesses,
		Answers:     answers,
	}
}

// Returns Feedback for (guess, answer) via the table.
// If either word is missing (shouldn’t happen with union), it falls back to Score().
func fbFromTable(st *ScoreTable, guess, answer string) Feedback {
	gidx, gok := st.GuessIndex[guess]
	aidx, aok := st.AnswerIndex[answer]
	if gok && aok {
		return st.Table[gidx*st.AnswersLen+aidx]
	}
	// Slow-path fallback: still correct.
	fb, _ := Score(guess, answer)
	return fb
}

// Returns only those targets that would produce the same feedback when scored
// against the provided guess. (Uses precomputed table when possible.)
func FilterCandidates(
	st *ScoreTable,
	candidates []string,
	guess string,
	want Feedback,
) []string {
	out := candidates[:0]
	gidx, gok := st.GuessIndex[guess]
	base := gidx * st.AnswersLen

	for _, t := range candidates {
		if aidx, aok := st.AnswerIndex[t]; gok && aok {
			if st.Table[base+aidx] == want {
				out = append(out, t)
			}
		} else {
			// fallback if somehow not in the table
			if got, _ := Score(guess, t); got == want {
				out = append(out, t)
			}
		}
	}
	return append([]string(nil), out...)
}

// Computes the expected information (bits) for 'guess' against current candidates.
// Uses table lookups; falls back to Score if a word isn’t indexed (should be rare).
func entropyForGuessFast(
	guess string,
	candidates []string,
	st *ScoreTable,
) float64 {
	if len(candidates) == 0 {
		return 0
	}
	counts := make(map[Feedback]int, 64)
	gidx, gok := st.GuessIndex[guess]
	base := gidx * st.AnswersLen

	for _, t := range candidates {
		if aidx, aok := st.AnswerIndex[t]; gok && aok {
			counts[st.Table[base+aidx]]++
		} else {
			fb, _ := Score(guess, t)
			counts[fb]++
		}
	}
	n := float64(len(candidates))
	var h float64
	for _, c := range counts {
		p := float64(c) / n
		h -= p * (math.Log(p) / math.Log(2)) // log2
	}
	return h
}

// Picks the guess (from guessSet) with the maximum entropy relative to the
// current candidates. Tie-break by preferring a guess that is itself still
// a possible target.
func ChooseNextGuess(
	guessSet,
	candidates []string,
	st *ScoreTable,
) (best string, bestEntropy float64) {
	inC := make(map[string]bool, len(candidates))
	for _, w := range candidates {
		inC[w] = true
	}

	best, bestEntropy = "", -1.0
	for _, g := range guessSet {
		h := entropyForGuessFast(g, candidates, st)
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

// Solves one game, using the provided observer to report each turn.
func SolveOneWithObserver(
	answer string,
	answers []string,
	guessSet []string,
	maxTurns int,
	obs func(Trace),
	st *ScoreTable,
) (won bool, turns int, hist []Step) {
	assertValidStart(st)
	assertValidAnswer(st, answer)

	candidates := append([]string(nil), answers...)
	guessed := make(map[string]bool, 16)

	for turn := 1; turn <= maxTurns; turn++ {
		pool := guessSet
		if turn >= 2 {
			pool = candidates
		}
		pool = excludeGuessed(pool, guessed)
		if len(pool) == 0 {
			pool = excludeGuessed(candidates, guessed)
			if len(pool) == 0 {
				return false, turn, hist
			}
		}

		var guess string
		var ent float64
		if turn == 1 {
			guess = StartWord
			// optional: keep telemetry comparable
			ent = entropyForGuessFast(guess, candidates, st)
		} else {
			guess, ent = ChooseNextGuess(pool, candidates, st)
		}
		if guess == "" {
			return false, turn, hist
		}
		guessed[guess] = true

		cBefore := len(candidates)
		fb := fbFromTable(st, guess, answer)
		hist = append(hist, Step{Guess: guess, Feedback: fb})

		// shrink candidate set
		candidates = FilterCandidates(st, candidates, guess, fb)
		cAfter := len(candidates)

		if obs != nil {
			obs(Trace{
				Turn:              turn,
				Guess:             guess,
				Feedback:          fb,
				CandidatesBefore:  cBefore,
				CandidatesAfter:   cAfter,
				PoolSize:          len(pool),
				ChosenEntropyBits: ent,
			})
		}

		if AllGreen(fb) {
			return true, turn, hist
		}
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

func assertValidStart(st *ScoreTable) {
	if _, ok := st.GuessIndex[StartWord]; !ok {
		panic("lib.StartWord " + StartWord + " is not in allowed/answers")
	}
}

func assertValidAnswer(st *ScoreTable, answer string) {
	if _, ok := st.AnswerIndex[answer]; !ok {
		panic("answer " + answer + " is not in answers list")
	}
}
