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
// All variants have the same length as the base (v1 behavior).
func Rotate(base string, count int) ([]string, error) {
	cfg := DefaultRotateConfig()
	cfg.Count = count
	cfg.StrictLength = true
	return RotateWithConfig(base, cfg)
}

// RotateWithConfig generates variants with full configuration control.
// When StrictLength is true or MinLength/MaxLength are both 0, it produces
// same-length substitution-only variants (v1 behavior). When length bounds
// are set, variants can grow or shrink by up to MaxLengthDelta chars.
func RotateWithConfig(base string, cfg RotateConfig) ([]string, error) {
	if cfg.Count < 1 {
		return nil, fmt.Errorf("count must be at least 1")
	}
	if len(base) == 0 {
		return nil, fmt.Errorf("base password must not be empty")
	}

	runes := []rune(base)
	baseLen := len(runes)

	// Resolve effective length bounds.
	minLen, maxLen := baseLen, baseLen
	variableLength := !cfg.StrictLength && (cfg.MinLength > 0 || cfg.MaxLength > 0)

	if variableLength {
		if cfg.MinLength > 0 {
			minLen = cfg.MinLength
		}
		if cfg.MaxLength > 0 {
			maxLen = cfg.MaxLength
		}
		if minLen > maxLen {
			return nil, fmt.Errorf("min-length (%d) cannot exceed max-length (%d)", minLen, maxLen)
		}
		if minLen < 1 {
			return nil, fmt.Errorf("min-length must be at least 1")
		}
		// Clamp to ±MaxLengthDelta from base.
		if minLen < baseLen-MaxLengthDelta {
			minLen = baseLen - MaxLengthDelta
		}
		if maxLen > baseLen+MaxLengthDelta {
			maxLen = baseLen + MaxLengthDelta
		}
	}

	subMuts := findMutations(runes)

	if variableLength {
		return generateVariableLengthVariants(runes, base, subMuts, minLen, maxLen, cfg.Count)
	}

	// Strict / default path: substitution-only, same length.
	if len(subMuts) == 0 {
		return nil, fmt.Errorf("password has no positions that can be varied (need letters or leet-speak characters)")
	}
	return generateSubstitutionVariants(runes, base, subMuts, cfg.Count)
}

