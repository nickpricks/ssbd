# WORKPLAN — PassForge Documentation & Codebase Overhaul

---

## Tracker

| # | Task | Status |
|---|------|--------|
| 1 | Move all `.md` files into `docs/` directory | done |
| 2 | Read all `.md` files to get full project overview | done |
| 3 | Move "Same Same But Different" (SSBD) algo to the top of all README and docs | done |
| 4 | Analyze codebase for updates, improvements, and necessary fixes | done |
| 5 | Update `man.md` with all new content and changes made | done |
| 6 | Update `man.md` with memory analysis (structs, constants, functions, line-by-line) | done |

---

## Details

### 1. Move `.md` files into `docs/` (done)
- Created `docs/` directory
- Moved: `arch.md`, `help.md`, `help_ext.md`, `man.md`, `PLAN.md`
- Kept `README.md` at project root
- Updated all cross-references in README.md and arch.md

### 2. Read all `.md` files — full overview (done)
- All docs read end-to-end
- Key gap: `rotator.go` / `rotator_test.go` fully implemented but **undocumented** in arch.md, help.md, man.md
- `passforge improve` mentioned in PLAN.md but not implemented
- SSBD prominent in README + PLAN but absent from technical docs

### 3. SSBD algo — top of all docs (done)
- Added SSBD blockquote banner to: README.md, arch.md, help.md, help_ext.md, man.md, PLAN.md

### 4. Codebase analysis (done)
Build/test status: all pass, 85.6% coverage, no race conditions.

#### Findings (prioritized)

**CRITICAL**
- Wordlist panic on missing embed — `wordlist.go:23` panics instead of returning error
- No CLI tests — `cmd/passforge/main.go` has 0% coverage

**HIGH**
- HIBP non-200 responses treated as "not breached" — rate limits/outages give false safety
- No `--exclude` flag validation — invalid Unicode could produce weird output
- Symbol pool hardcoded as 32 in scorer but actual charset is 31 — entropy slightly off

**MEDIUM**
- `normalizeBase()` in rotator.go is dead code (only used if exported/tested directly)
- `NoOpChecker` not exported — no way for users to use offline mode
- Capitalization edge case: empty word in passphrase would panic
- Magic numbers throughout scorer.go — no named constants

**LOW**
- Keyboard walk detection is O(n^2) — fine for passwords, but could optimize
- Missing godoc on all public types in config.go
- No integration tests for CLI exit codes / JSON output

### 5. Update `man.md` with new content
- Reflect all changes made in steps 1-4
- Add full `rotator.go` documentation (currently missing entirely)
- Update file paths (now under `docs/`)
- Document the `rotate` CLI command in main.go section
- Add any new functions, flags, or behaviors discovered

### 6. Memory analysis in `man.md`
- For each `.go` file, document:
  - Every struct and its fields — size, alignment, purpose
  - Every constant/var — type, value, memory footprint
  - Every function — signature, allocations, hot paths
  - Line-by-line annotation where meaningful
- Include estimated memory usage per struct instance
- Note any optimization opportunities

---

*Updated as tasks complete.*
