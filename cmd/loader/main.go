// The following code uses lib.LoadWords to verify the Wordle word lists can be loaded
// correctly. It runs simple assertions on the data and exits non-zero if anything fails.
package main

import (
	"fmt"
	"os"

	"github.com/lovrodukic/wordle-go/lib"
)

func assert(cond bool, msg string, args ...any) {
	if !cond {
		fmt.Fprintf(os.Stderr, "ASSERTION FAILED: "+msg+"\n", args...)
		os.Exit(1)
	}
}

func assertEqual[T comparable](got, want T, label string) {
	if got != want {
		fmt.Fprintf(os.Stderr, "ASSERTION FAILED: %s: got %v, want %v\n", label, got, want)
		os.Exit(1)
	}
}

func main() {
	answers, err := lib.LoadWords("data/answers.txt")
	assert(err == nil, "loading answers: %v", err)
	allowed, err := lib.LoadWords("data/allowed.txt")
	assert(err == nil, "loading allowed: %v", err)

	assert(len(answers) >= 2000, "answers length too small: %d", len(answers))
	assert(len(allowed) >= 10000, "allowed length too small: %d", len(allowed))

	assertEqual(answers[0], "aback", "answers[0]")

	fmt.Printf("SUCCESS: answers=%d allowed=%d; first answer=%q\n",
		len(answers), len(allowed), answers[0])
}
