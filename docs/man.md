# PassForge — Detailed Internal Reference (man)

Line-by-line documentation of every source file in the project.

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

---

## Table of Contents

- [internal/core/config.go](#internalcoreconfiggo)
- [internal/core/generator.go](#internalcoregeneratorgo)
- [internal/core/scorer.go](#internalcorescorergo)
- [internal/core/dictionary.go](#internalcoredictionarygo)
- [internal/core/suggester.go](#internalcoresugestergo)
- [internal/core/hibp.go](#internalcorehibpgo)
- [internal/core/wordlist.go](#internalcorewordlistgo)
- [internal/core/rotator.go](#internalcorerotatorgo)
- [internal/core/errors.go](#internalcoreerrorsgo)
- [cmd/passforge/main.go](#cmdpassforgemain-go)
- [Memory Analysis](#memory-analysis)
- [AI Knowledge Base](#ai-knowledge-base)

---

## `internal/core/config.go`

Defines all configuration structs and result types. No logic — just data shapes.

### Lines 3-11: `GeneratorConfig`

```go
type GeneratorConfig struct {
    Length       int
    Uppercase    bool
    Lowercase    bool
    Digits       bool
    Symbols      bool
    ExcludeChars string
}
```

Controls password generation. Each bool enables/disables a character class. `ExcludeChars` is a string of characters to filter out (e.g., `"0OIl1"` to remove ambiguous characters).

### Lines 13-22: `DefaultGeneratorConfig()`

Returns the default config: 16 characters, all classes enabled, no exclusions. This is what `passforge generate` uses with no flags.

### Lines 24-30: `PassphraseConfig`

```go
type PassphraseConfig struct {
    Words      int
    Separator  string
    Capitalize bool
    AddNumber  bool
}
```

Controls passphrase generation. `AddNumber` appends a random digit to a random word for extra entropy.

### Lines 32-40: `DefaultPassphraseConfig()`

Returns: 4 words, hyphen separator, capitalized, no number suffix.

### `RotateConfig` struct

```go
type RotateConfig struct {
    Count        int  // number of variants to produce
    MinLength    int  // minimum variant length (0 = same as base)
    MaxLength    int  // maximum variant length (0 = same as base)
    StrictLength bool // when true, all variants match base length exactly (v1 behavior)
}
```

Controls rotation variant generation. Zero-value `MinLength`/`MaxLength` means "same as base" (backward compatible). `StrictLength` forces v1 behavior regardless of length bounds.

### `DefaultRotateConfig()`

Returns: count 1, no length constraints, strict-length off.

### Lines 42-50: `ScoreResult`

```go
type ScoreResult struct {
    Score       int      `json:"score"`
    Label       string   `json:"label"`
    Entropy     float64  `json:"entropy"`
    Penalties   []string `json:"penalties"`
    Suggestions []string `json:"suggestions"`
    Breached    bool     `json:"breached"`
}
```

The output of `Score()`. JSON struct tags enable direct marshaling for `--json` output. `Penalties` lists the reasons points were deducted. `Suggestions` lists actionable improvement advice.

### Lines 52-64: `LabelForScore()`

Simple switch statement mapping numeric score to a human label:
- 80+ → "Very Strong"
- 60-79 → "Strong"
- 40-59 → "Fair"
- 0-39 → "Weak"

---

## `internal/core/generator.go`

Handles all random password and passphrase generation.

### Lines 10-15: Character set constants

```go
const (
    uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
    digitChars     = "0123456789"
    symbolChars    = "!@#$%^&*()-_=+[]{}|;:',.<>?/`~"
)
```

The full character pool for each class. `symbolChars` includes 30 printable ASCII symbols. These constants are used by both `buildCharset()` and `ensureClasses()`.

### Lines 17-45: `Generate()`

The main password generation function.

**Line 19:** Validates length >= 1.

**Line 23:** `buildCharset(cfg)` constructs the available characters based on which classes are enabled and which characters are excluded.

**Line 24:** If the charset is empty (all classes disabled, or all chars excluded), return an error.

**Lines 28-35:** Core generation loop. For each position in the password:
1. Call `cryptoRandInt(len(charset))` to get a uniform random index
2. Set `password[i] = charset[idx]`

This uses `crypto/rand`, not `math/rand`. Every character selection is cryptographically uniform.

**Lines 39-42:** `ensureClasses()` is a post-generation fixup. Due to randomness, a 8-char password with 4 enabled classes might not contain all classes. This function checks each enabled class and, if missing, replaces a random position with a character from that class.

### Lines 47-85: `GeneratePassphrase()`

**Line 53:** Loads the EFF wordlist (7776 words, cached after first call).

**Lines 58-69:** For each word position:
1. Pick a random index into the wordlist via `cryptoRandInt`
2. If `Capitalize` is enabled, uppercase the first letter: `strings.ToUpper(word[:1]) + word[1:]`
3. Store in `selected` slice

**Lines 71-82:** If `AddNumber` is enabled:
1. Pick a random word index
2. Pick a random digit 0-9
3. Append the digit to that word: `"Falcon"` → `"Falcon7"`

**Line 84:** Join all words with the configured separator.

### Lines 87-115: `buildCharset()`

Concatenates enabled character class strings into one pool, then filters out any characters in `ExcludeChars`. Uses `strings.Builder` for efficient string construction. Returns `[]byte` since all characters are ASCII.

### Lines 117-148: `ensureClasses()`

Post-generation class guarantee. For each enabled character class:
1. Check if the password already contains at least one character from that class
2. If not, pick a random position and a random character from the missing class
3. Replace `password[pos]` with that character

This ensures the password always satisfies the user's configuration, even for short passwords where random distribution might miss a class.

### Lines 150-157: `containsAnyOf()`

Checks whether any byte in the password exists in the given character string. Used by `ensureClasses()` to detect missing classes.

### Lines 159-165: `cryptoRandInt()`

Wraps `crypto/rand.Int()` to produce a uniform random integer in `[0, max)`. Uses `math/big.NewInt` because the crypto/rand API works with arbitrary-precision integers. This is the single source of randomness for the entire generator.

---

## `internal/core/scorer.go`

The strength scoring engine. Takes a password string, returns a `ScoreResult`.

### Lines 9-22: `leetMap`

Maps leet-speak characters to their letter equivalents:
```
@ → a, 4 → a, 8 → b, ( → c, 3 → e, 6 → g,
# → h, 1 → i, ! → i, | → i, 0 → o, $ → s,
5 → s, 7 → t, + → t, 2 → z
```

Used by `normalizeLeet()` to detect disguised common passwords.

### Lines 24-30: `keyboardRows`

The four main keyboard rows as strings. Used by `keyboardWalkPenalty()` to detect patterns like "qwerty" or "asdf".

### Lines 32-91: `Score()`

The main scoring function. Steps:

1. **Line 34:** Empty password → score 0, label "Weak", done.
2. **Line 38:** Calculate Shannon entropy.
3. **Line 39:** Convert entropy to a 0-100 base score.
4. **Lines 43-55:** Apply pattern penalties (sequences, repeats, keyboard walks). Each returns a point deduction (0 if not detected).
5. **Lines 57-64:** Dictionary check. First check the raw password against the common list (-40 penalty). If not found, normalize leet-speak and check again (-30 penalty). The raw check comes first because it's a stronger match.
6. **Lines 66-69:** Length bonus. Passwords over 12 characters get `+2 * (length - 12)` points, capped at +15.
7. **Lines 72-78:** Clamp score to [0, 100].
8. **Lines 80-90:** Build the `ScoreResult`, generate suggestions, return.

### Lines 93-100: `calculateEntropy()`

Shannon entropy formula: `length * log2(pool_size)`.

`pool_size` is determined by `characterPoolSize()`, which scans the password and tallies which classes are present: lowercase (+26), uppercase (+26), digits (+10), symbols (+32). Maximum pool = 94.

A 16-char password with all classes: `16 * log2(94)` = ~104.8 bits.

### Lines 133-144: `entropyToBaseScore()`

Maps entropy bits to a 0-100 score via `entropy * 0.78`. The 0.78 multiplier means:
- 28 bits (8 lowercase chars) → ~22 points
- 50 bits → ~39 points
- 80 bits → ~62 points
- 128 bits → 100 points (capped)

### Lines 146-175: `sequencePenalty()`

Detects ascending or descending character runs.

**Lines 154-166:** Lowercases the password, then iterates checking if each character is exactly +1 or -1 from the previous (e.g., `a→b→c` or `c→b→a`). Tracks the longest run.

**Lines 168-174:** Penalty thresholds:
- Run of 4+ → -15 points
- Run of 3 → -8 points
- Under 3 → no penalty

### Lines 177-205: `repeatPenalty()`

Same sliding-window approach but checks if consecutive characters are identical.

- Run of 4+ → -20 points
- Run of 3 → -10 points

### Lines 207-234: `keyboardWalkPenalty()`

Checks if the lowercased password contains a substring of 4+ consecutive characters from any keyboard row (forward or reversed).

**Lines 213-232:** For each keyboard row, for each window size from 4 to the row length, slide across and check if that substring appears in the password. Also checks the reversed pattern (e.g., "poiuytrewq").

- Match of 6+ chars → -20 points
- Match of 4-5 chars → -10 points

### Lines 236-247: `normalizeLeet()`

Converts a password to its de-leet-speaked form. Each rune is checked against `leetMap`; if found, replaced with the plain letter. Non-mapped characters are lowercased. Output is all-lowercase.

### Lines 249-255: `reverseString()`

Reverses a string by swapping runes from both ends toward the middle. Used by keyboard walk detection to catch reversed patterns.

---

## `internal/core/dictionary.go`

Manages the common password list for dictionary checking.

### Lines 12-52: `commonPasswordsList`

A Go string slice containing ~200 of the most commonly breached passwords. This is a curated subset — the full SecLists contains millions. Chosen to keep the binary small during development. Includes leet variants like `p@ssw0rd`, `passw0rd`, `pa$$word`.

### Lines 54-57: Package-level variables

```go
var (
    commonPasswordsOnce sync.Once
    commonPasswordsSet  map[string]struct{}
)
```

`sync.Once` ensures the set is built exactly once, even under concurrent access. The `map[string]struct{}` pattern is Go's idiomatic set — `struct{}` takes zero bytes of memory.

### Lines 59-66: `loadCommonPasswords()`

Called by `isCommonPassword()`. Inside `sync.Once.Do()`:
1. Pre-allocate the map with known capacity
2. Insert each password lowercased as a key

### Lines 68-73: `isCommonPassword()`

Calls `loadCommonPasswords()` (no-op after first call), then does a map lookup on the lowercased input. O(1) average time complexity.

---

## `internal/core/suggester.go`

Generates human-readable improvement suggestions.

### Lines 9-56: `Suggest()`

Takes the password string and its `ScoreResult`. Builds a list of suggestions:

**Lines 12-17:** Length checks:
- Under 12 chars → "Increase length to at least 12 characters"
- 12-15 chars → "Consider increasing length to 16+"

**Lines 19-30:** Missing character class checks. Uses `hasCharClass()` with `unicode.IsUpper`, `unicode.IsLower`, `unicode.IsDigit`, and `hasSymbols()` to detect what's absent.

**Lines 32-45:** Penalty-driven suggestions. Iterates the `result.Penalties` slice and maps each penalty string to a specific suggestion. Uses `strings.Contains()` for flexible matching.

**Lines 47-49:** If the password was found in a breach, adds the strongest warning.

**Lines 51-53:** If no other suggestions were generated but the score is still below 80, suggests using a randomly generated password.

### Lines 58-74: Helper functions

`hasCharClass()` — generic rune checker. Takes any `func(rune) bool` (like `unicode.IsUpper`) and returns true if any character matches.

`hasSymbols()` — returns true if any character is not a letter, digit, or space. This catches all punctuation and special characters.

---

## `internal/core/hibp.go`

Have I Been Pwned integration using the k-anonymity range API.

### Lines 12-15: `BreachChecker` interface

```go
type BreachChecker interface {
    IsBreached(password string) (bool, error)
}
```

Abstraction layer so the checker can be swapped for testing or offline mode. Both `HIBPChecker` and `NoOpChecker` implement this.

### Lines 17-28: `HIBPChecker` and `NewHIBPChecker()`

Creates an HTTP client with a 5-second timeout. To prevent DoS attacks from maliciously large responses, the response body is explicitly constrained using `io.LimitReader(resp.Body, 1024*1024)` (1 MiB cap).

### Lines 30-68: `IsBreached()`

The k-anonymity protocol:

**Line 32:** SHA-1 hash the password, format as uppercase hex. Output: 40-char string like `"5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8"`.

**Lines 33-34:** Split into prefix (first 5 chars) and suffix (remaining 35 chars).

**Line 36:** Build the API URL: `https://api.pwnedpasswords.com/range/{prefix}`.

**Lines 37-42:** Create HTTP request with:
- `User-Agent: PassForge-PasswordChecker` (HIBP requires a user agent)
- `Add-Padding: true` (pads response to prevent response-length analysis)

**Lines 44-57:** Execute the request, read the response body.

**Lines 59-65:** Parse the response. Each line is `"SUFFIX:COUNT"` format. Compare each suffix (case-insensitive) to our suffix. If found → password is breached.

**Line 67:** If no match found → password is not in the database.

### Lines 70-75: `NoOpChecker`

Always returns `false, nil`. Used for:
- Tests (avoid hitting the real API)
- Offline mode
- When the user doesn't pass `--breach`

---

## `internal/core/wordlist.go`

Loads and caches the EFF Large Wordlist for passphrase generation.

### Lines 9-10: Embed directive

```go
//go:embed wordlist/eff_large.txt
var embeddedFS embed.FS
```

The `//go:embed` directive tells the Go compiler to include `wordlist/eff_large.txt` in the binary. At runtime, `embeddedFS` provides filesystem-like access to the embedded data. No file I/O required.

### Lines 12-15: Cache variables

```go
var (
    wordlistOnce sync.Once
    wordlist     []string
)
```

`sync.Once` ensures parsing happens exactly once. The `wordlist` slice is the cached result.

### Lines 17-36: `LoadWordlist()`

Inside `sync.Once.Do()`:
1. **Line 20:** Read the embedded file (always succeeds — it's compiled in)
2. **Line 25:** Split by newlines
3. **Lines 27-31:** Parse each line. EFF format is tab-separated: `"11111\tword"`. Split on tab, take the second field, trim whitespace.
4. Pre-allocate the slice with `make([]string, 0, len(lines))` for efficiency.

Returns the cached slice on all subsequent calls.

---

## `internal/core/rotator.go`

The "Same Same But Different" rotation variant engine. Generates unique password variants by cycling leet-speak substitutions, case flips, symbol positions, and length mutations through a base password.

### `reverseLeet` and `buildReverseLeet()`

Inverts the global `leetMap` (from scorer.go) so that plain letters map to their possible leet substitutions. For example, `'a' → ['@', '4']`, `'s' → ['$', '5']`. Built once at package init via `buildReverseLeet()`.

### `Rotate()`

The v1 public API. Signature: `Rotate(base string, count int) ([]string, error)`. Delegates to `RotateWithConfig()` with `StrictLength: true`, preserving backward compatibility. All variants have the same length as the base.

### `RotateWithConfig()`

The v2 public API. Signature: `RotateWithConfig(base string, cfg RotateConfig) ([]string, error)`.

1. **Input validation** — count >= 1, base not empty.
2. **Resolve length bounds** — if `StrictLength` or no length flags set, min/max = base length. Otherwise, clamp to ±`MaxLengthDelta` (3) from base.
3. **Find substitution mutations** — `findMutations()` identifies case-flip and leet-swap positions.
4. **Dispatch** — if variable-length, calls `generateVariableLengthVariants()`. Otherwise calls `generateSubstitutionVariants()` (v1 path).

### `generateSubstitutionVariants()`

The v1 generation path. Uses `applyMutationCycle()` with a `seen` map for dedup. Safety cap: `count * len(mutations) * 4` attempts.

### `generateVariableLengthVariants()`

The v2 generation path. Combines substitution mutations with length mutations:

1. Finds length mutation candidates via `findLengthMutations()`.
2. Checks feasibility (e.g., can't shrink if no repeat runs exist).
3. For each cycle, calls `buildVariableLengthVariant()` which:
   - Decomposes the cycle into a delta choice (grow/shrink/same), a length mutation index, and a substitution cycle
   - Applies substitution mutations first
   - Applies growth (insert/append/prepend) or shrink (drop-repeat) mutations
   - Validates the result length falls within bounds
4. Deduplicates via `seen` map.

### Length mutation types

```go
type lengthMutKind int
const (
    lmInsert     // insert a random char at a position
    lmAppend     // append a symbol/digit to end
    lmPrepend    // prepend a symbol/digit to start
    lmDropRepeat // remove one char from a consecutive repeat run
)
```

`lengthMutation` struct holds the kind, position, and character pool (for growth operations).

### `findLengthMutations()`

Scans the base password and returns candidate length-changing operations:
- **Append/prepend** — always available, pool is `digitChars + symbolChars`
- **Insert candidates** — up to 8 evenly-spaced inter-character positions, pool chosen contextually (digit neighbors → digit pool, symbol neighbors → symbol pool, else lowercase + digits)
- **Drop-repeat candidates** — one per run of 2+ identical consecutive runes

### `applyLengthMutation()`

Applies a single growth mutation (insert, append, or prepend). Uses `cryptoRandInt()` to select a random character from the mutation's char pool. Returns a new rune slice one element longer.

### `applyDropRepeat()`

Finds the first consecutive repeat run and removes one character. Returns a new rune slice one element shorter, or an error if no repeats exist.

### `mutation` struct

```go
type mutation struct {
    pos        int
    original   rune
    alternates []rune
}
```

Represents a single mutable position. `pos` is the index into the rune slice. `original` is the character as it appears in the base. `alternates` is a deduplicated list of replacement runes (case flips, leet forms, or reverse-leet forms).

### `findMutations()`

Scans every rune in the base password and builds a list of `mutation` structs.

For each rune, three mutation sources are checked:
1. **Case flip** — if the rune is a letter, offer the opposite case.
2. **Leet substitution** — if the rune is a letter, look up `reverseLeet` for leet equivalents.
3. **Reverse leet** — if the rune is a leet character (found in `leetMap`), offer the plain letter and its uppercase form.

Each position's alternates are deduplicated via `dedupRunes()` to avoid redundant variants.

### `applyMutationCycle()`

Produces a single variant from a cycle number using **mixed-radix enumeration**. The cycle number is treated as a mixed-radix number where each digit selects the state at a mutation position:
- `choices = len(alternates) + 1` (alternates plus the original)
- `pick = remaining % choices` selects which form to use
- `remaining /= choices` shifts to the next digit

### `dedupRunes()`

Removes duplicates and the original rune from an alternates list. Uses a `map[rune]bool` for O(1) dedup.

### `normalizeBase()`

Converts a password to its plain lowercase form by applying `leetMap` substitutions and lowercasing all letters. Used in tests to verify variants preserve the base structure.

---

## `cmd/passforge/main.go`

CLI entry point. Maps user commands to core library functions.

### Lines 13-34: `main()` and root command

**Line 13:** The global `jsonOutput` flag was refactored out. The CLI now parses flags correctly isolated per subcommand, ensuring secure variable scoping.

**Lines 16-20:** Root command definition. `Use: "passforge"` sets the binary name. `Short` and `Long` are help text.

**Line 22:** `PersistentFlags` makes `--json` available on ALL subcommands (not just root).

**Lines 24-29:** Register all six subcommand constructors: `generate`, `passphrase`, `check`, `suggest`, `rotate`, `bulk`.

**Lines 31-33:** Execute. Cobra handles parsing, routing, help text, and error display. If any subcommand returns an error, we exit with code 1.

### Lines 35-62: `generateCmd()`

**Line 36:** Initialize a default config. Cobra flags will mutate this struct's fields directly via pointers.

**Lines 41-51:** The run function: call `core.Generate(cfg)`, print the result as plain text or JSON.

**Lines 54-59:** Flag bindings. `IntVarP(&cfg.Length, "length", "l", ...)` means:
- `&cfg.Length` — pointer to the field to set
- `"length"` — long flag name (`--length`)
- `"l"` — short flag name (`-l`)
- `cfg.Length` — default value (16)

### Lines 64-89: `passphraseCmd()`

Same pattern. Flags: `--words`/`-w`, `--separator`/`-s`, `--capitalize`, `--number`.

### Lines 91-149: `checkCmd()`

**Line 92:** Local `breachCheck` bool for the `--breach` flag.

**Line 100:** Score the password using the core library.

**Lines 102-114:** If `--breach` is enabled, create an `HIBPChecker` and check. On breach hit:
- Set `result.Breached = true`
- Cap score at 10 (via `min()`)
- Re-derive the label
- Append penalty and suggestion

**Lines 116-133:** Output formatting. Plain text shows score, entropy, issues, and suggestions. JSON mode uses `printJSON()`.

**Lines 135-141:** Exit codes for scripting:
- Returns `core.ErrBreached` (Exit 2) if breached
- Returns `core.ErrWeak` (Exit 1) if score < 40 (Weak)
- Returns any operational failure directly as error (Exit 3)
- Implicit Exit 0 if perfectly strong/acceptable

### Lines 151-182: `suggestCmd()`

Scores the password, then displays only the score and suggestions (no entropy or penalties). Lighter output for quick advice.

### `rotateCmd()`

The "Same Same But Different" CLI command. Aliased as `ssbd`.

Initializes a `core.DefaultRotateConfig()` and binds flags:
- `--count`/`-n` — number of variants to generate
- `--min-length` — minimum variant length (0 = same as input)
- `--max-length` — maximum variant length (0 = same as input)
- `--strict-length` — force all variants to match input length exactly

Calls `core.RotateWithConfig(password, cfg)` to generate variants. JSON output includes both the base password and variants array. Plain text numbers each variant: `1: P@sswor4`, `2: pAs$wor4`, etc.

### Lines 220-257: `bulkCmd()`

**Line 186:** `count` variable for `--count`/`-n` flag (default 10).

**Lines 191-199:** Generate loop — calls `core.Generate(cfg)` N times, collecting into a slice.

**Lines 201-208:** Output as JSON array or one-per-line plain text.

### Lines 223-227: `printJSON()`

Utility function. Creates a `json.NewEncoder` on stdout with 2-space indentation. Used by all subcommands when `--json` is set.

---

## `internal/core/errors.go`

Centralizes all error definitions and formatting message templates to avoid literal duplication.

### Lines 7-17: `Err...` Constants

Defines standard root errors the core library and CLI will match against using `errors.Is`.
- `ErrWeak` / `ErrBreached` — Used by CLI exit routing.
- `ErrInvalidConfig` / `ErrInvalidConstraint` / `ErrRandFailure` / `ErrNoVariants` — Core logic failures.

### Lines 21-65: `Msg...` Constants

String formatting templates used with `fmt.Errorf`. Centralizing these allows identical error messages across the framework, prevents redeclaration issues, and eases future internationalization. Examples include `MsgErrLengthTooShort`, `MsgErrCryptoRand`, and `MsgErrLimitedMutations`.

---
---

# Memory Analysis

Detailed memory layout, allocation patterns, and per-line cost analysis for every struct, constant, variable, and function in PassForge. All sizes assume 64-bit architecture (Go `GOARCH=amd64`). Sizes verified against Go's alignment rules: structs are padded to align each field on its natural boundary, and the struct itself is padded to a multiple of the largest field's alignment.

---

## `internal/core/config.go` — Memory Analysis

### `GeneratorConfig` struct (lines 4-11)

```go
type GeneratorConfig struct {
    Length       int       // 8 bytes (offset 0)
    Uppercase    bool      // 1 byte  (offset 8)
    Lowercase    bool      // 1 byte  (offset 9)
    Digits       bool      // 1 byte  (offset 10)
    Symbols      bool      // 1 byte  (offset 11)
    // 4 bytes padding                (offset 12-15)
    ExcludeChars string    // 16 bytes (offset 16): pointer (8) + length (8)
}
```

| Field | Type | Size | Offset | Notes |
|---|---|---|---|---|
| `Length` | `int` | 8 B | 0 | Default: 16 |
| `Uppercase` | `bool` | 1 B | 8 | |
| `Lowercase` | `bool` | 1 B | 9 | |
| `Digits` | `bool` | 1 B | 10 | |
| `Symbols` | `bool` | 1 B | 11 | |
| *padding* | — | 4 B | 12 | Aligns `ExcludeChars` to 8-byte boundary |
| `ExcludeChars` | `string` | 16 B | 16 | String header: ptr + len |
| **Total** | | **32 B** | | Stack-allocated when passed by value |

**Allocation pattern:** `DefaultGeneratorConfig()` returns by value — no heap allocation. The struct is typically created on the stack in `main.go` and mutated via Cobra flag pointers.

### `PassphraseConfig` struct (lines 25-30)

```go
type PassphraseConfig struct {
    Words      int       // 8 bytes (offset 0)
    Separator  string    // 16 bytes (offset 8): pointer (8) + length (8)
    Capitalize bool      // 1 byte  (offset 24)
    AddNumber  bool      // 1 byte  (offset 25)
    // 6 bytes padding                (offset 26-31)
}
```

| Field | Type | Size | Offset | Notes |
|---|---|---|---|---|
| `Words` | `int` | 8 B | 0 | Default: 4 |
| `Separator` | `string` | 16 B | 8 | Default: `"-"` (1 byte backing) |
| `Capitalize` | `bool` | 1 B | 24 | |
| `AddNumber` | `bool` | 1 B | 25 | |
| *padding* | — | 6 B | 26 | Pad to 8-byte alignment |
| **Total** | | **32 B** | | Stack-allocated |

### `ScoreResult` struct (lines 43-50)

```go
type ScoreResult struct {
    Score       int       // 8 bytes  (offset 0)
    Label       string    // 16 bytes (offset 8)
    Entropy     float64   // 8 bytes  (offset 24)
    Penalties   []string  // 24 bytes (offset 32): ptr (8) + len (8) + cap (8)
    Suggestions []string  // 24 bytes (offset 56): ptr (8) + len (8) + cap (8)
    Breached    bool      // 1 byte   (offset 80)
    // 7 bytes padding                (offset 81-87)
}
```

| Field | Type | Size | Offset | Notes |
|---|---|---|---|---|
| `Score` | `int` | 8 B | 0 | 0-100, clamped |
| `Label` | `string` | 16 B | 8 | "Weak" / "Fair" / "Strong" / "Very Strong" |
| `Entropy` | `float64` | 8 B | 24 | Shannon entropy in bits |
| `Penalties` | `[]string` | 24 B | 32 | Slice header; backing array on heap |
| `Suggestions` | `[]string` | 24 B | 56 | Slice header; backing array on heap |
| `Breached` | `bool` | 1 B | 80 | |
| *padding* | — | 7 B | 81 | Pad to 8-byte alignment |
| **Total** | | **88 B** | | Header only; slice backing arrays are additional |

**Heap cost:** Each `Penalties`/`Suggestions` entry is a string header (16 B) pointing to a backing string. A typical ScoreResult with 3 penalties and 4 suggestions allocates ~112 B of string headers on the heap, plus the string data itself.

### `RotateConfig` struct

```go
type RotateConfig struct {
    Count        int   // 8 bytes (offset 0)
    MinLength    int   // 8 bytes (offset 8)
    MaxLength    int   // 8 bytes (offset 16)
    StrictLength bool  // 1 byte  (offset 24)
    // 7 bytes padding            (offset 25-31)
}
```

| Field | Type | Size | Offset |
|---|---|---|---|
| `Count` | `int` | 8 B | 0 |
| `MinLength` | `int` | 8 B | 8 |
| `MaxLength` | `int` | 8 B | 16 |
| `StrictLength` | `bool` | 1 B | 24 |
| *padding* | — | 7 B | 25 |
| **Total** | | **32 B** | Stack-allocated |

### `LabelForScore()` (lines 53-64)

Zero allocation. Returns string literals (which live in the read-only data segment). The switch statement compiles to at most 3 comparisons.

---

## `internal/core/generator.go` — Memory Analysis

### Constants (lines 10-15)

```go
const (
    uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"  // 26 bytes in rodata
    lowercaseChars = "abcdefghijklmnopqrstuvwxyz"  // 26 bytes in rodata
    digitChars     = "0123456789"                   // 10 bytes in rodata
    symbolChars    = "!@#$%^&*()-_=+[]{}|;:',.<>?/`~"  // 30 bytes in rodata
)
```

**Total rodata:** 92 bytes. String constants are stored in the binary's read-only data segment. No runtime allocation.

### `Generate()` (lines 18-66)

Per-call allocations:

| Allocation | Size | Location | Notes |
|---|---|---|---|
| `enabledClassChars(cfg)` | ~4 strings | heap | Filtered class strings |
| `randomPermutation(len, n)` | `cfg.Length * 8` B | heap | `[]int` for Fisher-Yates shuffle |
| `reserved` map | ~`n * 16` B | heap | `map[int]bool` for reserved positions |
| `password` `[]byte` | `cfg.Length` B | heap | The output buffer |
| `string(password)` | `cfg.Length` B | heap | Final string copy |

**For a 16-char password (default):** ~128 B heap for the shuffle indices, ~32 B for reserved map overhead, 16 B for the byte slice, 16 B for the string = ~200 B total heap. Plus ~4 `crypto/rand.Int()` calls each allocating a `*big.Int` (~40 B each).

**Hot path:** `cryptoRandInt()` is the most-called function — once per password character plus class guarantees. Each call allocates a `big.Int` on the heap. For a 16-char password: ~20 calls = ~800 B of `big.Int` allocations (short-lived, collected next GC).

### `GeneratePassphrase()` (lines 68-106)

| Allocation | Size | Notes |
|---|---|---|
| `selected` `[]string` | `cfg.Words * 16` B | Slice of string headers |
| `strings.ToUpper(word[:1]) + word[1:]` | ~avg 7 B per word | New string per capitalized word |
| `fmt.Sprintf("%s%d", ...)` | ~8-12 B | Only if `AddNumber` is true |
| `strings.Join(...)` | total passphrase length | Final output string |

**For 4-word passphrase (default):** ~128 B heap total (word selection + join). Wordlist itself is loaded once and reused (see wordlist.go).

### `buildCharset()` (lines 108-136)

Uses `strings.Builder` internally. For default config (all classes): builds a ~92-char string, then converts to `[]byte`. Heap: ~184 B (builder buffer + byte slice copy). If `ExcludeChars` is set, a second builder is used for filtering: additional ~92 B.

### `cryptoRandInt()` (lines 198-204)

Each call allocates:
- `big.NewInt(int64(max))` — 40 B on heap (big.Int struct + internal limbs)
- `rand.Int()` return value — 40 B on heap

Both are short-lived and collected by GC almost immediately. This is the single most allocation-heavy function in the project per call, but total cost per password generation is modest.

---

## `internal/core/scorer.go` — Memory Analysis

### `leetMap` (lines 10-22)

```go
var leetMap = map[rune]rune{ ... }  // 17 entries
```

**Memory:** Go maps use hash tables with 8-element buckets. 17 entries = 3 buckets. Each bucket is ~208 B (keys + values + overflow pointer + tophash). Total: ~700 B heap, allocated once at package init.

`rune` is an alias for `int32` (4 bytes). Key-value pair: 8 bytes per entry, but map overhead dominates.

### `keyboardRows` (lines 24-30)

```go
var keyboardRows = []string{ ... }  // 4 strings
```

**Memory:** Slice header (24 B) + backing array of 4 string headers (64 B) = 88 B heap. The string data itself ("qwertyuiop", etc.) is in rodata (35 bytes total).

### `Score()` (lines 33-91)

Per-call allocations:

| Line | Allocation | Size | Notes |
|---|---|---|---|
| 41 | `var penalties []string` | 0 B initial | Nil slice, grows on append |
| 44-55 | `penalties = append(...)` | 0-5 strings | Typical: 0-3 appends, each 16 B header |
| 88 | `Suggest(password, result)` | see suggester | Returns `[]string` |

**Penalty functions** (`sequencePenalty`, `repeatPenalty`, `keyboardWalkPenalty`) each allocate:
- `strings.ToLower(password)`: one string copy (~password length)
- `[]rune(password)` or `[]rune(strings.ToLower(...))`: 4 bytes per rune

**Typical Score() call for a 16-char password:** ~400 B heap (lowercase copies, rune slices, penalties, suggestions). No allocations in the scoring math itself.

### `calculateEntropy()` (lines 93-100)

Zero heap allocation. `characterPoolSize()` scans runes in-place, returns an int. `math.Log2()` is pure computation.

### `characterPoolSize()` (lines 102-131)

Zero allocation. Iterates the string's runes via `range` (no conversion needed), sets 4 booleans, returns an int.

### `entropyToBaseScore()` (lines 133-144)

Zero allocation. Pure arithmetic: multiply + cast + compare.

### `sequencePenalty()` (lines 146-175)

| Allocation | Size | Notes |
|---|---|---|
| `[]rune(strings.ToLower(password))` | `len * 4` B | Rune slice for comparison |
| `strings.ToLower()` | `len` B | Lowercase copy |

**For 16-char password:** 64 B (rune slice) + 16 B (lowercase string) = 80 B heap.

### `repeatPenalty()` (lines 177-205)

| Allocation | Size | Notes |
|---|---|---|
| `[]rune(password)` | `len * 4` B | Rune conversion |

**For 16-char password:** 64 B heap. No lowercase needed (compares original runes).

### `keyboardWalkPenalty()` (lines 207-234)

| Allocation | Size | Notes |
|---|---|---|
| `strings.ToLower(password)` | `len` B | One lowercase copy |
| `reverseString(pattern)` | `pattern_len * 4` B | Per pattern checked |

**Worst case:** Checks all window sizes across 4 rows. For a 16-char password, typically generates ~40 reversed patterns (4-10 chars each). Total: ~16 B (lowercase) + ~300 B (reversed patterns) = ~316 B heap. **Most reversed patterns are short-lived and collected quickly.**

**Optimization note:** This is O(rows * sum(row_len - w + 1) for w in 4..row_len), but with early return on first match. Worst case (no match found) iterates ~120 substrings.

### `normalizeLeet()` (lines 236-247)

| Allocation | Size | Notes |
|---|---|---|
| `strings.Builder` internal buffer | `len` B | Grows as runes are written |
| Final `sb.String()` | `len` B | Copy of builder buffer |

**For 16-char password:** ~32 B heap.

### `reverseString()` (lines 249-255)

| Allocation | Size | Notes |
|---|---|---|
| `[]rune(s)` | `len * 4` B | Rune conversion |
| `string(runes)` | `len` B | Back to string |

---

## `internal/core/dictionary.go` — Memory Analysis

### `commonPasswordsList` (lines 12-52)

```go
var commonPasswordsList = []string{ ... }  // ~200 entries
```

**Memory:** Slice header (24 B) + backing array of ~200 string headers (3,200 B) = ~3,224 B heap. The string data itself averages ~8 bytes per password = ~1,600 B in rodata. **Total: ~4,824 B.**

This slice is kept alive for the lifetime of the program (package-level variable).

### `commonPasswordsSet` (lines 54-57)

```go
var commonPasswordsSet map[string]struct{}
```

**After `loadCommonPasswords()` is called:**

Go maps with `struct{}` values are optimized — the value takes 0 bytes. Each bucket holds up to 8 entries. For ~200 entries: ~25 buckets.

| Component | Size | Notes |
|---|---|---|
| Map header | 8 B | Pointer to `hmap` struct |
| `hmap` struct | 48 B | Count, bucket count, hash seed, etc. |
| Buckets (25) | ~5,200 B | 8 tophash + 8 key headers per bucket |
| Key string data | ~1,600 B | Lowercased copies of passwords |
| **Total** | **~6,856 B** | One-time cost |

`sync.Once` overhead: 12 B (uint32 `done` + Mutex).

**Combined dictionary memory:** ~11,700 B (~11.4 KB). Allocated once, never freed.

### `isCommonPassword()` (lines 68-73)

Per-call: `strings.ToLower(password)` allocates one string copy (~password length). Map lookup itself is O(1) with no allocation.

---

## `internal/core/suggester.go` — Memory Analysis

### `Suggest()` (lines 9-56)

| Allocation | Size | Notes |
|---|---|---|
| `var suggestions []string` | 0 B initial | Nil slice |
| Each `append(...)` | 16 B per entry | String header; literals are in rodata |
| Slice growth | varies | Go doubles capacity; typical: 0→1→2→4→8 |

**Typical call (3-4 suggestions):** Initial nil slice → first append allocates backing array of 1 (16 B) → grows to 2 (32 B) → grows to 4 (64 B). Total: ~64 B for headers + the suggestion strings themselves are string literals (zero-cost, rodata).

### `hasCharClass()` (lines 58-65)

Zero allocation. Iterates runes in-place, calls the provided `func(rune) bool`.

### `hasSymbols()` (lines 67-74)

Zero allocation. Same pattern as `hasCharClass()`.

---

## `internal/core/hibp.go` — Memory Analysis

### `HIBPChecker` struct (lines 19-21)

```go
type HIBPChecker struct {
    Client *http.Client  // 8 bytes (pointer)
}
```

**Total struct:** 8 B. The `http.Client` itself is ~80 B (Transport pointer, Timeout, Jar pointer, etc.), allocated separately on heap.

### `NewHIBPChecker()` (lines 24-28)

Allocates:
- `HIBPChecker` struct: 8 B
- `http.Client` struct: ~80 B
- `http.Transport` (default, shared): 0 B (uses `http.DefaultTransport`)

**Total:** ~88 B heap per checker instance.

### `IsBreached()` (lines 31-68)

Per-call allocations:

| Line | Allocation | Size | Notes |
|---|---|---|---|
| 32 | `sha1.Sum([]byte(password))` | `len` B | `[]byte` conversion; SHA-1 state is stack-allocated (96 B) |
| 32 | `fmt.Sprintf("%X", ...)` | 40 B | Hex-encoded hash string |
| 36 | URL string concatenation | ~50 B | `"https://.../" + prefix` |
| 37 | `http.NewRequest(...)` | ~400 B | Request struct + headers |
| 54 | `io.ReadAll(resp.Body)` | **~30 KB** | HIBP returns ~800 hash suffixes with padding |
| 59 | `strings.Split(string(body), "\n")` | **~30 KB** | String conversion + slice of ~800 string headers |
| 61 | `strings.SplitN(...)` per line | ~32 B × 800 | Per-line split |

**Total per breach check:** ~90 KB heap (dominated by the HIBP API response body). This is a network-bound operation — the allocation cost is negligible compared to the HTTP round-trip latency (~200-500ms).

### `NoOpChecker` (lines 71-75)

```go
type NoOpChecker struct{}  // 0 bytes
```

Zero-size struct. `&NoOpChecker{}` allocates 0 bytes (Go optimizes zero-size allocations to a single global pointer). `IsBreached()` returns immediately with no allocations.

---

## `internal/core/wordlist.go` — Memory Analysis

### Embedded data (lines 9-10)

```go
//go:embed wordlist/eff_large.txt
var embeddedFS embed.FS
```

**Binary size impact:** The EFF Large Wordlist is ~85 KB of text. This is added to the compiled binary at build time. The `embed.FS` struct itself is ~32 B (interface-like header pointing into the embedded data section).

### Cache variables (lines 12-15)

| Variable | Type | Size | Notes |
|---|---|---|---|
| `wordlistOnce` | `sync.Once` | 12 B | `uint32` + `Mutex` |
| `wordlist` | `[]string` | 24 B | Slice header (nil until loaded) |

### `LoadWordlist()` (lines 17-36)

One-time allocations (inside `sync.Once.Do`):

| Step | Allocation | Size | Notes |
|---|---|---|---|
| `embeddedFS.ReadFile(...)` | ~85 KB | Copies embedded data to heap |
| `string(data)` | ~85 KB | Byte-to-string conversion |
| `strings.Split(...)` | ~7,776 string headers | ~124 KB | One header per line |
| `strings.SplitN(line, "\t", 2)` per line | ~32 B × 7,776 | ~249 KB | Per-line tab split |
| `strings.TrimSpace(parts[1])` per word | ~7 B × 7,776 | ~54 KB | Word string copies |
| `wordlist` backing array | 7,776 × 16 B | ~124 KB | String headers |

**Total one-time cost:** ~721 KB heap. After loading, the intermediate strings (full file, split lines) become garbage and are collected by GC. **Steady-state retained:** ~178 KB (wordlist slice + 7,776 word strings).

**Optimization opportunity:** The current implementation creates many intermediate allocations. A single-pass parser that indexes into the embedded data directly (without copying) could reduce steady-state to ~124 KB (just the string headers, pointing into the embedded rodata). However, the current approach is simpler and the cost is a one-time ~721 KB spike.

---

## `internal/core/rotator.go` — Memory Analysis

### `reverseLeet`

```go
var reverseLeet = buildReverseLeet()  // map[rune][]rune
```

Built from the 17-entry `leetMap`. The reverse map has fewer keys (unique plain letters): `a, b, c, e, g, h, i, o, s, t, z` = 11 keys.

| Component | Size | Notes |
|---|---|---|
| Map header + hmap | 56 B | |
| Buckets (2) | ~416 B | 11 entries across 2 buckets |
| Value slices (11) | 11 × 24 B = 264 B | Slice headers |
| Rune backing arrays | ~80 B | 1-3 runes per key, 4 B each |
| **Total** | **~816 B** | One-time, package init |

### `mutation` struct

```go
type mutation struct {
    pos        int       // 8 bytes  (offset 0)
    original   rune      // 4 bytes  (offset 8)
    // 4 bytes padding              (offset 12)
    alternates []rune    // 24 bytes (offset 16): ptr (8) + len (8) + cap (8)
}
```

| Field | Type | Size | Offset |
|---|---|---|---|
| `pos` | `int` | 8 B | 0 |
| `original` | `rune` (`int32`) | 4 B | 8 |
| *padding* | — | 4 B | 12 |
| `alternates` | `[]rune` | 24 B | 16 |
| **Total** | | **40 B** | |

### `lengthMutation` struct

```go
type lengthMutation struct {
    kind     lengthMutKind // 8 bytes  (offset 0) — int-sized enum
    pos      int           // 8 bytes  (offset 8)
    charPool string        // 16 bytes (offset 16): ptr (8) + len (8)
}
```

| Field | Type | Size | Offset |
|---|---|---|---|
| `kind` | `lengthMutKind` (`int`) | 8 B | 0 |
| `pos` | `int` | 8 B | 8 |
| `charPool` | `string` | 16 B | 16 |
| **Total** | | **32 B** | |

### `Rotate()` — v1 wrapper

Delegates to `RotateWithConfig()` with `StrictLength: true`. Allocates one `RotateConfig` (32 B, stack).

### `RotateWithConfig()` — v2 entry point

Per-call allocations (substitution-only path, same as v1):

| Allocation | Size | Notes |
|---|---|---|
| `[]rune(base)` | `len * 4` B | Rune conversion of base |
| `findMutations()` | `n * 40` B | `n` mutation structs (typically 4-10) |
| `seen` map | varies | `map[string]bool`, grows with variants |
| `variants` slice | `count * 16` B | String headers |
| `applyMutationCycle()` per cycle | `len * 4` B | New rune slice per variant |
| `string(variant)` per cycle | `len` B | String from runes |

**For `Rotate("p@sSwor4", 5)`:** ~1 KB heap.

Additional allocations for variable-length path:

| Allocation | Size | Notes |
|---|---|---|
| `findLengthMutations()` | `m * 32` B | `m` length mutation structs (typically 5-15) |
| `applyLengthMutation()` per growth | `(len+1) * 4` B | New rune slice, one longer |
| `applyDropRepeat()` per shrink | `(len-1) * 4` B | New rune slice, one shorter |
| `cryptoRandInt()` per growth | ~80 B | `big.Int` allocation |

**For `RotateWithConfig("p@sSwor4", {Count: 10, MinLength: 8, MaxLength: 11})`:** ~2-3 KB heap (more cycles attempted, more rune slice allocations).

### `findMutations()`

Allocates a `[]mutation` slice (up to one per rune) and per-mutation `[]rune` alternates. For an 8-char password with 6 mutable positions: ~340 B.

### `findLengthMutations()`

Allocates a `[]lengthMutation` slice. For an 8-char password: 2 (append/prepend) + ~5 (inserts) + 0-3 (drop candidates) = ~7-10 entries × 32 B = ~224-320 B.

### `applyMutationCycle()`

Allocates one `[]rune` copy of the base (len × 4 B). Pure arithmetic for the mixed-radix selection.

### `applyLengthMutation()`

Allocates a new `[]rune` slice one element longer than input. One `cryptoRandInt()` call (~80 B for `big.Int`).

### `applyDropRepeat()`

Allocates a new `[]rune` slice one element shorter than input. No randomness needed.

### `dedupRunes()`

Allocates a `map[rune]bool` (~100 B for small sets) and a `[]rune` result (~16-32 B). Short-lived.

### `normalizeBase()`

Uses `strings.Builder`. Allocates ~len bytes for the output string. Used in tests to verify variants preserve base structure.

---

## `cmd/passforge/main.go` — Memory Analysis

### Package-level variables

*Refactored out.* The CLI previously used a global `jsonOutput` bool, but this was removed to improve memory hygiene and testability.

### `main()` (lines 15-34)

| Allocation | Size | Notes |
|---|---|---|
| `cobra.Command` root | ~600 B | Cobra command struct + internal maps |
| 6 × subcommand | ~3,600 B | Each subcommand is ~600 B |
| Flag sets | ~2,000 B | ~200 B per flag × ~10 flags |
| **Total startup** | **~6,200 B** | One-time CLI setup |

### `printJSON()` (lines 259-263)

| Allocation | Size | Notes |
|---|---|---|
| `json.NewEncoder(os.Stdout)` | ~100 B | Encoder struct with buffer |
| `enc.Encode(v)` | varies | Reflection-based marshaling; allocates internal buffers |

Typical JSON encoding of a `ScoreResult`: ~500 B heap (reflection metadata + output buffer).

---

## Summary — Total Memory Footprint

### Baseline (program startup, before any command)

| Component | Size | Notes |
|---|---|---|
| Go runtime | ~4 MB | Goroutine stacks, GC metadata, type info |
| Cobra + flags | ~6 KB | Command tree setup |
| `leetMap` | ~700 B | 17-entry map |
| `keyboardRows` | ~88 B | 4 strings |
| `reverseLeet` | ~816 B | 11-entry map with rune slices |
| `commonPasswordsList` | ~4.8 KB | 200 string headers + rodata |
| String constants (rodata) | ~92 B | Character class strings |
| **Total baseline** | **~4.01 MB** | Dominated by Go runtime |

### After first use

| Component | Additional Cost | Notes |
|---|---|---|
| Common passwords set | ~6.9 KB | On first `Score()` call |
| EFF wordlist | ~178 KB (steady) | On first `GeneratePassphrase()` call; ~721 KB peak during loading |
| **Total with all caches warm** | **~4.20 MB** | |

### Per-operation costs

| Operation | Heap per call | Notes |
|---|---|---|
| `Generate()` (16-char) | ~1 KB | Dominated by crypto/rand big.Int allocations |
| `GeneratePassphrase()` (4-word) | ~200 B | Reuses cached wordlist |
| `Score()` (16-char) | ~500 B | Rune conversions + penalties + suggestions |
| `Rotate()` (8-char, 5 variants) | ~1 KB | Mutations + variant strings |
| `RotateWithConfig()` (8-char, 10 var-length) | ~2-3 KB | + length mutations + growth rune slices |
| `IsBreached()` | ~90 KB | Dominated by HIBP API response body |

All per-operation allocations are short-lived and collected by Go's GC within the same or next GC cycle. No memory leaks detected.

---
---

# AI Knowledge Base

> **Audience:** AI coding assistants working on the PassForge codebase.
> This section is a self-contained orientation guide — project identity, architecture, conventions, and operational details.

---

## Project Identity

| Key | Value |
|---|---|
| **Name** | PassForge |
| **Module** | `github.com/passforge/passforge` |
| **Go version** | 1.26.0 |
| **CLI framework** | [Cobra](https://github.com/spf13/cobra) v1.10.2 |
| **Randomness** | `crypto/rand` (cryptographically secure) — never `math/rand` |
| **Signature feature** | **SSBD** — "Same Same But Different" password rotation engine |
| **Binary name** | `passforge` |
| **License** | See repository root |

---

## Directory Layout

```
passforge/
├── cmd/passforge/
│   └── main.go              # CLI entry point (Cobra commands)
├── internal/core/
│   ├── config.go            # All structs, constants, defaults
│   ├── generator.go         # Password & passphrase generation
│   ├── scorer.go            # Strength scoring engine
│   ├── dictionary.go        # Common password list (sync.Once)
│   ├── suggester.go         # Improvement suggestions
│   ├── hibp.go              # HIBP k-anonymity breach checker
│   ├── errors.go            # Centralized messaging & typed errors
│   ├── wordlist.go          # EFF Large Wordlist loader (embed.FS)
│   ├── rotator.go           # SSBD rotation variant engine
│   ├── wordlist/
│   │   └── eff_large.txt    # 7776-word EFF diceware list (embedded)
│   ├── *_test.go            # Unit tests for each module
├── docs/
│   ├── man.md               # This file — detailed internal reference
│   ├── arch.md              # Architecture overview
│   ├── help.md              # CLI help documentation
│   ├── help_ext.md          # Extended help documentation
│   └── PLAN.md              # Development roadmap
├── Makefile                  # Build, test, run, clean targets
├── README.md                 # User-facing project overview
├── WORKPLAN.md               # Work planning document
├── go.mod / go.sum           # Go module files
└── .gitignore
```

---

## Dependency Graph

```
cmd/passforge/main.go
  └── internal/core/
        ├── config.go        ← structs & constants (imported by all)
        ├── generator.go     ← uses config, wordlist, crypto/rand
        ├── scorer.go        ← uses config, dictionary
        ├── dictionary.go    ← standalone (sync.Once + map)
        ├── suggester.go     ← uses config, scorer results
        ├── errors.go        ← central error logic
        ├── hibp.go          ← standalone (net/http, SHA-1)
        ├── wordlist.go      ← standalone (embed.FS, sync.Once)
        └── rotator.go       ← uses config, scorer (leetMap), generator (cryptoRandInt), crypto/rand
```

**Key rule:** All core logic lives in `internal/core/`. The CLI layer (`cmd/passforge/main.go`) is a thin Cobra wrapper — no business logic belongs there.

---

## CLI Commands & Flag Mapping

| Command | Aliases | Core Function | Key Flags |
|---|---|---|---|
| `generate` | — | `core.Generate(cfg)` | `--length/-l`, `--upper`, `--lower`, `--digits`, `--symbols`, `--exclude` |
| `passphrase` | — | `core.GeneratePassphrase(cfg)` | `--words/-w`, `--separator/-s`, `--capitalize`, `--number` |
| `check` | — | `core.Score(pw)` | `--breach` (enables HIBP check) |
| `suggest` | — | `core.Score(pw)` + display suggestions | — |
| `rotate` | `ssbd` | `core.RotateWithConfig(pw, cfg)` | `--count/-n`, `--min-length`, `--max-length`, `--strict-length` |
| `bulk` | — | `core.Generate(cfg)` × N | `--count/-n`, all generate flags |

**Global flag:** `--json` on all commands — outputs structured JSON via `printJSON()`.

**Exit codes** (`check` command only):
- `0` — password is acceptable
- `1` — score < 40 (Weak)
- `2` — password found in HIBP breach database
- `3` — operational error or invalid input

---

## Scoring Algorithm Quick Reference

### Base score
`entropy_bits × 0.78` (capped at 100). Entropy = `length × log2(pool_size)`.

### Pool sizes
| Class | Size |
|---|---|
| Lowercase | 26 |
| Uppercase | 26 |
| Digits | 10 |
| Symbols | 32 |
| **Max pool** | **94** |

### Penalties

| Pattern | Threshold | Penalty |
|---|---|---|
| Dictionary match (exact) | — | −40 |
| Dictionary match (leet-normalized) | — | −30 |
| Sequential chars (ascending/descending) | run ≥ 4 | −15 |
| Sequential chars | run = 3 | −8 |
| Repeated chars | run ≥ 4 | −20 |
| Repeated chars | run = 3 | −10 |
| Keyboard walk | match ≥ 6 | −20 |
| Keyboard walk | match 4–5 | −10 |

### Bonuses
- Length > 12: `+2 × (length − 12)`, max +15

### Labels
| Score Range | Label |
|---|---|
| 80–100 | Very Strong |
| 60–79 | Strong |
| 40–59 | Fair |
| 0–39 | Weak |

### Breach override
If breached: score capped at 10, label becomes "Weak".

---

## Makefile Targets

```
make help          # Show all targets
make build         # go build -o passforge ./cmd/passforge
make test          # go test -v ./...
make bench         # go test -bench=. -benchmem ./...
make vet           # go vet ./...
make fmt           # go fmt ./...
make cover         # go test -cover ./...
make all           # vet + test + bench
make all-clean     # clean + all (full reset & verify)
make clean         # go clean --cache && rm -f passforge

# Run commands (examples):
make generate ARGS="--length 20"
make passphrase ARGS="--words 4"
make check ARGS="MyP@ssw0rd"
make suggest ARGS="hello123"
make rotate ARGS="p@sSwor4 --count 5"
make ssbd ARGS="p@sSwor4 --count 5"
make bulk ARGS="--count 10 --length 16"
```

---

## Design Patterns & Conventions

### Caching pattern
Both `dictionary.go` and `wordlist.go` use `sync.Once` for lazy, thread-safe initialization.
The pattern is: package-level `var xxxOnce sync.Once` + `var xxx <type>` + a `loadXxx()` function.

### Interface abstraction
`BreachChecker` interface in `hibp.go` enables test doubles (`NoOpChecker`) without mocking.

### `crypto/rand` only
All randomness flows through `cryptoRandInt()` in `generator.go`. Uses `crypto/rand.Int()` with `*big.Int`. Never use `math/rand` anywhere.

### Embedded assets
The EFF wordlist is compiled into the binary via `//go:embed`. No runtime file I/O for wordlist access.

### Rotation engine (SSBD)
- **v1 API:** `Rotate(base, count)` — strict same-length variants via `StrictLength: true`
- **v2 API:** `RotateWithConfig(base, cfg)` — supports variable-length via `MinLength`/`MaxLength`
- Uses **mixed-radix enumeration** to cycle through mutation combinations deterministically
- Length mutations: insert, append, prepend (grow) or drop-repeat (shrink), bounded by `MaxLengthDelta` (±3)

### Config naming
All tunable constants live in the `const` block at the top of `config.go`. Structs use `XxxConfig` naming with a `DefaultXxxConfig()` factory function.

### Testing patterns
- Tests live alongside source files as `*_test.go` in the same package
- Table-driven tests (Go idiomatic `[]struct{ name; ... }` + `t.Run`)
- Breach checker tests use `NoOpChecker` to avoid network calls
- Generator tests use large samples (length 100+) to verify statistical properties

---

## Common Pitfalls

1. **Don't add logic to `main.go`** — it's a thin Cobra wrapper. All business logic goes in `internal/core/`.
2. **Don't use `math/rand`** — always use `cryptoRandInt()` from `generator.go`.
3. **`sync.Once` is load-bearing** — `dictionary.go` and `wordlist.go` rely on it for thread safety. Don't refactor it away.
4. **`leetMap` is shared** — `scorer.go` defines it, `rotator.go` inverts it via `buildReverseLeet()`. Changes to `leetMap` affect both scoring and rotation.
5. **Struct alignment matters** — Go structs are padded for alignment. The Memory Analysis section documents exact layouts. Reordering fields can change struct size.
6. **EFF wordlist is embedded** — don't try to read it from disk at runtime. It's compiled into the binary.
7. **HIBP timeout is 5 seconds** — `NewHIBPChecker()` sets `http.Client{Timeout: 5 * time.Second}`. Don't remove this.
8. **`make clean` clears Go cache** — `go clean --cache` is intentional. It ensures a truly fresh build.

---

## Test Coverage Map

| File | Test File | Test Count | Coverage Areas |
|---|---|---|---|
| `generator.go` | `generator_test.go` | 8 tests | Default config, custom length, class presence, exclusion, uniqueness |
| `rotator.go` | `rotator_test.go` | 16 tests | Basic variants, same-length, structure preservation, edge cases, v2 variable-length, length mutations |
| `scorer.go` | `scorer_test.go` | 9 tests | Empty, common passwords, leet, sequences, repeats, keyboard walks, strong, length bonus, labels |
| `suggester.go` | `suggester_test.go` | 5 tests | Short, missing classes, common, strong |
| `wordlist.go` | `wordlist_test.go` | 2 tests | Load, idempotent reload |
| `main.go` | (in `cmd/passforge/`) | 4 tests | SSBD alias, JSON output, min/max length flags, strict length |

**Total: 44+ unit tests.** Run with `make test`.

---

## Changelog

| Date | Change Summary |
|---|---|
| 2026-03-06 | Security hardening sweep. Added `golang.org/x/term` for hidden password echo. Replaced global errors with centralized `internal/core/errors.go`. Guarded HIBP check with `io.LimitReader` (1 MiB memory cap). Enforced strict exit codes (0, 1, 2, 3) across CLI. |
| 2026-03-06 | Alignment reformatting across `config.go`, `generator_test.go`, `rotator.go`, `scorer_test.go`. Added `all-clean` Makefile target. Enhanced `clean` to clear Go build cache. Bumped Go version from 1.25 to 1.26. |
| 2026-03-02 | Release automation via GitHub Actions. Cross-platform builds. |
| 2026-02-22 | SSBD v2: variable-length rotation variants. `RotateConfig` struct. `--min-length`, `--max-length`, `--strict-length` flags. |
| 2026-02-21 | Makefile created. Clean, test, vet, bench, fmt, cover targets. Help command. |
| — | Initial commit: generate, check, suggest, rotate, bulk, passphrase, HIBP breach check. |
