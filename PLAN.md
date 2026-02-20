# PassForge - Implementation Plan & Recommendations

## Table of Contents

- [1. Platform Strategy](#1-platform-strategy)
- [2. Architecture & Implementation Plan](#2-architecture--implementation-plan)
- [3. Deployment & Distribution](#3-deployment--distribution)
- [4. Maintenance Strategy](#4-maintenance-strategy)
- [5. Use Cases](#5-use-cases)
- [6. Arbitrary / Stretch Goals](#6-arbitrary--stretch-goals)
- [7. Milestone Roadmap](#7-milestone-roadmap)
- [8. Risk Register](#8-risk-register)

---

## 1. Platform Strategy

### Verdict: Go first, then fan out to other languages

| Platform | Priority | Language | Framework | Justification |
|---|---|---|---|---|
| **CLI / Console** | **P0 - build first** | Go | `cobra` | Fastest to ship; single binary; cross-compiles trivially |
| **Web** | **P1** | Go | [Fiber v3](https://gofiber.io/) | Express-like API on Fasthttp; serves static frontend + API |
| **Desktop** | **P2** | Go | [Fyne](https://fyne.io/) | Pure Go GUI toolkit; cross-platform; no CGo on most targets |
| **Rust rewrite** | **Future** | Rust | clap / WASM | Core library rewrite for WASM-only web, Tauri desktop |
| **Other languages** | **Future** | Zig, Odin, etc. | TBD | Educational / experimental rewrites |

**Why Go first?**

- Single binary, zero dependencies for end users.
- Fast compile times mean tight iteration loops.
- Excellent stdlib for HTTP, crypto/rand, and SHA-1 (needed for HIBP).
- Fiber gives us a production-grade web server with minimal boilerplate.
- Fyne provides native-feeling desktop apps without leaving Go.
- Cross-compilation is trivial (`GOOS=linux go build`).

**Why keep Rust/others as future milestones?**

- Rust excels at WASM — a future client-only web version with zero server.
- Zig/Odin are educational targets for exploring the design in other paradigms.
- The Go version proves the product; rewrites optimize for specific platforms.

### Why not a server-side password store?

Passwords should never leave the user's device or be persisted. The web version generates and scores passwords client-side in the browser (via JS/WASM) or via a local-only Fiber server. There is zero reason for a remote backend.

---

## 2. Architecture & Implementation Plan

### 2.1 Core Library (`internal/core`)

Shared Go package consumed by CLI, web, and desktop.

```
internal/
  core/
    generator.go     // Random password + passphrase generation
    scorer.go        // Strength scoring (entropy, patterns, dictionary)
    suggester.go     // Suggestion engine
    hibp.go          // HIBP k-anonymity check
    wordlist.go      // Embedded EFF wordlist (embed directive)
    config.go        // GeneratorConfig, ScorerConfig structs
```

**Key design decisions:**

1. **No global state.** Every function takes a config struct and returns a result.
2. **`crypto/rand`** for cryptographic randomness everywhere.
3. **Dictionary embedded at compile time** via `//go:embed` (~300KB compressed). No runtime file I/O.
4. **HIBP is behind an interface** `BreachChecker` so it can be stubbed in tests and swapped for a no-op offline.

### 2.2 CLI (`cmd/passforge`)

- Built with `cobra`.
- Subcommands: `generate`, `passphrase`, `check`, `suggest`, `bulk`.
- Output formats: plain text (default), JSON (`--json`), TSV (`--tsv`).
- Shell completions via cobra's built-in generation for bash/zsh/fish.
- Exit codes: 0 = strong, 1 = weak, 2 = breached (useful for scripting).

### 2.3 Web (`cmd/passforge-web`)

- Go Fiber v3 server serving a static SPA frontend + JSON API.
- API endpoints: `POST /api/generate`, `POST /api/check`, `POST /api/suggest`.
- Frontend: single HTML file + vanilla JS (or htmx for interactivity).
- Strength meter: animated color bar (red -> yellow -> green).
- Designed to run locally (`localhost:3000`). No auth, no cookies, no analytics.
- Can also be deployed as a self-hosted instance.

### 2.4 Desktop (`cmd/passforge-desktop`)

- Fyne app using the core library directly (no HTTP layer needed).
- Features: generate tab, check tab, suggest tab.
- Clipboard integration with auto-clear timer.
- System tray icon (if Fyne supports it via extensions).
- Single binary, cross-platform.

### 2.5 Future: Rust / WASM

- Rewrite `internal/core` in Rust as `passforge-core` crate.
- Compile to WASM for a fully client-side web version (no server).
- Tauri desktop app wrapping the WASM frontend.
- This becomes the "v2" architecture.

---

## 3. Deployment & Distribution

### 3.1 CLI

| Channel | Method |
|---|---|
| GitHub Releases | Cross-compiled binaries via GoReleaser (linux, macos, windows, arm64) |
| Homebrew | Tap formula: `brew install passforge/tap/passforge` |
| Go install | `go install github.com/<user>/passforge/cmd/passforge@latest` |
| AUR | PKGBUILD for Arch users |
| Docker | Minimal scratch-based image |

**CI pipeline (GitHub Actions):**

```
on push to main:
  -> go fmt / go vet / staticcheck
  -> go test ./...
  -> go build (matrix: linux, macos, windows x amd64, arm64)
  -> goreleaser on tag
```

### 3.2 Web

- `go build cmd/passforge-web` produces a single binary with embedded static assets.
- Deploy anywhere: Docker, fly.io, systemd service, etc.
- Or run locally: `passforge-web` starts on `localhost:3000`.

### 3.3 Desktop

- `fyne package` produces `.app` (macOS), `.exe` (Windows), `.tar.gz` (Linux).
- Distribute via GitHub Releases.

---

## 4. Maintenance Strategy

### 4.1 Dependencies

- Run `govulncheck` in CI to catch known vulnerabilities.
- Dependabot for automated dependency PRs.
- Keep dependency tree minimal — stdlib is preferred over third-party.

### 4.2 Wordlist Updates

- EFF wordlist is stable (last updated 2016). No frequent updates needed.
- Common-password dictionary: refresh annually from public breach compilations (SecLists).

### 4.3 HIBP API

- The API is stable and free. Monitor for breaking changes via their blog.
- Implement retry with exponential backoff. Degrade gracefully if the API is down.

### 4.4 Versioning

- Semantic versioning (`MAJOR.MINOR.PATCH`).
- Single version across CLI/web/desktop.

### 4.5 Testing

- Unit tests for every scoring rule and generation mode.
- Table-driven tests (Go convention) for the generator and scorer.
- Integration test: `Score(Generate(config))` should always return >= 70 for default config.
- Benchmark tests for hot paths (generation, scoring).

---

## 5. Use Cases

### 5.1 Individual / Personal

| Scenario | How PassForge helps |
|---|---|
| Creating accounts | Generate a strong unique password, copy to clipboard |
| Auditing existing passwords | Paste each password into `check` to find weak ones |
| Migrating to a password manager | `passforge bulk --count 50 --json` to seed a manager's import |
| Offline environments | CLI works without internet; HIBP check is optional |

### 5.2 Developer / DevOps

| Scenario | How PassForge helps |
|---|---|
| Generating secrets in scripts | `API_KEY=$(passforge generate -l 32 --charset hex)` |
| CI/CD secret rotation | Integrate into pipeline scripts |
| Pre-commit hook | `passforge check "$DB_PASSWORD" || echo "weak password detected"` |
| Generating JWT secrets | `passforge generate -l 64 --charset base64` |

### 5.3 Enterprise / Team

| Scenario | How PassForge helps |
|---|---|
| Password policy enforcement | Embed the scorer in internal tools; reject passwords below threshold |
| Security training | Host the web version internally as a teaching tool |
| Compliance | CSPRNG generation, NIST-aligned scoring |

---

## 6. Arbitrary / Stretch Goals

These are nice-to-have ideas **not committed to any milestone**.

- **Rotation Variants ("Same Same But Different")** — Given a base password, generate a sequence of rotation variants that shift leet-speak substitutions, case patterns, and symbol positions across cycles. Designed for environments with forced periodic password changes (e.g., weekly rotation on a standalone office machine). The user memorizes one base pattern; PassForge produces the next variant in the sequence. Example: `p@sSwor4` → `P@sswor4` → `pAs$wor4` → `p@ssWor4` → ... Each variant scores well individually and is visually distinct from the previous one, but stays recognizable to the user. CLI: `passforge rotate "p@sSwor4" --count 5`. Core: `rotator.go` — substitution-map cycling, positional case/symbol shifting, dedup, scoring gate (reject variants that score below threshold).
- **Improve Command** — Takes a weak password and returns a strengthened variant that preserves the recognizable structure but patches weaknesses (injects missing character classes, extends length, breaks patterns). CLI: `passforge improve "hello123"` → `hE!lo1&23xKm`.
- **Browser Extension** — Auto-detect password fields, generate and fill.
- **Python/Node bindings** — Via CGo + shared library or gRPC.
- **Password Manager Integration** — CLI plugin for Bitwarden/1Password CLI.
- **Gamification** — "Password strength challenge" in the web UI.
- **Multilingual Passphrases** — Wordlists in multiple languages.
- **TUI mode** — Interactive terminal UI via `bubbletea` or `charm`.

---

## 7. Milestone Roadmap

### M1: Core + CLI (Go) ← CURRENT

- [ ] Scaffold Go module and project structure
- [ ] Implement `generator.go` — random password generation
- [ ] Implement `generator.go` — passphrase generation with EFF wordlist
- [ ] Implement `scorer.go` — entropy calculation
- [ ] Implement `scorer.go` — pattern detection (keyboard walks, sequences, repeats)
- [ ] Implement `scorer.go` — dictionary check (embedded, compressed)
- [ ] Implement `scorer.go` — leet-speak normalization
- [ ] Implement `suggester.go` — suggestion engine
- [ ] Implement `hibp.go` — k-anonymity breach check
- [ ] Implement CLI — all subcommands via cobra
- [ ] Write unit + table-driven tests (>90% coverage on core)
- [ ] CI pipeline (fmt, vet, staticcheck, test, build matrix)
- [ ] GitHub Release automation (GoReleaser)
- [ ] Homebrew tap

### M1.5: CLI Extras (Go)

- [ ] Implement `rotator.go` — rotation variant engine ("Same Same But Different")
  - Substitution-map cycling: rotate which characters get leet/case/symbol treatment
  - Positional shifting: move the substitution window across the base password each cycle
  - Dedup: ensure no two variants in a sequence are identical
  - Scoring gate: reject variants that fall below a configurable strength threshold
- [ ] Implement `improve.go` — password improvement engine
  - Preserve recognizable structure of the input password
  - Inject missing character classes, extend length, break detected patterns
- [ ] CLI: `passforge rotate <password> --count N` — generate N rotation variants
- [ ] CLI: `passforge improve <password>` — return a strengthened variant
- [ ] Unit tests for rotator and improve (table-driven, coverage for edge cases)

### M2: Web (Go + Fiber)

- [ ] Fiber v3 server with JSON API
- [ ] Static SPA frontend (HTML + JS or htmx)
- [ ] Strength meter animation
- [ ] Embedded static assets via `//go:embed`
- [ ] Docker image

### M3: Desktop (Go + Fyne)

- [ ] Fyne app scaffold
- [ ] Generate / Check / Suggest tabs
- [ ] Clipboard integration with auto-clear
- [ ] Cross-platform packaging

### M4: Rust / WASM rewrite (Future)

- [ ] Rust workspace (`passforge-core`, `passforge-cli`)
- [ ] WASM compilation via wasm-pack
- [ ] Client-side-only web version
- [ ] Tauri desktop app

### M5: Other languages (Future)

- [ ] Zig implementation
- [ ] Odin implementation
- [ ] Performance comparison across implementations

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| HIBP API goes down or changes | Low | Low | Graceful degradation; breach check is optional |
| Users trust PassForge as a password manager | Medium | High | Prominent "we do not store passwords" messaging |
| Dictionary file bloats binary size | Medium | Medium | Compression + `//go:embed`; keep list under 100k entries |
| Supply chain attack on dependencies | Low | Critical | `govulncheck` in CI; minimal dependency tree |
| Fyne UI looks non-native | Medium | Low | Acceptable tradeoff; can revisit with Wails/Tauri later |
| Fiber v3 breaking changes | Low | Medium | Pin version; Fiber has stable release cadence |

---

## Summary

1. **Start with Go CLI.** It forces a clean library API and ships fast.
2. **Use Fiber for web.** Fast, Express-like, serves embedded SPA + API.
3. **Use Fyne for desktop.** Pure Go, cross-platform, single binary.
4. **Defer Rust/WASM.** Go proves the product; Rust optimizes for client-only web later.
5. **Never store passwords.** Generator and checker, not a vault. Stateless.
6. **Open source it.** MIT license, public repo.
