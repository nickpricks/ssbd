# PassForge — Architecture

Complete project structure, file reference, and setup guide.

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

---

## Directory Structure

```
passforge/
│
├── cmd/                            # Executable entry points (one per target)
│   ├── passforge/                  # CLI application
│   │   └── main.go                # Cobra command definitions, flag bindings, output formatting
│   ├── passforge-web/             # Web server (M2 — Fiber) [placeholder]
│   └── passforge-desktop/         # Desktop app (M3 — Fyne) [placeholder]
│
├── internal/                       # Private packages (not importable by external code)
│   └── core/                      # Shared library — the brains of PassForge
│       ├── config.go              # Configuration structs and result types
│       ├── generator.go           # Password and passphrase generation (crypto/rand)
│       ├── scorer.go              # Strength scoring engine (entropy, patterns, leet-speak)
│       ├── dictionary.go          # Common password list (~200 entries, O(1) lookup)
│       ├── suggester.go           # Actionable improvement suggestions
│       ├── hibp.go                # Have I Been Pwned k-anonymity breach check
│       ├── errors.go              # Centralized error constants and formatted message templates
│       ├── wordlist.go            # EFF wordlist loader (//go:embed, sync.Once)
│       ├── rotator.go             # "Same Same But Different" rotation variant engine
│       ├── wordlist/              # Embedded data
│       │   └── eff_large.txt      # EFF Large Wordlist — 7776 words for passphrases
│       ├── generator_test.go      # Tests: generation, character classes, exclusions, uniqueness
│       ├── rotator_test.go        # Tests: rotation variants, uniqueness, dedup, edge cases
│       ├── scorer_test.go         # Tests: scoring, penalties, leet-speak, labels
│       ├── suggester_test.go      # Tests: suggestion output for various password types
│       ├── hibp_test.go           # Tests: HIBP check logic and NoOpChecker
│       └── wordlist_test.go       # Tests: wordlist loading, word count, idempotency
│
├── Makefile                       # Build, test, bench, vet, fmt — all common tasks
├── .gitignore                     # Go binaries, IDE files, .env, .claude/, OS junk
├── go.mod                         # Go module definition and direct dependencies
├── go.sum                         # Dependency checksums (auto-managed by Go)
│
├── README.md                      # Project overview, features, scoring algorithm, roadmap
├── CLAUDE.md                      # Claude Code guidance for AI assistants working in this repo
├── WORKPLAN.md                    # Completed sprint tracker for v0.1.6 Security Hardening
├── docs/                          # All documentation (except README)
│   ├── PLAN.md                    # Full implementation plan, architecture decisions, risk register
│   ├── review.md           # Code review findings
│   ├── README.md                  # Documentation index
│   ├── arch.md                    # This file — project structure, file map, setup guide
│   ├── help.md                    # Internal process overview (how scoring, generation, etc. work)
│   ├── help_ext.md                # External package reference (cobra, pflag, stdlib usage)
│   ├── man.md                     # Detailed line-by-line source code documentation
│   └── specs/
│       └── 2026-03-23-password-vault-design.md  # Password Vault design spec (v0.3.0)
```

---

## File Reference

### Documentation Files

| File | Purpose | Audience |
|---|---|---|
| [README.md](../README.md) | Project overview — what PassForge is, features, tech stack, roadmap | Everyone (first thing you read) |
| [CLAUDE.md](../CLAUDE.md) | Claude Code guidance — build commands, architecture summary, conventions, project status | AI assistants and contributors using Claude Code |
| [PLAN.md](PLAN.md) | Implementation plan + codebase audit — platform strategy, milestones, feature tiers, design decisions, risk register | Contributors, architects |
| [review.md](review.md) | Code review findings — issues identified during automated review | Contributors, maintainers |
| [arch.md](arch.md) | This file — directory structure, file map, setup/run instructions | New developers, onboarding |
| [help.md](help.md) | Internal process overview — how generation, scoring, suggestions, breach checking work at a high level | Developers wanting to understand the logic |
| [help_ext.md](help_ext.md) | External package reference — what cobra, pflag, mousetrap do and how we use them; notable stdlib packages | Developers new to the dependencies |
| [man.md](man.md) | Detailed line-by-line source documentation — every function, every design decision | Deep reference when reading source code |
| [WORKPLAN.md](../WORKPLAN.md) | Completed sprint tracker for v0.1.6 Security Hardening — broken down tasks and progress | Historical reference |

### Reading order for new contributors

