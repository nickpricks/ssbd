# PassForge — Internal Process Overview

How the core library works, explained simply.

---

## Data Flow

```
User Input → CLI (cobra) → Core Library → Output (text/JSON)
                              │
                              ├── generator.go   → random password or passphrase
                              ├── scorer.go      → strength score 0-100
                              ├── suggester.go   → improvement suggestions
                              ├── hibp.go        → breach check (optional)
                              ├── dictionary.go  → common password lookup
                              └── wordlist.go    → EFF wordlist for passphrases
```

---

## Password Generation (`generator.go`)

1. Build a character set from enabled classes (uppercase, lowercase, digits, symbols)
2. Remove any excluded characters
3. For each position in the password, pick a random character from the charset using `crypto/rand`
4. After generation, verify at least one character from each enabled class is present — if not, replace a random position with one from the missing class
5. Return the password string

**Passphrase generation** picks N random words from the EFF wordlist (7776 words), optionally capitalizes them and appends a random digit.

---

## Strength Scoring (`scorer.go`)

Scoring happens in stages, starting from an entropy-based base score and applying penalties/bonuses:

1. **Calculate entropy** — `length * log2(pool_size)` where pool size is determined by which character classes are present (26 lower + 26 upper + 10 digits + 32 symbols)
2. **Convert entropy to base score** — multiply by 0.78, cap at 100
3. **Apply penalties:**
   - Sequential characters (abc, 321) → -8 or -15 points
   - Repeated characters (aaa, 1111) → -10 or -20 points
   - Keyboard walks (qwerty, asdf) → -10 or -20 points
   - Found in common password list → -40 points
   - Leet-speak variant of common password → -30 points
4. **Apply length bonus** — passwords longer than 12 chars get up to +15 points
5. **Clamp** the score to 0-100
6. **Map score to label:** 0-39 = Weak, 40-59 = Fair, 60-79 = Strong, 80-100 = Very Strong

---

## Dictionary Check (`dictionary.go`)

- ~200 most common passwords are embedded in the binary as a Go string slice
- On first access, they're loaded into a `map[string]struct{}` for O(1) lookups
- `sync.Once` ensures this happens exactly once (thread-safe)
- All comparisons are case-insensitive

---

## Leet-Speak Normalization (`scorer.go`)

Before dictionary checking, passwords are normalized:
- `@` → `a`, `$` → `s`, `0` → `o`, `3` → `e`, `1` → `i`, etc.
- So `p@$$w0rd` becomes `password` and gets caught by the dictionary

---

## Suggestion Engine (`suggester.go`)

Looks at the password and the score result, then generates human-readable advice:
- Missing character classes → "Add uppercase letters"
- Too short → "Increase length to at least 12 characters"
- Detected patterns → "Avoid sequential characters"
- Common password → "This is a commonly used password"
- Breached → "This password appeared in a data breach"

---

## Breach Check (`hibp.go`)

Uses the Have I Been Pwned Pwned Passwords API with k-anonymity:

1. SHA-1 hash the password
2. Send only the first 5 characters of the hash to `api.pwnedpasswords.com`
3. API returns all hash suffixes that match that prefix
4. Check if our full hash suffix appears in the response
5. The full password (or full hash) **never leaves the machine**

This is optional — enabled via `--breach` flag. Fails gracefully if the API is unreachable.

---

## Wordlist (`wordlist.go`)

- The EFF Large Wordlist (7776 words) is embedded at compile time via `//go:embed`
- Parsed once on first access: each line is `"11111\tword"` format, we extract just the word
- Cached in a package-level slice behind `sync.Once`
- Used only for passphrase generation

---

## CLI Layer (`cmd/passforge/main.go`)

Thin wrapper that maps cobra subcommands to core library calls:

| Command | Core function | Description |
|---|---|---|
| `generate` | `core.Generate(cfg)` | Random password |
| `passphrase` | `core.GeneratePassphrase(cfg)` | EFF wordlist passphrase |
| `check` | `core.Score(pw)` + optional `HIBP` | Strength check |
| `suggest` | `core.Score(pw)` (includes suggestions) | Improvement tips |
| `bulk` | `core.Generate(cfg)` in a loop | Multiple passwords |

Exit codes: `0` = strong, `1` = weak (score < 40), `2` = breached.

All commands support `--json` for machine-readable output.
