package core

import (
	"math"
	"strings"
	"unicode"
)

// Common leet-speak substitutions for normalization.
var leetMap = map[rune]rune{
	'@': 'a', '4': 'a',
	'8': 'b',
	'(': 'c',
	'3': 'e',
	'6': 'g',
	'#': 'h',
	'1': 'i', '!': 'i', '|': 'i',
	'0': 'o',
	'$': 's', '5': 's',
	'7': 't', '+': 't',
	'2': 'z',
}

// Keyboard rows for walk detection.
var keyboardRows = []string{
	"qwertyuiop",
	"asdfghjkl",
	"zxcvbnm",
	"1234567890",
}

// Score evaluates password strength and returns a ScoreResult.
func Score(password string) ScoreResult {
	if len(password) == 0 {
		return ScoreResult{Score: 0, Label: "Weak", Penalties: []string{"empty password"}}
	}

	entropy := calculateEntropy(password)
	score := entropyToBaseScore(entropy)

	var penalties []string

	// Pattern penalties
	if pen := sequencePenalty(password); pen > 0 {
		score -= pen
		penalties = append(penalties, "contains sequential characters")
	}
	if pen := repeatPenalty(password); pen > 0 {
		score -= pen
		penalties = append(penalties, "contains repeated characters")
	}
	if pen := keyboardWalkPenalty(password); pen > 0 {
		score -= pen
		penalties = append(penalties, "contains keyboard walk pattern")
	}

	// Dictionary penalty (check both raw and leet-normalized)
	if isCommonPassword(password) {
		score -= 40
		penalties = append(penalties, "found in common password list")
	} else if isCommonPassword(normalizeLeet(password)) {
		score -= 30
		penalties = append(penalties, "leet-speak variant of common password")
	}

	// Length bonus
	if len(password) > 12 {
		bonus := int(math.Min(float64((len(password)-12)*2), 15))
		score += bonus
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	result := ScoreResult{
		Score:     score,
		Label:     LabelForScore(score),
		Entropy:   entropy,
		Penalties: penalties,
	}

	// Generate suggestions
	result.Suggestions = Suggest(password, result)

	return result
}

// calculateEntropy computes Shannon entropy based on the character pool used.
func calculateEntropy(password string) float64 {
	poolSize := characterPoolSize(password)
	if poolSize <= 1 {
		return 0
	}
	return float64(len(password)) * math.Log2(float64(poolSize))
}

func characterPoolSize(password string) int {
	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}

	pool := 0
	if hasLower {
		pool += 26
	}
	if hasUpper {
		pool += 26
	}
	if hasDigit {
		pool += 10
	}
	if hasSymbol {
		pool += 32
	}
	return pool
}

func entropyToBaseScore(entropy float64) int {
	// Map entropy to 0-100 scale.
	// ~28 bits (8 char lowercase) = ~25
	// ~50 bits = ~50
	// ~80 bits = ~75
	// ~128+ bits = 100
	score := int(entropy * 0.78)
	if score > 100 {
		score = 100
	}
	return score
}

// sequencePenalty detects ascending/descending character sequences (abc, 321).
func sequencePenalty(password string) int {
	if len(password) < 3 {
		return 0
	}

	maxRun := 1
	currentRun := 1
	runes := []rune(strings.ToLower(password))

	for i := 1; i < len(runes); i++ {
		diff := runes[i] - runes[i-1]
		if diff == 1 || diff == -1 {
			currentRun++
			if currentRun > maxRun {
				maxRun = currentRun
			}
		} else {
			currentRun = 1
		}
	}

	if maxRun >= 4 {
		return 15
	}
	if maxRun >= 3 {
		return 8
	}
	return 0
}

// repeatPenalty detects repeated characters (aaa, 1111).
func repeatPenalty(password string) int {
	if len(password) < 3 {
		return 0
	}

	maxRun := 1
	currentRun := 1
	runes := []rune(password)

	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			currentRun++
			if currentRun > maxRun {
				maxRun = currentRun
			}
		} else {
			currentRun = 1
		}
	}

	if maxRun >= 4 {
		return 20
	}
	if maxRun >= 3 {
		return 10
	}
	return 0
}

// keyboardWalkPenalty detects keyboard walk patterns (qwerty, asdf).
func keyboardWalkPenalty(password string) int {
	lower := strings.ToLower(password)

	for _, row := range keyboardRows {
		// Check for substrings of length 4+ from keyboard rows
		for windowSize := 4; windowSize <= len(row); windowSize++ {
			for start := 0; start <= len(row)-windowSize; start++ {
				pattern := row[start : start+windowSize]
				if strings.Contains(lower, pattern) {
					if windowSize >= 6 {
						return 20
					}
					return 10
				}
				// Also check reversed
				reversed := reverseString(pattern)
				if strings.Contains(lower, reversed) {
					if windowSize >= 6 {
						return 20
					}
					return 10
				}
			}
		}
	}
	return 0
}

// normalizeLeet converts leet-speak characters to their letter equivalents.
func normalizeLeet(password string) string {
	var sb strings.Builder
	for _, r := range password {
		if replacement, ok := leetMap[r]; ok {
			sb.WriteRune(replacement)
		} else {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
