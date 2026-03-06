# External Packages

Reference for all third-party dependencies used by PassForge.

> **Same Same But Different** — PassForge's signature rotation engine. One strong base, many unique variants.
> `p@sSwor4 → P@sswor4 → pAs$wor4 → p@ssWor4 → pa$Swor4`

---

## Direct Dependencies

### `github.com/spf13/cobra` v1.10.2

**What it is:** The most widely used CLI framework for Go. Used by Docker, Kubernetes, Hugo, GitHub CLI, and hundreds of other tools.

**What it does for us:** Provides the entire command-line interface structure — subcommands (`generate`, `check`, `suggest`, `passphrase`, `bulk`), flag parsing (`--length`, `--json`, `--breach`), help text generation, and argument validation.

**Where it's used:** `cmd/passforge/main.go`

**Key concepts:**
- `cobra.Command` — each subcommand is a struct with `Use`, `Short`, `Long`, `RunE`, and `Args` fields
- `cmd.Flags()` — binds CLI flags (e.g., `--length 20`) to Go variables
- `PersistentFlags()` — flags inherited by all subcommands (we use this for `--json`)
- `cobra.ExactArgs(1)` — enforces that exactly one positional argument is required
- `RunE` — the function that runs when the command is invoked; returns an error instead of panicking

**Example from our code:**
```go
cmd := &cobra.Command{
    Use:   "generate",
    Short: "Generate a random password",
    RunE: func(cmd *cobra.Command, args []string) error {
        pw, err := core.Generate(cfg)
        // ...
    },
}
cmd.Flags().IntVarP(&cfg.Length, "length", "l", cfg.Length, "password length")
```

`IntVarP` binds the `--length` flag (short form `-l`) directly to `cfg.Length`. When the user runs `passforge generate -l 20`, cobra parses the flag and sets the value before `RunE` executes.

**Docs:** https://github.com/spf13/cobra

---

### `golang.org/x/term` v0.40.0

**What it is:** Part of the extended Go standard library ("x" sub-repository). It provides primitives to manipulate terminal state.

**What it does for us:** Provides the secure "hidden echo" password input (`term.ReadPassword`), ensuring passwords typed interactively into commands like `check` and `rotate` don't leak into terminal history or standard CLI output.

**Where it's used:** `cmd/passforge/main.go`

**Docs:** https://pkg.go.dev/golang.org/x/term

---

## Indirect Dependencies

These are pulled in automatically by cobra. You don't import them directly, but they're in `go.sum`.

### `github.com/spf13/pflag` v1.0.9

**What it is:** POSIX-compliant flag parsing library. Drop-in replacement for Go's `flag` package.

**What it does:** Powers cobra's flag system. Supports `--long-flag` and `-s` short flags, boolean flags, default values, and flag grouping.

**Why cobra uses it:** Go's stdlib `flag` package only supports `-flag` style (single dash). pflag adds `--flag` (double dash) which is the POSIX/GNU convention that users expect.

**Docs:** https://github.com/spf13/pflag

---

### `github.com/inconshreveable/mousetrap` v1.1.0

**What it is:** A tiny Windows-only library (no-op on other platforms).

**What it does:** Detects if the program was launched by double-clicking the `.exe` in Windows Explorer (rather than from a terminal). Cobra uses this to show a "press any key to continue" prompt so the window doesn't instantly close.

**Why it's here:** Cobra imports it for Windows UX. On macOS/Linux it compiles to nothing.

**Docs:** https://github.com/inconshreveable/mousetrap

---

## Standard Library Packages (notable usage)

These aren't external dependencies but are worth documenting since they're critical to PassForge.

### `crypto/rand`

Used in `generator.go`. Provides cryptographically secure random number generation. On macOS/Linux this reads from the OS entropy pool (`/dev/urandom`). On Windows it uses `CryptGenRandom`. This is **not** `math/rand` — it's safe for security-sensitive operations.

### `crypto/sha1`

Used in `hibp.go`. Computes SHA-1 hashes for the HIBP k-anonymity protocol. We only send the first 5 characters of the hash to the API — the full password never leaves the machine.

### `math/big`

Used in `generator.go` via `rand.Int(rand.Reader, big.NewInt(n))`. This is how you get a uniform random integer in the range `[0, n)` from `crypto/rand` — the stdlib doesn't provide a simpler API for this.

### `embed`

Used in `wordlist.go`. The `//go:embed` directive tells the Go compiler to include the EFF wordlist file directly in the binary at compile time. No file I/O at runtime — the data is baked into the executable.

### `net/http`

Used in `hibp.go`. Makes HTTPS GET requests to the HIBP Pwned Passwords API. We set a custom `User-Agent` header and a 5-second timeout.

### `encoding/json`

Used in `cmd/passforge/main.go`. Handles `--json` output formatting with `json.NewEncoder` and pretty-printing via `SetIndent`.

### `sync`

Used in `wordlist.go` and `dictionary.go`. `sync.Once` ensures the wordlist and common-passwords set are loaded exactly once, even if called concurrently. This is a thread-safe lazy initialization pattern.

### `io`

Used in `hibp.go`. The `io.LimitReader` enforces a strict 1 MiB cap on the response payload read from the HIBP API. This defends against decompression-bomb and resource exhaustion (DoS) attacks.

### `unicode/utf8`

Used in `scorer.go` and `suggester.go`. We process text using `utf8.RuneCountInString` to properly evaluate password strength and generation bounds accurately across multi-byte Unicode strings (e.g., emojis and non-Latin scripts), rather than naive byte counting.
