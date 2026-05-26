# Story Map: Go Migration — Phase 2 (Core Port)

## Plan

Migrate hostie from Bun/TypeScript to Go to drive the compiled binary from 61.20 MB down to ≤18 MB, while preserving every parity surface from v1 and adding auto-apply on mutation.

---

## Phase: Core port

Port the entire pre-CLI/pre-TUI v1 logic (domain types, validators, ULID, YAML I/O, render, marker block, atomic etc-hosts write, fileio) to Go under `go/internal/`, stand up the cross-binary golden harness, and close all Phase-1 deferred review debt — so Phase 3 (CLI) and Phase 4 (TUI) compose **only** against tested, parity-validated core packages.

---

### Story 1: Domain layer (types, validators, IDs) ports 1:1

**Purpose:** A Phase 3 (CLI) or Phase 4 (TUI) worker can `import "github.com/hungthai1401/hostie/go/internal/domain"` and get the same data shape, the same validation predicates, and the same ULID generator that v1 used — with ≥60 table-driven tests proving each predicate accepts/rejects the same inputs.

**Why Now:** Domain has zero dependencies on any other Phase 2 package — it's pure data + pure functions. Every other Phase 2 story (`yaml`, `render`, `etchosts`, `fileio`) imports `domain.HostsFile`/`domain.Entry`/`domain.Group`. Without it nothing else compiles.

**Contributes To:** Exit-state clause 1 (`domain/` ports types, validators, id; 60+ table-driven tests).

**Creates:**
- `go/internal/domain/types.go` — `Entry`, `Group`, `HostsFile`
- `go/internal/domain/types_test.go`
- `go/internal/domain/id.go` — `NewID()` via `oklog/ulid/v2.MonotonicEntropy`, goroutine-safe
- `go/internal/domain/id_test.go` — monotonicity under concurrent generation; uniqueness over N=10_000
- `go/internal/domain/validators.go` — `ValidateHostname`, `ValidateIPv4`, `ValidateIPv6`, `ValidateIP`, `ValidateNoDuplicates`
- `go/internal/domain/validators_test.go` — 31 hostname cases + 29 IP cases + N duplicate-detection cases, all as one or more `t.Run(tt.name, …)` tables

**Unlocks:** Stories 2 (yaml), 3 (render), 4 (etchosts marker), 5 (etchosts atomic), 6 (fileio) — all need `domain.HostsFile` to operate on.

**Done Looks Like:** `cd go && go test ./internal/domain/... -count=1 -v` runs ≥60 named subtests (`t.Run` names visible in `-v` output), all green; `go test ./internal/domain -race` green; `staticcheck ./internal/domain/...` zero findings.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-domain-types-2sw` | Port domain.Entry / Group / HostsFile types | Field shapes match v1; struct tags for yaml.v3 align with `~/.hosts` keys; tiny unit test for zero-value safety |
| `hosts-cli-go-mig-p2-domain-id-a5r` | Port ULID monotonic ID generator (oklog/ulid/v2) | Goroutine-safe `Reader`; tests cover concurrent uniqueness + monotonicity |
| `hosts-cli-go-mig-p2-domain-validators-rhk` | Port validators (hostname / IPv4 / IPv6 / duplicates) with 60+ table-driven tests | One `t.Run` per case; covers v1's positive/negative tables verbatim; embeds Critical Pattern "Parameterize Malformed-Input Tests" in description |

---

### Story 2: YAML I/O is a single seam — Marshal/Unmarshal funnels every `~/.hosts` read and write

**Purpose:** Every `~/.hosts` read and write in the Go tree goes through **one** package with **one** serializer and **one** parser — the v1 divergence between `file-io.ts` and `yaml.ts` cannot recur structurally.

**Why Now:** Needs `domain.HostsFile` (Story 1). Blocks Story 6 (fileio) directly. Independent of Stories 3–5.

**Contributes To:** Exit-state clause 2 (`core/yaml/` single seam, schema validation, round-trip fixed-point on fixture corpus).

**Creates:**
- `go/internal/core/yaml/serialize.go` — `Marshal(domain.HostsFile) ([]byte, error)` with `yaml.v3` `Encoder.SetIndent(2)`
- `go/internal/core/yaml/parse.go` — `Unmarshal([]byte) (domain.HostsFile, error)` with schema check (`version: 1` literal + structural well-formedness)
- `go/internal/core/yaml/yaml_test.go` — round-trip fixed-point on ≥3 fixtures (including ≥1 v1 fixture)
- `go/test/fixtures/hosts/` — fixture corpus (copied or symlinked from v1 `tests/fixtures/`)

**Unlocks:** Story 6 (fileio); Story 7 (golden harness) — the harness's first parity surface is YAML round-trip, which requires this package.

**Done Looks Like:** `go test ./internal/core/yaml -count=1 -v` round-trips every fixture twice (Marshal → Unmarshal → Marshal) with byte-equal second marshal; schema check rejects ≥3 known-malformed inputs (missing `version`, `version: 2`, non-list `groups`) with stable error strings.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-yaml-marshal-9ey` | Implement core/yaml Marshal/Unmarshal with schema validation | yaml.v3, SetIndent(2); schema rejects unknown version, structural malformations; embeds Critical Pattern "One Renderer, One Parser" |
| `hosts-cli-go-mig-p2-yaml-roundtrip-z64` | Port v1 YAML fixture corpus + round-trip fixed-point tests | ≥3 fixtures (incl. ≥1 from v1); Marshal→Unmarshal→Marshal byte-equal; also asserts decoded `HostsFile` deep-equals between rounds |

