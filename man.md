# PassForge — Detailed Internal Reference (man)

Line-by-line documentation of every source file in the project.

---

## Table of Contents

- [internal/core/config.go](#internalcoreconfiggo)
- [internal/core/generator.go](#internalcoregeneratorgo)
- [internal/core/scorer.go](#internalcorescorergo)
- [internal/core/dictionary.go](#internalcoredictionarygo)
- [internal/core/suggester.go](#internalcoresugestergo)
- [internal/core/hibp.go](#internalcorehibpgo)
- [internal/core/wordlist.go](#internalcorewordlistgo)
- [cmd/passforge/main.go](#cmdpassforgemain-go)

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

Creates an HTTP client with a 5-second timeout. The timeout prevents the CLI from hanging if the HIBP API is slow or unreachable.

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

## `cmd/passforge/main.go`

CLI entry point. Maps user commands to core library functions.

### Lines 13-33: `main()` and root command

**Line 13:** `jsonOutput` is a package-level bool, set by `--json` flag.

**Lines 16-20:** Root command definition. `Use: "passforge"` sets the binary name. `Short` and `Long` are help text.

**Line 22:** `PersistentFlags` makes `--json` available on ALL subcommands (not just root).

**Lines 24-28:** Register all subcommand constructors.

**Lines 30-32:** Execute. Cobra handles parsing, routing, help text, and error display. If any subcommand returns an error, we exit with code 1.

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
- `os.Exit(2)` if breached
- `os.Exit(1)` if score < 40 (Weak)
- Implicit `os.Exit(0)` otherwise

### Lines 151-182: `suggestCmd()`

Scores the password, then displays only the score and suggestions (no entropy or penalties). Lighter output for quick advice.

### Lines 184-221: `bulkCmd()`

**Line 186:** `count` variable for `--count`/`-n` flag (default 10).

**Lines 191-199:** Generate loop — calls `core.Generate(cfg)` N times, collecting into a slice.

**Lines 201-208:** Output as JSON array or one-per-line plain text.

### Lines 223-227: `printJSON()`

Utility function. Creates a `json.NewEncoder` on stdout with 2-space indentation. Used by all subcommands when `--json` is set.
