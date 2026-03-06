package core

import (
	"strings"
	"testing"
	"unicode"
)

func TestGenerate_DefaultConfig(t *testing.T) {
	cfg := DefaultGeneratorConfig()
	pw, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != cfg.Length {
		t.Errorf("expected length %d, got %d", cfg.Length, len(pw))
	}
}

func TestGenerate_CustomLength(t *testing.T) {
	for _, length := range []int{4, 8, 16, 32, 64, 128} {
		cfg := DefaultGeneratorConfig()
		cfg.Length = length
		pw, err := Generate(cfg)
		if err != nil {
			t.Fatalf("length %d: unexpected error: %v", length, err)
		}
		if len(pw) != length {
			t.Errorf("expected length %d, got %d", length, len(pw))
		}
	}
}

func TestGenerate_LengthTooShortForClasses(t *testing.T) {
	// All 4 classes enabled but length is only 3 — impossible to satisfy
	cfg := GeneratorConfig{Length: 3, Uppercase: true, Lowercase: true, Digits: true, Symbols: true}
	_, err := Generate(cfg)
	if err == nil {
		t.Error("expected error when length < enabled character classes")
	}

	// Single class with length 1 should still work
	cfg = GeneratorConfig{Length: 1, Lowercase: true}
	pw, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != 1 {
		t.Errorf("expected length 1, got %d", len(pw))
	}
}

func TestGenerate_EnsureAllClassesPresent(t *testing.T) {
	// With a short password (length=4) and all classes, every class must appear.
	// Run multiple times to catch probabilistic failures.
	cfg := GeneratorConfig{Length: 4, Uppercase: true, Lowercase: true, Digits: true, Symbols: true}
	for i := 0; i < 200; i++ {
		pw, err := Generate(cfg)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		hasUpper := strings.ContainsFunc(pw, unicode.IsUpper)
		hasLower := strings.ContainsFunc(pw, unicode.IsLower)
		hasDigit := strings.ContainsFunc(pw, unicode.IsDigit)
		hasSymbol := false
		for _, r := range pw {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				hasSymbol = true
				break
			}
		}
		if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
			t.Errorf("iteration %d: password %q missing a required class (upper=%v lower=%v digit=%v symbol=%v)",
				i, pw, hasUpper, hasLower, hasDigit, hasSymbol)
		}
	}
}

func TestGenerate_CharacterClasses(t *testing.T) {
	tests := []struct {
		name       string
		cfg        GeneratorConfig
		wantUpper  bool
		wantLower  bool
		wantDigit  bool
		wantSymbol bool
	}{
		{
			name:      "only lowercase",
			cfg:       GeneratorConfig{Length: 50, Lowercase: true},
			wantLower: true,
		},
		{
			name:      "only uppercase",
			cfg:       GeneratorConfig{Length: 50, Uppercase: true},
			wantUpper: true,
		},
		{
			name:      "only digits",
			cfg:       GeneratorConfig{Length: 50, Digits: true},
			wantDigit: true,
		},
		{
			name:       "all classes",
			cfg:        GeneratorConfig{Length: 50, Uppercase: true, Lowercase: true, Digits: true, Symbols: true},
			wantUpper:  true,
			wantLower:  true,
			wantDigit:  true,
			wantSymbol: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw, err := Generate(tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasUpper := strings.ContainsFunc(pw, unicode.IsUpper)
			hasLower := strings.ContainsFunc(pw, unicode.IsLower)
			hasDigit := strings.ContainsFunc(pw, unicode.IsDigit)
			hasSymbol := false
			for _, r := range pw {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					hasSymbol = true
					break
				}
			}

			if tt.wantUpper && !hasUpper {
				t.Error("expected uppercase characters")
			}
			if !tt.wantUpper && hasUpper {
				t.Error("unexpected uppercase characters")
			}
			if tt.wantLower && !hasLower {
				t.Error("expected lowercase characters")
			}
			if !tt.wantLower && hasLower {
				t.Error("unexpected lowercase characters")
			}
			if tt.wantDigit && !hasDigit {
				t.Error("expected digit characters")
			}
			if !tt.wantDigit && hasDigit {
				t.Error("unexpected digit characters")
			}
			if tt.wantSymbol && !hasSymbol {
				t.Error("expected symbol characters")
			}
		})
	}
}

func TestGenerate_ExcludeChars(t *testing.T) {
	cfg := GeneratorConfig{
		Length:       100,
		Lowercase:    true,
		Uppercase:    true,
		ExcludeChars: "aeiouAEIOU",
	}
	pw, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range pw {
		if strings.ContainsRune("aeiouAEIOU", r) {
			t.Errorf("password contains excluded character: %c", r)
		}
	}
}

func TestGenerate_InvalidLength(t *testing.T) {
	cfg := GeneratorConfig{Length: 0, Lowercase: true}
	_, err := Generate(cfg)
	if err == nil {
		t.Error("expected error for length 0")
	}
}

func TestGenerate_NoCharset(t *testing.T) {
	cfg := GeneratorConfig{Length: 10}
	_, err := Generate(cfg)
	if err == nil {
		t.Error("expected error when no character classes enabled")
	}
}

func TestGeneratePassphrase_Default(t *testing.T) {
	cfg := DefaultPassphraseConfig()
	pp, err := GeneratePassphrase(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	words := strings.Split(pp, cfg.Separator)
	if len(words) != cfg.Words {
		t.Errorf("expected %d words, got %d", cfg.Words, len(words))
	}
}

func TestGeneratePassphrase_Capitalize(t *testing.T) {
	cfg := PassphraseConfig{Words: 5, Separator: "-", Capitalize: true}
	pp, err := GeneratePassphrase(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, word := range strings.Split(pp, "-") {
		if len(word) > 0 && !unicode.IsUpper(rune(word[0])) {
			t.Errorf("expected capitalized word, got %q", word)
		}
	}
}

func TestGeneratePassphrase_CustomSeparator(t *testing.T) {
	cfg := PassphraseConfig{Words: 3, Separator: ".", Capitalize: false}
	pp, err := GeneratePassphrase(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(pp, ".") {
		t.Error("expected '.' separator in passphrase")
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	cfg := DefaultGeneratorConfig()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pw, err := Generate(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[pw] {
			t.Error("generated duplicate password")
		}
		seen[pw] = true
	}
}
