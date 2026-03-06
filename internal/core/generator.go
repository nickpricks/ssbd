package core

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const (
	uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
	digitChars     = "0123456789"
	symbolChars    = "!@#$%^&*()-_=+[]{}|;:',.<>?/`~"
)

// Generate produces a cryptographically random password based on the given config.
func Generate(cfg GeneratorConfig) (string, error) {
	if cfg.Length < 1 {
		return "", fmt.Errorf(MsgErrPasswordLength1)
	}

	enabledClasses := countEnabledClasses(cfg)
	if cfg.Length < enabledClasses {
		return "", fmt.Errorf(MsgErrLengthTooShort, cfg.Length, enabledClasses)
	}

	charset := buildCharset(cfg)
	if len(charset) == 0 {
		return "", fmt.Errorf(MsgErrNoCharset)
	}

	password := make([]byte, cfg.Length)

	// First, place one random character from each enabled class at distinct
	// random positions. This guarantees every required class is represented
	// without the risk of later placements overwriting earlier ones.
	classes := enabledClassChars(cfg)
	positions, err := randomPermutation(cfg.Length, len(classes))
	if err != nil {
		return "", fmt.Errorf(MsgErrCryptoRand, err)
	}
	reserved := make(map[int]bool, len(positions))
	for i, cls := range classes {
		charIdx, err := cryptoRandInt(len(cls))
		if err != nil {
			return "", fmt.Errorf(MsgErrCryptoRand, err)
		}
		password[positions[i]] = cls[charIdx]
		reserved[positions[i]] = true
	}

	// Fill the remaining positions from the full charset.
	for i := range password {
		if reserved[i] {
			continue
		}
		idx, err := cryptoRandInt(len(charset))
		if err != nil {
			return "", fmt.Errorf(MsgErrCryptoRand, err)
		}
		password[i] = charset[idx]
	}

	return string(password), nil
}

// GeneratePassphrase produces a passphrase from the EFF wordlist.
func GeneratePassphrase(cfg PassphraseConfig) (string, error) {
	if cfg.Words < 1 {
		return "", fmt.Errorf(MsgErrPassphraseWords1)
	}

	words := LoadWordlist()
	if len(words) == 0 {
		return "", fmt.Errorf(MsgErrEmptyWordlist)
	}

	selected := make([]string, cfg.Words)
	for i := range selected {
		idx, err := cryptoRandInt(len(words))
		if err != nil {
			return "", fmt.Errorf(MsgErrCryptoRand, err)
		}
		word := words[idx]
		if cfg.Capitalize {
			word = strings.ToUpper(word[:1]) + word[1:]
		}
		selected[i] = word
	}

	if cfg.AddNumber {
		// Append a random digit to a random word.
		wordIdx, err := cryptoRandInt(len(selected))
		if err != nil {
			return "", fmt.Errorf(MsgErrCryptoRand, err)
		}
		digit, err := cryptoRandInt(10)
		if err != nil {
			return "", fmt.Errorf(MsgErrCryptoRand, err)
		}
		selected[wordIdx] = fmt.Sprintf("%s%d", selected[wordIdx], digit)
	}

	return strings.Join(selected, cfg.Separator), nil
}

func buildCharset(cfg GeneratorConfig) []byte {
	var sb strings.Builder
	if cfg.Uppercase {
		sb.WriteString(uppercaseChars)
	}
	if cfg.Lowercase {
		sb.WriteString(lowercaseChars)
	}
	if cfg.Digits {
		sb.WriteString(digitChars)
	}
	if cfg.Symbols {
		sb.WriteString(symbolChars)
	}

	charset := sb.String()

	if cfg.ExcludeChars != "" {
		filtered := strings.Builder{}
		for _, c := range charset {
			if !strings.ContainsRune(cfg.ExcludeChars, c) {
				filtered.WriteRune(c)
			}
		}
		charset = filtered.String()
	}

	return []byte(charset)
}

// enabledClassChars returns the character set string for each enabled class,
// filtered by ExcludeChars. Classes that become empty after filtering are omitted.
func enabledClassChars(cfg GeneratorConfig) []string {
	raw := []struct {
		enabled bool
		chars   string
	}{
		{cfg.Uppercase, uppercaseChars},
		{cfg.Lowercase, lowercaseChars},
		{cfg.Digits, digitChars},
		{cfg.Symbols, symbolChars},
	}
	var classes []string
	for _, r := range raw {
		if !r.enabled {
			continue
		}
		filtered := filterExcluded(r.chars, cfg.ExcludeChars)
		if len(filtered) > 0 {
			classes = append(classes, filtered)
		}
	}
	return classes
}

func filterExcluded(chars, exclude string) string {
	if exclude == "" {
		return chars
	}
	var sb strings.Builder
	for _, c := range chars {
		if !strings.ContainsRune(exclude, c) {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// countEnabledClasses returns how many character classes are enabled in the config.
func countEnabledClasses(cfg GeneratorConfig) int {
	return len(enabledClassChars(cfg))
}

// randomPermutation picks n distinct random indices from [0, total).
func randomPermutation(total, n int) ([]int, error) {
	indices := make([]int, total)
	for i := range indices {
		indices[i] = i
	}
	// Fisher-Yates shuffle for the first n elements.
	for i := 0; i < n; i++ {
		j, err := cryptoRandInt(total - i)
		if err != nil {
			return nil, err
		}
		indices[i], indices[i+j] = indices[i+j], indices[i]
	}
	return indices[:n], nil
}

func cryptoRandInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
