package core

import "testing"

func TestScore_EmptyPassword(t *testing.T) {
	result := Score("")
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
	if result.Label != "Weak" {
		t.Errorf("expected label Weak, got %s", result.Label)
	}
}

func TestScore_CommonPasswords(t *testing.T) {
	commons := []string{"password", "123456", "qwerty", "letmein", "admin"}
	for _, pw := range commons {
		result := Score(pw)
		if result.Score >= 40 {
			t.Errorf("%q: expected score < 40, got %d", pw, result.Score)
		}
	}
}

func TestScore_LeetSpeak(t *testing.T) {
	tests := []struct {
		password string
		wantPen  bool
	}{
		{"p@$$w0rd", true},
		{"$umm3r", true}, // leet for "summer"
		{"xK9#mQ2!", false},
	}
	for _, tt := range tests {
		result := Score(tt.password)
		hasPen := false
		for _, p := range result.Penalties {
			if p == "leet-speak variant of common password" {
				hasPen = true
			}
		}
		if tt.wantPen && !hasPen {
			t.Errorf("%q: expected leet-speak penalty", tt.password)
		}
		if !tt.wantPen && hasPen {
			t.Errorf("%q: unexpected leet-speak penalty", tt.password)
		}
	}
}

func TestScore_SequencePenalty(t *testing.T) {
	tests := []struct {
		password string
		wantPen  bool
	}{
		{"abcdef", true},
		{"123456", true},
		{"fedcba", true},
		{"azbycx", false},
	}
	for _, tt := range tests {
		result := Score(tt.password)
		hasPen := false
		for _, p := range result.Penalties {
			if p == "contains sequential characters" {
				hasPen = true
			}
		}
		if tt.wantPen && !hasPen {
			t.Errorf("%q: expected sequence penalty", tt.password)
		}
		if !tt.wantPen && hasPen {
			t.Errorf("%q: unexpected sequence penalty", tt.password)
		}
	}
}

func TestScore_RepeatPenalty(t *testing.T) {
	tests := []struct {
		password string
		wantPen  bool
	}{
		{"aaabbb", true},
		{"111111", true},
		{"abcabc", false},
	}
	for _, tt := range tests {
		result := Score(tt.password)
		hasPen := false
		for _, p := range result.Penalties {
			if p == "contains repeated characters" {
				hasPen = true
			}
		}
		if tt.wantPen && !hasPen {
			t.Errorf("%q: expected repeat penalty", tt.password)
		}
		if !tt.wantPen && hasPen {
			t.Errorf("%q: unexpected repeat penalty", tt.password)
		}
	}
}

func TestScore_KeyboardWalkPenalty(t *testing.T) {
	tests := []struct {
		password string
		wantPen  bool
	}{
		{"qwerty", true},
		{"asdf", true},
		{"zxcv", true},
		{"abxy", false},
	}
	for _, tt := range tests {
		result := Score(tt.password)
		hasPen := false
		for _, p := range result.Penalties {
			if p == "contains keyboard walk pattern" {
				hasPen = true
			}
		}
		if tt.wantPen && !hasPen {
			t.Errorf("%q: expected keyboard walk penalty", tt.password)
		}
		if !tt.wantPen && hasPen {
			t.Errorf("%q: unexpected keyboard walk penalty", tt.password)
		}
	}
}

func TestScore_StrongPassword(t *testing.T) {
	// A long random-looking password should score well
	result := Score("kX9#mQ2!pL7&nR4@wZ8$")
	if result.Score < 60 {
		t.Errorf("expected strong score (>=60), got %d", result.Score)
	}
}

func TestScore_LengthBonus(t *testing.T) {
	short := Score("aB3$xY7!")          // 8 chars
	long := Score("aB3$xY7!kM2#pN9@")  // 16 chars
	if long.Score <= short.Score {
		t.Errorf("longer password should score higher: short=%d, long=%d", short.Score, long.Score)
	}
}

func TestLabelForScore(t *testing.T) {
	tests := []struct {
		score int
		label string
	}{
		{0, "Weak"},
		{20, "Weak"},
		{39, "Weak"},
		{40, "Fair"},
		{59, "Fair"},
		{60, "Strong"},
		{79, "Strong"},
		{80, "Very Strong"},
		{100, "Very Strong"},
	}
	for _, tt := range tests {
		got := LabelForScore(tt.score)
		if got != tt.label {
			t.Errorf("score %d: expected %q, got %q", tt.score, tt.label, got)
		}
	}
}

func TestScore_GeneratedPasswordIsStrong(t *testing.T) {
	// Integration test: generated passwords should always score well
	cfg := DefaultGeneratorConfig()
	for i := 0; i < 50; i++ {
		pw, err := Generate(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := Score(pw)
		if result.Score < 60 {
			t.Errorf("generated password %q scored only %d", pw, result.Score)
		}
	}
}

func TestNormalizeLeet(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"p@$$w0rd", "password"},
		{"h3ll0", "hello"},
		{"1337", "ieet"},
		{"normal", "normal"},
	}
	for _, tt := range tests {
		got := normalizeLeet(tt.input)
		if got != tt.want {
			t.Errorf("normalizeLeet(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
