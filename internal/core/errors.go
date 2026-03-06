package core

import "errors"

// --- ERROR CONSTANTS ---

var (
	// CLI Errors
	ErrWeak     = errors.New("weak password")
	ErrBreached = errors.New("breached password")

	// Core Errors
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrInvalidConstraint = errors.New("invalid rotation constraint")
	ErrRandFailure       = errors.New("random generation failure")
	ErrNoVariants        = errors.New("could not generate unique variants")
)

// --- ERROR MESSAGES ---

const (
	// Config
	MsgGenLengthTooShort  = "%w: generator length must be at least 1"
	MsgGenNoCharClass     = "%w: at least one character class must be enabled"
	MsgPassphraseNoWords  = "%w: passphrase words must be at least 1"
	MsgRotateCountInvalid = "%w: rotate count must be at least 1"

	// Generator
	MsgErrPasswordLength1  = "password length must be at least 1"
	MsgErrLengthTooShort   = "password length %d is too short to include all %d enabled character classes"
	MsgErrNoCharset        = "no characters available: enable at least one character class"
	MsgErrCryptoRand       = "crypto/rand failed: %w"
	MsgErrPassphraseWords1 = "passphrase must have at least 1 word"
	MsgErrEmptyWordlist    = "wordlist is empty"

	// HIBP
	MsgErrCreatingRequest = "creating request: %w"
	MsgErrHIBPRequest     = "HIBP API request failed: %w"
	MsgErrHIBPStatus      = "HIBP API returned status %d"
	MsgErrHIBPRead        = "reading HIBP response: %w"

	// Rotator
	MsgErrCountMin1         = "%w: count must be at least 1"
	MsgErrBaseEmpty         = "%w: base password must not be empty"
	MsgErrMinExceedsMax     = "%w: min-length (%d) cannot exceed max-length (%d)"
	MsgErrMinLen1           = "%w: min-length must be at least 1"
	MsgErrNoVaryPositions   = "%w: password has no positions that can be varied (need letters or leet-speak characters)"
	MsgErrLimitedMutations  = "%w: could only generate %d unique variants (password has limited mutation points)"
	MsgErrCannotShrink      = "%w: cannot generate shorter variants: password has no redundant repeats to drop"
	MsgErrNoPositions       = "%w: password has no positions that can be varied"
	MsgErrOutsideBounds     = "%w: could not generate any variants within length bounds [%d, %d]"
	MsgErrRequestedVariants = "%w: could only generate %d unique variants (requested %d)"
	MsgErrCannotShrinkMore  = "cannot shrink further"
	MsgErrVariantBounds     = "variant length %d outside bounds [%d, %d]"
	MsgErrNoCharPool        = "no char pool for mutation"
	MsgErrRandFailWrapped   = "%w: %v"
	MsgErrUnsupportedGrowth = "unsupported growth mutation kind: %d"
	MsgErrNoRepeatRuns      = "no repeat runs to drop"

	// CLI
	MsgErrReadingStdin       = "reading stdin: %w"
	MsgErrReadingPassword    = "reading password: %w"
	MsgErrBreachInconclusive = "breach check inconclusive: %w"
	MsgWarnBreachFailed      = "Warning: breach check failed: %v\n"
)
