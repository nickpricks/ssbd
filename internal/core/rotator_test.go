package core

import (
	"testing"
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