1. **README.md** — what does this project do?
2. **CLAUDE.md** — build commands, conventions, and project status (especially useful for AI contributors)
3. **arch.md** (this file) — how is it organized? how do I run it?
4. **help.md** — how do the internals work?
5. **help_ext.md** — what are the external dependencies?
6. **man.md** — deep dive into specific files/functions
7. **PLAN.md** — future plans, architecture decisions, and codebase audit
8. **WORKPLAN.md** — completed sprint tracker for v0.1.6

### Source Files

#### `cmd/passforge/main.go`

The CLI entry point. Defines six cobra subcommands:

| Command | What it does | Core function called |
|---|---|---|
| `generate` | Random password | `core.Generate(cfg)` |
| `passphrase` | EFF wordlist passphrase | `core.GeneratePassphrase(cfg)` |
| `check` | Score a password's strength | `core.Score(pw)` + optional `HIBPChecker` |
| `suggest` | Improvement suggestions | `core.Score(pw)` (includes suggestions) |
| `rotate` / `ssbd` | Rotation variants (Same Same But Different) | `core.RotateWithConfig(pw, cfg)` |
| `bulk` | Generate N passwords | `core.Generate(cfg)` in a loop |

Global flag `--json` enables JSON output on all commands. Passwords can optionally be supplied via secure stdin or interactive hidden prompt to avoid bash-history leaks. Exit codes: `0` strong, `1` weak, `2` breached, `3` operational failure.

#### `internal/core/errors.go`

A central dictionary of all `Err...` base errors and `Msg...` string formatting templates used throughout the `core` library and CLI. Centralizes the string literals for easier maintenance, testing, and translation.

#### `internal/core/config.go`

Data types only — no logic. Defines:
- `GeneratorConfig` — length, character class toggles, exclusion list
- `PassphraseConfig` — word count, separator, capitalization, number suffix
- `RotateConfig` — count, min/max length, strict-length toggle
- `ScoreResult` — score, label, entropy, penalties, suggestions, breach flag
- `LabelForScore()` — maps 0-100 score to Weak/Fair/Strong/Very Strong

#### `internal/core/generator.go`

Two public functions:
- `Generate(cfg)` — builds charset from config, picks random characters via `crypto/rand`, ensures all enabled classes are represented
- `GeneratePassphrase(cfg)` — picks random words from EFF list, optionally capitalizes and appends a digit

Helper functions: `buildCharset()`, `ensureClasses()`, `containsAnyOf()`, `cryptoRandInt()`

#### `internal/core/scorer.go`

Single public function `Score(password)` that returns a `ScoreResult`. Pipeline:
1. Calculate Shannon entropy → base score
2. Subtract penalties: sequences (-8/-15), repeats (-10/-20), keyboard walks (-10/-20), dictionary match (-40), leet-speak match (-30)
3. Add length bonus (up to +15 for passwords > 12 chars)
4. Clamp to 0-100
5. Generate suggestions via `Suggest()`

Helper functions: `calculateEntropy()`, `characterPoolSize()`, `entropyToBaseScore()`, `sequencePenalty()`, `repeatPenalty()`, `keyboardWalkPenalty()`, `normalizeLeet()`, `reverseString()`

#### `internal/core/dictionary.go`

~200 most common breached passwords stored as a Go slice, loaded into a `map[string]struct{}` on first access via `sync.Once`. Case-insensitive O(1) lookups.

#### `internal/core/suggester.go`

`Suggest(password, result)` — examines the password and its score result, returns a list of human-readable improvement tips (missing classes, length, patterns, dictionary hits, breach warnings).

#### `internal/core/hibp.go`

HIBP k-anonymity protocol:
1. SHA-1 hash the password
2. Send first 5 hex chars to `api.pwnedpasswords.com/range/`
3. Check if remaining 35 chars appear in the response

Defines `BreachChecker` interface with two implementations:
- `HIBPChecker` — real HTTP client with 5s timeout and an `io.LimitReader` (1 MiB cap) to prevent DoS from maliciously large responses
- `NoOpChecker` — always returns false (offline/testing)

#### `internal/core/rotator.go`

The "Same Same But Different" engine. Two public functions:
- `Rotate(base, count)` — v1 API, same-length substitution-only variants (delegates to `RotateWithConfig` with `StrictLength: true`)
- `RotateWithConfig(base, cfg)` — v2 API with full configuration: variable-length variants via insertions, appends, prepends, and repeat-dropping, constrained by `MinLength`/`MaxLength` (±3 chars from base)

