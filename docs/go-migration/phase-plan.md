# Phase Plan: Go Migration

## Feature Summary

Rewrite hostie from Bun/TypeScript to a single statically-linked Go binary using the Charm stack (Cobra + Bubble Tea + Bubbles + Lipgloss), driving the compiled binary from **61.20 MB** down to ≤18 MB (aspirational ≤10 MB). The migration preserves the `~/.hosts` schema, `/etc/hosts` marker-block contract, exit-code taxonomy, TUI keybindings, and fuzzy-search weighting (D5/D15). The one intentional behavior change is **auto-apply on mutation** (D11–D14): every CLI mutator and TUI mutation now writes through to `/etc/hosts`, escalating via `sudo` when necessary, with `--dry-run` as the sole opt-out. Ships as **2.0.0** via GitHub Releases only; npm distribution is dropped.

## Why Multiple Phases

Three forcing constraints make this multi-phase, not single-shot:

1. **Layered import graph.** v1's structure is `domain ← core ← {cli, tui}`. Building the CLI requires `apply.Runner`, which requires `core/etchosts/atomic.go`, which requires `core/etchosts/marker.go`. Standing up the TUI requires the same core plus the `apply.Runner` contract from the CLI phase. Trying to ship all four layers in one pass loses the ability to detect parity drift early — the golden harness (D9) needs ported CLI commands to exercise the v1 binary against.
2. **Cutover risk concentration.** The cutover commit deletes 100% of the v1 source tree, drops npm distribution, and bumps to 2.0.0. If any earlier phase has unresolved parity issues, the cutover commit is unsafe. Phases let the golden harness gate every transition; cutover becomes a green-CI formality.
3. **HIGH-risk items are concentrated in Phases 2 and 4** (privileged write, sudo reexec, modal/focus port, sudo TTY handoff, fuzzy score parity). Phasing lets validating spike them with the dependent code already in place, rather than as standalone proofs that may not survive integration.

## Phase Overview

| Phase | Name | What it delivers | Est. beads |
|-------|------|------------------|------------|
| 1 | Bootstrap | `go/` module exists; Go CI lane green; size measurement vs ceiling; first compiled binary | ~6 |
| 2 | Core port | Domain + YAML + render + etchosts atomic write + fileio; golden harness operational | ~10 |
| 3 | CLI port | All Cobra subcommands incl. completion; auto-apply via `apply.Runner`; `--dry-run` on all mutators; CLI golden parity green | ~12 |
| 4 | TUI port | Bubble Tea Model/Update/View; all components + modals + search; teatest coverage; sudo handoff via `tea.ExecProcess` | ~14 |
| 5 | Cutover | Single commit: delete `src/`, move `go/*` to root, retag 2.0.0, drop npm, update docs | ~4 |

**Estimated total: ~46 beads.**

## Phase Details

### Phase 1: Bootstrap

**Entry State:**
- `feature/go-migration` branch active with design.md + discovery.md + approach.md committed.
- v1 source tree (`src/`) untouched, v1 CI lanes green.
- No `go/` directory exists.

**Exit State:**
- `go/go.mod` exists with pinned versions of `spf13/cobra`, `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss`, `gopkg.in/yaml.v3`, `oklog/ulid/v2`, `sahilm/fuzzy`.
- `go/cmd/hostie/main.go` compiles to a do-nothing binary that prints version and exits 0.
- New CI lane (`.github/workflows/ci.yml`) runs `go build`, `go test ./...`, `go vet`, `staticcheck` (advisory) on the Go tree; matrix `[ubuntu-latest, macos-latest]`.
- Size ceiling step in CI: per-platform `ls -l hostie-$os-$arch` asserts ≤ 18 MB; warns at > 10 MB. Documented actual size in `docs/go-migration/size-budget.md`.
- v1 lanes still green; nothing in `src/` was touched.

**Story Previews:**
1. *Go module bootstrap* — `go.mod` + `cmd/hostie/main.go` (version-only stub) + import-only proofs for all Charm deps so we measure realistic dep weight.
2. *Go CI lane* — extend `ci.yml` with parallel Go jobs; `go build`, `go test`, `go vet`, `staticcheck` (advisory).
3. *Size measurement* — `docs/go-migration/size-budget.md` recording first stripped-binary size per platform; CI ceiling job.

### Phase 2: Core port

**Entry State:** Phase 1 exit state.

