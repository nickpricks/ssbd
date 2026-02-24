package core

// Default configuration values. Change these to adjust global behavior.
const (
	// Generator defaults
	DefaultPasswordLength = 16
	DefaultBulkCount      = 1

	// Passphrase defaults
	DefaultPassphraseWords     = 4
	DefaultPassphraseSeparator = "-"

	// Rotation defaults
	DefaultRotateCount    = 1
	MaxLengthDelta        = 3 // variants can grow or shrink by at most this many chars

	// Scoring thresholds
	ScoreMax             = 100
	ScoreMin             = 0
	WeakThreshold        = 40 // scores below this are "Weak"
	StrongThreshold      = 60 // scores at or above this are "Strong"
	VeryStrongThreshold  = 80 // scores at or above this are "Very Strong"
	BreachScoreCap       = 10 // breached passwords are capped at this score

	// Scoring penalties
	DictionaryPenalty    = 40
	LeetDictionaryPenalty = 30
	SequencePenaltySmall = 8
	SequencePenaltyLarge = 15
	RepeatPenaltySmall   = 10
	RepeatPenaltyLarge   = 20
	KeyboardPenaltySmall = 10
	KeyboardPenaltyLarge = 20

	// Scoring bonuses
	LengthBonusThreshold = 12 // passwords longer than this get a bonus
	LengthBonusMax       = 15
	LengthBonusMultiplier = 2

	// Entropy scaling
	EntropyMultiplier = 0.78
)

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
		Length:    DefaultPasswordLength,
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
		Words:     DefaultPassphraseWords,
		Separator: DefaultPassphraseSeparator,
		Capitalize: true,
		AddNumber:  false,
	}
}

// RotateConfig controls password rotation behavior.
type RotateConfig struct {
	Count        int  // number of variants to produce
	MinLength    int  // minimum variant length (0 = same as base)
	MaxLength    int  // maximum variant length (0 = same as base)
	StrictLength bool // when true, all variants match base length exactly (v1 behavior)
}

// DefaultRotateConfig returns sensible defaults for password rotation.
func DefaultRotateConfig() RotateConfig {
	return RotateConfig{
		Count: DefaultRotateCount,
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
	case score >= VeryStrongThreshold:
		return "Very Strong"
	case score >= StrongThreshold:
		return "Strong"
	case score >= WeakThreshold:
		return "Fair"
	default:
		return "Weak"
	}
}