---

### Story 3: Render produces ONE marker-block shape (no padding) — v1's two-shape divergence is gone

**Purpose:** There is exactly one `RenderManagedBlock` in the codebase. v1's `wrapManagedBlock` (with blank-line padding) and `apply.ts:renderManagedBlock` (without padding) are collapsed to the latter on-disk shape. A `--dry-run` output difference vs. v1 is documented for 2.0.0 release notes.

**Why Now:** Needs `domain.Entry`/`domain.Group` (Story 1). Independent of Stories 2, 4, 5, 6 — render is pure formatting. Blocks Story 4 (etchosts marker) indirectly because marker tests compare against rendered shapes.

**Contributes To:** Exit-state clause 3 (single-shape renderer).

**Creates:**
- `go/internal/core/render/render.go` — `RenderEntry`, `RenderManagedBlock`, `RenderHostsFile`
- `go/internal/core/render/render_test.go` — 18-case port matching v1 `render.test.ts`
- `docs/go-migration/release-notes-2.0.0.md` (or append) — single-line note: "`apply --dry-run` no longer emits blank-line padding inside the managed block"

**Unlocks:** Story 7 (golden harness) — harness needs a stable renderer to compare against v1's output.

**Done Looks Like:** `go test ./internal/core/render -count=1 -v` 18 cases green; manual eyeball of `RenderManagedBlock` output on a 3-entry `HostsFile` shows no blank line between `# BEGIN HOSTIE` and the first entry; release notes file contains the divergence note.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-render-0fg` | Implement core/render with single block shape + 18-case test port | Pure functions; document the v1→v2 padding difference in release-notes-2.0.0.md |

---

### Story 4: Marker extraction is ONE function — strict/lenient split is gone, malformed-input table runs

**Purpose:** Reading the managed block out of `/etc/hosts` (or putting it back) goes through a single `ExtractManagedBlock` + `ReplaceManagedBlock` pair. The v1 strict-vs.-lenient extractor split is gone. The malformed-input table from v1 is ported verbatim and asserted at the highest layer.

**Why Now:** Needs `domain.HostsFile` (Story 1). Blocks Story 5 (atomic write) — atomic write composes marker replacement.

**Contributes To:** Exit-state clause 4 (one extractor, malformed-input table ported).

**Creates:**
- `go/internal/core/etchosts/marker.go` — `ExtractManagedBlock(content string) (block string, found bool)`, `ReplaceManagedBlock(content string, block string) string`
- `go/internal/core/etchosts/marker_test.go` — 19 extract cases + 14 replace cases + the full malformed-input table

**Unlocks:** Story 5 (atomic write).

**Done Looks Like:** `go test ./internal/core/etchosts -run TestMarker -count=1 -v` ≥33 cases green; the malformed-input table is a single `t.Run` table with every case from v1 represented (verifiable by case-count parity with the v1 test file).

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-etchosts-marker-9o1` | Implement core/etchosts/marker.go (single extractor + replacer) | One function per direction; embeds Critical Patterns "One Renderer, One Parser" + "Parameterize Malformed-Input Tests" |
| `hosts-cli-go-mig-p2-etchosts-marker-table-sbj` | Port malformed-input table from v1 etchosts.test.ts to marker_test.go | Verbatim port; case count must match v1 |

