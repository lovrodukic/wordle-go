// Live bench with CPU-chunked workers and a single top-anchored renderer.
package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lovrodukic/wordle-go/lib"
)

type result struct {
	Answer string
	Won    bool
	Turns  int
}

type gameReport struct {
	Idx        int
	Answer     string
	Won        bool
	Turns      int
	BoardLines []string
}

func main() {
	// --- Hardcoded settings ---
	answersFile := "data/answers.txt"
	allowedFile := "data/allowed.txt"
	maxTurns := 6

	workers := runtime.NumCPU()
	runtime.GOMAXPROCS(workers)

	answers, err := lib.LoadWords(answersFile)
	if err != nil {
		log.Fatalf("load answers: %v", err)
	}
	allowed, err := lib.LoadWords(allowedFile)
	if err != nil {
		log.Fatalf("load allowed: %v", err)
	}
	if len(answers) == 0 {
		log.Fatal("no answers")
	}
	if workers > len(answers) {
		workers = len(answers)
	}

	// Build the precomputed score table (one time)
	scoreTable := lib.BuildScoreTable(allowed, answers)

	start := time.Now()
	results := runBench(answers, allowed, maxTurns, workers, scoreTable)
	elapsed := time.Since(start)

	// --- Summary (print below the display block) ---
	const displayHeight = 1 /*progress*/ + 6 /*board*/ + 1 /*result*/ + 3 /*padding*/
	fmt.Printf("\x1b[%dB", displayHeight)

	total := len(results)
	solved := 0
	sumTurns := 0
	for _, r := range results {
		if r.Won {
			solved++
			sumTurns += r.Turns
		}
	}
	avg := 0.0
	if solved > 0 {
		avg = float64(sumTurns) / float64(solved)
	}
	fmt.Printf("\n=== summary ===\n")
	fmt.Printf("answers=%d  workers=%d  maxTurns=%d\n", total, workers, maxTurns)
	fmt.Printf("elapsed=%s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("solved=%d  failed=%d\n", solved, total-solved)
	if solved > 0 {
		fmt.Printf("average turns (solved) = %.3f\n", avg)
	}
}

// Engine
func runBench(answers, allowed []string, maxTurns, workers int, st *lib.ScoreTable) []result {
	total := len(answers)
	out := make([]result, total)

	// Split into contiguous chunks
	type jobRange struct{ start, end int }
	ranges := make([]jobRange, 0, workers)
	chunk := (total + workers - 1) / workers
	for w := 0; w < workers; w++ {
		s, e := w*chunk, (w+1)*chunk
		if s >= total {
			break
		}
		if e > total {
			e = total
		}
		ranges = append(ranges, jobRange{start: s, end: e})
	}

	// Shared progress
	var completed int64
	var solvedSoFar int64
	var sumTurnsSoFar int64

	// UI comms (coalesced)
	events := make(chan *gameReport, 4096) // large buffer to avoid backpressure
	// Also keep the latest report in an atomic for tick-driven refreshes
	var latest atomic.Value

	// Renderer lifecycle
	stopUI := make(chan struct{})
	var uiWG sync.WaitGroup
	uiWG.Add(1)
	go func() {
		defer uiWG.Done()
		renderLoop(total, maxTurns, &completed, &solvedSoFar, &sumTurnsSoFar, events, &latest, stopUI)
	}()

	// Workers (one per chunk)
	var wg sync.WaitGroup
	wg.Add(len(ranges))
	for _, rg := range ranges {
		startIdx, endIdx := rg.start, rg.end
		go func() {
			defer wg.Done()
			for i := startIdx; i < endIdx; i++ {
				ans := answers[i]

				// Build compact board lines via observer
				board := make([]string, 0, maxTurns)
				obs := func(t lib.Trace) {
					board = append(board, fmt.Sprintf("%s  %s", lib.TilesToEmoji(t.Feedback), t.Guess))
				}

				won, turns, _ := lib.SolveOneWithObserver(ans, answers, allowed, maxTurns, obs, st)
				out[i] = result{Answer: ans, Won: won, Turns: turns}

				// Update counters first so renderer sees consistent totals
				atomic.AddInt64(&completed, 1)
				if won {
					atomic.AddInt64(&solvedSoFar, 1)
					atomic.AddInt64(&sumTurnsSoFar, int64(turns))
				}

				// Publish event (non-blocking)
				ev := &gameReport{
					Idx:        i,
					Answer:     ans,
					Won:        won,
					Turns:      turns,
					BoardLines: board,
				}
				latest.Store(ev)
				select {
				case events <- ev:
				default:
					// queue full -> drop; renderer will repaint on next tick
				}
			}
		}()
	}

	// Wait for workers, then stop UI
	wg.Wait()
	close(stopUI)
	uiWG.Wait()
	return out
}

// Renderer
func renderLoop(
	total, maxTurns int,
	completed, solvedSoFar, sumTurnsSoFar *int64,
	events <-chan *gameReport,
	latest *atomic.Value,
	stop <-chan struct{},
) {
	// Terminal setup: clear once, hide cursor, pre-fill lines
	const displayHeight = 1 /*progress*/ + 6 /*board*/ + 1 /*result*/ + 3 /*padding*/
	fmt.Print("\x1b[?25l")                                                // hide cursor
	defer fmt.Print("\x1b[?25h")                                          // show cursor on exit
	fmt.Print("\x1b[2J\x1b[H")                                            // clear + home
	for i := 0; i < displayHeight; i++ {
		fmt.Println()
	}

	startTime := time.Now()
	tick := time.NewTicker(time.Second / 10)
	defer tick.Stop()

	// When events flood in, coalesce them and render once
	var needRender bool
	for {
		select {
		case <-stop:
			// final paint with latest
			v := latest.Load()
			var gr *gameReport
			if v != nil {
				gr = v.(*gameReport)
			}
			doRender(total, maxTurns, startTime, completed, solvedSoFar, sumTurnsSoFar, gr)
			return

		case ev := <-events:
			if ev != nil {
				needRender = true
			}

		case <-tick.C:
			var gr *gameReport
			if v := latest.Load(); v != nil {
				gr = v.(*gameReport)
			}
			if needRender {
				doRender(total, maxTurns, startTime, completed, solvedSoFar, sumTurnsSoFar, gr)
				needRender = false
			} else {
				// periodic heartbeat so progress/rate stay fresh
				doRender(total, maxTurns, startTime, completed, solvedSoFar, sumTurnsSoFar, gr)
			}
		}
	}
}

func doRender(
	total, maxTurns int,
	startTime time.Time,
	completed, solvedSoFar, sumTurnsSoFar *int64,
	gr *gameReport,
) {
	// Home to top-left
	fmt.Print("\x1b[H")

	c := int(atomic.LoadInt64(completed))
	s := int(atomic.LoadInt64(solvedSoFar))
	sum := int(atomic.LoadInt64(sumTurnsSoFar))

	avg := 0.0
	if s > 0 {
		avg = float64(sum) / float64(s)
	}

	elapsed := time.Since(startTime)
	rate := float64(c) / (elapsed.Seconds() + 1e-9)

	frac := float64(c) / float64(total)
	bar := progressBar(frac, 40)

	lines := make([]string, 0, 16)
	lines = append(lines, fmt.Sprintf(
		"%s %d/%d (%.2f%%)  rate:%.1f/s  elapsed:%s  avg(solved):%.3f",
		bar, c, total, 100*frac, rate, elapsed.Round(100*time.Millisecond), avg,
	))

	// Game board (up to maxTurns lines)
	if gr != nil {
		for i := 0; i < maxTurns; i++ {
			if i < len(gr.BoardLines) {
				lines = append(lines, gr.BoardLines[i])
			} else {
				lines = append(lines, "")
			}
		}
		if gr.Won {
			lines = append(lines, fmt.Sprintf("Result %d/%d: %s — ✔ solved in %d", c, total, gr.Answer, gr.Turns))
		} else {
			lines = append(lines, fmt.Sprintf("Result %d/%d: %s — ✘ failed in %d", c, total, gr.Answer, gr.Turns))
		}
	} else {
		for i := 0; i < maxTurns+1; i++ {
			lines = append(lines, "")
		}
	}

	// extra padding for stability
	lines = append(lines, "", "")

	// Paint exactly displayHeight lines
	for _, ln := range lines {
		fmt.Print("\x1b[2K") // clear line
		fmt.Println(ln)
	}
}

// Simple ASCII progress bar.
func progressBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + repeat("#", filled) + repeat("-", width-filled) + "]"
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
