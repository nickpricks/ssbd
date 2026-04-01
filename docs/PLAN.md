# PassForge - Implementation Plan & Recommendations

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

## Table of Contents

- [1. Platform Strategy](#1-platform-strategy)
- [2. Architecture & Implementation Plan](#2-architecture--implementation-plan)
- [3. Deployment & Distribution](#3-deployment--distribution)
- [4. Maintenance Strategy](#4-maintenance-strategy)
- [5. Use Cases](#5-use-cases)
- [6. Codebase Audit (March 2026)](#6-codebase-audit-march-2026)
- [7. Feature Additions (Tiered)](#7-feature-additions-tiered)
- [8. Personal vs Public Use Case](#8-personal-vs-public-use-case)
- [9. Open Design Decisions](#9-open-design-decisions)
- [10. Milestone Roadmap](#10-milestone-roadmap)
- [11. Risk Register](#11-risk-register)
- [12. Code Review Findings](#12-code-review-findings)

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

**Why not a server-side password store?** Passwords should never leave the user's device in plaintext. The vault (v0.3.0) stores encrypted blobs locally; cloud sync sees only ciphertext.

### Desktop Framework Comparison

| Approach | Pros | Cons |
|----------|------|------|
| **Fyne** (current plan) | Single binary, Go-only | Non-native feel, limited widgets |
| **Wails** (recommended) | Native OS chrome + web UI, smaller than Electron | Requires HTML/CSS/JS frontend |
| **Tauri** | Smallest binaries, Rust-powered | Requires Rust + JS, two ecosystems |

### Browser Extension — Two Paths

| Path | Approach | Bundle Size | Maintenance |
|------|----------|-------------|-------------|
| **A: Go WASM** | Compile Go core to WASM | ~5-10MB | Single codebase |
| **B: TypeScript** | Rewrite core in TS | <100KB | Two codebases |

**Recommendation**: Start with WASM for correctness, optimize if size matters. Use popup-only mode (no content scripts) to avoid scary permissions.

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
- Exit codes: `0` = strong, `1` = weak, `2` = breached, `3` = operational error
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

## 6. Codebase Audit (March 2026)

> Sections 6–9 were consolidated from `PLAUDE.md` (Plan + Claude audit document, March 2026).

**Audit date**: 2026-03-23
**Codebase**: 2,652 lines across 15 Go files, 72 tests, 86.8% core coverage

| Metric | Value |
|--------|-------|
| Source files | 15 (.go) |
| Total lines | ~2,652 |
| Test count | 72 |
| Core coverage | 86.8% |
| CLI coverage | 39.8% |
| External deps | Cobra only |
| Build status | All tests pass |
| Race conditions | None detected |

**Strengths**:
- Cryptographically sound (crypto/rand, Fisher-Yates, k-anonymity HIBP)
- Clean architecture (core logic fully separated from CLI)
- Sophisticated rotation algorithm (SSBD v1 + v2)
- Minimal dependency tree

**Weaknesses (at time of audit — all fixed in v0.1.6)**:
- ~~Entropy calculation bug (symbol pool 32 vs actual 30)~~ ✅ Fixed: SymbolPoolSize corrected
- ~~Sparse CLI test coverage~~ ✅ Fixed: CLI integration tests added
- No CI/release pipeline *(still outstanding — v0.2.0)*
- ~~Passwords visible in process arguments~~ ✅ Fixed: stdin/prompt input via `getPassword()`
- ~~Panic on missing embed (wordlist.go)~~ ✅ Fixed: wordlist size validation added

### What Can Be Removed

| Item | Location | Reason | Risk |
|------|----------|--------|------|
| `normalizeBase()` | `rotator.go` | Dead code, never called | ✅ **Removed in v0.1.6** |
| `cmd/passforge-web/` | Empty directory | Placeholder with no files | None — still present |
| `cmd/passforge-desktop/` | Empty directory | Placeholder with no files | None — still present |
| `improve` from v0.1.5 scope | README.md, PLAN.md | Promised but not implemented; move to future milestone | Deferred to v0.2.0 |
| Symbol pool `32` hardcode | `scorer.go` | Correction, not removal; actual charset is 31 | ✅ **Fixed in v0.1.6** |
| `NoOpChecker` in prod code | `hibp.go` | Test mock in production file | ✅ **Moved to test file in v0.1.6** |
| Global `jsonOutput` var | `main.go` | Mutable global state | ✅ **Replaced with pointer passing in v0.1.6** |

**Verdict**: Most removals completed in v0.1.6. Remaining: empty placeholder dirs, `improve` promise.

---

## 7. Feature Additions (Tiered)

### Tier 1 — High-Value, Low-Effort

| Feature | Description | Effort | Impact |
|---------|-------------|--------|--------|
| `passforge improve <pw>` | Transform weak password into strong one preserving structure | Medium | High |
| `--min-score` on `rotate` | Filter rotation output through `Score()` | Low | High |
| `--clipboard` / `-c` | Copy to clipboard + auto-clear after N seconds - without using any 3rd party | Low | High |
| ~~`--stdin` input~~ | ~~Read password from stdin instead of CLI args~~ | ~~Low~~ | ✅ **Done in v0.1.6** |
| `--quiet` / `-q` | Suppress non-essential output for scripting | Very Low | Medium |

### Tier 2 — Medium-Effort, High-Differentiation

| Feature | Description | Effort | Impact |
|---------|-------------|--------|--------|
| `passforge audit` | Batch check passwords from file/stdin | Medium | High |
| Pattern-aware generation | `--pattern "Cvcc-9999-Cvcc"` | Medium | Medium-High |
| Scoring profiles | `--profile strict/relaxed` or custom JSON | Medium | Medium |
| Shell completions | bash/zsh/fish via Cobra built-in | Very Low | Medium |
| `--version` flag | Build-time version injection via ldflags | Very Low | Low |

### Tier 3 — High-Effort, Strategic

| Feature | Description | Effort | Impact |
|---------|-------------|--------|--------|
| TUI mode | Interactive terminal UI via bubbletea | High | High |
| Browser extension | Auto-detect password fields, generate + fill | Very High | Highest reach |
| WASM build | Client-side web version, Go compiles natively | Medium | High |

---

## 8. Personal vs Public Use Case

### Current State (updated post-v0.1.6): Personal tool with production-quality core

| Aspect | Personal Tool | Public Tool | PassForge Today |
|--------|--------------|-------------|----------------|
| Distribution | `go install` | Homebrew, Docker, Releases | No release pipeline *(v0.2.0)* |
| Error handling | Panics OK | Must never panic | ✅ Typed errors, no panics |
| Security | Trust yourself | Assume hostile env | ✅ Stdin input, HIBP hard-fail |
| Testing | Manual smoke | CI + cross-platform | Tests pass, no CI *(v0.2.0)* |
| Versioning | Git tags | SemVer + changelog | No releases *(v0.2.0)* |

### Recommended Path

```
PERSONAL (v0.x)     →     PUBLIC (v1.0)     →     PLATFORM (v2.0)
- Use daily               - CI pipeline            - Web UI
- Fix critical bugs        - GoReleaser             - Browser extension
- Add clipboard/stdin      - Security hardening     - Desktop app
- Polish CLI UX            - 90%+ coverage          - Docker image
                           - CHANGELOG + SemVer
                           - SECURITY.md
```

**Key insight**: Core library is production-quality. What's missing is trust infrastructure (CI, releases, security hardening), not features.

---

## 9. Open Design Decisions

### `improve` Command Strategy

Two approaches:

| Approach | Example | Strength | Memorability |
|----------|---------|----------|-------------|
| **Surgical** | `summer2024` to `Summer2024!` | Moderate gain | High |
| **Aggressive** | `summer2024` to `$uMm3r#2024!kQ` | Large gain | Lower |

**Decision**: TBD — consider `--strength mild/medium/strong` flag

### Password Vault Design

**Status**: Design spec complete (2026-03-23). Planned for **v0.3.0**. Branch: `feature/vault-password-tracking`.

**Spec**: `docs/specs/2026-03-23-password-vault-design.md` — 8 sections + 3 appendices covering data model, encryption (AES-256-GCM + Argon2id), CLI commands, reuse detection, sync interface, Git driver, cloud adapters, and implementation phases.

**Core tension resolved**: Current password encrypted (retrievable). History stored as SHA-256 hashes only (reuse detection, never retrievable). Cloud sees only encrypted blobs. PassForge's principle becomes: "passwords never leave your device *in plaintext*."

---

## 10. Milestone Roadmap

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

### M1.6: Security Hardening (from [Code Review](review.md)) — Done

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

### M2: Password Vault — Planned

- [ ] Encrypted JSON vault (`~/.passforge/vault.json.enc`)
- [ ] AES-256-GCM encryption + Argon2id key derivation
- [ ] Password history tracking (SHA-256 hash-only, reuse detection)
- [ ] `passforge vault` CLI commands (store, get, list, history)
- [ ] `VaultSyncer` interface (Git driver default)
- [ ] Design spec: `docs/specs/2026-03-23-password-vault-design.md`

### M3: Web (Go + Fiber)

- [ ] Fiber v3 server with JSON API
- [ ] Static SPA frontend (HTML + JS/htmx)
- [ ] Strength meter animation
- [ ] Embedded static assets via `//go:embed`
- [ ] Docker image

### M4: Desktop (Go + Fyne)

- [ ] Fyne app scaffold
- [ ] Generate / Check / Suggest / Rotate tabs
- [ ] Clipboard integration with auto-clear
- [ ] Cross-platform packaging

### M5: Rust / WASM (Future)

- [ ] Rust core rewrite, WASM compilation
- [ ] Client-side-only web version
- [ ] Tauri desktop app

### M6: Polyglot (Future)

- [ ] Zig implementation
- [ ] Odin implementation
- [ ] Cross-language performance benchmarks

---

## 11. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| HIBP API changes/downtime | Low | Low | Graceful degradation; breach check is optional |
| Users treat PassForge as a vault | Medium | High | Prominent "we do not store passwords" messaging |
| Dictionary bloats binary | Medium | Medium | Compression + `//go:embed`; cap at ~100k entries |
| Supply chain attack | Low | Critical | `govulncheck` in CI; minimal dependencies |
| Fyne UI non-native feel | Medium | Low | Acceptable; can revisit with Wails/Tauri |

---

## 12. Code Review Findings

A full automated code review was performed on 2026-03-05 using three specialized agents (code review, silent failure analysis, type design analysis). The review identified 2 critical, 3 high, 6 medium, and 4 low severity issues.

The findings are tracked as **M1.6: Security Hardening** in the roadmap above. For the complete review with code samples, recommendations, and type design ratings, see [review.md](review.md).

**Key themes:**
- **Security** — passwords visible in process args, HIBP failures silently downgraded
- **Correctness** — byte-vs-rune length, keyboard walk penalty returns early
- **Robustness** — unbounded HTTP reads, crypto errors swallowed, config zero-values are invalid
- **Code quality** — global mutable state, mock in production code, dead code

**Positive observations:** crypto usage is correct, k-anonymity for HIBP, embedded wordlist, `sync.Once` lazy loading, clean architecture, 85%+ test coverage on core.

---

## Summary

1. **Go CLI shipped.** Core library + 6 commands + SSBD rotation engine.
2. **Security hardening done** — all critical/high findings from code review resolved (M1.6).
3. **CLI polish next** — `improve` command, scoring gate, CI pipeline (M1.x).
4. **Password vault planned** — encrypted local vault with history tracking (M2).
5. **Fiber for web** — next major feature milestone after vault (M3).
6. **Fyne for desktop** — after web (M4).
7. **Rust/WASM deferred** — Go proves the product first.
8. **Passwords never leave the device in plaintext.** The vault stores encrypted blobs locally; cloud sync sees only ciphertext.
9. **Open source.** MIT license.

---

*Originally created as the implementation plan. Sections 6–9 consolidated from PLAUDE.md (codebase audit, March 2026). Updated 2026-04-01.*