---

### Story 5: Atomic etc-hosts write — HIGH-risk pipeline with the v1 bugs fixed

**Purpose:** `WriteEtcHosts(path, content)` writes through a hardened pipeline that doesn't follow symlinks, doesn't EXDEV-fail across filesystems, doesn't preserve dangerous mode bits, and cleans up its tempfile on every exit path. The HIGH-risk item from approach.md §4 is retired with a spike artifact in `.spikes/go-migration/p2-atomic-write/FINDINGS.md`.

**Why Now:** Needs Story 4 (marker) for replace logic. Blocks Phase 3 — Phase 3's `apply/privilege.go` (sudo reexec) assumes `WriteEtcHosts` is already hardened.

**Contributes To:** Exit-state clauses 5 (atomic.go corrected pipeline) and 9 (HIGH-risk atomic-write spike resolved).

**Creates:**
- `go/internal/core/etchosts/atomic.go` — `WriteEtcHosts(path, content string) error`
- `go/internal/core/etchosts/atomic_test.go` — symlink-attack, mode-clamping, EPERM-swallow scope, signal-cleanup, cross-FS rejection (or proof of same-FS guarantee)
- `.spikes/go-migration/p2-atomic-write/FINDINGS.md` — spike record; reproducer scripts kept, tarballs gitignored

**Unlocks:** Phase 3 (privilege.go can build on a known-good base).

**Done Looks Like:** `go test ./internal/core/etchosts -run TestAtomic -count=1 -v` all cases green on both `ubuntu-latest` and `macos-latest`; spike FINDINGS.md committed; the bead's Verification section walks through each of the 5 sub-properties (lstat, same-FS tempfile, mode clamp, EPERM swallow, signal cleanup) with a green assertion.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-atomic-spike-jdb` | Spike: harden atomic etc-hosts write (symlink, EXDEV, mode, EPERM, signal) | `.spikes/go-migration/p2-atomic-write/` with reproducer + FINDINGS.md; blocks the implementation bead; HIGH risk |
| `hosts-cli-go-mig-p2-atomic-impl-o1d` | Implement core/etchosts/atomic.go WriteEtcHosts with 5-property test suite | Depends on the spike bead and Story 4's marker; embeds Critical Pattern "Atomic File Replacement Requires lstat + Same-FS Tempdir + Permission Clamping" verbatim |

---

### Story 6: Fileio funnels all `~/.hosts` access through one package

**Purpose:** Anything in the Go tree that needs to read or write `~/.hosts` calls `fileio.ReadHostsFile`/`fileio.WriteHostsFile`. No package besides `fileio` imports `os.UserHomeDir`. The v1 cached-homedir gotcha cannot recur because Go re-reads `HOME` at every call.

**Why Now:** Needs Story 1 (`domain`) and Story 2 (`yaml`). Pure composition layer.

**Contributes To:** Exit-state clause 6 (`fileio/` single funnel; `os.UserHomeDir` at call time).

**Creates:**
- `go/internal/core/fileio/readwrite.go` — `ReadHostsFile`, `WriteHostsFile`, internal `expandTilde`
- `go/internal/core/fileio/readwrite_test.go` — 10-case port; covers `~/` expansion via `HOME=<tmpdir>`; covers missing-file ENOENT; covers malformed-YAML pass-through error

**Unlocks:** Phase 3 (CLI mutators read/write `~/.hosts` through this seam).