**Exit State:**
- `go/internal/domain/` ports `types`, `validators`, `id` with 60+ table-driven tests matching v1 coverage.
- `go/internal/core/{yaml,render,etchosts,fileio}/` ports all v1 core logic; collapses v1's two `extractManagedBlock` variants to one and v1's two YAML serialization call sites to one.
- `atomic.go` uses `os.Lstat` (not `Stat`) and same-FS tempfile (not cross-FS `mkdtemp`); chown swallows `unix.EPERM` only; ported malformed-input table-driven tests pass.
- `go/test/golden/harness_test.go` exists and runs against v1 binary (SHA-pinned tarball downloaded in CI) for at least YAML round-trip on a fixture corpus; one fixture passes end-to-end.
- All HIGH-risk core spikes from validating are resolved (atomic-write symlink/mode/EPERM/signal cleanup).

**Story Previews:**
1. *Domain layer* — types, validators (60-case table), id; pure functions, no I/O.
2. *YAML I/O* — single serializer + parser package; schema validation; round-trip fixed-point test on fixture corpus.
3. *Render + marker block* — single renderer (no padding), single `extractManagedBlock`; malformed-input table ported verbatim.
4. *Atomic etc-hosts write* — lstat + same-dir tempfile + clamped perms + chown-EPERM-swallow + rename; symlink-attack test passes.
5. *Fileio* — `~/.hosts` read/write funneled through one package; `os.UserHomeDir()` resolves v1's cached-homedir gotcha.
6. *Golden harness* — pinned v1 binary, fixture corpus, first parity check (YAML round-trip) green.

### Phase 3: CLI port

**Entry State:** Phase 2 exit state.

**Exit State:**
- Cobra root + all v1 subcommands (`add`, `rm`, `enable`, `disable`, `list`, `apply`, `group add`, `completion`, `version`) implemented under `go/internal/cli/commands/`; `group rm/list/mv` ported as parsed-but-unimplemented stubs (exit 1, same message as v1).
- `apply.Runner` in `go/internal/apply/` orchestrates YAML write → render → `/etc/hosts` write; used by every CLI mutator.
- `--dry-run` flag added to every mutating command (`add`, `rm`, `enable`, `disable`, `group add`); `apply --dry-run` preserved.
- `apply/privilege.go` implements the sudo reexec + hidden `__apply-privileged` subcommand using `0600` tempfile payload per D12.
- Golden harness covers `list --json` + `apply --dry-run` parity surfaces against the pinned v1 binary; CI fails on divergence.
- HIGH-risk threat model (`__apply-privileged`) resolved from validating; `docs/go-migration/threat-model.md` committed.
- Per-command test parity: ~90 CLI test cases ported; all green.
- Compiled binary still ≤18 MB.

**Story Previews:**
1. *Cobra root + exitcodes* — root command, version (`-ldflags="-X main.version"`), exit-code constants, hidden `__apply-privileged` subcommand skeleton.
2. *Apply seam* — `apply.Runner` (orchestration), `dryrun.go` (pure compute), wired but not yet called from commands.
3. *Privilege flow* — `privilege.go`: `canWriteEtcHosts` probe, sudo reexec, `__apply-privileged` validates path/owner/mode/payload, unlinks on every exit incl. signals.
4. *Read-only commands* — `list` (with `--json`), `apply` (with existing `--dry-run`), `version`.
5. *Mutating commands* — `add`, `rm`, `enable`, `disable`, each gaining `--dry-run` and auto-apply via `apply.Runner`.
6. *Group commands* — `group add` (with `--dry-run` + auto-apply); `group rm/list/mv` stubs.
7. *Completion + harness expansion* — Cobra completion command (behavior-tested, not byte-snapshotted); golden harness expanded to `list --json` + `apply --dry-run` surfaces.

### Phase 4: TUI port

**Entry State:** Phase 3 exit state.

**Exit State:**
- Bubble Tea `Model`/`Update`/`View` under `go/internal/tui/app/` renders the same 3-pane layout (sidebar / main / statusbar) as v1.
- 9 components ported under `go/internal/tui/components/` (Layout, GroupTree, EntryList, StatusBar, ConfirmModal, HelpModal, EntryEditorModal, GroupCreatorModal, MoveToGroupModal); v1 keybindings and modal flow preserved (D5/D15).
- `go/internal/tui/store/state.go` — plain struct + `sync.RWMutex`, equivalent semantics to v1 zustand store.
- `go/internal/tui/search/weighted.go` — sahilm/fuzzy + weighted aggregator (host×2/alias×1.5/IP×1/group×0.5); score-parity spike resolved against fuse.js fixture corpus.
- TUI mutations dispatch `apply.Runner` via `tea.Cmd`; results update status bar; sudo handoff via `tea.ExecProcess` validated on macOS Terminal, iTerm2, Alacritty, Linux gnome-terminal.
- ~109 v1 TUI test cases ported via teatest; previously-flaky MoveToGroupModal Esc test passes deterministically.
- Hands-on UAT against the v1 keybinding checklist completes successfully before exit.
- Compiled binary still ≤18 MB.

