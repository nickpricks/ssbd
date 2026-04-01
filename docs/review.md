# PassForge Code Review

> Automated review performed 2026-03-05 using three specialized agents:
> code review, silent failure analysis, and type design analysis.

---

## Summary

| Severity | Count | Key Areas |
|----------|-------|-----------|
| Critical | 2 | Password exposure in process args; HIBP silent failure |
| High | 3 | Byte-vs-rune length; keyboard walk penalty bug; unbounded HTTP read |
| Medium | 6 | `os.Exit` in Cobra handler; global state; type safety; wordlist parsing; crypto error swallowing; mock in prod |
| Low | 4 | Pool size off-by-one; dead code; flag UX; test gaps |

---

## Critical

### 1. Passwords exposed in process arguments

**Files:** `cmd/passforge/main.go:98-100, 157, 194`

The `check`, `suggest`, and `rotate` commands take passwords as positional CLI arguments. On most Unix systems, command-line arguments are visible to all users via `ps aux` or `/proc/<pid>/cmdline`.

```
passforge check MySecret123!   # visible in process table
```

**Recommendation:** Read from stdin when no argument is provided, or prompt interactively with terminal echo disabled:

```
echo "MySecret123!" | passforge check -
passforge check   # prompts with hidden input
```

### 2. HIBP errors silently downgraded — breached passwords reported as safe

**File:** `cmd/passforge/main.go:103-114`

When a user explicitly passes `--breach` and the check fails (network error, HTTP 429 rate limit, HTTP 503 outage), the error is printed as a stderr warning and the password proceeds as if not breached. Exit code 0 (strong) is returned.

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Warning: breach check failed: %v\n", err)
    // password continues as "not breached" — false safety
}
```

**Hidden failure modes:**
- HTTP 429 (rate limiting) — no real results, user unaware
- HTTP 503 (service outage) — all passwords pass
- DNS resolution failure — offline user doesn't realize checks are skipped
- TLS certificate errors — potential MITM, silently ignored

**Recommendation:** When the user explicitly requests `--breach` and the check fails, hard-fail by default with a distinct exit code (e.g., 3 for "breach check inconclusive"). Offer `--breach-warn-only` for soft-failure opt-in.

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: breach check failed: %v\n", err)
    fmt.Fprintf(os.Stderr, "Cannot confirm password safety. Use without --breach to skip.\n")
    os.Exit(3)
}
```

---

## High

### 3. `len(password)` uses byte length, not rune length

**Files:** `internal/core/scorer.go:99, 67` · `internal/core/suggester.go:13`

Entropy calculation and length checks use `len(password)` (byte count) instead of `utf8.RuneCountInString()`. Multi-byte Unicode input (accented characters, CJK, emoji) inflates entropy scores.

```go
// Current — byte count
return float64(len(password)) * math.Log2(float64(poolSize))

// Fixed — character count
return float64(utf8.RuneCountInString(password)) * math.Log2(float64(poolSize))
```

The `sequencePenalty` and `repeatPenalty` functions correctly convert to `[]rune`, but `keyboardWalkPenalty` and the suggester do not.

### 4. Keyboard walk penalty returns on first small match, missing larger ones

**File:** `internal/core/scorer.go:211-233`

The loop iterates `windowSize` from 4 upward. A password containing `"qwerty"` (6 chars) matches `"qwer"` first at window size 4 and returns `KeyboardPenaltySmall` (-10) instead of continuing to find the 6-character match that should return `KeyboardPenaltyLarge` (-20).

```go
// Current — iterates small to large, returns on first match
for windowSize := 4; windowSize <= len(row); windowSize++ {
    // ...
    if strings.Contains(lower, pattern) {
        if windowSize >= 6 { return KeyboardPenaltyLarge }
        return KeyboardPenaltySmall  // exits here for "qwerty"
    }
}
```

**Fix:** Iterate from largest to smallest window size, or track the maximum match across all iterations.

### 5. Unbounded HIBP response read

**File:** `internal/core/hibp.go:54`

```go
body, err := io.ReadAll(resp.Body)
```

