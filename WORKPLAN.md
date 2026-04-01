# WORKPLAN — PassForge v0.1.6: Security Hardening

> Current focus: address critical and high-severity findings from [code review](docs/review.md).

---

## Tracker

| # | Task | Priority | Status |
|---|------|----------|--------|
| 1 | Stdin/prompt password input | Critical | done |
| 2 | HIBP hard-fail on `--breach` errors | Critical | done |
| 3 | `io.LimitReader` on HIBP response | High | done |
| 4 | Keyboard walk penalty fix (largest-first) | High | done |
| 5 | Rune-based entropy scoring | High | done |
| 6 | `ScoreResult.MarkBreached()` method | Medium | done |
| 7 | Distinct exit codes + typed errors from `RunE` | Medium | done |
| 8 | Typed errors in rotator | Medium | done |
| 9 | Config `Validate()` methods | Medium | done |
| 10 | Wordlist size validation | Low | done |
| 11 | Cleanup: SymbolPoolSize, dead code, NoOpChecker, global state, flag UX | Low | done |
| 12 | Test coverage: CLI integration, HIBP mock, Unicode, bulk, passphrase | — | done |
| 13 | Updations of docs/arch (Complete project structure, file reference, and setup guide), docs/man (Line-by-line documentation of every source file in the project.), help (How the core library works, explained simply.), helpext (Reference for all third-party dependencies used by PassForge. with a bit of explanation.) | — | done |

---

## Details

### 1. Stdin/prompt password input (Critical)

**Files:** `cmd/passforge/main.go`

CLI args are visible via `ps aux` / `/proc/<pid>/cmdline`. Read from stdin when `-` is passed or no arg is given; prompt with hidden echo otherwise.

```bash
echo "MySecret123!" | passforge check -
passforge check          # prompts with hidden input
```

### 2. HIBP hard-fail (Critical)

**Files:** `cmd/passforge/main.go`

When `--breach` is explicit and the check fails (network, HTTP 429/503, DNS), exit with code 3 ("breach check inconclusive") instead of silently proceeding as "not breached." Add `--breach-warn-only` flag for opt-in soft failure.

### 3. `io.LimitReader` on HIBP response (High)

**File:** `internal/core/hibp.go:54`

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
```

### 4. Keyboard walk penalty fix (High)

**File:** `internal/core/scorer.go:211-233`

Iterate from largest to smallest window size so `qwertyuiop` (10 chars) returns `KeyboardPenaltyLarge` instead of matching `qwer` first at window size 4.

### 5. Rune-based entropy scoring (High)

**Files:** `internal/core/scorer.go:99,67` · `internal/core/suggester.go:13`

Replace `len(password)` with `utf8.RuneCountInString(password)` for correct entropy on multi-byte Unicode input.

### 6. `ScoreResult.MarkBreached()` (Medium)

**File:** `internal/core/scorer.go` (new method)

Atomic method to set `Breached`, cap `Score`, update `Label`, append penalty and suggestion — prevents inconsistent state from manual field mutation.

### 7. Distinct exit codes (Medium)

**File:** `cmd/passforge/main.go`

- 0 = strong, 1 = weak, 2 = breached, 3 = operational error
- Return typed errors from `RunE`, handle exit codes in `main()` after `Execute()`
- Remove `os.Exit` calls inside Cobra handlers

### 8. Typed errors in rotator (Medium)

**File:** `internal/core/rotator.go`

Distinguish recoverable constraint errors from fatal `crypto/rand` failures so callers know when to retry vs. abort.

### 9. Config `Validate()` methods (Medium)

Add `Validate()` to `GeneratorConfig`, `PassphraseConfig`, `RotateConfig` to catch zero-value and invalid states early rather than deferring to consumer functions.

### 10. Wordlist size validation (Low)

**File:** `internal/core/wordlist.go`

After parsing, validate a minimum word count (~7000) to catch silent data loss from format corruption.

### 11. Cleanup items (Low)

- Fix `SymbolPoolSize` 32 → 31 (`internal/core/scorer.go`)
- Remove `normalizeBase` dead code or move to test file (`internal/core/rotator.go`)
- Move `NoOpChecker` to test file (`internal/core/hibp.go`)
- Eliminate global `jsonOutput` var — pass via command context or struct
- Consider `--no-upper`, `--no-lower` flag pattern for disabling defaults

### 12. Test coverage gaps

| Area | Gap |
|------|-----|
| CLI integration | No tests for exit codes or JSON output |
| HIBP checker | No mock HTTP server tests |
| Unicode | No multi-byte character tests for scorer |
| `bulkCmd` | Untested |
| `GeneratePassphrase` | `AddNumber: true` path untested |
| `cryptoRandInt` | Error paths untested |
| Rotator | Partial-result-with-error path untested |

---

## After v0.1.6

The following are deferred to **M1.x / v0.2.0 (CLI Polish)**:

- `improve.go` — password improvement engine (`passforge improve`)
- Scoring gate — `--min-score` flag on `rotate`
- CI pipeline (fmt, vet, staticcheck, test matrix)
- GoReleaser + Homebrew tap
- Expanded dictionary (~100k SecLists)
- Shell completions (bash/zsh/fish)

See [PLAN.md](docs/PLAN.md) for the full roadmap.

---

*Updated 2026-03-06.*
