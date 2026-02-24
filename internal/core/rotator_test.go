package core

import (
	"strings"
	"testing"
	"unicode"
)

func TestRotate_BasicVariants(t *testing.T) {
	variants, err := Rotate("p@sSwor4", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 5 {
		t.Fatalf("expected 5 variants, got %d", len(variants))
	}

	// All variants must differ from the base.
	for i, v := range variants {
		if v == "p@sSwor4" {
			t.Errorf("variant %d is identical to base: %s", i, v)
		}
	}

	// All variants must be unique.
	seen := make(map[string]bool)
	for i, v := range variants {
		if seen[v] {
			t.Errorf("variant %d is a duplicate: %s", i, v)
		}
		seen[v] = true
	}
}

func TestRotate_SameLength(t *testing.T) {
	base := "p@sSwor4"
	variants, err := Rotate(base, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range variants {
		if len([]rune(v)) != len([]rune(base)) {
			t.Errorf("variant %d has different length: base=%d variant=%d (%s)", i, len([]rune(base)), len([]rune(v)), v)
		}
	}
}

func TestRotate_PreservesStructure(t *testing.T) {
	// Variants should normalize to the same base word.
	base := "p@sSwor4"
	baseNorm := normalizeBase(base)
	variants, err := Rotate(base, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range variants {
		vNorm := normalizeBase(v)
		if vNorm != baseNorm {
			t.Errorf("variant %d normalized form differs: base=%q variant=%q (from %q)", i, baseNorm, vNorm, v)
		}
	}
}

func TestRotate_EmptyPassword(t *testing.T) {
	_, err := Rotate("", 5)
	if err == nil {
		t.Error("expected error for empty password")
	}
}

func TestRotate_ZeroCount(t *testing.T) {
	_, err := Rotate("p@ssword", 0)
	if err == nil {
		t.Error("expected error for zero count")
	}
}

func TestRotate_SimplePassword(t *testing.T) {
	// A password with only letters can still be rotated via case flips.
	variants, err := Rotate("password", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(variants))
	}
	for i, v := range variants {
		if v == "password" {
			t.Errorf("variant %d is identical to base", i)
		}
	}
}

func TestRotate_AllDigitsNoMutations(t *testing.T) {
	// "999" has no mutable positions (no letters, no leet chars).
	_, err := Rotate("999", 1)
	if err == nil {
		t.Error("expected error for password with no mutable positions")
	}
}

func TestRotate_SingleChar(t *testing.T) {
	variants, err := Rotate("a", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0] == "a" {
		t.Error("variant is identical to base")
	}
}

// --- SSBD v2 tests ---

func TestRotateWithConfig_DefaultMatchesV1(t *testing.T) {
	// Zero-value MinLength/MaxLength should produce same-length variants.
	base := "p@sSwor4"
	cfg := DefaultRotateConfig()
	cfg.Count = 5
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	baseLen := len([]rune(base))
	for i, v := range variants {
		if len([]rune(v)) != baseLen {
			t.Errorf("variant %d length %d != base %d: %s", i, len([]rune(v)), baseLen, v)
		}
	}
}

func TestRotateWithConfig_VariableLengthGrow(t *testing.T) {
	base := "p@ssword"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 10, MinLength: baseLen, MaxLength: baseLen + 3}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 10 {
		t.Fatalf("expected 10 variants, got %d", len(variants))
	}

	hasLonger := false
	for i, v := range variants {
		vLen := len([]rune(v))
		if vLen < baseLen || vLen > baseLen+3 {
			t.Errorf("variant %d length %d outside [%d, %d]: %s", i, vLen, baseLen, baseLen+3, v)
		}
		if vLen > baseLen {
			hasLonger = true
		}
	}
	if !hasLonger {
		t.Error("expected at least one variant longer than base")
	}
}

func TestRotateWithConfig_VariableLengthShrink(t *testing.T) {
	// Base with repeat runs that can be dropped.
	base := "p@@sswword"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 10, MinLength: baseLen - 3, MaxLength: baseLen}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasShorter := false
	for i, v := range variants {
		vLen := len([]rune(v))
		if vLen < baseLen-3 || vLen > baseLen {
			t.Errorf("variant %d length %d outside [%d, %d]: %s", i, vLen, baseLen-3, baseLen, v)
		}
		if vLen < baseLen {
			hasShorter = true
		}
	}
	if !hasShorter {
		t.Error("expected at least one variant shorter than base")
	}
}

func TestRotateWithConfig_StrictLengthOverrides(t *testing.T) {
	base := "p@sSwor4"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 5, MinLength: 4, MaxLength: 20, StrictLength: true}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range variants {
		if len([]rune(v)) != baseLen {
			t.Errorf("variant %d length %d != base %d with StrictLength: %s", i, len([]rune(v)), baseLen, v)
		}
	}
}

func TestRotateWithConfig_BoundsValidation(t *testing.T) {
	// min > max should error.
	_, err := RotateWithConfig("password", RotateConfig{Count: 1, MinLength: 20, MaxLength: 10})
	if err == nil {
		t.Error("expected error when MinLength > MaxLength")
	}
}

