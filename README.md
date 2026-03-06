# PassForge

### *"Not just your ordinary password generator — we judge yours too."*

> **Same Same But Different** — PassForge's signature rotation engine. One strong base password, many unique variants. Leet-speak cycling, case flips, symbol shifts — each variant passes "must differ" checks while keeping the same muscle memory.
> ```
> p@sSwor4  →  P@sswor4  →  pAs$wor4  →  p@ssWor4  →  pa$Swor4
> ```

---

A password toolkit that generates strong passwords, scores their strength, and provides actionable suggestions to improve weak ones. Built in Go. No passwords stored. Ever.

## Problem

Somewhere out there, someone's bank account is protected by `password123`. Breached-password databases contain **billions** of entries, and the most common passwords persist year after year.

PassForge combines four capabilities in one tool:

1. **Generate** — cryptographically random passwords and passphrases
2. **Check** — score any password's strength against entropy, patterns, and dictionary checks
3. **Suggest** — offer concrete, minimal edits to strengthen a weak password
4. **Rotate** — "Same Same But Different" — generate a sequence of password variants for forced rotation policies

## Quick Start

```bash
passforge generate --length 20              # Random 20-char password
passforge passphrase --words 4              # 4-word passphrase
passforge check "MyP@ssw0rd"               # Score: 42/100 — Fair
passforge check --breach "password123"     # Check against HIBP
passforge suggest "hello123"               # Improvement suggestions
passforge rotate "p@sSwor4" --count 5      # 5 rotation variants
passforge ssbd "p@sSwor4" --count 5        # ^ alias for rotate
passforge rotate "p@sSwor4" -n 5 --min-length 8 --max-length 11  # variable-length variants
passforge bulk --count 10 --length 16      # 10 passwords at once
passforge generate --json                  # JSON output
```

