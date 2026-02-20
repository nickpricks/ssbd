package core

// GeneratorConfig controls password generation behavior.
type GeneratorConfig struct {
	Length     int
	Uppercase  bool
	Lowercase  bool
	Digits     bool
	Symbols    bool
	ExcludeChars string // characters to exclude from generation
}

// DefaultGeneratorConfig returns sensible defaults for password generation.
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		Length:    16,
		Uppercase: true,
		Lowercase: true,
		Digits:    true,
		Symbols:   true,
	}
}

// PassphraseConfig controls passphrase generation behavior.
type PassphraseConfig struct {
	Words     int
	Separator string
	Capitalize bool // capitalize first letter of each word
	AddNumber  bool // append a random digit to a random word
}

// DefaultPassphraseConfig returns sensible defaults for passphrase generation.
func DefaultPassphraseConfig() PassphraseConfig {
	return PassphraseConfig{
		Words:     4,
		Separator: "-",
		Capitalize: true,
		AddNumber:  false,
	}
}

// ScoreResult holds the output of a password strength check.
type ScoreResult struct {
	Score       int      `json:"score"`       // 0-100
	Label       string   `json:"label"`       // Weak, Fair, Strong, Very Strong
	Entropy     float64  `json:"entropy"`     // bits of entropy
	Penalties   []string `json:"penalties"`   // reasons for deductions
	Suggestions []string `json:"suggestions"` // how to improve
	Breached    bool     `json:"breached"`    // found in HIBP
}

// LabelForScore returns a human-readable label for a numeric score.
func LabelForScore(score int) string {
	switch {
	case score >= 80:
		return "Very Strong"
	case score >= 60:
		return "Strong"
	case score >= 40:
		return "Fair"
	default:
		return "Weak"
	}
}