func TestRotateWithConfig_NoRepeatsCannotShrink(t *testing.T) {
	// "abcdef" has no repeat runs, so shrinking should fail.
	base := "abcdef"
	baseLen := len([]rune(base))
	_, err := RotateWithConfig(base, RotateConfig{Count: 1, MinLength: 1, MaxLength: baseLen - 1})
	if err == nil {
		t.Error("expected error when shrinking a password with no repeat runs")
	}
}

func TestRotateWithConfig_LargeCount(t *testing.T) {
	base := "p@sSwor4"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 30, MinLength: baseLen, MaxLength: baseLen + 3}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 30 {
		t.Fatalf("expected 30 variants, got %d", len(variants))
	}

	seen := make(map[string]bool)
	for i, v := range variants {
		if seen[v] {
			t.Errorf("duplicate variant %d: %s", i, v)
		}
		seen[v] = true
		vLen := len([]rune(v))
		if vLen < baseLen || vLen > baseLen+3 {
			t.Errorf("variant %d length %d outside bounds: %s", i, vLen, v)
		}
	}
}

func TestRotateWithConfig_AllDigitsCanGrow(t *testing.T) {
	// "999" has no substitution mutations but should work with variable length.
	base := "999"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 3, MinLength: baseLen, MaxLength: baseLen + 3}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) < 1 {
		t.Fatal("expected at least 1 variant")
	}
	for i, v := range variants {
		if v == base {
			t.Errorf("variant %d identical to base", i)
		}
	}
}

func TestRotateWithConfig_UniqueVariants(t *testing.T) {
	base := "p@sSwor4"
	baseLen := len([]rune(base))
	cfg := RotateConfig{Count: 15, MinLength: baseLen - 1, MaxLength: baseLen + 2}
	variants, err := RotateWithConfig(base, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	seen := make(map[string]bool)
	for i, v := range variants {
		if seen[v] {
			t.Errorf("duplicate variant %d: %s", i, v)
		}
		seen[v] = true
		if v == base {
			t.Errorf("variant %d is identical to base", i)
		}
	}
}

func TestFindLengthMutations_RepeatDetection(t *testing.T) {
	runes := []rune("aabbcc")
	muts := findLengthMutations(runes)
	dropCount := 0
	for _, m := range muts {
		if m.kind == lmDropRepeat {
			dropCount++
		}
	}
	if dropCount != 3 {
		t.Errorf("expected 3 drop candidates for 'aabbcc', got %d", dropCount)
	}
}

func TestFindLengthMutations_NoRepeats(t *testing.T) {
	runes := []rune("abcdef")
	muts := findLengthMutations(runes)
	for _, m := range muts {
		if m.kind == lmDropRepeat {
			t.Error("unexpected drop candidate for 'abcdef'")
		}
	}
}

func TestFindLengthMutations_AlwaysHasAppendPrepend(t *testing.T) {
	runes := []rune("x")
	muts := findLengthMutations(runes)
	hasAppend, hasPrepend := false, false
	for _, m := range muts {
		if m.kind == lmAppend {
			hasAppend = true
		}
		if m.kind == lmPrepend {
			hasPrepend = true
		}
	}
	if !hasAppend {
		t.Error("expected append candidate")
	}
	if !hasPrepend {
		t.Error("expected prepend candidate")
	}
}

func TestApplyLengthMutation_Insert(t *testing.T) {
	runes := []rune("abcd")
	lm := lengthMutation{kind: lmInsert, pos: 2, charPool: "xyz"}
	result, err := applyLengthMutation(runes, lm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("expected length 5, got %d: %s", len(result), string(result))
	}
	// First two chars preserved.
	if string(result[:2]) != "ab" {
		t.Errorf("expected prefix 'ab', got %s", string(result[:2]))
	}
	// Inserted char from pool.
	if !strings.ContainsRune("xyz", result[2]) {
		t.Errorf("inserted char %c not from pool 'xyz'", result[2])
	}
}

func TestApplyLengthMutation_Append(t *testing.T) {
	runes := []rune("abc")
	lm := lengthMutation{kind: lmAppend, pos: 3, charPool: growPool}
	result, err := applyLengthMutation(runes, lm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected length 4, got %d", len(result))
	}
	if string(result[:3]) != "abc" {
		t.Errorf("expected prefix 'abc', got %s", string(result[:3]))
	}
	// Appended char should be digit or symbol.
	ch := result[3]
	if !unicode.IsDigit(ch) && !strings.ContainsRune(symbolChars, ch) {
		t.Errorf("appended char %c not from growPool", ch)
	}
}

func TestApplyDropRepeat(t *testing.T) {
	runes := []rune("aabb")
	result, err := applyDropRepeat(runes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected length 3, got %d: %s", len(result), string(result))
	}
}

func TestApplyDropRepeat_NoRepeats(t *testing.T) {
	runes := []rune("abcd")
	_, err := applyDropRepeat(runes)
	if err == nil {
		t.Error("expected error when no repeat runs exist")
	}
}
