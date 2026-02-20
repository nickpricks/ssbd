package core

import (
	"fmt"
	"strings"
	"unicode"
)

// Reverse leet map: letter -> possible substitutions.
// Used to generate variants by swapping between plain and leet forms.
var reverseLeet = buildReverseLeet()

func buildReverseLeet() map[rune][]rune {
	m := make(map[rune][]rune)
	for leet, plain := range leetMap {
		m[plain] = append(m[plain], leet)
	}
	return m
}

// Rotate generates count distinct variants of the base password by cycling
// leet-speak substitutions, case flips, and symbol positions through it.
// Each variant is guaranteed to differ from the base and from all other variants.
func Rotate(base string, count int) ([]string, error) {
	if count < 1 {
		return nil, fmt.Errorf("count must be at least 1")
	}
	if len(base) == 0 {
		return nil, fmt.Errorf("base password must not be empty")
	}

	// Find all mutable positions: positions where we can toggle case or
	// swap between a letter and its leet equivalent.
	runes := []rune(base)
	mutations := findMutations(runes)

	if len(mutations) == 0 {
		return nil, fmt.Errorf("password has no positions that can be varied (need letters or leet-speak characters)")
	}

	seen := make(map[string]bool)
	seen[base] = true
	var variants []string

	// Generate variants by cycling through mutation positions.
	// Each cycle applies a different combination of mutations.
	cycle := 0
	maxAttempts := count * len(mutations) * 4
	for len(variants) < count && cycle < maxAttempts {
		variant := applyMutationCycle(runes, mutations, cycle)
		key := string(variant)
		if !seen[key] {
			seen[key] = true
			variants = append(variants, key)
		}
		cycle++
	}

	if len(variants) < count {
		return variants, fmt.Errorf("could only generate %d unique variants (password has limited mutation points)", len(variants))
	}

	return variants, nil
}

// mutation represents a position in the password that can be toggled.
type mutation struct {
	pos       int
	original  rune
	alternates []rune // possible replacement runes
}

// findMutations identifies every position that can be varied.
func findMutations(runes []rune) []mutation {
	var mutations []mutation
	for i, r := range runes {
		var alts []rune

		lower := unicode.ToLower(r)

		// Case flip (for letters).
		if unicode.IsLetter(r) {
			if unicode.IsUpper(r) {
				alts = append(alts, unicode.ToLower(r))
			} else {
				alts = append(alts, unicode.ToUpper(r))
			}
		}

		// Leet-speak swap: if r is a letter, offer leet forms.
		if unicode.IsLetter(r) {
			if leets, ok := reverseLeet[lower]; ok {
				for _, l := range leets {
					if l != r {
						alts = append(alts, l)
					}
				}
			}
		}

		// Reverse leet: if r is a leet char, offer the plain letter forms.
		if plain, ok := leetMap[r]; ok {
			if plain != r {
				alts = append(alts, plain)
				alts = append(alts, unicode.ToUpper(plain))
			}
		}

		if len(alts) > 0 {
			mutations = append(mutations, mutation{
				pos:       i,
				original:  r,
				alternates: dedupRunes(alts, r),
			})
		}
	}
	return mutations
}

// applyMutationCycle produces a variant by selecting which mutations to apply
// based on the cycle number. Different cycles produce different combinations.
func applyMutationCycle(base []rune, mutations []mutation, cycle int) []rune {
	result := make([]rune, len(base))
	copy(result, base)

	// Strategy: for each mutation point, decide whether to apply it and which
	// alternate to use based on the cycle number. We treat the cycle as a
	// mixed-radix number where each digit selects the state at that position.
	remaining := cycle + 1 // +1 so cycle 0 produces the first variant, not the base
	for _, m := range mutations {
		choices := len(m.alternates) + 1 // +1 for keeping the original
		pick := remaining % choices
		remaining /= choices

		if pick == 0 {
			result[m.pos] = m.original
		} else {
			result[m.pos] = m.alternates[pick-1]
		}
	}

	return result
}

// dedupRunes removes duplicates and the original rune from alts.
func dedupRunes(alts []rune, original rune) []rune {
	seen := make(map[rune]bool)
	seen[original] = true
	var result []rune
	for _, r := range alts {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}
	return result
}

// normalizeBase converts a password to its plain lowercase form for analysis.
func normalizeBase(password string) string {
	var sb strings.Builder
	for _, r := range password {
		if plain, ok := leetMap[r]; ok {
			sb.WriteRune(plain)
		} else {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}
