package core

import (
	"fmt"
	"strings"
	"unicode"
)

// Suggest returns actionable suggestions to improve a password given its score result.
func Suggest(password string, result ScoreResult) []string {
	var suggestions []string

	if len(password) < LengthBonusThreshold {
		suggestions = append(suggestions, fmt.Sprintf("Increase length to at least %d characters", LengthBonusThreshold))
	}
	if len(password) < DefaultPasswordLength && len(password) >= LengthBonusThreshold {
		suggestions = append(suggestions, fmt.Sprintf("Consider increasing length to %d+ characters for extra strength", DefaultPasswordLength))
	}

	if !hasCharClass(password, unicode.IsUpper) {
		suggestions = append(suggestions, "Add uppercase letters")
	}
	if !hasCharClass(password, unicode.IsLower) {
		suggestions = append(suggestions, "Add lowercase letters")
	}
	if !hasCharClass(password, unicode.IsDigit) {
		suggestions = append(suggestions, "Add numbers")
	}
	if !hasSymbols(password) {
		suggestions = append(suggestions, "Add special characters (!@#$%^&*)")
	}

	for _, pen := range result.Penalties {
		switch {
		case strings.Contains(pen, "sequential"):
			suggestions = append(suggestions, "Avoid sequential characters (abc, 123)")
		case strings.Contains(pen, "repeated"):
			suggestions = append(suggestions, "Avoid repeated characters (aaa, 111)")
		case strings.Contains(pen, "keyboard walk"):
			suggestions = append(suggestions, "Avoid keyboard patterns (qwerty, asdf)")
		case strings.Contains(pen, "common password"):
			suggestions = append(suggestions, "This is a commonly used password — choose something unique")
		case strings.Contains(pen, "leet-speak"):
			suggestions = append(suggestions, "Simple letter substitutions (@ for a, 0 for o) don't add real security")
		}
	}

	if result.Breached {
		suggestions = append(suggestions, "This password appeared in a data breach — do not use it")
	}

	if len(suggestions) == 0 && result.Score < VeryStrongThreshold {
		suggestions = append(suggestions, "Consider using a randomly generated password or passphrase")
	}

	return suggestions
}

func hasCharClass(password string, check func(rune) bool) bool {
	for _, r := range password {
		if check(r) {
			return true
		}
	}
	return false
}

func hasSymbols(password string) bool {
	for _, r := range password {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