**Done Looks Like:** `grep -r "UserHomeDir\|os\.UserHomeDir" go/internal/` returns exactly the call sites inside `fileio/`; `go test ./internal/core/fileio -count=1 -v` green; tests set `HOME=$TMPDIR` and never touch the real `~/.hosts`.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-fileio-p06` | Implement core/fileio Read/Write with HOME-env injection + 10-case tests | Composes yaml.Marshal/Unmarshal; tests use `t.Setenv("HOME", t.TempDir())` |
| `hosts-cli-go-mig-p2-clipboard-audit-fn0` | Audit transitive dep github.com/atotto/clipboard (Phase-1 surprise) | Determine whether clipboard exec is reachable from core/; if yes, surface to design.md threat model; if gated to TUI (bubbles), record + close |

---

### Story 7: Golden harness operational — first parity surface green against pinned v1 binary

**Purpose:** A `go test -tags golden` invocation downloads a SHA-pinned v1 release tarball, runs both binaries against fixture YAML, and asserts byte-equal canonical output. The harness is the cutover safety net — Phase 3 and Phase 4 plug new parity surfaces in without backfilling infrastructure.

**Why Now:** Last story in Phase 2. Needs Stories 1, 2, 3 (the v2 side needs a working Marshal/Unmarshal/Render pipeline to produce something to diff). Independent of Stories 4–6 for the first surface (YAML round-trip); Stories 4–5 enable extensions in Phase 3 (`apply --dry-run` surface).

**Contributes To:** Exit-state clause 7 (`go/test/golden/harness_test.go` operational + ≥1 fixture passes).

**Creates:**
- `go/test/golden/harness_test.go` — build-tagged with `//go:build golden`; downloads + SHA-verifies pinned v1 tarball; caches under `.cache/`; first parity surface = YAML round-trip
- `go/test/golden/.cache/` — gitignored
- `docs/go-migration/golden-pin.md` — pinned v1 release URL, version, SHA-256, rotation policy
- `.github/workflows/ci.yml` — add `go-golden` job behind tag gate (runs on push to `feature/go-migration` + on PR to `main`)

**Unlocks:** Phase 3 CLI ports get parity-tested per-command. Phase 4 TUI ports get parity-tested per-screen.

**Done Looks Like:** Locally: `cd go && go test -tags golden ./test/golden -v` downloads (first run) or hits cache (subsequent runs), runs ≥3 fixtures through both binaries, asserts byte-equal YAML round-trip, reports `PASS`. In CI: the `go-golden` job is green on the latest push. `docs/go-migration/golden-pin.md` lists URL + SHA-256 and the harness refuses to proceed on SHA mismatch.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p2-golden-pin-spike-dbo` | Spike: pin v1 release for golden harness (URL + SHA-256 + cache shape) | `.spikes/go-migration/p2-golden-pin/` with download/verify reproducer; blocks the harness bead |
| `hosts-cli-go-mig-p2-golden-harness-nn5` | Implement go/test/golden/harness_test.go + YAML round-trip parity surface | Build-tagged `//go:build golden`; refuses on SHA mismatch; cache layout under .cache/ (gitignored); ≥3 fixtures pass |
| `hosts-cli-go-mig-p2-golden-ci-bbz` | Add CI job go-golden behind tag gate | Mirrors go-checks structure post-collapse; runs on push + PR-to-main; uploads divergence reports as artifact on failure |

---

### Story 8: Phase 1 review debt closed — no carry-in to Phase 3

**Purpose:** The 4 deferred Phase-1-review P2 beads close inside Phase 2 (not at Phase 3 start). Future phases inherit a clean review-debt slate.

**Why Now:** These touch CI workflows + tests that change as Stories 1–7 land — folding them in now avoids re-touching the same files in Phase 3. Sequenced last because `ci-job-sprawl-n2y` collapses the same jobs Stories 1–7 just expanded, and `artifact-prefix-ck9` needs to coordinate with the Phase-5 cutover plan.

**Contributes To:** Exit-state clause 8 (4 deferred P2 beads closed).

**Creates / Modifies:**
- `.github/workflows/ci.yml` — `go-build`/`go-test`/`go-vet`/`go-staticcheck` collapsed into one `go-checks` job per OS using step composition (kept as separate steps with `id:` for log clarity; collapsed at the job level so they share setup-go + cache)
- `.github/workflows/release.yml` — `hostie-go-*` artifact name decision recorded (either renamed to `hostie-*` with v1 Bun job aliased, or explicitly carried to phase-plan.md Phase 5 as a cutover step)
- `go/cmd/hostie/main_test.go` — adds `dev` default fallback assertion + version format contract assertion
- `.github/workflows/release.yml` (size-check step) — replaces `stat -c%s` with `wc -c <"file"` (POSIX-portable, identical on macOS BSD and Linux GNU)

