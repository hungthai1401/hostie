# Phase 2 Contract: Go Migration — Core Port

## Entry State

- Phase 1 exit state holds: `go/go.mod` pinned, `go/cmd/hostie/main.go` stub compiles, 4 Go CI jobs + size-check green on `feature/go-migration`, size measured at 2.50–2.57 MB across 4 platforms (15.4 MB headroom vs. ≤18 MB D2 ceiling).
- v1 source tree (`src/`, `tests/`, `build.ts`, `package.json`, `bun.lock`, `tsconfig.json`) **untouched** since Phase 1 baseline (`git diff main..feature/go-migration -- src/ tests/` returns empty per D6).
- Branch `feature/go-migration` active; tree clean.
- `go/internal/` exists as an empty package skeleton (`.gitkeep`); no business logic yet.
- Bun test suite green except for 1 pre-existing flake on `main` (`versionCommand > shows correct version from package.json`) — confirmed unrelated to this branch.
- Cobra version handler uses native `cobra.Command{Version: …}` + `SetVersionTemplate` (Phase 1 P2 inline fix); no hand-rolled `--version` flag.
- All 4 P2-fixed review findings closed (`tag-injection-qb9`, `release-coupling-47t`, `cobra-version-on5`, `action-pinning-9mi`).
- 4 deferred Phase-1-review P2 beads remain open and explicitly scheduled into this phase:
  `ci-job-sprawl-n2y`, `artifact-prefix-ck9`, `test-contract-ej6`, `stat-portability-rci`.
- A v1 release tarball is available on GitHub Releases (any tagged 1.x release) to serve as the golden-harness reference binary.
- `go` toolchain ≥ 1.23.0 installed (bubbletea v1.3.6 transitive requirement, confirmed in Phase 1).

---

## Exit State

### Core packages (all under `go/internal/`)

1. **`domain/`** ports `types`, `validators`, `id` to Go:
   - `types.go` defines `Entry`, `Group`, `HostsFile` with the same field shape as v1 (`id string`, ULID-keyed).
   - `validators.go` exports `ValidateHostname`, `ValidateIPv4`, `ValidateIPv6`, `ValidateIP`, `ValidateNoDuplicates` — pure predicates with the same accept/reject set as v1.
   - `id.go` provides monotonic-entropy ULID via `oklog/ulid/v2` with a goroutine-safe `Reader`.
   - **≥ 60 table-driven test cases ported** (31 hostname + 29 IP + N duplicates, matching v1 coverage).
