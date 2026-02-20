# PassForge — Architecture

Complete project structure, file reference, and setup guide.

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
│       ├── wordlist.go            # EFF wordlist loader (//go:embed, sync.Once)
│       ├── wordlist/              # Embedded data
│       │   └── eff_large.txt      # EFF Large Wordlist — 7776 words for passphrases
│       ├── generator_test.go      # Tests: generation, character classes, exclusions, uniqueness
│       ├── scorer_test.go         # Tests: scoring, penalties, leet-speak, labels
│       ├── suggester_test.go      # Tests: suggestion output for various password types
│       └── wordlist_test.go       # Tests: wordlist loading, word count, idempotency
│
├── .gitignore                     # Go binaries, IDE files, .env, .claude/, OS junk
├── go.mod                         # Go module definition and direct dependencies
├── go.sum                         # Dependency checksums (auto-managed by Go)
│
├── README.md                      # Project overview, features, scoring algorithm, roadmap
├── PLAN.md                        # Full implementation plan, architecture decisions, risk register
├── arch.md                        # This file — project structure, file map, setup guide
├── help.md                        # Internal process overview (how scoring, generation, etc. work)
├── help_ext.md                    # External package reference (cobra, pflag, stdlib usage)
└── man.md                         # Detailed line-by-line source code documentation
```

---

## File Reference

### Documentation Files

| File | Purpose | Audience |
|---|---|---|
| [README.md](README.md) | Project overview — what PassForge is, features, tech stack, roadmap | Everyone (first thing you read) |
| [PLAN.md](PLAN.md) | Implementation plan — platform strategy, milestone roadmap, deployment, risk register | Contributors, architects |
| [arch.md](arch.md) | This file — directory structure, file map, setup/run instructions | New developers, onboarding |
| [help.md](help.md) | Internal process overview — how generation, scoring, suggestions, breach checking work at a high level | Developers wanting to understand the logic |
| [help_ext.md](help_ext.md) | External package reference — what cobra, pflag, mousetrap do and how we use them; notable stdlib packages | Developers new to the dependencies |
| [man.md](man.md) | Detailed line-by-line source documentation — every function, every design decision | Deep reference when reading source code |

### Reading order for new contributors

1. **README.md** — what does this project do?
2. **arch.md** (this file) — how is it organized? how do I run it?
3. **help.md** — how do the internals work?
4. **help_ext.md** — what are the external dependencies?
5. **man.md** — deep dive into specific files/functions
6. **PLAN.md** — future plans and architecture decisions

### Source Files

#### `cmd/passforge/main.go`

The CLI entry point. Defines five cobra subcommands:

| Command | What it does | Core function called |
|---|---|---|
| `generate` | Random password | `core.Generate(cfg)` |
| `passphrase` | EFF wordlist passphrase | `core.GeneratePassphrase(cfg)` |
| `check` | Score a password's strength | `core.Score(pw)` + optional `HIBPChecker` |
| `suggest` | Improvement suggestions | `core.Score(pw)` (includes suggestions) |
| `bulk` | Generate N passwords | `core.Generate(cfg)` in a loop |

Global flag `--json` enables JSON output on all commands. Exit codes: `0` strong, `1` weak, `2` breached.

#### `internal/core/config.go`

Data types only — no logic. Defines:
- `GeneratorConfig` — length, character class toggles, exclusion list
- `PassphraseConfig` — word count, separator, capitalization, number suffix
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
- `HIBPChecker` — real HTTP client with 5s timeout
- `NoOpChecker` — always returns false (offline/testing)

#### `internal/core/wordlist.go`

Loads the EFF Large Wordlist (7776 words) via `//go:embed`. Parses the tab-separated format once on first call, caches the result. Used only by `GeneratePassphrase()`.

#### Test Files

All tests live alongside the code they test in `internal/core/`:
- `generator_test.go` — default config, custom lengths, character classes, exclusions, invalid input, uniqueness
- `scorer_test.go` — empty passwords, common passwords, leet-speak, sequences, repeats, keyboard walks, strong passwords, length bonus, labels, generated-password-is-strong integration test
- `suggester_test.go` — short passwords, missing classes, common passwords, strong passwords
- `wordlist_test.go` — word count (7776), known words, idempotent loading

---

## Setup

### Prerequisites

- **Go 1.21+** (the project uses `go 1.25.0` in go.mod but any recent Go works)
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

```bash
# Format code
go fmt ./...

# Vet for issues
go vet ./...

# Build all packages (checks compilation without producing binaries)
go build ./...

# Run directly without building
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