// generateSubstitutionVariants is the v1 generation path: same-length, substitution-only.
func generateSubstitutionVariants(runes []rune, base string, mutations []mutation, count int) ([]string, error) {
	seen := make(map[string]bool)
	seen[base] = true
	var variants []string

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

// generateVariableLengthVariants produces variants that may differ in length from the base.
func generateVariableLengthVariants(runes []rune, base string, subMuts []mutation, minLen, maxLen, count int) ([]string, error) {
	baseLen := len(runes)
	lenMuts := findLengthMutations(runes)

	// Check feasibility: need shrink but no drop candidates.
	if maxLen < baseLen && !hasDropCandidates(lenMuts) {
		return nil, fmt.Errorf("cannot generate shorter variants: password has no redundant repeats to drop")
	}

	// If no substitution mutations and no length mutations, we can't do anything.
	if len(subMuts) == 0 && len(lenMuts) == 0 {
		return nil, fmt.Errorf("password has no positions that can be varied")
	}

	seen := make(map[string]bool)
	seen[base] = true
	var variants []string

	subSpace := max(len(subMuts), 1)
	lenSpace := max(len(lenMuts), 1)
	maxAttempts := count * subSpace * lenSpace * 6

	cycle := 0
	for len(variants) < count && cycle < maxAttempts {
		variant, err := buildVariableLengthVariant(runes, subMuts, lenMuts, minLen, maxLen, baseLen, cycle)
		if err != nil {
			cycle++
			continue
		}
		key := string(variant)
		if !seen[key] {
			seen[key] = true
			variants = append(variants, key)
		}
		cycle++
	}

	if len(variants) < count {
		if len(variants) == 0 {
			return nil, fmt.Errorf("could not generate any variants within length bounds [%d, %d]", minLen, maxLen)
		}
		return variants, fmt.Errorf("could only generate %d unique variants (requested %d)", len(variants), count)
	}
	return variants, nil
}

// buildVariableLengthVariant produces a single candidate by combining a substitution
// cycle with a length mutation selection.
func buildVariableLengthVariant(base []rune, subMuts []mutation, lenMuts []lengthMutation, minLen, maxLen, baseLen, cycle int) ([]rune, error) {
	// Decompose cycle into: how many length mutations to apply (0–MaxLengthDelta),
	// which length mutation to use, and which substitution cycle.
	maxGrow := min(MaxLengthDelta, maxLen-baseLen)
	maxShrink := min(MaxLengthDelta, baseLen-minLen)
	if maxGrow < 0 {
		maxGrow = 0
	}
	if maxShrink < 0 {
		maxShrink = 0
	}

	// deltaChoices: 0 (no change), +1..+maxGrow, -1..-maxShrink
	deltaChoices := 1 + maxGrow + maxShrink

	lenSpace := max(len(lenMuts), 1)
	deltaIdx := cycle % deltaChoices
	lenIdx := (cycle / deltaChoices) % lenSpace
	subCycle := cycle / (deltaChoices * lenSpace)

	// Determine target delta from base length.
	var delta int
	if deltaIdx == 0 {
		delta = 0
	} else if deltaIdx <= maxGrow {
		delta = deltaIdx
	} else {
		delta = -(deltaIdx - maxGrow)
	}

	// Start with a substitution variant.
	var current []rune
	if len(subMuts) > 0 {
		current = applyMutationCycle(base, subMuts, subCycle)
	} else {
		current = make([]rune, len(base))
		copy(current, base)
	}

	// Apply length mutations.
	if delta > 0 && len(lenMuts) > 0 {
		for i := 0; i < delta; i++ {
			lm := lenMuts[(lenIdx+i)%len(lenMuts)]
			if lm.kind == lmDropRepeat {
				continue // skip drops when growing
			}
			applied, err := applyLengthMutation(current, lm)
			if err != nil {
				return nil, err
			}
			current = applied
		}
	} else if delta < 0 && len(lenMuts) > 0 {
		for i := 0; i < -delta; i++ {
			// Find a drop candidate that's valid for the current runes.
			dropped := false
			for j := 0; j < len(lenMuts); j++ {
				lm := lenMuts[(lenIdx+j)%len(lenMuts)]
				if lm.kind != lmDropRepeat {
					continue
				}
				applied, err := applyDropRepeat(current)
				if err == nil {
					current = applied
					dropped = true
					break
				}
			}
			if !dropped {
				return nil, fmt.Errorf("cannot shrink further")
			}
		}
	}

	// Verify length bounds.
	if len(current) < minLen || len(current) > maxLen {
		return nil, fmt.Errorf("variant length %d outside bounds [%d, %d]", len(current), minLen, maxLen)
	}

	return current, nil
}

// lengthMutKind identifies the type of length-changing operation.
type lengthMutKind int

const (
	lmInsert    lengthMutKind = iota // insert a random char at a position
	lmAppend                         // append a symbol/digit to end
	lmPrepend                        // prepend a symbol/digit to start
	lmDropRepeat                     // remove one char from a consecutive repeat run
)

// lengthMutation describes a candidate length-changing operation.
type lengthMutation struct {
	kind     lengthMutKind
	pos      int    // rune index to act on
	charPool string // pool to draw from (empty for drop)
}

const growPool = digitChars + symbolChars

// findLengthMutations identifies candidate length-changing operations.
func findLengthMutations(runes []rune) []lengthMutation {
	var muts []lengthMutation
	n := len(runes)

	// Append and prepend are always available.
	muts = append(muts, lengthMutation{kind: lmAppend, pos: n, charPool: growPool})
	muts = append(muts, lengthMutation{kind: lmPrepend, pos: 0, charPool: growPool})

	// Insert candidates at evenly-spaced positions (cap at 8).
	maxInserts := 8
	if n < maxInserts {
		maxInserts = n
	}
	if maxInserts > 0 {
		step := n / maxInserts
		if step < 1 {
			step = 1
		}
		for i := 1; i < n; i += step {
			pool := contextualPool(runes, i)
			muts = append(muts, lengthMutation{kind: lmInsert, pos: i, charPool: pool})
			if len(muts) >= maxInserts+2 { // +2 for append/prepend
				break
			}
		}
	}

	// Drop-repeat candidates: one per run of 2+ identical consecutive runes.
	for i := 0; i < n-1; {
		runStart := i
		for i < n-1 && runes[i] == runes[i+1] {
			i++
		}
		if i > runStart {
			muts = append(muts, lengthMutation{kind: lmDropRepeat, pos: runStart})
		}
		i++
	}

	return muts
}

// contextualPool chooses a character pool based on the neighbors of a position.
func contextualPool(runes []rune, pos int) string {
	if pos > 0 {
		r := runes[pos-1]
		if unicode.IsDigit(r) {
			return digitChars
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return symbolChars
		}
	}
	return lowercaseChars + digitChars
}

// applyLengthMutation applies a single growth mutation (insert/append/prepend).
func applyLengthMutation(runes []rune, lm lengthMutation) ([]rune, error) {
	if lm.charPool == "" {
		return nil, fmt.Errorf("no char pool for mutation")
	}
	idx, err := cryptoRandInt(len(lm.charPool))
	if err != nil {
		return nil, err
	}
	ch := rune(lm.charPool[idx])

	switch lm.kind {
	case lmAppend:
		return append(runes, ch), nil
	case lmPrepend:
		result := make([]rune, 0, len(runes)+1)
		result = append(result, ch)
		result = append(result, runes...)
		return result, nil
	case lmInsert:
		pos := lm.pos
		if pos > len(runes) {
			pos = len(runes)
		}
		result := make([]rune, 0, len(runes)+1)
		result = append(result, runes[:pos]...)
		result = append(result, ch)
		result = append(result, runes[pos:]...)
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported growth mutation kind: %d", lm.kind)
	}
}

// applyDropRepeat finds the first consecutive repeat run in runes and removes one char.
func applyDropRepeat(runes []rune) ([]rune, error) {
	for i := 0; i < len(runes)-1; i++ {
		if runes[i] == runes[i+1] {
			result := make([]rune, 0, len(runes)-1)
			result = append(result, runes[:i]...)
			result = append(result, runes[i+1:]...)
			return result, nil
		}
	}
	return nil, fmt.Errorf("no repeat runs to drop")
}

// hasDropCandidates returns true if any length mutation is a drop-repeat.
func hasDropCandidates(muts []lengthMutation) bool {
	for _, m := range muts {
		if m.kind == lmDropRepeat {
			return true
		}
	}
	return false
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