Uses mixed-radix enumeration for substitution mutations, two-phase pipeline for length mutations. Deduplicates against the base and all prior variants.

#### `internal/core/wordlist.go`

Loads the EFF Large Wordlist (7776 words) via `//go:embed`. Parses the tab-separated format once on first call, caches the result. Used only by `GeneratePassphrase()`.

#### Test Files

All tests live alongside the code they test in `internal/core/`:
- `generator_test.go` — default config, custom lengths, character classes, exclusions, invalid input, uniqueness
- `rotator_test.go` — variant uniqueness, dedup, count validation, empty input, limited mutation points, structure preservation, variable-length growth/shrink, strict-length override, bounds validation, length mutation helpers
- `scorer_test.go` — empty passwords, common passwords, leet-speak, sequences, repeats, keyboard walks, strong passwords, length bonus, labels, generated-password-is-strong integration test
- `suggester_test.go` — short passwords, missing classes, common passwords, strong passwords
- `wordlist_test.go` — word count (7776), known words, idempotent loading

---

## Setup

### Prerequisites

- **Go 1.26+** (go.mod specifies `go 1.26.0`)
- **Git** (to clone the repo)

### Clone and Build

```bash
# Clone
git clone https://github.com/<user>/passforge.git
cd passforge

# Download dependencies
go mod tidy

# Build the CLI binary
go build -o passforge ./cmd/passforge

# Or install globally
go install ./cmd/passforge
```

### Run

```bash
# Generate a 20-char password
./passforge generate --length 20

# Generate a 4-word passphrase
./passforge passphrase --words 4 --separator "-"

# Check strength of a password
./passforge check "MyP@ssw0rd"

# Check with breach detection (hits HIBP API)
./passforge check --breach "password123"

# Get improvement suggestions
./passforge suggest "hello123"

# Bulk generate 10 passwords as JSON
./passforge bulk --count 10 --length 16 --json

# Generate digits-only PIN
./passforge generate --length 6 --upper=false --lower=false --symbols=false

# Exclude ambiguous characters
./passforge generate --length 20 --exclude "0OIl1"

# Rotation variants — same length (default)
./passforge rotate "p@sSwor4" --count 5

# Rotation variants — variable length
./passforge rotate "p@sSwor4" --count 5 --min-length 8 --max-length 11

# Rotation variants — strict length (force same as base)
./passforge rotate "p@sSwor4" --count 5 --strict-length
```

### Run Tests

```bash
# All tests
go test ./...

# Verbose output
go test ./... -v

# With coverage
go test ./... -cover

# Specific package
go test ./internal/core/ -v
```

### Development

A `Makefile` is provided for common tasks. Run `make help` to see all targets.

```bash
make build                    # Build the passforge binary
make run ARGS="generate -l 20"  # Run without building
make test                     # All tests (verbose)
make bench                    # Benchmarks with memory stats
make vet                      # Static analysis
make fmt                      # Format all Go files
make cover                    # Tests with coverage report
make all                      # vet + test + bench
make clean                    # Remove build artifacts
```

Or use raw Go commands:

```bash
go fmt ./...
go vet ./...
go build ./...
go run ./cmd/passforge generate --length 20
```

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     User Interface                       │
├──────────────┬──────────────────┬───────────────────────┤
│  CLI (cobra) │  Web (Fiber) M2  │  Desktop (Fyne) M3   │
│  cmd/passforge│ cmd/passforge-web│ cmd/passforge-desktop │
└──────┬───────┴────────┬─────────┴──────────┬────────────┘
       │                │                    │
       └────────────────┼────────────────────┘
                        │
                        ▼
       ┌────────────────────────────────┐
       │        internal/core           │
       │    (shared library — no UI)    │
       ├────────────────────────────────┤
       │  Generate()      → password    │
       │  GeneratePassphrase() → phrase │
       │  Score()         → result      │
       │  Suggest()       → []string    │
       │  IsBreached()    → bool        │
       └───────┬──────────────┬─────────┘
               │              │
        ┌──────┴──────┐  ┌───┴────────────┐
        │  Embedded   │  │  HIBP API      │
        │  wordlist   │  │  (optional,    │
        │  dictionary │  │   network)     │
        └─────────────┘  └────────────────┘
```

All three UI targets (CLI, Web, Desktop) call the same `internal/core` library. The core has zero knowledge of how it's being consumed. Data flows in via config structs, out via result structs. The only external network call is the optional HIBP breach check.