**Unlocks:** Phase 3 starts with zero P2 carry-in.

**Done Looks Like:** All 4 of `hosts-cli-p1-review-p2-ci-job-sprawl-n2y`, `hosts-cli-p1-review-p2-artifact-prefix-ck9`, `hosts-cli-p1-review-p2-test-contract-ej6`, `hosts-cli-p1-review-p2-stat-portability-rci` are closed via `br close` with the reason citing the commit SHA that landed the fix. `br list --status open` no longer shows any `hosts-cli-p1-review-p2-*` bead.

**Beads:** Use the existing 4 beads (do not create new ones — pick them up and close them):

| Bead ID | Title |
|---------|-------|
| `hosts-cli-p1-review-p2-ci-job-sprawl-n2y` | Collapse 4 Go CI jobs into one go-checks job per OS |
| `hosts-cli-p1-review-p2-artifact-prefix-ck9` | Rename hostie-go-* artifacts before cutover plan locks |
| `hosts-cli-p1-review-p2-test-contract-ej6` | Strengthen main_test.go — assert default 'dev' fallback + format contract |
| `hosts-cli-p1-review-p2-stat-portability-rci` | Replace 'stat -c%s' with portable size measurement |

---

## Story Order Check

- **Story 1 (domain) first**: no other story compiles without `domain.HostsFile`/`Entry`/`Group`. Pure data + pure functions, zero external deps beyond `oklog/ulid/v2`.
- **Stories 2 (yaml), 3 (render), 4 (marker)** can run in parallel after Story 1 — different files, different package paths, no inter-dependencies between them. Validating + swarming may run them concurrently.
- **Story 5 (atomic)** depends on Story 4 (uses `ReplaceManagedBlock`). Its spike sub-bead can run in parallel with Stories 2 + 3.
- **Story 6 (fileio)** depends on Stories 1 + 2 (composes `domain.HostsFile` + `yaml.Marshal/Unmarshal`). Cannot start before both land.
- **Story 7 (golden harness)** depends on Stories 1 + 2 + 3 (needs a working v2 pipeline to diff against v1). Last technical story.
- **Story 8 (Phase-1 debt sweep)** runs last and touches CI YAML files. Doing it earlier would force re-edits as Story 7 adds the `go-golden` job. Doing it later (Phase 3) reintroduces the carry-in the contract explicitly forbids.

If Story 5's atomic-write spike cannot pass the symlink-attack test (Pivot Signal #2), Phase 2 stops at Story 4 and the design is revisited — Story 5 must land before Phases 3+ are safe to begin.

---

## Coverage Check

| Exit-state clause (from phase-2-contract.md) | Covered by |
|---|---|
| 1. `domain/` ports types, validators, id; 60+ tests | Story 1 |
| 2. `core/yaml/` single seam, schema validation, round-trip fixed-point | Story 2 |
| 3. `core/render/` single shape (no padding), divergence documented | Story 3 |
| 4. `core/etchosts/marker.go` one extractor, malformed-input table ported | Story 4 |
| 5. `core/etchosts/atomic.go` corrected pipeline (lstat / same-FS / clamp / EPERM / signal cleanup) | Story 5 |
| 6. `core/fileio/` single funnel; `os.UserHomeDir` at call time | Story 6 |
| 7. `go/test/golden/harness_test.go` operational; ≥1 fixture passes | Story 7 |
| 8. 4 deferred P2 beads closed | Story 8 |
| 9. HIGH-risk atomic-write spike resolved | Story 5 (spike sub-bead) |
| 10. `go build` / `go test` / `go vet` green on matrix | Cross-cutting; every story's bead asserts in its Done Looks Like |
| 11. `staticcheck` zero findings on Phase 2 packages | Cross-cutting; every story's bead asserts |
| 12. Binary size still ≤ 18 MB; size-budget.md updated | Story 8 (size-check job re-runs and records) |
| 13. v1 source tree still untouched | Cross-cutting invariant; reviewing skill verifies |
| 14. All Phase 2 beads closed via `br close` | Final step before phase reviewing |
| 15. No CLI surface yet — main.go still version-only stub | Cross-cutting invariant; no bead modifies main.go beyond Story 8's test additions |

Every clause covered. No orphan stories. No orphan exit-state clauses.