> For setup, build, and run instructions see **[arch.md](docs/arch.md#setup)**.
> Quick dev: `make build`, `make test`, `make bench`, `make all` — see `make help` for all targets.
> Or run any command above directly: `make run ARGS="generate --length 20"`

## Features

### Core (M1 — CLI)

| Feature | Description |
|---|---|
| Random generation | Configurable length (8-128), character classes, exclusion lists |
| Passphrase mode | `correct-horse-battery-staple` style via EFF wordlist |
| Strength scoring | 0-100 score: entropy + pattern detection + dictionary + leet-speak normalization |
| Breach check | Optional k-anonymity check against Have I Been Pwned (only sends first 5 chars of SHA-1) |
| Suggestion engine | Ranked suggestions to improve weak passwords |
| Rotation variants | **Same Same But Different** — generate password variants for forced rotation cycles |
| Bulk generation | Generate N passwords at once, output as plain text, JSON, or TSV |
| Scriptable | Exit codes: 0 = strong, 1 = weak, 2 = breached |

### Web (M2 — Fiber)

| Feature | Description |
|---|---|
| Local web UI | Single-page app served by Go Fiber on `localhost:3000` |
| JSON API | `POST /api/generate`, `/api/check`, `/api/suggest` |
| Strength meter | Animated color bar with real-time scoring |
| Single binary | Static assets embedded via `//go:embed` |

### Desktop (M3 — Fyne)

| Feature | Description |
|---|---|
| Native GUI | Cross-platform via Fyne toolkit |
| Clipboard | Copy with auto-clear timer |
| Single binary | No runtime dependencies |

## Scoring Algorithm

The scoring engine combines:

1. **Shannon entropy** — bit entropy based on character pool and length
2. **Pattern penalty** — keyboard walks (`qwerty`), repeats (`aaa`), sequences (`123`, `abc`)
3. **Dictionary penalty** — checks against ~100k common passwords
4. **Leet-speak normalization** — `p@$$w0rd` is still `password`
5. **Length bonus** — exponential reward for length > 12
6. **Breach flag** — HIBP hit caps score at 10, regardless of other factors

## Same Same But Different

Every office has *that policy*: change your password every week. So you end up with `January2024!`, `February2024!`, `March2024!` — predictable, scripted, defeated by anyone who sees one of them.

PassForge's **rotation variant engine** takes a different approach. You pick one strong base password and PassForge generates a sequence of variants that cycle leet-speak substitutions, case positions, and symbol placements through the password:

```
p@sSwor4  →  P@sswor4  →  pAs$wor4  →  p@ssWor4  →  pa$Swor4
```

Each variant:
- Looks different from the last (passes "must differ from previous password" checks)
- Scores well individually (variants below a strength threshold are rejected)
- Stays recognizable to you (same base structure, same muscle memory)

```bash
passforge rotate "p@sSwor4" --count 5     # Next 5 variants
passforge rotate "p@sSwor4" --count 12    # A full quarter of weekly rotations
```

Built for the real world: standalone office machines, legacy systems with forced rotation, air-gapped environments. Secure, lazy, smart.

**Variable-length variants** — with `--min-length` / `--max-length`, variants can grow or shrink by 1–3 characters via insertions, appends, prepends, or repeat-dropping. Use `--strict-length` to lock all variants to the base length.

```bash
passforge rotate "p@sSwor4" --count 5 --min-length 8 --max-length 11
passforge rotate "p@sSwor4" --count 5 --strict-length
```

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Language | **Go** | Single binary, fast compilation, excellent stdlib (crypto/rand, net/http) |
| CLI | `cobra` | Mature, subcommands, shell completions |
| Web | [Fiber v3](https://gofiber.io/) | Fasthttp-based, Express-like API |
| Desktop | [Fyne](https://fyne.io/) | Pure Go GUI toolkit, cross-platform |
| Future | Rust, Zig, Odin | WASM web version, Tauri desktop, experimental rewrites |

## Documentation

| File | What it covers |
|---|---|
| [arch.md](docs/arch.md) | Full directory structure, file map, setup/build/run instructions, architecture diagram |
| [help.md](docs/help.md) | How the internals work — generation, scoring, suggestions, breach checking |
| [help_ext.md](docs/help_ext.md) | External package reference — cobra, pflag, and notable stdlib usage |
| [man.md](docs/man.md) | Detailed line-by-line source code documentation |
| [PLAN.md](docs/PLAN.md) | Implementation plan, platform strategy, milestones, risk register |

## 🔮 Roadmap

| Version | Milestone | Status | Features | Estimate |
|---|---|---|---|---|
| **v0.1.0** | Core + CLI | 🟢 Done | Password & passphrase generation, strength scoring (entropy, patterns, dictionary, leet-speak), suggestion engine, HIBP breach check, JSON output, bulk generation, scriptable exit codes | — |
| **v0.1.5** | Same Same But Different | 🟢 Done | Rotation variant engine (`passforge rotate` / `ssbd`), variable-length variants (`--min-length`/`--max-length`/`--strict-length`) | — |
| **v0.1.6** | Security Hardening | 🔵 Next | Stdin/prompt password input (no process-table exposure), HIBP hard-fail on `--breach` errors, `io.LimitReader` on HIBP response, keyboard walk penalty fix, `ScoreResult.MarkBreached()` method, rune-based entropy scoring, distinct exit codes, typed errors in rotator, config `Validate()` methods | ~1 week |
| **v0.2.0** | CLI Polish | ⚪ Planned | Password improvement command (`passforge improve`), scoring gate (`--min-score` on rotate), CI pipeline (fmt, vet, staticcheck, test matrix), GoReleaser, Homebrew tap, shell completions, expanded dictionary (~100k SecLists) | ~1 week |
| **v0.3.0** | Web (Fiber) | ⚪ Planned | Fiber v3 server, JSON API (`/api/generate`, `/api/check`, `/api/suggest`), static SPA frontend (HTML + JS/htmx), real-time strength meter, embedded assets via `//go:embed`, Docker image | ~2 weeks |
| **v0.4.0** | Desktop (Fyne) | ⚪ Planned | Fyne GUI app, Generate / Check / Suggest tabs, clipboard copy with auto-clear timer, cross-platform packaging (.app, .exe, .tar.gz) | ~2 weeks |
| **v0.5.0** | Extras | ⚪ Planned | QR code output, pronounceable password mode, custom charset templates, TUI mode (bubbletea), man page generation | ~2 weeks |
| **v1.0.0** | Stable Release | ⚪ Planned | API freeze, full test coverage, documentation site, security audit, stable binary distribution across all channels | ~1 month |
| **v2.0.0** | Rust / WASM | 🔮 Future | Rust core rewrite, WASM compilation for client-only web (zero server), Tauri desktop app | TBD |
| **v3.0.0** | Polyglot | 🔮 Future | Zig and Odin implementations, cross-language performance benchmarks | TBD |

### Legend

| Icon | Meaning |
|---|---|
| 🟢 | Complete |
| 🔵 | Up next |
| ⚪ | Planned |
| 🔮 | Future / experimental |

> For the full implementation plan, architecture decisions, and risk register, see [PLAN.md](docs/PLAN.md).

## License

MIT — because good security tools should be free and auditable.

## Contributing

Contributions welcome. See [PLAN.md](docs/PLAN.md) for the roadmap and architecture. Bring your own entropy.
