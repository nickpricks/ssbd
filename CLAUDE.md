# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Info

- **Go version**: 1.26.0 (feature/vault-password-tracking branch)
- **GitHub remote**: `nickpricks/ssbd` (directory name `passforge/` differs from repo name)
- **Dependencies**: Minimal — `cobra` (CLI), `x/term` (hidden password input). No other external deps.

## Build & Dev Commands

```bash
make build                # Build binary → ./passforge
make test                 # go test -v ./...
make bench                # go test -bench=. -benchmem ./...
make vet                  # go vet ./...
make fmt                  # go fmt ./...
make cover                # go test -cover ./...
make all                  # vet + test + bench
make clean                # Remove build artifacts + go cache
```

Run a single test:
```bash
go test -v -run TestName ./internal/core/
go test -v -run TestName ./cmd/passforge/
```

Run CLI without building:
```bash
go run ./cmd/passforge generate --length 20
make generate ARGS="--length 20"
```

## Architecture

Go CLI tool (Cobra) with all business logic in `internal/core/`. The CLI layer (`cmd/passforge/main.go`) is a thin shell that wires Cobra flags to config structs and calls core functions.

### Core library (`internal/core/`)

| File | Role |
|---|---|
| `config.go` | All config structs (`GeneratorConfig`, `PassphraseConfig`, `RotateConfig`, `ScoreResult`) and scoring constants. Data types only — no logic. |
| `errors.go` | Sentinel errors (`ErrWeak`, `ErrBreached`, `ErrInvalidConfig`, etc.) and `Msg*` format strings. All error messages centralized here. |
| `generator.go` | `Generate()` and `GeneratePassphrase()`. Uses `crypto/rand` exclusively. Fisher-Yates shuffle to guarantee all enabled character classes appear. |
| `scorer.go` | `Score()` — entropy → base score, subtract pattern penalties (sequences, repeats, keyboard walks, dictionary, leet-speak), add length bonus, clamp [0,100]. Also contains `leetMap` and `normalizeLeet()`. |
| `suggester.go` | `Suggest()` — examines password + ScoreResult, returns human-readable improvement tips. |
| `rotator.go` | "Same Same But Different" engine. `Rotate()` (v1, same-length) and `RotateWithConfig()` (v2, variable-length ±3 chars). Uses mixed-radix enumeration over mutation points (case flips, leet swaps). `reverseLeet` is the inverse of `leetMap` from scorer.go. |
| `hibp.go` | `BreachChecker` interface. `HIBPChecker` (k-anonymity via HIBP API) and `NoOpChecker` (offline/testing). |
| `dictionary.go` | ~200 common passwords, loaded into `map[string]struct{}` via `sync.Once`. Case-insensitive O(1) lookup. |
| `wordlist.go` | EFF Large Wordlist (7776 words) loaded via `//go:embed` + `sync.Once`. Used only by `GeneratePassphrase()`. |

### Cross-file relationships

- `scorer.go` defines `leetMap` (letter→leet); `rotator.go` defines `reverseLeet` (leet→letter) as its inverse. Changes to one must be reflected in the other.
- `ScoreResult` (config.go) has `MarkBreached()` method that caps score and sets the breach flag — used by CLI's `check` command after HIBP lookup.
- `Suggest()` (suggester.go) takes the output of `Score()` (scorer.go) — the `ScoreResult.Penalties` slice drives which suggestions are returned.

### Data flow

Config structs flow in → core functions process → result structs flow out. The core has zero knowledge of how it's consumed (CLI, web, or desktop). The only external network call is the optional HIBP breach check.

### CLI commands and exit codes

Six subcommands: `generate`, `passphrase`, `check`, `suggest`, `rotate` (alias: `ssbd`), `bulk`. Global `--json` flag on all commands. Exit codes: 0 = strong, 1 = weak, 2 = breached, 3 = operational error.

## Key Conventions

- All randomness uses `crypto/rand` (never `math/rand`) — see `cryptoRandInt()` in generator.go
- Scoring constants are named in `config.go` (e.g., `DictionaryPenalty`, `WeakThreshold`) — reference these instead of magic numbers
- Embedded data (wordlist, dictionary) uses `sync.Once` for lazy loading
- Tests live alongside source files in `internal/core/` and `cmd/passforge/`
- The `internal/` directory prevents external imports — all public API is through the CLI
- Error handling: sentinel errors in `errors.go`, format strings prefixed `Msg*`, always wrap with `%w`
- Config structs have `Validate()` methods — call before passing to core functions
- Password input: `getPassword(args)` handles CLI arg, stdin pipe, and hidden terminal prompt
- CLI commands receive `*bool` pointer to `jsonOutput` — no global mutable state

## Project Status

v0.1.6 (Security Hardening) is complete. Current work: v0.3.0 Password Vault (design spec on `feature/vault-password-tracking`). See `docs/WORKPLAN.md` for detailed items and `README.md` roadmap table for the full version plan.

## Test Coverage

- `internal/core/`: ~87% coverage
- `cmd/passforge/`: ~40% coverage (CLI integration tests for rotate/ssbd commands)

## Design Docs

| File | Contents |
|---|---|
| `docs/PLAN.md` | Implementation plan + codebase audit (March 2026) — architecture, milestones, feature tiers, design decisions |
| `docs/WORKPLAN.md` | Detailed work items and known issues |
| `docs/specs/2026-03-23-password-vault-design.md` | Password Vault v0.3.0 design spec — AES-256-GCM + Argon2id vault, history hashing, sync interface |

### Version number locations

When inserting or renumbering milestones, update the roadmap tables in ALL of: `README.md`, `docs/PLAN.md` (section 10), and `CLAUDE.md`. The `docs/arch.md` Go version must also track `go.mod`.
