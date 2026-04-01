# PassForge Vault — Password Tracking & Cloud Sync Design Spec

> **Status**: Draft (brainstorming approved, awaiting final review)
> **Author**: Claude + Nick
> **Date**: 2026-03-23
> **Branch**: `feature/vault-password-tracking`

---

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Section 1: Data Model](#section-1-data-model)
- [Section 2: Encryption Layer](#section-2-encryption-layer)
- [Section 3: CLI Commands](#section-3-cli-commands)
- [Section 4: Reuse Detection](#section-4-reuse-detection)
- [Section 5: VaultSyncer Interface](#section-5-vaultsyncer-interface)
- [Section 6: Git Sync Driver](#section-6-git-sync-driver)
- [Section 7: Cloud Adapter Guide](#section-7-cloud-adapter-guide)
- [Section 8: Implementation Phases](#section-8-implementation-phases)
- [Appendix A: Threat Model](#appendix-a-threat-model)
- [Appendix B: File Layout](#appendix-b-file-layout)
- [Appendix C: Comparison with Existing Tools](#appendix-c-comparison-with-existing-tools)

---

## Overview

### What This Is

PassForge Vault adds **local-first password tracking** to PassForge's existing generate/score/rotate workflow. It answers three questions that PassForge currently cannot:

1. **"Have I used this password before?"** — Reuse detection via hashed history
2. **"What's my current password for GitHub?"** — Encrypted retrieval of the latest password per service
3. **"Are my passwords getting stronger?"** — Strength trend tracking over time

### What This Is NOT

- **Not a full password manager.** No browser autofill, no browser extension integration (yet), no shared vaults, no team features.
- **Not cloud-dependent.** Works 100% offline. Cloud sync is opt-in and layered.
- **Not a new product.** It's a natural extension of PassForge's existing `generate → score → rotate` workflow.

### Why Now

PassForge's PLAN.md Risk Register (item 2) identifies "Users treat PassForge as a vault" as a Medium likelihood / High impact risk. Rather than fighting this expectation, we channel it into a secure, privacy-first implementation that stays true to the core principle: **passwords never leave the user's device** (unless the user explicitly opts into sync).

### Core Tension Resolved

PLAN.md line 32 states: *"Why not a server-side password store? Passwords should never leave the user's device."*

The vault resolves this by:
- **Local encryption**: All passwords encrypted with AES-256-GCM before touching disk
- **Key derivation from master passphrase**: Argon2id, user-controlled
- **Cloud sync of encrypted blobs only**: If opted in, cloud providers see opaque bytes
- **History as hashes**: Previous passwords stored as irreversible SHA-256 hashes

---

## Design Principles

These principles guide every decision in this spec. When in doubt, refer back here.

| # | Principle | Implication |
|---|-----------|-------------|
| 1 | **Local-first** | Works 100% offline. Network is never required. |
| 2 | **Zero-knowledge cloud** | Cloud providers see only encrypted blobs. No plaintext ever leaves the device. |
| 3 | **Minimal new dependencies** | Only `golang.org/x/crypto/argon2` added. Everything else is Go stdlib. |
| 4 | **Follow existing patterns** | Use `config.go` for types, interfaces like `BreachChecker`, error patterns from `errors.go`. |
| 5 | **Phased delivery** | Local vault ships first. Git sync second. Cloud adapters third. Each phase is independently useful. |
| 6 | **Defense in depth** | Even if the vault file is stolen, it's useless without the master passphrase. Even if the master is compromised, old passwords are hashes, not ciphertext. |

---

## Section 1: Data Model

### 1.1 Vault Container

The vault is a single JSON document encrypted as a whole and stored as `vault.json.enc`. When decrypted into memory, it has this structure:

```go
// VaultData is the top-level decrypted vault structure.
type VaultData struct {
    Version    int                    `json:"version"`     // schema version (starts at 1)
    DeviceID   string                 `json:"device_id"`   // unique ID for this device
    CreatedAt  time.Time              `json:"created_at"`  // vault creation timestamp
    ModifiedAt time.Time              `json:"modified_at"` // last modification timestamp
    Entries    map[string]*VaultEntry `json:"entries"`     // keyed by UUID
}
```

**Why a single encrypted file?**
- Whole-file encryption means **zero metadata leakage** — an attacker who obtains the file cannot even see how many services are stored, what they're named, or when entries were last modified.
- Simpler than per-entry encryption: one decrypt operation on open, one encrypt on save.
- The tradeoff (entire file rewritten on every change) is acceptable for vaults under ~10,000 entries (< 10MB), which covers any individual user.

### 1.2 Vault Entry

Each entry represents one service/account with its current password and history:

```go
// VaultEntry represents a single service's password lifecycle.
type VaultEntry struct {
    ID        string         `json:"id"`        // UUID v4, stable across sync
    Service   string         `json:"service"`   // "github.com", "aws-console"
    Label     string         `json:"label"`     // optional: "work", "personal"
    Username  string         `json:"username"`  // optional: "nick@example.com"
    Current   CurrentPassword `json:"current"`  // the active, retrievable password
    History   []HistoryEntry `json:"history"`   // previous passwords (hashed only)
    Metadata  EntryMetadata  `json:"metadata"`  // sync + tracking metadata
}
```

### 1.3 Current Password (Encrypted, Retrievable)

```go
// CurrentPassword holds the active password in encrypted form.
type CurrentPassword struct {
    Ciphertext []byte    `json:"ciphertext"` // AES-256-GCM encrypted password
    Nonce      []byte    `json:"nonce"`      // unique per encryption operation
    Score      int       `json:"score"`      // strength score at time of save
    Source     string    `json:"source"`     // "generate", "rotate", "passphrase", "manual"
    Rotation   *RotationRef `json:"rotation,omitempty"` // if source="rotate"
    CreatedAt  time.Time `json:"created_at"` // when this password was set
}
```

**Why store the nonce separately from ciphertext?**
Go's `crypto/cipher` GCM implementation typically prepends the nonce to the ciphertext via `Seal()`. We store them separately for clarity and to support future migration to different AEAD constructions without re-encrypting everything.

### 1.4 Rotation Reference

```go
// RotationRef stores enough info to regenerate an SSBD variant
// without needing the ciphertext. Defense-in-depth: if the vault
// key is compromised, the attacker still needs the base password
// to use this reference.
type RotationRef struct {
    BaseHash   string       `json:"base_hash"`   // SHA-256 of the base password
    CycleIndex int          `json:"cycle_index"`  // which SSBD variant was used
    Config     RotateConfig `json:"config"`       // the RotateConfig that produced it
}
```

**Why store rotation references?**
SSBD rotation is **deterministic**: `RotateWithConfig("base", config)[n]` always produces the same nth variant. By storing `hash(base) + cycle_index + config`, a user who remembers their base password can regenerate the exact variant without ever storing it. This is a unique capability that no other password manager offers.

### 1.5 History Entry (Hashed, NOT Retrievable)

```go
// HistoryEntry stores a non-retrievable record of a previous password.
// Used for reuse detection ("don't use your last 3 passwords").
type HistoryEntry struct {
    Hash      string    `json:"hash"`       // SHA-256(password) — irreversible
    Score     int       `json:"score"`      // strength at time of use
    Source    string    `json:"source"`     // how it was created
    RetiredAt time.Time `json:"retired_at"` // when it was replaced
}
```

**Why SHA-256 and not bcrypt for history hashes?**
- History hashes are used for **exact match detection** (O(1) lookup: "is this new password identical to an old one?"), not for password verification against brute force.
- bcrypt/scrypt/argon2 are designed to be slow (to resist offline attacks on stored credentials). Here, the passwords are already gone — we only store a fingerprint to detect reuse.
- SHA-256 is fast, deterministic, and sufficient for this use case. An attacker with the vault file would need the master passphrase to decrypt the vault first, and even then would only get hashes, not passwords.

### 1.6 Entry Metadata (Sync Support)

```go
// EntryMetadata tracks sync and modification state.
type EntryMetadata struct {
    CreatedAt  time.Time      `json:"created_at"`  // first creation
    UpdatedAt  time.Time      `json:"updated_at"`  // last modification
    DeviceID   string         `json:"device_id"`   // which device last wrote this
    VClock     map[string]int `json:"vclock"`       // vector clock for CRDT merge
}
```

**Why vector clocks?**
When two devices edit **different** entries, the merge is trivial (take both). When two devices edit the **same** entry, we need to determine which edit is "newer" in a distributed system where clocks can drift. Vector clocks solve this without requiring synchronized time.

Each device increments its own counter on every edit. During merge:
- If clock A dominates clock B → take A (A is strictly newer)
- If neither dominates → conflict (both edited since last sync) → last-writer-wins by timestamp, or user-prompted resolution

### 1.7 Constants

```go
// vault_config.go — following the config.go pattern

const (
    VaultSchemaVersion    = 1
    VaultMaxHistoryDepth  = 5    // keep last N passwords per service
    VaultDefaultPath      = "~/.passforge/vault.json.enc"
    VaultSaltFile         = "~/.passforge/vault.salt"
    VaultBackupSuffix     = ".backup"
    VaultLockTimeout      = 5 * time.Minute  // auto-lock after inactivity
)
```

---

## Section 2: Encryption Layer

### 2.1 Key Derivation: Argon2id

The master passphrase is never stored. It's used to derive an encryption key via Argon2id (winner of the Password Hashing Competition, 2015):

```go
// DeriveKey produces a 256-bit key from a master passphrase and salt.
func DeriveKey(passphrase string, salt []byte) []byte {
    return argon2.IDKey(
        []byte(passphrase),
        salt,
        3,         // time iterations (OWASP minimum for Argon2id)
        64*1024,   // memory: 64 MiB (resists GPU attacks)
        4,         // parallelism (threads)
        32,        // key length: 256 bits
    )
}
```

**Parameter choices explained:**
| Parameter | Value | Why |
|-----------|-------|-----|
| Time | 3 iterations | OWASP 2024 minimum recommendation for Argon2id |
| Memory | 64 MiB | Makes GPU/ASIC attacks expensive. ~0.3s on modern hardware. |
| Parallelism | 4 | Matches typical core count. Higher = more CPU bound. |
| Key length | 32 bytes | AES-256 requires a 256-bit key |

**Salt generation and storage:**
- A 16-byte random salt is generated via `crypto/rand` on vault creation
- Stored in `~/.passforge/vault.salt` (separate from vault file)
- The salt is NOT secret — its purpose is to prevent rainbow table attacks
- On sync, the salt travels with the vault (included in the synced bundle)

### 2.2 Encryption: AES-256-GCM

```go
// EncryptVault encrypts the vault data with the derived key.
func EncryptVault(data []byte, key []byte) (ciphertext []byte, nonce []byte, err error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, nil, fmt.Errorf("creating cipher: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, fmt.Errorf("creating GCM: %w", err)
    }

    nonce = make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(crypto_rand.Reader, nonce); err != nil {
        return nil, nil, fmt.Errorf("%w: generating nonce: %v", ErrRandFailure, err)
    }

    ciphertext = gcm.Seal(nil, nonce, data, nil)
    return ciphertext, nonce, nil
}
```

**Why AES-256-GCM?**
- **Authenticated encryption**: GCM provides both confidentiality AND integrity. If anyone tampers with the ciphertext, decryption fails (detects corruption and attacks).
- **Go stdlib**: `crypto/aes` + `crypto/cipher` are in the standard library. No CGO, no external C libraries.
- **Performance**: AES-NI hardware acceleration on modern CPUs makes encryption/decryption near-instant even for large vaults.
- **Industry standard**: Used by virtually every serious encryption system (TLS, GPG, etc.).

### 2.3 Per-Entry Encryption

Individual passwords within the vault are **double-encrypted**:
1. The password itself is encrypted with AES-256-GCM using the vault key → stored in `CurrentPassword.Ciphertext`
2. The entire vault JSON (including all ciphertexts) is encrypted again as a whole → stored as `vault.json.enc`

**Why double encryption?**
- **Defense in depth**: If the outer encryption is somehow bypassed (bug, partial key leak), individual passwords are still encrypted.
- **Granular nonces**: Each password gets its own nonce, so re-encrypting the vault with updated entries doesn't reuse nonces.
- **Forward compatibility**: Allows future per-entry key rotation without re-encrypting the entire vault.

### 2.4 Vault File Format

The on-disk file is a binary blob:

```
[4 bytes: magic "PFVT"]
[2 bytes: format version (uint16 big-endian)]
[12 bytes: outer nonce]
[remaining: AES-256-GCM ciphertext + 16-byte GCM tag]
```

The magic bytes allow tools like `file(1)` to identify PassForge vault files. The format version allows future migration without breaking existing vaults.

### 2.5 Memory Safety

```go
// SecureZero overwrites a byte slice with zeros.
// Called on vault key and decrypted data when locking.
func SecureZero(b []byte) {
    for i := range b {
        b[i] = 0
    }
    // Note: Go's GC may have already copied this data.
    // For defense-in-depth, not a guarantee. True memory
    // protection would require mlock(2) + mprotect(2),
    // which is a future enhancement.
}
```

**Honest limitations:**
- Go's garbage collector may copy the key/plaintext before we can zero it. This is a known limitation of managed-memory languages for cryptographic applications.
- For Phase 1, `SecureZero` is best-effort. A future enhancement could use `golang.org/x/sys/unix` for `mlock()` to pin memory pages and prevent swapping.
- This is the same tradeoff that `age`, `sops`, and other Go-based encryption tools make.

### 2.6 New Dependency

The **only** new external dependency:

```
golang.org/x/crypto/argon2
```

This is maintained by the Go team and is effectively stdlib. PassForge already depends on `golang.org/x/crypto` indirectly (via Cobra's SSH key support in `go.sum`). The actual import adds ~50KB to the binary.

---

## Section 3: CLI Commands

### 3.1 Command Overview

New subcommand: `passforge vault` with these sub-subcommands:

```
passforge vault init          Initialize a new vault
passforge vault unlock        Unlock the vault (start session)
passforge vault lock          Lock the vault (end session)
passforge vault add           Add a password entry
passforge vault get           Retrieve a password
passforge vault list          List all entries
passforge vault history       Show password history for a service
passforge vault check-reuse   Check if a password was previously used
passforge vault rotate        Rotate a service's password via SSBD
passforge vault sync          Sync vault with remote (if configured)
passforge vault export        Export vault to plaintext JSON (dangerous)
passforge vault import        Import from other password managers
passforge vault destroy       Delete the vault entirely
```

### 3.2 Detailed Command Specs

#### `vault init`

Creates a new vault. Prompts for master passphrase (with confirmation). Generates salt. Writes empty encrypted vault.

```bash
$ passforge vault init
Create master passphrase: ********
Confirm master passphrase: ********
Vault created at ~/.passforge/vault.json.enc
Remember: if you lose your master passphrase, your vault CANNOT be recovered.
```

Flags:
- `--path <path>` — custom vault location (default: `~/.passforge/vault.json.enc`)
- `--json` — output result as JSON

Error cases:
- Vault already exists → `"vault already exists at <path>. Use --force to overwrite."`
- Passphrase too short (< 8 chars) → `"master passphrase must be at least 8 characters"`
- Passphrase confirmation mismatch → `"passphrases do not match"`

#### `vault unlock`

Decrypts vault into memory. Starts a session (background process or file lock).

```bash
$ passforge vault unlock
Master passphrase: ********
Vault unlocked (47 entries). Auto-locks in 5 minutes.
```

**Session model**: The unlocked vault key is held in memory by a background daemon process (`passforge-agent`) that listens on a Unix domain socket at `~/.passforge/agent.sock`. Other `passforge vault` commands communicate with this agent to avoid re-prompting for the master passphrase on every operation.

Flags:
- `--timeout <duration>` — auto-lock timeout (default: 5m, 0 = no auto-lock)

#### `vault lock`

Zeros the key from the agent's memory, removes the socket.

```bash
$ passforge vault lock
Vault locked. Agent stopped.
```

#### `vault add`

Adds a new entry or updates an existing one. If the service already exists, the current password is demoted to history.

```bash
# Add from generate (most common flow)
$ passforge generate --length 20 | passforge vault add github.com

# Add from rotate
$ passforge rotate "base" --count 1 | passforge vault add github.com --source rotate

# Add manually (prompts for password)
$ passforge vault add github.com
Password: ********
Added github.com (Score: 87, Source: manual)

# Add with label
$ passforge vault add github.com --label "work" --username "nick@company.com"
```

Flags:
- `--label <label>` — optional human-readable label
- `--username <user>` — optional username/email
- `--source <source>` — override auto-detected source (generate/rotate/passphrase/manual)
- `--json` — JSON output

When updating an existing service:
```bash
$ passforge vault add github.com
Password: ********
github.com already exists. Current password will be moved to history.
Continue? [y/N]: y
Updated github.com (Score: 91, Source: manual). Previous password archived.
```

#### `vault get`

Retrieves and decrypts the current password for a service. Copies to clipboard with auto-clear.

```bash
$ passforge vault get github.com
Password copied to clipboard (clears in 10s).
Score: 87 | Source: rotate | Age: 12 days

# Show password in terminal instead
$ passforge vault get github.com --show
Password: $uMm3r#2024!kQ
Score: 87 | Source: rotate | Age: 12 days

# JSON output
$ passforge vault get github.com --json
{"service":"github.com","password":"$uMm3r#2024!kQ","score":87,"source":"rotate","age_days":12}
```

Flags:
- `--show` — display password in terminal (default: clipboard only)
- `--clipboard-timeout <seconds>` — clipboard auto-clear time (default: 10)
- `--json` — JSON output

#### `vault list`

Lists all entries without revealing passwords.

```bash
$ passforge vault list
SERVICE          LABEL     SCORE  SOURCE    AGE     HISTORY
github.com       work      87     rotate    12d     3 prev
aws-console      —         94     generate  3d      1 prev
gmail            personal  72     manual    45d     0 prev

47 entries. Vault last modified: 2026-03-23.
```

Flags:
- `--sort <field>` — sort by: service, score, age (default: service)
- `--filter <query>` — fuzzy search across service+label
- `--weak` — show only entries with score < StrongThreshold (60)
- `--stale <days>` — show only entries older than N days
- `--json` — JSON output

#### `vault history`

Shows the password history for a specific service (scores and dates, never passwords).

```bash
$ passforge vault history github.com
SERVICE: github.com (work)
CURRENT: Score 87 | rotate | 2026-03-11
HISTORY:
  1. Score 72 | manual  | 2025-11-01 (retired 142d ago)
  2. Score 65 | manual  | 2025-06-15 (retired 280d ago)
  3. Score 58 | manual  | 2025-01-20 (retired 426d ago)

Trend: +29 points over 14 months. Improving.
```

#### `vault check-reuse`

Checks if a candidate password matches any current or historical hash.

```bash
$ echo "MyOldPassword" | passforge vault check-reuse
REUSED: This password was previously used for github.com (retired 142d ago).

$ echo "TotallyNew123!" | passforge vault check-reuse
OK: This password has not been used before.
```

Exit codes follow existing convention:
- 0 = not reused (safe)
- 1 = reused (warning)

#### `vault rotate`

Convenience command that combines SSBD rotation with vault update:

```bash
$ passforge vault rotate github.com
Current password for github.com decrypted.
Generating 5 SSBD variants...

  1. P@s$wOr4!xQ    Score: 89
  2. p@sSwOR4!kZ     Score: 85
  3. PA$$wor4!mR     Score: 91  ← recommended
  4. p@ssW0R4!nT     Score: 83
  5. pA$sWoR4!qB     Score: 86

Select variant [1-5, or Enter for recommended]: 3
Updated github.com. Previous password archived.
```

#### `vault sync`

Sync with configured remote. Details in Sections 5-7.

```bash
$ passforge vault sync
Pulling from origin... 2 new entries from MacBook.
Pushing... 1 local change.
Synced. 49 entries across 2 devices.
```

#### `vault export` / `vault import`

```bash
# Export to plaintext JSON (DANGEROUS — shows all passwords)
$ passforge vault export > vault-backup.json
WARNING: This file contains ALL your passwords in PLAINTEXT.
Store it securely and delete when done.

# Import from 1Password/Bitwarden CSV
$ passforge vault import --format bitwarden passwords.csv
Imported 142 entries. Run `passforge vault list --weak` to find weak passwords.
```

Supported import formats:
- Bitwarden CSV
- 1Password CSV
- KeePass XML
- Generic CSV (`service,username,password`)

### 3.3 Integration with Existing Commands

Existing commands gain an optional `--save <service>` flag:

```bash
# Generate and auto-save to vault
$ passforge generate --length 20 --save github.com
Generated: xK9!mQ2$pL7@nR4wZ5&t
Saved to vault: github.com (Score: 94)

# Rotate and auto-save
$ passforge rotate "base" --count 5 --save github.com
# ... shows variants, prompts for selection, saves to vault

# Check and update vault score
$ passforge check "existing" --save github.com --update-score
Score: 72 (Fair). Updated vault entry score.
```

This keeps the vault opt-in — existing workflows are unchanged unless `--save` is used.

---

## Section 4: Reuse Detection

### 4.1 Algorithm

When a new password is being saved for any service:

```
1. Compute candidate_hash = SHA-256(new_password)
2. For each entry in vault:
   a. Compare candidate_hash against entry.Current (decrypt + hash)
   b. Compare candidate_hash against each entry.History[i].Hash
3. If match found:
   - Return (true, matched_service, matched_age)
   - CLI shows: "REUSED: This password was previously used for <service> (<age> ago)"
4. If no match:
   - Return (false, "", 0)
```

### 4.2 Performance

For a vault with N entries and average H history entries each:
- Hash comparisons: N * (1 + H) = N * (1 + 5) = 6N for default history depth
- For 1,000 entries: 6,000 string comparisons → < 1ms

No performance concern here. The bottleneck is always the Argon2id key derivation on unlock (~0.3s), not the reuse check.

### 4.3 Cross-Service Reuse

The check runs across ALL services, not just the target service. This catches the dangerous pattern of reusing the same password across multiple sites:

```bash
$ passforge vault add twitter.com
Password: ********
WARNING: This password is currently used for github.com.
Using the same password across services is dangerous.
Continue anyway? [y/N]:
```

### 4.4 Near-Miss Detection (Future Enhancement)

Phase 1 detects **exact** reuse only. A future enhancement could detect near-misses:
- Same password with different case (`Password123` vs `password123`)
- Leet-speak variants (`p@ssword` vs `password`)
- Password + common suffix (`password1` vs `password`)

This leverages PassForge's existing `normalizeLeet()` function from `scorer.go`. Implementation is deferred to avoid scope creep.

---

## Section 5: VaultSyncer Interface

### 5.1 Interface Definition

```go
// VaultSyncer handles bidirectional sync between local vault and a remote.
// All data passed through this interface is already encrypted — implementations
// never see plaintext passwords.
type VaultSyncer interface {
    // Push sends local changes to the remote.
    // entries contains only entries modified since last sync.
    Push(ctx context.Context, entries []SyncEntry) error

    // Pull retrieves remote changes since last sync.
    // Returns entries that are newer on the remote.
    Pull(ctx context.Context) ([]SyncEntry, error)

    // Resolve handles a conflict when both local and remote
    // modified the same entry since last sync.
    Resolve(local, remote SyncEntry) (SyncEntry, error)

    // Status returns the current sync state.
    Status(ctx context.Context) (*SyncStatus, error)

    // Init initializes the remote (create repo, bucket, etc).
    Init(ctx context.Context, config SyncConfig) error
}
```

### 5.2 Supporting Types

```go
// SyncEntry is an encrypted vault entry plus sync metadata.
// The syncer never decrypts — it just moves bytes.
type SyncEntry struct {
    ID         string         `json:"id"`
    Ciphertext []byte         `json:"ciphertext"` // entire VaultEntry, encrypted
    Nonce      []byte         `json:"nonce"`
    VClock     map[string]int `json:"vclock"`
    UpdatedAt  time.Time      `json:"updated_at"`
    DeviceID   string         `json:"device_id"`
    Deleted    bool           `json:"deleted"` // tombstone for deletions
}

// SyncConfig holds configuration for a sync backend.
type SyncConfig struct {
    Backend    string            `json:"backend"`     // "git", "supabase", "firebase"
    Remote     string            `json:"remote"`      // backend-specific remote URL
    Options    map[string]string `json:"options"`     // backend-specific options
}

// SyncStatus reports the current sync state.
type SyncStatus struct {
    LastSyncAt   time.Time `json:"last_sync_at"`
    PendingPush  int       `json:"pending_push"`  // local changes not yet pushed
    PendingPull  int       `json:"pending_pull"`  // remote changes not yet pulled
    DeviceCount  int       `json:"device_count"`  // known devices
    Conflicts    int       `json:"conflicts"`     // unresolved conflicts
}
```

### 5.3 Merge Strategy

The merge algorithm follows the CRDT pattern:

```
For each remote entry R:
    Find matching local entry L (by ID):

    If L does not exist:
        Accept R (new entry from another device)

    If L exists:
        Compare vector clocks:
        If R.VClock dominates L.VClock:
            Accept R (remote is strictly newer)
        If L.VClock dominates R.VClock:
            Keep L (local is strictly newer, push later)
        If neither dominates (concurrent edits):
            Conflict → Resolve(L, R)
```

Default conflict resolution: **last-writer-wins** by `UpdatedAt` timestamp. This is simple and predictable. A future enhancement could prompt the user for manual resolution.

### 5.4 Tombstones

Deletions are synced as tombstones (`Deleted: true`). This prevents a deleted entry from reappearing when syncing with a device that still has it. Tombstones are garbage-collected after 30 days.

---

## Section 6: Git Sync Driver

### 6.1 How It Works

The Git sync driver stores each encrypted entry as a separate file in a Git repository. Git handles transport (push/pull), history (log), and basic conflict detection (merge).

```
~/.passforge/vault-sync/
  .git/
  entries/
    a1b2c3d4-e5f6-7890-abcd-ef1234567890.enc
    f9e8d7c6-b5a4-3210-fedc-ba0987654321.enc
    ...
  meta.enc          // vault metadata (device ID, schema version)
  .gitignore        // ignore local-only files
```

### 6.2 File-Per-Entry Strategy

**Why one file per entry instead of one big file?**
- Git can detect which entries changed (per-file diffs)
- Merging non-conflicting changes is automatic (different files = no conflict)
- Partial sync is possible (future: sparse checkout)
- Deletion tracking is natural (file removal = git rm)

**Tradeoff**: An attacker with repo access can see the number of entries and when each was modified (from Git timestamps). They cannot see service names, passwords, or any content — those are encrypted. This is acceptable because the Git remote should itself be private (private GitHub repo or self-hosted Gitea).

### 6.3 Git Operations

```go
// GitSyncer implements VaultSyncer using a local Git repository.
type GitSyncer struct {
    repoPath string
    remote   string // e.g., "origin"
}

func (g *GitSyncer) Push(ctx context.Context, entries []SyncEntry) error {
    // 1. Write each entry to entries/<uuid>.enc
    // 2. git add entries/
    // 3. git commit -m "vault sync from <device_id> at <timestamp>"
    // 4. git push origin main
}

func (g *GitSyncer) Pull(ctx context.Context) ([]SyncEntry, error) {
    // 1. git fetch origin
    // 2. git diff HEAD..origin/main --name-only
    // 3. For each changed file: read, parse SyncEntry
    // 4. git merge origin/main (fast-forward if possible)
    // 5. Return changed entries
}
```

### 6.4 Setup

```bash
# Initialize Git sync with a private GitHub repo
$ passforge vault sync init --backend git --remote git@github.com:nick/passforge-vault.git
Initialized sync repo at ~/.passforge/vault-sync/
Remote: git@github.com:nick/passforge-vault.git
First push: 47 entries synced.

# On another device
$ passforge vault sync init --backend git --remote git@github.com:nick/passforge-vault.git
Cloned sync repo. Pulled 47 entries.
Merge with local vault? [y/N]: y
Merged. 47 entries total.
```

### 6.5 Conflict Handling in Git

Since each entry is a separate encrypted file:
- **Different entries edited on different devices**: Git auto-merges (no conflict)
- **Same entry edited on both devices**: Git marks as conflict → PassForge uses vector clock to resolve → takes the newer version

---

## Section 7: Cloud Adapter Guide

### 7.1 Architecture

Cloud adapters are implementations of the `VaultSyncer` interface. They convert PassForge's sync operations into backend-specific API calls. The vault core never knows which backend is being used.

```
┌─────────────────────────────────────────────┐
│              Vault Core                      │
│  (encrypt, decrypt, merge, detect reuse)     │
├──────────────────────────────────────────────┤
│           VaultSyncer Interface               │
├──────────┬───────────┬───────────────────────┤
│  GitSync │ SupaSync  │ FirebaseSync          │
│  (free)  │ (realtime)│ (realtime)            │
└──────────┴───────────┴───────────────────────┘
```

### 7.2 Supabase Adapter (Future)

```go
type SupabaseSyncer struct {
    client  *supabase.Client
    project string
}
```

**Supabase schema:**
```sql
CREATE TABLE vault_entries (
    id         UUID PRIMARY KEY,
    user_id    UUID REFERENCES auth.users(id),
    ciphertext BYTEA NOT NULL,   -- encrypted entry blob
    nonce      BYTEA NOT NULL,
    vclock     JSONB NOT NULL,
    device_id  TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted    BOOLEAN DEFAULT false
);

-- Row-level security: users can only access their own entries
ALTER TABLE vault_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Users access own entries" ON vault_entries
    USING (auth.uid() = user_id);
```

**Key points:**
- Supabase sees only encrypted blobs — zero knowledge of password content
- Row-level security ensures user isolation
- Realtime subscriptions enable live sync across devices
- Free tier: 500MB, 50K rows — covers any individual user
- Authentication via Supabase Auth (email/password or OAuth)

### 7.3 Firebase Adapter (Future)

```go
type FirebaseSyncer struct {
    client *firestore.Client
    uid    string
}
```

**Firestore structure:**
```
users/{uid}/entries/{entry_id}
  ciphertext: Bytes
  nonce: Bytes
  vclock: Map
  device_id: String
  updated_at: Timestamp
  deleted: Boolean
```

**Key points:**
- Firestore security rules enforce user isolation
- Built-in offline persistence (SDK caches locally)
- Snapshot listeners for realtime sync
- Free tier: 1GB storage, 50K reads/day

### 7.4 Adding a Custom Backend

To add a new sync backend:

1. Implement the `VaultSyncer` interface
2. Register the backend in a factory:

```go
// sync_registry.go
var syncBackends = map[string]func(SyncConfig) (VaultSyncer, error){
    "git":      NewGitSyncer,
    "supabase": NewSupabaseSyncer,
    "firebase": NewFirebaseSyncer,
}
```

3. Configure via `passforge vault sync init --backend <name> --remote <url>`

---

## Section 8: Implementation Phases

### Phase 1: Local Vault (v0.3.0) — 3-4 weeks

**Goal**: Fully functional local password tracking. No sync.

**New files:**
```
internal/core/
    vault.go           // VaultData, VaultEntry, encryption/decryption
    vault_config.go    // Constants, VaultConfig struct
    vault_store.go     // Read/write vault file, backup
    vault_reuse.go     // Reuse detection algorithm
    vault_test.go      // Unit tests

cmd/passforge/
    vault_cmd.go       // All vault subcommands
    agent.go           // Background agent for session management
```

**Scope:**
- [x] Data model (all types from Section 1)
- [ ] Argon2id key derivation + AES-256-GCM encryption
- [ ] `vault init`, `unlock`, `lock` commands
- [ ] `vault add`, `get`, `list`, `history` commands
- [ ] Reuse detection (`vault check-reuse`)
- [ ] `--save` flag on `generate`, `rotate`, `passphrase`
- [ ] Clipboard integration with auto-clear
- [ ] Background agent for session management
- [ ] `vault export` / `vault import` (Bitwarden CSV at minimum)
- [ ] Tests: 80%+ coverage on vault code

**Dependencies added:**
- `golang.org/x/crypto/argon2`

### Phase 2: Git Sync (v0.4.0) — 2-3 weeks

**Goal**: Sync vault across devices via Git.

**New files:**
```
internal/core/
    vault_sync.go      // VaultSyncer interface, merge algorithm
    vault_sync_git.go  // Git sync implementation
    vault_sync_test.go // Sync tests
```

**Scope:**
- [ ] `VaultSyncer` interface
- [ ] Git sync driver (push/pull/merge)
- [ ] Vector clock merge with last-writer-wins conflict resolution
- [ ] `vault sync init`, `vault sync`, `vault sync status` commands
- [ ] Tombstone support for deletions
- [ ] File-per-entry encryption in sync repo

**Dependencies added:**
- None (shells out to `git` CLI, no Go git library needed)

### Phase 3: Cloud Adapters (v0.5.0+) — 4-6 weeks

**Goal**: Optional cloud sync for users who want realtime.

**New files:**
```
internal/core/
    vault_sync_supabase.go  // Supabase adapter
    vault_sync_firebase.go  // Firebase adapter
```

**Scope:**
- [ ] Supabase sync adapter
- [ ] Firebase sync adapter
- [ ] Backend registry/factory pattern
- [ ] Auth flow (OAuth/API key) for cloud backends
- [ ] Realtime sync for supported backends

**Dependencies added:**
- Supabase Go client (conditional build tag)
- Firebase Go client (conditional build tag)

### Phase 4: Polish (v1.0.0)

- [ ] `vault rotate` convenience command
- [ ] Near-miss reuse detection (leet normalization)
- [ ] `vault audit` — batch strength check + breach check
- [ ] Memory locking (`mlock`) for key material
- [ ] Shell completions for vault subcommands
- [ ] Man page updates

---

## Appendix A: Threat Model

| Threat | Mitigation | Residual Risk |
|--------|-----------|---------------|
| Vault file stolen | AES-256-GCM + Argon2id makes brute-force infeasible for strong master passphrases | Weak master passphrase = game over |
| Master passphrase compromised | Old passwords are SHA-256 hashes (not retrievable). Only current passwords are at risk. | Current passwords exposed |
| Memory dump / cold boot | `SecureZero` on lock. Future: `mlock()`. | Go GC may have copied data |
| MITM on sync | Git uses SSH. Cloud uses TLS. Data is encrypted before transmission. | Trust in transport security |
| Malicious sync server | Server only sees encrypted blobs. Cannot decrypt without master passphrase. | Entry count and modification times visible to server |
| Device loss | Vault is encrypted at rest. Auto-lock after timeout. | If device was unlocked at time of loss |
| Supply chain (Argon2id) | `golang.org/x/crypto` is Go-team maintained, audited | Theoretical supply chain risk |

### Security Recommendations for Users

1. **Use a strong master passphrase** — 4+ word passphrase recommended (use `passforge passphrase --words 5` to generate one)
2. **Enable auto-lock** — default 5 minutes, don't disable
3. **Use private Git repos** for sync — never push vault to a public repo
4. **Back up the vault file** — losing it + master passphrase = all passwords lost
5. **Rotate the master passphrase** periodically — `passforge vault change-master`

---

## Appendix B: File Layout

### After Phase 1 (Local Only)

```
~/.passforge/
    vault.json.enc    // encrypted vault (binary: magic + nonce + ciphertext)
    vault.salt        // Argon2id salt (16 bytes, not secret)
    vault.json.enc.backup  // auto-backup before each write
    agent.sock        // Unix domain socket for session agent
    config.json       // vault settings (path, timeout, clipboard duration)
```

### After Phase 2 (Git Sync)

```
~/.passforge/
    vault.json.enc        // local vault (primary)
    vault.salt
    agent.sock
    config.json
    vault-sync/           // Git sync repo
        .git/
        entries/          // one encrypted file per entry
            <uuid>.enc
            ...
        meta.enc          // sync metadata
```

---

## Appendix C: Comparison with Existing Tools

| Feature | PassForge Vault | 1Password | Bitwarden | pass (Unix) |
|---------|----------------|-----------|-----------|-------------|
| **Local-first** | Yes | No (cloud) | Self-host option | Yes |
| **SSBD rotation** | Yes (unique) | No | No | No |
| **Reuse detection** | Yes (hash-based) | Yes | Yes | No |
| **Strength trends** | Yes | Limited | Limited | No |
| **Cloud sync** | Opt-in (Git/cloud) | Built-in | Built-in | Git |
| **Browser extension** | Future | Yes | Yes | Yes (via plugin) |
| **CLI-native** | Yes | CLI available | CLI available | Yes |
| **Open source** | Yes | No | Yes | Yes |
| **Encryption** | AES-256-GCM + Argon2id | AES-256-CBC + PBKDF2 | AES-256-CBC + PBKDF2 | GPG |
| **Zero new deps** | ~1 (x/crypto) | N/A | N/A | GPG + tree |

**PassForge's unique differentiators:**
1. **SSBD rotation tracking** — no other tool stores rotation lineage and can regenerate variants
2. **Hash-only history** — previous passwords are hashes, not ciphertext (defense in depth)
3. **CLI-first workflow** — designed for developers who live in the terminal
4. **Minimal architecture** — single Go binary, single file vault, optional sync

---

*End of spec. This document will be updated as design decisions evolve during implementation.*
