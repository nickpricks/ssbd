# PassForge — Internal Process Overview

How the core library works, explained simply.

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

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
                              ├── wordlist.go    → EFF wordlist for passphrases
                              ├── rotator.go     → "Same Same But Different" variants
                              └── errors.go      → Centralized messaging and typed errors
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

This is optional — enabled via `--breach` flag. Fails gracefully if the API is unreachable. Uses an `io.LimitReader` (1 MiB) to prevent memory-exhaustion attacks from malicious or oversized responses.

---

## Wordlist (`wordlist.go`)

- The EFF Large Wordlist (7776 words) is embedded at compile time via `//go:embed`
- Parsed once on first access: each line is `"11111\tword"` format, we extract just the word
- Cached in a package-level slice behind `sync.Once`
- Used only for passphrase generation

---

## Rotation Variants — "Same Same But Different" (`rotator.go`)

Generates unique password variants for forced rotation policies. You keep one strong base password; PassForge produces variants that look different but share the same muscle memory.

### Substitution mutations (v1 — same length)

1. **Find mutation points** — scan the base password for positions that can be varied:
   - Letters can be case-flipped (`a` ↔ `A`)
   - Letters can be leet-substituted (`a` → `@`, `s` → `$`)
   - Leet characters can be reversed (`@` → `a`, `$` → `s`)
2. **Mixed-radix enumeration** — each cycle number maps to a unique combination of mutations. The cycle is treated as a mixed-radix number where each digit selects which form to use at each position.
3. **Deduplication** — a `seen` map ensures no variant matches the base or any previous variant.
4. **Safety cap** — if the password has few mutation points, generation stops when the variant space is exhausted.

```
passforge rotate "p@sSwor4" --count 5
1: P@sswor4
2: pAs$wor4
3: p@ssWor4
4: pa$Swor4
5: P@sSWor4
```

### Length mutations (v2 — variable length)

When `--min-length` and/or `--max-length` are provided, variants can grow or shrink by up to 3 characters:

1. **Find length mutation candidates** — scan the base for:
   - **Insert positions** — up to 8 evenly-spaced gaps where a random char can be inserted (pool chosen contextually from neighbors)
   - **Append/prepend** — always available, using digits and symbols
   - **Drop-repeat** — consecutive repeated characters (`aa`, `ss`) where one can be removed
2. **Two-phase pipeline** — each variant is built by first applying a substitution mutation cycle, then applying one or more length mutations (insert, append, prepend, or drop)
3. **Bounds enforcement** — every variant's length is checked against `[min-length, max-length]`. The delta from base is clamped to ±3 characters.
4. **`--strict-length`** — forces all variants to match the base length exactly (v1 behavior)

```
passforge rotate "p@sSwor4" --count 5 --min-length 8 --max-length 11
1: P@sSwor4
2: P@sSwor4:
3: !P@sSwor4?
4: ^uP@sSwor4:
5: !P@sSwor4
```

---

## CLI Layer (`cmd/passforge/main.go`)

Thin wrapper that maps cobra subcommands to core library calls:

| Command | Core function | Description |
|---|---|---|
| `generate` | `core.Generate(cfg)` | Random password |
| `passphrase` | `core.GeneratePassphrase(cfg)` | EFF wordlist passphrase |
| `check` | `core.Score(pw)` + optional `HIBP` | Strength check |
| `suggest` | `core.Score(pw)` (includes suggestions) | Improvement tips |
| `rotate` / `ssbd` | `core.RotateWithConfig(pw, cfg)` | Rotation variants (Same Same But Different) |
| `bulk` | `core.Generate(cfg)` in a loop | Multiple passwords |

Security feature: Passwords can be passed via stdin (e.g. `echo "pass" | passforge check`) or interactively typed into a hidden prompt inside the terminal, ensuring they don't leak to `ps`/`top` outputs or `.bash_history`.

Exit codes: `0` = strong, `1` = weak (score < 40), `2` = breached, `3` = operational/input error.

All commands support `--json` for machine-readable output.