A malicious or compromised server could return a multi-gigabyte response causing OOM. The HIBP range API typically returns ~30-50 KB.

**Fix:**
```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
```

---

## Medium

### 6. `os.Exit` inside Cobra `RunE` handler

**File:** `cmd/passforge/main.go:137-142`

```go
if result.Breached { os.Exit(2) }
if result.Score < core.WeakThreshold { os.Exit(1) }
```

Problems:
- Bypasses deferred cleanup functions
- Makes the command untestable (kills the test process)
- Exit code 1 is ambiguous — Cobra uses 1 for command errors, and this uses 1 for weak passwords

**Recommendation:** Return typed errors from `RunE` and handle exit codes in `main()` after `Execute()` returns. Use distinct exit codes: 0 = strong, 1 = weak, 2 = breached, 3 = operational error.

### 7. Global mutable `jsonOutput` variable

**File:** `cmd/passforge/main.go:13`

```go
var jsonOutput bool
```

Package-level mutable state shared across tests. `TestRotateAlias_SSBD_JSON` manually resets it. Causes data races if tests run in parallel.

**Fix:** Pass the flag value through the command context or a struct.

### 8. `ScoreResult` mutation is unprotected

**File:** `cmd/passforge/main.go:109-114`

The caller manually mutates 5 fields to mark a password as breached:

```go
result.Breached = true
result.Score = min(result.Score, core.BreachScoreCap)
result.Label = core.LabelForScore(result.Score)
result.Penalties = append(result.Penalties, "found in data breach")
result.Suggestions = append(result.Suggestions, "...")
```

Skipping any single line produces an inconsistent result (e.g., `Score: 10, Label: "Very Strong"`).

**Fix:** Add a `MarkBreached()` method:

```go
func (r *ScoreResult) MarkBreached() {
    r.Breached = true
    if r.Score > BreachScoreCap {
        r.Score = BreachScoreCap
    }
    r.Label = LabelForScore(r.Score)
    r.Penalties = append(r.Penalties, "found in data breach")
    r.Suggestions = append(r.Suggestions,
        "This password appeared in a data breach — do not use it")
}
```

### 9. Silent data loss in wordlist parsing

**File:** `internal/core/wordlist.go:26-33`

Lines not matching the tab-separated format are silently dropped. If the embedded file is corrupted (e.g., tabs replaced with spaces), all lines drop and the only error is a generic "wordlist is empty."

**Fix:** Validate minimum word count after parsing:

```go
const expectedMinWordlistSize = 7000
if len(wordlist) < expectedMinWordlistSize {
    panic(fmt.Sprintf("passforge: wordlist has only %d words (expected %d, skipped %d)",
        len(wordlist), expectedMinWordlistSize, skipped))
}
```

### 10. Rotator silently swallows `crypto/rand` failures

**File:** `internal/core/rotator.go:134-139`

All errors from `buildVariableLengthVariant` are discarded, including potential `crypto/rand` failures. If the RNG is broken, the loop burns through `maxAttempts` and returns a misleading "could not generate variants within length bounds" error.

**Fix:** Distinguish recoverable constraint errors from fatal crypto errors using typed errors.

### 11. `NoOpChecker` in production code

**File:** `internal/core/hibp.go:70-75`

A mock implementation that always returns "not breached" lives in production code. Its existence invites future misuse as a silent fallback.

**Recommendation:** Move to `hibp_test.go` or a dedicated test utility package.

---

## Low

### 12. Symbol pool size off by one

**File:** `internal/core/scorer.go`

`SymbolPoolSize` is 32 but the actual charset contains 31 symbols. Entropy is slightly overestimated.

### 13. `normalizeBase` is dead code

**File:** `internal/core/rotator.go:467`

Only used in tests. Duplicates `normalizeLeet` in `scorer.go`. Should be removed or moved to a test file.

### 14. Boolean flag UX

**File:** `cmd/passforge/main.go`

With `DefaultGeneratorConfig()` setting all classes to `true`, `--upper` does nothing (already true). Users must use `--upper=false` to disable. Consider `--no-upper`, `--no-lower` pattern for clarity.

### 15. Test coverage gaps