**Story Previews:**
1. *Store + search* — `store/state.go` + `search/weighted.go`; pure data + algorithm; teatest-independent.
2. *Layout + non-modal components* — Layout, GroupTree, EntryList, StatusBar; render-only.
3. *Root Model/Update/View skeleton* — Bubble Tea program boots, renders Layout, handles `j`/`k`/`Tab`/`q`; no mutations yet.
4. *Modal pattern spike* — ConfirmModal end-to-end (open via `d`, route Esc/Enter/y/n, close, return result); proves the pattern.
5. *Remaining modals* — HelpModal, EntryEditorModal, GroupCreatorModal, MoveToGroupModal — fan out once spike validates the pattern.
6. *Mutations + auto-apply* — wire Space/d/a/e/g/m through `apply.Runner` via `tea.Cmd`; status bar reflects outcome.
7. *Sudo TTY handoff* — `tea.ExecProcess(sudoCmd, callback)`; smoke on four terminals; document any quirks.
8. *Search mode + UAT* — `/` enters search mode, results filter the main pane; manual UAT walkthrough against v1 checklist.

### Phase 5: Cutover

**Entry State:** Phase 4 exit state. All v1 + v2 CI lanes green. Golden harness green. UAT signed off.

**Exit State:**
- Single commit:
  - Deletes `src/`, `tests/`, `build.ts`, `bun.lock`, `package.json`, `tsconfig.json`.
  - Moves `go/*` to repo root (`cmd/`, `internal/`, `test/`, `go.mod`, `go.sum`).
  - Updates `README.md`, `AGENTS.md`, `CLAUDE.md` to reflect Go-only state, auto-apply behavior, dropped npm distribution.
  - Removes Bun CI jobs; keeps Go CI jobs (now the only jobs).
  - Updates `.github/workflows/release.yml` to drop the Bun matrix entirely (Go matrix already there from Phase 1).
- Tag `v2.0.0` pushed; release workflow produces 4 platform binaries + checksums + auto-generated release notes covering: npm removal, auto-apply behavior change, `--dry-run` on all mutators, sudo escalation, `tea.ExecProcess` for TUI sudo handoff.
- Post-cutover smoke: `gh release download v2.0.0` on each platform; binary runs, version prints, basic add/list/apply round-trips work.

**Story Previews:**
1. *Doc rewrite* — README, AGENTS, CLAUDE updated for Go-only state + auto-apply + npm removal.
2. *Cutover commit* — delete `src/`, move `go/*` to root, prune CI of Bun jobs; on a dedicated `cutover/v2.0.0` working branch first, then merged to `feature/go-migration`.
3. *Release 2.0.0* — tag, verify workflow output, post-release smoke per platform.
4. *Post-cutover compounding* — capture learnings from the full migration in `docs/learnings/` per the compounding skill.

## Phase Order Check

- **Phase 1 → 2:** Phase 2 ports the `core/` package which imports nothing outside `domain/`. But to land any Go code at all, `go.mod` + CI lane must exist first; otherwise commits don't compile, tests don't run, and size measurement is moot. Cannot merge Phase 1 into Phase 2.
- **Phase 2 → 3:** CLI commands (Phase 3) import `core/yaml`, `core/render`, `core/etchosts`, `core/fileio`. `apply.Runner` (introduced in Phase 3) orchestrates these. Without Phase 2, Phase 3 has no business logic to invoke. Merging would create a giant blast-radius and break the golden harness's ability to gate early.
- **Phase 3 → 4:** TUI mutations dispatch `apply.Runner`. The Runner's contract (signature, error shape, dry-run interface) must exist before the TUI can be written against it. The sudo handoff in Phase 4 also reuses `apply/privilege.go` from Phase 3. If we tried TUI first, we'd either ship two privilege paths or stub the runner and rewrite later.
- **Phase 4 → 5:** Cutover deletes `src/` and drops npm. Doing this before the TUI ships means users on `main` would have neither a working TS TUI nor a working Go TUI. Cutover must come last — when both halves are interchangeable from a user's perspective.

## Approval Summary

- **Total phases:** 5
- **Total estimated beads:** ~46
- **Highest-risk phase:** Phase 4 (TUI) — sudo TTY handoff, 9-component port, modal-focus translation, fuzzy score parity all concentrate here. Phase 3 is second-highest (privileged write path, `__apply-privileged` threat model).
- **Preparing first:** Phase 1 (Bootstrap)

**HARD-GATE:** Approve this phase plan before I prepare Phase 1's contract, story map, and beads.