2. **`core/yaml/`** is a **single** package exporting `Marshal(HostsFile) ([]byte, error)` and `Unmarshal([]byte) (HostsFile, error)`:
   - Uses `yaml.v3` `Encoder.SetIndent(2)`.
   - Schema validation runs on Unmarshal (`version: 1` literal check + structural well-formedness).
   - All `~/.hosts` I/O in the new Go tree funnels through this one package — v1's `file-io.ts` vs. `yaml.ts` divergence does not recur.
   - Round-trip fixed-point test passes on a fixture corpus (≥ 3 fixtures, including 1 from v1's existing test fixtures).
3. **`core/render/`** exports `RenderEntry`, `RenderManagedBlock`, `RenderHostsFile` as a **single shape — no blank-line padding inside markers** (the v1 disk shape; collapses v1's two-shape divergence between `wrapManagedBlock` and `renderManagedBlock`). Documented as a `--dry-run` output difference for the 2.0.0 release notes file (release notes file may be a stub; this contract only requires the documentation note exists).
4. **`core/etchosts/marker.go`** exports **one** `ExtractManagedBlock` and one `ReplaceManagedBlock` function — no strict/lenient variants. Malformed-input table from v1 (`etchosts.test.ts` cases) ported verbatim as a table-driven test; every case asserted.
5. **`core/etchosts/atomic.go`** exports `WriteEtcHosts(path, content string) error` with the corrected pipeline:
   - `os.Lstat` (not `os.Stat`) on target — does not follow symlinks.
   - Tempfile created in `filepath.Dir(target)` via `os.CreateTemp` (not `os.MkdirTemp` in `os.TempDir()`) — same filesystem, no `EXDEV` on rename.
   - Mode clamped to `mode & 0o777 & ^0o022` (drop world-write, setuid, setgid, sticky).
   - `chown` is best-effort: swallows **only** `unix.EPERM`; any other error propagates.
   - Tempfile unlinked on every error exit path including signal interruption (`defer` + signal-aware cleanup).
   - Pure functional: no global state; `path` and `content` are the sole inputs.
6. **`core/fileio/`** exports `ReadHostsFile(path string) (HostsFile, error)` and `WriteHostsFile(path string, h HostsFile) error`:
   - Internally calls `os.UserHomeDir()` at every call (no cached homedir gotcha — Go reads `HOME` env at call time, unlike Bun's cached `os.homedir()`).
   - All `~/.hosts` reads + writes in the Go tree go through this package; YAML serializer/parser is the only callee from here.

### Golden harness

7. **`go/test/golden/harness_test.go`** exists and runs in CI under a build tag (`//go:build golden`) so it doesn't run on every `go test ./...`:
   - Downloads a **SHA-pinned** tarball of a tagged v1 release (URL + SHA-256 recorded in `docs/go-migration/golden-pin.md`); refuses to proceed if the SHA mismatches.
   - Caches the unpacked binary under `go/test/golden/.cache/v1-<version>/hostie` (gitignored).
   - **At least one fixture passes end-to-end** for the YAML round-trip parity surface: feed fixture YAML into both v1 and v2 binaries, capture the canonical-form output of each, assert byte-equal.
   - Has clear extension points (TODOs are fine) for the parity surfaces that land in Phase 3 (`list --json`, `apply --dry-run`).

### Phase 1 carryover beads closed

8. The 4 deferred P2 beads are closed inside this phase (not deferred again):
   - `ci-job-sprawl-n2y`: 4 Go CI jobs (`go-build`, `go-test`, `go-vet`, `go-staticcheck`) collapsed into one `go-checks` job per OS using `make` or step composition; CI wall-time on `feature/go-migration` does not regress.
   - `artifact-prefix-ck9`: `hostie-go-*` artifact names in `release.yml` decided — either renamed now to the Phase 5 cutover scheme (`hostie-*` matching v1) with the Bun job temporarily aliased, OR explicitly documented as a Phase-5 cutover step in `phase-plan.md`. No silent carry into cutover.
   - `test-contract-ej6`: `cmd/hostie/main_test.go` asserts both the `dev` default fallback (no `-ldflags`) and the version format contract (`hostie v<version>\n`).
   - `stat-portability-rci`: size-check workflow replaces `stat -c%s` with `wc -c <"file"` or `ls -l | awk` so it runs identically on macOS BSD `stat` and Linux GNU `stat`.

### HIGH-risk spikes resolved (carry-in from validating)

9. The HIGH-risk items from approach.md §4 that fall in the Core phase are resolved with concrete artifacts before the corresponding bead closes:
   - **`core/etchosts/atomic.go`**: spike under `.spikes/go-migration/p2-atomic-write/` demonstrates symlink-attack rejection, mode clamping, EPERM-swallow scope, and signal-cleanup. FINDINGS.md commits the decision.
   - Other HIGH-risk items (`apply/privilege.go`, TUI components, search, sudo handoff) are **out of scope for Phase 2** — they belong to Phases 3 and 4.

### Cross-cutting invariants

10. `go build ./...` + `go test ./...` + `go vet ./...` all green on the matrix `[ubuntu-latest, macos-latest]`.
11. `staticcheck` advisory check produces **zero** findings against the new Phase 2 packages (it has been advisory through Phase 1; Phase 2 sets the floor).
12. Binary size measured again after Phase 2: still ≤ 18 MB (no regression flag fires). Recorded in `docs/go-migration/size-budget.md` as a delta from the Phase 1 baseline.
13. v1 source tree still untouched (`git diff main..feature/go-migration -- src/ tests/` still empty).
14. All Phase 2 beads closed via `br close`; the epic remains open until Phase 5 cutover.
15. **No CLI surface yet** — `hostie` binary still prints version and exits. The core packages exist and are tested in isolation but are not wired into `main.go`. (Wiring happens in Phase 3.)

---

## Demo Story

A developer pulls `feature/go-migration` after Phase 2 lands. They open `go/internal/domain/validators_test.go` and see 60+ table cases, each marked `t.Run(tt.name, …)`, all green under `go test ./internal/domain`. They open `go/internal/core/etchosts/atomic_test.go` and see a symlink-attack test that creates `/tmp/hostie-demo/target` as a symlink to a sibling file, runs `WriteEtcHosts`, and asserts the symlink is **replaced** (not followed) and the sibling is untouched. They `cd go/test/golden && go test -tags golden -v` — the harness downloads the pinned v1 tarball (or hits cache), spins up both binaries on three fixture `~/.hosts` files, and reports `PASS: yaml-roundtrip (3 fixtures, 0 divergences)`. They re-run the Phase 1 size check: binary is still ≤ 18 MB. They open `.github/workflows/ci.yml` — the 4 Go jobs are now one `go-checks` job per OS, taking less wall time than before. They run `./dist/hostie --version` from the still-present v1 binary path and see v1 still works untouched. Nothing user-visible has changed in the `hostie` Go binary itself — it still just prints version. The Core port is **internal-only** and **integration-ready** for Phase 3.

---

## Unlocks

- **Phase 3 (CLI port) can begin** with all core seams (`yaml`, `render`, `etchosts/marker`, `etchosts/atomic`, `fileio`) tested in isolation. Phase 3's `apply.Runner` composes these packages; without them, Phase 3 cannot start.
- **Golden harness is operational** so every Phase 3 CLI command port can be parity-tested against the pinned v1 binary as it lands — Phase 3 doesn't have to backfill harness infrastructure.
- **The duplicate-extractor and dual-renderer divergences from v1 are gone** before any new CLI/TUI code can reintroduce them. The "One Renderer, One Parser" critical pattern is enforced structurally.
- **The atomic-write HIGH risk is retired** before privileged sudo reexec lands in Phase 3 — Phase 3's `privilege.go` builds on a known-good `WriteEtcHosts`.
- **Phase-1 review debt is zero** entering Phase 3: the 4 deferred P2 beads are closed; no carry-in mass to triage at Phase 3 start.

---

## Pivot Signals

- **`yaml.v3` cannot reproduce v1 fixture corpus on round-trip** (semantic fixed-point fails) — escalate before completing the `core/yaml/` bead. The golden harness might still pass on a curated fixture set, but if real-user `~/.hosts` shapes break round-trip, the parity contract D15 cannot hold. Fallback: investigate `sigs.k8s.io/yaml` or a custom marshaller; do not paper over with format coercion.
- **Atomic-write spike cannot pass the symlink-attack test on macOS** (HFS+/APFS semantics differ from ext4 around tempfile-as-symlink-target races) — escalate before closing the atomic.go bead. Phase 3's privilege path assumes a hardened `WriteEtcHosts`; without it, the threat-model in D12 is unsound.
- **Golden harness cannot download or verify a SHA-pinned v1 tarball** (e.g., releases endpoint changes, SHA collision with mirror) — escalate. The harness is the parity contract; if it can't gate cutover, the whole migration loses its safety net.
- **Phase 2 binary size > 18 MB** (extremely unlikely given 15.4 MB Phase 1 headroom, but record-as-pivot signal per D2). Drop a Charm dep or restructure imports before continuing.
- **The transitive `github.com/atotto/clipboard` dep (via `bubbles/textinput`) shells out to `pbcopy`/`xclip` in ways that touch the threat model** — surface during the dep audit baked into the `fileio` bead's review. If clipboard exec is reachable from `core/`, escalate to design.md amendment; if it's gated behind `bubbles` (TUI-only, Phase 4), record and move on.
- **Phase 1 carryover P2 work expands during fix** (e.g., `ci-job-sprawl` collapse breaks a needed isolation between `go test` and `go vet`) — abandon the collapse, document the rationale in the bead closure, and keep the 4 jobs.
- **bubbletea v2 lands as a transitive bump** despite the dependabot major-ignore (e.g., via a different upstream pulling it in) — pin explicitly with `// indirect` lock and escalate. Phase 4's API contract depends on v1.x.