| Area | Gap |
|------|-----|
| CLI integration | No tests for exit codes or JSON output |
| HIBP checker | No mock HTTP server tests for `HIBPChecker.IsBreached` |
| Unicode | No multi-byte character tests for scorer |
| `bulkCmd` | Untested |
| `GeneratePassphrase` | `AddNumber: true` path untested |
| `cryptoRandInt` | Error paths untested (hard without DI) |
| Rotator | Partial-result-with-error path untested |

---

## Type Design Analysis

### Ratings

| Type | Encapsulation | Invariant Expression | Enforcement | Priority Fix |
|------|:---:|:---:|:---:|---|
| `ScoreResult` | 1/10 | 2/10 | 2/10 | Add `MarkBreached()` method |
| `GeneratorConfig` | 2/10 | 3/10 | 4/10 | Add `Validate()` method |
| `RotateConfig` | 2/10 | 2/10 | 5/10 | Split into `RotateStrict`/`RotateVariable` constructors |
| `PassphraseConfig` | 2/10 | 3/10 | 4/10 | Add upper bound on `Words` |
| `HIBPChecker` | 3/10 | 5/10 | 4/10 | Make `Client` field unexported |
| Internal types | 6/10 | 5/10 | 5/10 | Fine as-is |

### Key Type Issues

**ScoreResult** — Every field is exported and mutable. The CLI directly mutates 5 fields to mark a breach. Skipping any step produces an inconsistent Score/Label pair. This is the most dangerous pattern in the codebase.

**RotateConfig** — Zero values for `MinLength`/`MaxLength` are sentinel values meaning "use base length." The interaction between `StrictLength`, `MinLength`, `MaxLength`, and base length creates a complex state space that is invisible from the type definition. `StrictLength: true` with length bounds set silently ignores the bounds.

**GeneratorConfig** — Zero value is maximally invalid (`Length: 0`, all booleans false). `ExcludeChars` can silently empty an enabled class without warning.

**HIBPChecker** — `Client` is exported and can be set to nil after construction, causing a nil pointer dereference in `IsBreached()`.

### Cross-Cutting: The Zero-Value Problem

Every config type's zero value is invalid:
- `GeneratorConfig{}` — Length 0, no character classes
- `PassphraseConfig{}` — Words 0
- `RotateConfig{}` — Count 0

The `Default*Config()` functions mitigate this, but validation is deferred to consumer functions rather than enforced at construction.

---

## Positive Observations

- **Crypto usage is correct** — `crypto/rand` used throughout, no `math/rand`
- **K-anonymity for HIBP** — full password/hash never leaves the machine
- **Embedded wordlist** — `//go:embed` eliminates runtime file I/O
- **`sync.Once` lazy loading** — dictionary and wordlist are efficient and thread-safe
- **Generator validation is thorough** — all `crypto/rand` errors properly wrapped and propagated
- **Clean architecture** — UI-agnostic core library, well-separated layers
- **Good test coverage** — 85%+ on core library
- **Deterministic rotation** — same input always produces same variant sequence
- **Rotator input validation** — thorough with specific error messages
- **Interface-driven HIBP** — `BreachChecker` interface enables clean testability

---

## Recommended Fix Priority

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| 1 | Stdin support for password input | Medium | Eliminates process-table exposure |
| 2 | Hard-fail on `--breach` errors | Low | Prevents false safety on breach checks |
| 3 | `io.LimitReader` on HIBP response | Trivial | Prevents OOM from malicious server |
| 4 | Fix keyboard walk penalty iteration | Low | Correct penalty for long walks like `qwertyuiop` |
| 5 | `ScoreResult.MarkBreached()` method | Low | Atomic breach marking, eliminates inconsistency risk |
| 6 | Use `utf8.RuneCountInString` in scorer | Low | Correct entropy for Unicode input |
| 7 | Distinct exit codes | Medium | Disambiguate Cobra errors from weak passwords |
| 8 | Wordlist size validation | Trivial | Catch silent data loss at build time |
| 9 | Typed errors in rotator | Medium | Surface crypto failures instead of masking them |
| 10 | Config `Validate()` methods | Medium | Strengthen library API for future consumers |
