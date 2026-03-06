# PassForge - Implementation Plan & Recommendations

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

## Table of Contents

- [1. Platform Strategy](#1-platform-strategy)
- [2. Architecture & Implementation Plan](#2-architecture--implementation-plan)
- [3. Deployment & Distribution](#3-deployment--distribution)
- [4. Maintenance Strategy](#4-maintenance-strategy)
- [5. Use Cases](#5-use-cases)
- [6. Stretch Goals](#6-stretch-goals)
- [7. Milestone Roadmap](#7-milestone-roadmap)
- [8. Risk Register](#8-risk-register)
- [9. Code Review Findings](#9-code-review-findings)

---

## 1. Platform Strategy

| Platform | Priority | Language | Framework | Status |
|---|---|---|---|---|
| **CLI** | **P0** | Go | `cobra` | **Shipped** |
| **Web** | **P1** | Go | [Fiber v3](https://gofiber.io/) | Planned |
| **Desktop** | **P2** | Go | [Fyne](https://fyne.io/) | Planned |
| **Rust / WASM** | **Future** | Rust | clap / WASM | Future |
| **Other languages** | **Future** | Zig, Odin | TBD | Future |

**Why Go first?** Single binary, zero dependencies, fast compile times, excellent stdlib for crypto/rand, net/http, SHA-1. Cross-compilation is trivial.

**Why not a server-side password store?** Passwords should never leave the user's device. All processing is local and stateless.

---

## 2. Architecture & Implementation Plan

### 2.1 Core Library (`internal/core`)

Shared Go package consumed by CLI, web, and desktop.

```
internal/core/
    config.go        // Global constants, config structs, defaults
    generator.go     // Random password + passphrase generation (crypto/rand)
    scorer.go        // Strength scoring (entropy, patterns, dictionary, leet-speak)
    dictionary.go    // Common password list (~200 entries, O(1) lookup via sync.Once)
    suggester.go     // Actionable improvement suggestions
    rotator.go       // "Same Same But Different" rotation variant engine
    hibp.go          // HIBP k-anonymity breach check (BreachChecker interface)
    wordlist.go      // Embedded EFF wordlist (//go:embed, sync.Once)
```

**Key design decisions:**

1. **Config-driven.** All defaults live as named constants in `config.go` — no magic numbers in logic code.
2. **`crypto/rand`** for cryptographic randomness everywhere.
3. **Dictionary embedded at compile time** via `//go:embed`. No runtime file I/O.
4. **HIBP behind an interface** — `BreachChecker` with `HIBPChecker` (real) and `NoOpChecker` (offline/testing).
5. **Deterministic rotation** — `Rotate()` uses mixed-radix enumeration for reproducible variant sequences. `RotateWithConfig()` extends this with variable-length mutations (insert, append, prepend, drop-repeat) constrained by `MinLength`/`MaxLength`.

### 2.2 CLI (`cmd/passforge`)

Built with `cobra`. Six subcommands:

| Command | Core function | Description |
|---|---|---|
| `generate` | `core.Generate(cfg)` | Random password |
| `passphrase` | `core.GeneratePassphrase(cfg)` | EFF wordlist passphrase |
| `check` | `core.Score(pw)` + optional HIBP | Strength check |
| `suggest` | `core.Score(pw)` → suggestions | Improvement tips |
| `rotate` / `ssbd` | `core.RotateWithConfig(pw, cfg)` | Same Same But Different variants |
| `bulk` | `core.Generate(cfg)` in loop | Multiple passwords |

- Output: plain text (default) or JSON (`--json`)
- Exit codes: `0` = strong, `1` = weak, `2` = breached
- `Makefile` provides `make <command> ARGS="..."` shortcuts for all subcommands

### 2.3 Web (`cmd/passforge-web`) — Planned

- Fiber v3 server, JSON API, static SPA frontend
- Localhost only by default, no auth/cookies/analytics
- Strength meter with animated color bar

### 2.4 Desktop (`cmd/passforge-desktop`) — Planned

- Fyne app, Generate/Check/Suggest tabs
- Clipboard with auto-clear timer
- Single binary, cross-platform

### 2.5 Future: Rust / WASM

- Rust core rewrite → WASM for client-only web
- Tauri desktop app

---

## 3. Deployment & Distribution

### CLI

| Channel | Method |
|---|---|
| GitHub Releases | Cross-compiled binaries via GoReleaser |
| Homebrew | `brew install passforge/tap/passforge` |
| Go install | `go install github.com/<user>/passforge/cmd/passforge@latest` |
| Docker | Minimal scratch-based image |

### CI Pipeline (planned)

```
on push to main:
  → go fmt / go vet / staticcheck
  → go test ./...
  → go build (linux, macos, windows × amd64, arm64)
  → goreleaser on tag
```

---

## 4. Maintenance Strategy

- **Dependencies:** `govulncheck` in CI, Dependabot, minimal dependency tree
- **Wordlist:** EFF list is stable (2016). Common-password list: refresh annually from SecLists
- **HIBP API:** Stable and free. Graceful degradation if unreachable
- **Versioning:** Semantic versioning, single version across all platforms
- **Testing:** Unit + table-driven tests, integration test (`Score(Generate(config))` >= 70), benchmarks for hot paths

---

## 5. Use Cases

| Audience | Scenario | Example |
|---|---|---|
| Personal | Generate strong password | `passforge generate -l 20` |
| Personal | Audit existing passwords | `passforge check "MyP@ssw0rd"` |
| Personal | Forced rotation | `passforge rotate "p@sSwor4" --count 12` |
| Developer | Generate secrets in scripts | `API_KEY=$(passforge generate -l 32)` |
| Developer | Pre-commit hook | `passforge check "$DB_PASSWORD" \|\| exit 1` |
| Enterprise | Policy enforcement | Embed scorer in internal tools |
| Enterprise | Security training | Host web version internally |

---

## 6. Stretch Goals

Nice-to-have ideas **not committed to any milestone**.

- **Improve Command** — `passforge improve "hello123"` → strengthened variant preserving structure
- **Browser Extension** — auto-detect password fields, generate and fill
- **TUI mode** — interactive terminal UI via `bubbletea`
- **Multilingual Passphrases** — wordlists in multiple languages
- **Password Manager Integration** — CLI plugin for Bitwarden/1Password

---

## 7. Milestone Roadmap

### M1: Core + CLI (Go) — Done

- [x] Go module and project structure
- [x] `generator.go` — random password + passphrase generation
- [x] `scorer.go` — entropy, pattern detection, dictionary check, leet-speak normalization
- [x] `suggester.go` — suggestion engine
- [x] `hibp.go` — k-anonymity breach check
- [x] `config.go` — centralized constants and config structs
- [x] CLI — all subcommands via cobra (`generate`, `passphrase`, `check`, `suggest`, `bulk`)
- [x] Unit + table-driven tests (85%+ coverage on core)
- [x] `Makefile` with build, test, bench, vet, fmt, and per-command run targets
- [x] `clean` target — `go clean --cache && rm -f passforge`
  - Clears the Go build cache and removes the compiled `passforge` binary
  - `all-clean` composite target runs `clean` then `all` (vet + test + bench) for a full reset-and-verify cycle
  - ⚠️ **Windows note:** `rm -f` is a Unix command. On PowerShell it works via the `Remove-Item` alias, but `-f` is interpreted as `-Force`, which is compatible. The `&&` operator requires PowerShell 7+. For broader Windows compat, consider `go clean --cache; if ($?) { Remove-Item -Force -ErrorAction SilentlyContinue passforge }` or use `cmd /c` in the Makefile.

### M1.5: SSBD + CLI Extras (Go) — Done

- [x] `rotator.go` — rotation variant engine ("Same Same But Different")
  - Substitution-map cycling (leet/case/symbol)
  - Mixed-radix enumeration for deterministic variant generation
  - Dedup: no two variants in a sequence are identical
- [x] CLI: `passforge rotate <password> --count N` (alias: `ssbd`)
- [x] Unit tests for rotator (uniqueness, dedup, edge cases, alias)
- [x] **SSBD v2 — variable-length variants**
  - `--min-length` / `--max-length` flags to constrain variant output length
  - Length-changing mutations: insert random chars, append/prepend symbols, drop redundant repeats
  - Variants can grow or shrink by 1-3 chars within bounds
  - `--strict-length` flag for exact-length matching (current behavior, backward compat)

### M1.6: Security Hardening (from [Code Review](Claude-review.md))

- [x] **P1 — Critical: Stdin password input** — read from stdin or prompt with hidden echo to avoid process-table exposure (`cmd/passforge/main.go`)
- [x] **P2 — Critical: HIBP hard-fail** — when `--breach` is explicit and the check fails, exit with code 3 instead of degrading to "not breached." Add `--breach-warn-only` for soft-failure opt-in (`cmd/passforge/main.go`)
- [x] **P3 — High: `io.LimitReader` on HIBP response** — cap response read at 1 MiB to prevent OOM from malicious server (`internal/core/hibp.go`)
- [x] **P4 — High: Keyboard walk penalty iteration** — iterate largest-to-smallest window to catch `qwertyuiop` as a large walk, not just `qwer` as a small one (`internal/core/scorer.go`)
- [x] **P5 — Medium: `ScoreResult.MarkBreached()` method** — atomic breach marking to prevent inconsistent Score/Label pairs (`internal/core/scorer.go`)
- [x] **P6 — High: Rune-based entropy scoring** — use `utf8.RuneCountInString()` instead of `len()` in entropy calculation and suggester (`internal/core/scorer.go`, `internal/core/suggester.go`)
- [x] **P7 — Medium: Distinct exit codes** — 0 = strong, 1 = weak, 2 = breached, 3 = operational error; return typed errors from `RunE`, handle exit codes in `main()` (`cmd/passforge/main.go`)
- [x] **P8 — Low: Wordlist size validation** — validate minimum word count after parsing to catch silent data loss (`internal/core/wordlist.go`)
- [x] **P9 — Medium: Typed errors in rotator** — distinguish recoverable constraint errors from fatal `crypto/rand` failures (`internal/core/rotator.go`)
- [x] **P10 — Medium: Config `Validate()` methods** — add validation to `GeneratorConfig`, `PassphraseConfig`, `RotateConfig` to catch zero-value and invalid states early
- [x] Fix `SymbolPoolSize` (32 → 31) (`internal/core/scorer.go`)
- [x] Remove `normalizeBase` dead code or move to test file (`internal/core/rotator.go`)
- [x] Move `NoOpChecker` to test file (`internal/core/hibp.go`)
- [x] Eliminate global `jsonOutput` var — pass via command context or struct (`cmd/passforge/main.go`)
- [x] Boolean flag UX — consider `--no-upper`, `--no-lower` pattern for disabling defaults

### M1.x: CLI Polish

- [ ] `improve.go` — password improvement engine
  - Preserve recognizable structure, inject missing classes, extend length, break patterns
  - CLI: `passforge improve <password>`
- [ ] Scoring gate — reject variants that fall below a configurable strength threshold
  - Filter `Rotate()` output through `Score()`, keep only variants >= threshold
  - CLI: `--min-score` flag on `rotate` command
- [ ] Benchmarks for all algos
- [ ] CI pipeline (fmt, vet, staticcheck, test, build matrix)
- [ ] GoReleaser for cross-compiled binaries
- [ ] Homebrew tap
- [ ] Expanded dictionary (~100k entries from SecLists)
- [ ] Shell completions (bash/zsh/fish)

### M2: Web (Go + Fiber)

- [ ] Fiber v3 server with JSON API
- [ ] Static SPA frontend (HTML + JS/htmx)
- [ ] Strength meter animation
- [ ] Embedded static assets via `//go:embed`
- [ ] Docker image

### M3: Desktop (Go + Fyne)

- [ ] Fyne app scaffold
- [ ] Generate / Check / Suggest / Rotate tabs
- [ ] Clipboard integration with auto-clear
- [ ] Cross-platform packaging

### M4: Rust / WASM (Future)

- [ ] Rust core rewrite, WASM compilation
- [ ] Client-side-only web version
- [ ] Tauri desktop app

### M5: Polyglot (Future)

- [ ] Zig implementation
- [ ] Odin implementation
- [ ] Cross-language performance benchmarks

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| HIBP API changes/downtime | Low | Low | Graceful degradation; breach check is optional |
| Users treat PassForge as a vault | Medium | High | Prominent "we do not store passwords" messaging |
| Dictionary bloats binary | Medium | Medium | Compression + `//go:embed`; cap at ~100k entries |
| Supply chain attack | Low | Critical | `govulncheck` in CI; minimal dependencies |
| Fyne UI non-native feel | Medium | Low | Acceptable; can revisit with Wails/Tauri |

---

## 9. Code Review Findings

A full automated code review was performed on 2026-03-05 using three specialized agents (code review, silent failure analysis, type design analysis). The review identified 2 critical, 3 high, 6 medium, and 4 low severity issues.

The findings are tracked as **M1.6: Security Hardening** in the roadmap above. For the complete review with code samples, recommendations, and type design ratings, see [Claude-review.md](Claude-review.md).

**Key themes:**
- **Security** — passwords visible in process args, HIBP failures silently downgraded
- **Correctness** — byte-vs-rune length, keyboard walk penalty returns early
- **Robustness** — unbounded HTTP reads, crypto errors swallowed, config zero-values are invalid
- **Code quality** — global mutable state, mock in production code, dead code

**Positive observations:** crypto usage is correct, k-anonymity for HIBP, embedded wordlist, `sync.Once` lazy loading, clean architecture, 85%+ test coverage on core.

---

## Summary

1. **Go CLI shipped.** Core library + 6 commands + SSBD rotation engine.
2. **Security hardening next** — address critical/high findings from code review (M1.6).
3. **CLI polish** — `improve` command, scoring gate, CI pipeline (M1.x/M2.0).
4. **Fiber for web** — next major feature milestone.
5. **Fyne for desktop** — after web.
6. **Rust/WASM deferred** — Go proves the product first.
7. **Never store passwords.** Stateless generator and checker, not a vault.
8. **Open source.** MIT license.
