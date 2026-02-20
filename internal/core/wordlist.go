package core

import (
	"embed"
	"strings"
	"sync"
)

//go:embed wordlist/eff_large.txt
var embeddedFS embed.FS

var (
	wordlistOnce sync.Once
	wordlist     []string
)

// LoadWordlist returns the EFF large wordlist. It is loaded once and cached.
func LoadWordlist() []string {
	wordlistOnce.Do(func() {
		data, err := embeddedFS.ReadFile("wordlist/eff_large.txt")
		if err != nil {
			// Should never happen since the file is embedded at compile time.
			panic("passforge: embedded wordlist missing: " + err.Error())
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		wordlist = make([]string, 0, len(lines))
		for _, line := range lines {
			// EFF format: "11111\tword"
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				wordlist = append(wordlist, strings.TrimSpace(parts[1]))
			}
		}
	})
	return wordlist
}
