package lib

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func LoadWords(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []string
	seen := make(map[string]bool)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.ToLower(strings.TrimSpace(sc.Text()))
		if len(w) != 5 {
			continue
		}
		valid := true
		for _, r := range w {
			if !unicode.IsLetter(r) || r < 'a' || r > 'z' {
				valid = false
				break
			}
		}
		if !valid || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return out, nil
}
