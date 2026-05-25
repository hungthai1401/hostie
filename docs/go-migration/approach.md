# Approach: Go Migration

**Date:** 2025-11-22
**Feature:** go-migration
**Based on:**
- `docs/go-migration/design.md` (D1–D15 locked)
- `docs/go-migration/discovery.md`
- `docs/go-migration/go-port-feasibility.md`

---

## 1. Gap Analysis

| Component | Have (v1, TS/Bun) | Need (v2, Go) | Gap Size |
|-----------|-------------------|---------------|----------|
| Domain types | `src/domain/types.ts` (Entry/Group/HostsFile) | `go/internal/domain/types.go` — same shape, ULID string id | **New** (port) |
| Validators | `src/domain/validators.ts` (hostname/IPv4/IPv6/duplicates) | `go/internal/domain/validators.go` — identical predicates | **New** (port) |
| ID generator | `src/domain/id.ts` (monotonic ULID) | `go/internal/domain/id.go` via `oklog/ulid/v2` `MonotonicEntropy` | **New** (port) |
| YAML I/O | `src/core/yaml.ts` + `src/core/file-io.ts` (2 call sites, inconsistent options) | `go/internal/core/yaml/` — single serializer, all `~/.hosts` I/O funnels through it | **New** + collapse |
| Render | `src/core/render.ts` (`wrapManagedBlock` with blank padding) + `src/core/apply.ts:renderManagedBlock` (no padding) | `go/internal/core/render/` — **one** shape (no padding, the on-disk shape) | **New** + collapse |
| Marker / atomic write | `src/core/etchosts.ts` (`statSync`, cross-FS `mkdtempSync`) | `go/internal/core/etchosts/` — **fix bugs**: `os.Lstat`, same-dir `os.CreateTemp` | **New** + fix |
| Extract managed block | Two variants (strict in `apply.ts`, lenient in `etchosts.ts`) | One function in `go/internal/core/etchosts/marker.go` | **New** + collapse |
| Apply runner | `src/core/apply.ts:applyHostsFile` + ad-hoc reexec | `go/internal/apply/runner.go` + `privilege.go` + `dryrun.go` | **New** + expand |
| Sudo reexec | `src/core/apply.ts:reexecWithSudo` (EACCES-triggered only) | `go/internal/apply/privilege.go` with hidden `__apply-privileged` subcommand + `0600` tempfile payload | **New** (D12) |
| Exit codes | `src/cli/exit-codes.ts` enum | `go/internal/cli/exitcodes/codes.go` constants | **New** (port) |
| CLI parser | `src/cli/index.ts` (commander) | `go/internal/cli/commands/` (Cobra root + 9 subcommands) | **New** |
| Mutating commands | `add/rm/enable/disable/group add/apply` (no auto-apply) | Same surface + **auto-apply after YAML write** + **`--dry-run` on all mutators** (D11, D14) | **New** behavior |
| `group rm/list/mv` | Parsed-but-unimplemented (returns "not implemented") | Same — keep parsed-but-unimplemented; same exit 1 message | **New** (port stub) |
| Shell completion | Hand-rolled bash/zsh/fish scripts in `src/cli/commands/completion.ts` | Cobra-generated bash/zsh/fish/powershell | **New** (replace) |
| Version | `versionCommand` reads `package.json` | Go `-ldflags="-X main.version=…"` injected at build time | **New** |
| TUI shell | Ink `<App/>` + zustand store + React hooks | Bubble Tea `Model`/`Update`/`View` + plain struct store with `sync.RWMutex` | **New** (rewrite) |
| TUI components | 9 React components (`Layout`, `GroupTree`, `EntryList`, `StatusBar`, 5 modals) | Equivalent Bubble Tea components under `go/internal/tui/components/` | **New** (rewrite) |
| Search | `useSearch` over fuse.js with 4 weighted keys | `go/internal/tui/search/weighted.go` — sahilm/fuzzy + weighted aggregator | **New** (rewrite) |
| Keyboard | `useKeyboard` single `useInput` callback | Bubble Tea `KeyMsg` dispatch in root `Update` | **New** (rewrite) |
| Tests | ~424 tests across `tests/` + `src/**/__tests__/` | Port 1:1 to Go test files matching new package layout | **New** (port) |
| Golden harness | Does not exist | `go/test/golden/` — runs pinned v1 binary + new Go binary, diffs parity surfaces | **New** |
| Build | `build.ts` wrapping `bun build --compile --minify` (61.20 MB) | `go build -trimpath -ldflags="-s -w"` (target 10–15 MB) | **New** |
| Release workflow | `.github/workflows/release.yml` 4-target Bun matrix | Same workflow shape, `actions/setup-go@v5` + `GOOS`/`GOARCH` matrix + size-ceiling assertion | **Modify** |
| npm distribution | `package.json` `bin: hostie` | **Dropped** (D7) | **Remove** at cutover |

---

## 2. Recommended Approach

Greenfield Go module under `go/` (Layer-by-layer port, parallel tree per D6) implemented in five phases that mirror the v1 layering: **Bootstrap → Core → CLI → TUI → Cutover**. Each phase is independently demoable and CI-green before the next begins, so the v1 binary keeps shipping if any phase stalls. Auto-apply (D11–D14) lands in the **CLI phase** as part of `apply.Runner`, with the TUI consuming the same runner asynchronously in Phase 4. The cross-binary golden harness (D9) stands up at the end of the Core phase and gates every subsequent phase. Cutover is a single commit when all green.

### Why This Approach

- **Layered phases mirror v1 import graph** (domain ← core ← cli, core ← tui) so we never have to mock unported components — connects to the existing structure documented in discovery §"Architecture Snapshot".
- **`apply.Runner` is the single seam for auto-apply** — honors D11/D13/D14 by funneling CLI mutators *and* TUI mutations through one path that owns the YAML-write → render → apply pipeline. Without this seam, auto-apply would be duplicated in ~7 CLI commands plus 4 TUI key handlers.
- **Golden harness gates every phase after Core** — honors D9/D15 parity contract; catches drift early instead of at cutover.
- **`tea.ExecProcess` for sudo handoff** — research (go-port-feasibility §3) confirms this is the canonical Bubble Tea primitive; no custom TTY juggling. Reduces Phase 4 risk from HIGH to MEDIUM.
- **`0600` tempfile payload** — D12 amendment; portable on stock macOS/Linux; payload off-argv (no `ps` leak) and unlinked on every exit path.
- **Avoids gotchas from discovery**: `os.Lstat` (not `Stat`); `os.CreateTemp(filepath.Dir(target), …)` (same-FS, no EXDEV); single rendered shape (no padding) to eliminate the v1 render/apply divergence; single YAML serializer + parser (closes the `file-io.ts`/`yaml.ts` inconsistency).

### Key Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Module layout | `go/cmd/hostie` + `go/internal/{domain,core,apply,cli,tui}` | Honors D6; `internal/` keeps API surface unexported; matches v1 layering |
| Apply seam | `apply.Runner` in `go/internal/apply/` — used by CLI mutators AND TUI mutations | Single source of truth for D11/D13/D14; eliminates duplication |
| Privilege transport | `0600` tempfile path passed as argv to `__apply-privileged` subcommand; child re-validates, atomic-writes, unlinks | D12 amended; portable, off-argv, off-ps, signal-safe via `defer` |
| Etc-hosts path injection | `apply.Runner` takes `EtcHostsPath string` field (default `"/etc/hosts"`); tests pass tempdir path; sudo branch only triggers when path is literal `/etc/hosts` AND `unix.Access(W_OK)` fails | Eliminates v1's `spyOn(fs, …)` test pattern; tests stay fully unprivileged |
| `~/.hosts` path injection | `fileio.Reader`/`Writer` calls `os.UserHomeDir()` at call time | Go reads `HOME` env at call time (unlike Bun's cached `os.homedir()`) — tests just set `HOME=<tmpdir>` |
| Marker block shape | Single renderer in `core/render`, no blank-line padding inside markers (the v1 disk shape) | Collapses v1's render/apply divergence; documented as a `--dry-run` output change in 2.0.0 release notes |
| YAML serializer | One package (`go/internal/core/yaml/`); all `~/.hosts` I/O funnels through it; `yaml.v3` with `Encoder.SetIndent(2)` | Closes v1's `file-io.ts` vs `yaml.ts` inconsistency; D15 parity = semantic fixed-point, not bytes |
| Search weighting | Caller-implemented aggregator over `sahilm/fuzzy`: `combined[id] = max_field(weight_field * fuzzy_score)` with exact-match shortcut | sahilm/fuzzy has no built-in weighted multi-field; this matches fuse.js semantics; parity validated by golden score-corpus spike |
| TUI ↔ apply | Mutations dispatch `apply.Runner` via `tea.Cmd` returning a `tea.Msg` on completion; store updates from message | Bubble Tea idiom; off the UI goroutine; status line subscribes to messages |
| Sudo handoff in TUI | `tea.ExecProcess(sudoCmd, callback)` | Releases terminal, runs sudo blocking, re-acquires terminal, posts callback Msg — canonical Bubble Tea primitive |
| Version injection | `-ldflags="-X main.version=$VERSION"` in build | Avoids reading any embedded file; works with `-trimpath` |
| Cobra completion | Generated at runtime via `hostie completion <shell>`; tested by **behavior** (run against fixture inputs, assert outputs) | Cobra script bytes are not stable across minor versions; pin Cobra in `go.mod` to mitigate |
| Test layout | `*_test.go` co-located with implementation (Go idiom); top-level `go/test/golden/` for cross-binary harness; `go/test/e2e/` for compiled-binary smokes | Matches Go convention; smoke tests `go build` then exec per Phase 0 learning #2 |
| Golden harness | v1 binary pinned as **release-tagged tarball, SHA-pinned**, downloaded in CI; not built in-CI | Avoids requiring Bun in the Go CI lane; deterministic; survives cutover |
| Lint | `go vet` mandatory; `staticcheck` advisory at first, mandatory after Phase 2 settles | Hostie has no current lint config; introduce gradually to avoid noise |
| Size ceiling | Per-platform CI step asserts `ls -l hostie-$os-$arch` ≤ 18 MB; warns at > 10 MB | Honors D2; warning gives signal without blocking 10–15 MB band |

---

## 3. Alternatives Considered

### Option A: Incremental port (TS reduce, Go grow)

- **Description:** Port domain + core to Go, expose as a Go-built `hostie-core` binary; have the existing Bun TUI shell out to it; port the TUI last.
- **Why considered:** Lets us ship intermediate binary-size wins before the TUI port finishes.
- **Why rejected:** **Contradicts D1** (full rewrite, single binary). Also doubles the runtime surface: two binaries, two release pipelines, shell-out latency on every command. The 61 MB → ≤18 MB win comes entirely from removing Bun's embedded runtime; partial ports don't unlock it.

### Option B: Single-shot rewrite without phases

- **Description:** Stand up the entire Go tree (domain + core + CLI + TUI) in one branch, run all 424 ported tests, cut over.
- **Why considered:** Conceptually simpler; one commit, one tag.
- **Why rejected:** Loses the ability to detect parity drift early. A YAML round-trip bug or marker-block divergence wouldn't surface until the golden harness runs at the very end, with thousands of LOC to bisect. Phased delivery gives the harness real fixtures to chew on continuously from Phase 2 onward.

### Option C: Drop a Charm dep to hit 10 MB

- **Description:** Skip `bubbles` (use raw Bubble Tea + Lipgloss only); hand-roll list/spinner/textinput primitives.
- **Why considered:** Research suggests `bubbles` is the difference between 10 MB and 13 MB.
- **Why rejected:** D2 sets ≤10 MB as **aspirational, not required**. Hand-rolling list + textinput is 500–800 LOC of avoidable work that re-implements bugs the Charm team has already fixed. Document actual size in 2.0.0 release notes; revisit only if first build lands > 18 MB.

### Option D: Use `urfave/cli` instead of Cobra

- **Description:** Smaller binary footprint than Cobra; simpler API.
- **Why rejected:** **Contradicts D4.** Cobra's completion generator is the reason we picked it (replaces the hand-rolled completion scripts). `urfave/cli` v2 has weaker completion support; v3's is still maturing.

### Option E: Skip the cross-binary golden harness

- **Description:** Trust the 1:1 ported test suite (D9) as the parity proof.
- **Why rejected:** Ported tests verify Go-against-Go expectations. They cannot detect drift where the ported test itself was wrong (e.g., a test that asserts the wrong YAML quoting because it was authored to the new behavior). Cross-binary harness against the pinned v1 binary is the only check that compares **observable behavior** to **shipped behavior**. D9 explicitly requires it.

---

## 4. Risk Map

Every component appears here; workers calibrate care accordingly.

| Component | Risk Level | Reason | Verification Needed |
|-----------|------------|--------|---------------------|
| `domain/types.go`, `domain/id.go` | **LOW** | Pure data; trivial port; ULID lib is well-known | Proceed; tests port 1:1 |
| `domain/validators.go` | **LOW** | Pure functions; v1 has 31 + 29 = 60 test cases covering edge cases | Proceed; port `test.each` to `t.Run` |
| `core/yaml/` | **MEDIUM** | yaml.v3 quoting may differ from npm `yaml`; v1 has two inconsistent call sites to collapse | Round-trip fixed-point test on v1 fixture corpus (spike in validating) |
| `core/render/` | **LOW** | Pure function; collapses v1's two-shape divergence to one | Proceed; document `--dry-run` output change in release notes |
| `core/etchosts/marker.go` | **LOW** | Single function replacing v1's two variants; thoroughly tested in v1 (19 + 14 cases) | Proceed; port malformed-input table verbatim |
| `core/etchosts/atomic.go` | **HIGH** | Privileged write path; v1 has two bugs we MUST fix (statSync, cross-FS tempdir); EPERM-swallow semantics subtle | Spike: actual write to a tempdir-as-`/etc/hosts` with mode/uid/gid assertions; symlink-attack test |
| `core/fileio/` | **LOW** | Plain file I/O; Go's `os.UserHomeDir()` resolves the v1 caching gotcha | Proceed |
| `apply/runner.go` | **MEDIUM** | New seam; orchestrates YAML-write → render → apply; must be idempotent | Unit tests + integration via CLI commands |
| `apply/privilege.go` (sudo reexec + `__apply-privileged`) | **HIGH** | New code path; security-sensitive; tempfile lifecycle across signals; argv validation | Threat-model doc + spike: tempfile ownership/mode assertions, signal-handler unlink, reject-malformed payload, reject non-`$TMPDIR` paths |
| `apply/dryrun.go` | **LOW** | Pure compute; produces same diff `apply --dry-run` would | Asserted by golden harness |
| `cli/exitcodes/` | **LOW** | Four constants | Proceed |
| `cli/commands/` (Cobra root + all subcommands) | **MEDIUM** | New parser; commander → Cobra flag/arg semantics differ subtly; auto-apply wiring on every mutator | Per-command tests port 1:1; golden harness on `list --json` |
| `cli/commands/completion.go` | **MEDIUM** | Cobra-generated; not byte-stable across versions; behavior must match v1 enough for shells | Behavior test (run completion, assert key tokens present); pin Cobra version |
| `tui/store/` | **MEDIUM** | Zustand → plain struct + RWMutex; semantics differ around batched updates | Teatest coverage matching v1 store tests (13 + 3 + 1 = 17 cases) |
| `tui/components/` | **MEDIUM** | 9 components to rewrite; Ink declarative → Bubble Tea Elm-loop is non-trivial | Per-component teatest snapshots |
| `tui/components/` modals | **HIGH** | Modal focus/keyboard routing is the trickiest Ink→Bubble Tea translation; v1 had a known MoveToGroupModal Esc flake | Spike: stand up one modal end-to-end (recommend `ConfirmModal`) before fanning out |
| `tui/search/weighted.go` | **HIGH** | sahilm/fuzzy has no weighted multi-field; aggregator must match fuse.js ranking | Spike: score-parity on fixture query corpus, assert top-5 order |
| `tui/app/` (root Model/Update/View + sudo handoff) | **HIGH** | `tea.ExecProcess` for sudo handoff over TTY; multi-terminal validation needed | Spike: smoke on macOS Terminal, iTerm2, Alacritty, Linux gnome-terminal |
| Golden harness (`go/test/golden/`) | **MEDIUM** | New infrastructure; must pin v1 binary deterministically; CI must download SHA-pinned tarball | Stand up at end of Phase 2 with one fixture; expand over phases |
| GitHub Actions release workflow | **LOW** | Mechanical swap: setup-bun → setup-go; same matrix shape; checksums + upload reusable | Proceed; add size-ceiling step |
| Size ceiling | **MEDIUM** | 18 MB hard cap likely met; 10 MB aspirational likely missed | Measure on first Phase 1 build; document actual size |

### Risk Classification Reference

```
Pattern in codebase?   -> YES = LOW base
External dependency?   -> YES = HIGH base
Blast radius > 5 files? -> YES = HIGH
Otherwise              -> MEDIUM
```

### HIGH-Risk Summary (for validating skill)

The validating skill will create spikes for these items:

- **`core/etchosts/atomic.go`**: Does a `lstat`-based + same-dir-tempfile + clamped-perms + chown-EPERM-swallow + rename pipeline behave correctly when `/etc/hosts` is a symlink, a normal file, and a missing file? Symlink-attack test passes?
- **`apply/privilege.go`**: Does the `__apply-privileged` subcommand (a) reject any payload path outside `$TMPDIR`, (b) reject any payload not owned by `getuid()` parent, (c) reject any payload not mode `0600`, (d) reject any payload that doesn't parse as a well-formed hostie-managed block, (e) unlink the tempfile on every exit path including signal handlers?
- **`tui/components/` modals**: Stand up `ConfirmModal` end-to-end (open via `d` key, render, route Esc/Enter/y/n correctly, close, return result to parent) before fanning out to the other 4 modals. Demonstrates the modal-focus pattern works.
- **`tui/search/weighted.go`**: Does the sahilm/fuzzy + weighted aggregator produce the same top-5 ranking as fuse.js on a fixture query corpus drawn from real `~/.hosts` shapes? Define corpus, define exact equality predicate.
- **`tui/app/` sudo handoff**: Does `tea.ExecProcess` releasing the TTY to `sudo`, then re-acquiring after return, behave correctly on macOS Terminal, iTerm2, Alacritty, and Linux gnome-terminal? Document any terminal-specific quirks.

---

## 5. Proposed File Structure

```
go/
  cmd/
    hostie/
      main.go                          # Cobra root; if no args, launch TUI; version via -ldflags
  internal/
    domain/
      types.go                         # Entry, Group, HostsFile
      types_test.go
      id.go                            # ULID monotonic
      id_test.go
      validators.go                    # validateHostname, validateIPv4, validateIPv6, validateIP, validateNoDuplicates
      validators_test.go               # 60-case table-driven port
    core/
      yaml/
        serialize.go                   # Marshal HostsFile -> []byte; SetIndent(2)
        parse.go                       # Unmarshal []byte -> HostsFile; schema validation (version: 1, structural checks)
        yaml_test.go                   # round-trip fixed-point on fixture corpus
      render/
        render.go                      # renderEntry, renderManagedBlock, renderHostsFile (single shape, no padding)
        render_test.go                 # 18-case port
      etchosts/
        marker.go                      # extractManagedBlock (one function), replaceManagedBlock
        marker_test.go                 # 19+14-case port incl. malformed-input table
        atomic.go                      # WriteEtcHosts(path, content) — lstat + same-dir tempfile + clamped perms + rename
        atomic_test.go                 # symlink, mode preservation, EPERM-swallow, signal cleanup
      fileio/
        readwrite.go                   # ReadHostsFile, WriteHostsFile; expandTilde via os.UserHomeDir
        readwrite_test.go              # 10-case port
    apply/
      runner.go                        # Runner{HostsPath, EtcHostsPath}.Apply(hostsFile) -> Result
      runner_test.go
      privilege.go                     # canWriteEtcHosts; reexecWithSudo; __apply-privileged subcommand handler
      privilege_test.go                # threat-model assertions (payload path validation, ownership, mode, signal cleanup)
      dryrun.go                        # DryRun(hostsFile) -> Diff
      dryrun_test.go
    cli/
      exitcodes/
        codes.go                       # Success=0, Validation=1, IOError=2, Permission=3
      commands/
        root.go                        # Cobra root cmd; --version; no-args -> TUI
        add.go                         # + --dry-run; calls apply.Runner after fileio.Write
        rm.go                          # + --dry-run
        enable.go                      # + --dry-run
        disable.go                     # + --dry-run
        list.go                        # --json; no auto-apply (read-only)
        apply.go                       # existing --dry-run preserved
        group.go                       # group add (+ --dry-run); group rm/list/mv stubs (exit 1 "not implemented")
        completion.go                  # Cobra-generated bash/zsh/fish/powershell
        version.go                     # prints "hostie v$version"
        apply_privileged.go            # hidden "__apply-privileged" subcommand; Cobra Hidden: true
        commands_test.go               # per-command tests port 1:1
    tui/
      app/
        model.go                       # root Model struct
        update.go                      # KeyMsg dispatch, modal routing, apply.Runner Cmd dispatch
        view.go                        # Lipgloss layout composition
        app_test.go                    # teatest end-to-end smokes
      store/
        state.go                       # plain struct + sync.RWMutex
        state_test.go                  # 17-case port
      search/
        weighted.go                    # sahilm/fuzzy + aggregator (host×2/alias×1.5/IP×1/group×0.5)
        weighted_test.go               # 11-case port + score-parity corpus
      components/
        layout.go                      # Sidebar/Main/StatusBar Lipgloss frame
        grouptree.go                   # recursive tree renderer
        entrylist.go                   # table renderer
        statusbar.go
        confirm_modal.go
        help_modal.go
        entry_editor_modal.go          # form with field navigation
        group_creator_modal.go
        move_to_group_modal.go
        components_test.go             # per-component teatest snapshots (mirror v1 9-component test files)
  test/
    golden/
      fixtures/                        # *.hosts (~/.hosts inputs), *.etchosts (rendered outputs)
      harness_test.go                  # downloads pinned v1 binary; runs v1 and v2 against each fixture; diffs YAML round-trip + list --json + apply --dry-run
      v1_binary/                       # .gitignore'd; populated by harness setup
    e2e/
      smoke_test.go                    # `go build` then exec; basic CLI surface checks
  go.mod
  go.sum

.github/workflows/
  ci.yml                               # MODIFY: add Go lane (go build, go test, go vet, staticcheck)
  release.yml                          # MODIFY: setup-go + GOOS/GOARCH matrix + size-ceiling assertion

docs/go-migration/
  approach.md                          # this file
  threat-model.md                      # written during validating (spike output)
  size-budget.md                       # written during validating (actual size measurements)
```

---

## 6. Dependency Order

```
Layer 1 (parallel — no deps):
  - domain/types.go
  - domain/id.go
  - cli/exitcodes/codes.go
  - go.mod bootstrap + CI lane

Layer 2 (parallel — depend on Layer 1):
  - domain/validators.go             (uses domain/types)
  - core/yaml/                       (uses domain/types)
  - core/render/                     (uses domain/types)
  - core/etchosts/marker.go          (uses domain/types)

Layer 3 (parallel — depend on Layer 2):
  - core/etchosts/atomic.go          (uses core/etchosts/marker)
  - core/fileio/                     (uses core/yaml, domain/types)

Layer 4 (sequential — single seam):
  - apply/dryrun.go                  (uses core/render)
  - apply/runner.go                  (uses core/fileio, core/render, core/etchosts, apply/dryrun)
  - apply/privilege.go               (uses apply/runner; introduces __apply-privileged subcommand)

Layer 5 (parallel — CLI commands; depend on apply/ + fileio):
  - cli/commands/root.go             (must land first; defines Cobra root for siblings to register against)
  - cli/commands/{add,rm,enable,disable,list,apply,group,completion,version}.go
  - cli/commands/apply_privileged.go (hidden subcommand)

Layer 6 (parallel — TUI; depends on Layer 5 for apply.Runner contract):
  - tui/store/state.go
  - tui/search/weighted.go
  - tui/components/* (each component independent; layout first)
  - tui/app/{model,update,view}.go   (depends on store + components + apply.Runner)

Layer 7 (sequential — gating):
  - test/golden/harness_test.go      (depends on Layer 5 CLI commands existing)
  - test/e2e/smoke_test.go           (depends on compiled binary from cmd/hostie)
  - Size-ceiling CI step             (depends on release.yml + cross-compiled artifacts)

Layer 8 (final — single commit):
  - Cutover: delete src/, build.ts, package.json, bun.lock; move go/* to root; update README/AGENTS; tag v2.0.0
```

### Parallelizable Groups

- **Group A (Bootstrap):** go.mod, CI lane, `domain/types`, `domain/id`, `exitcodes` — no internal deps, parallel.
- **Group B (Pure functions):** `domain/validators`, `core/yaml`, `core/render`, `core/etchosts/marker` — depend only on `domain/types`, parallel.
- **Group C (I/O):** `core/etchosts/atomic`, `core/fileio` — parallel once Group B done.
- **Group D (Apply seam, sequential):** `apply/dryrun` → `apply/runner` → `apply/privilege`. Sequential because each layer builds on the previous.
- **Group E (CLI commands):** all 9 subcommands + hidden `__apply-privileged` parallel once `root.go` + apply seam are in.
- **Group F (TUI):** `store`, `search`, individual `components/*` parallel; `app/` last once others done.
- **Group G (Harness + smokes):** golden harness + e2e smoke + size-ceiling CI; parallel once CLI is in.
- **Cutover:** single commit; not parallelizable.

---

## 7. Institutional Learnings Applied

| Learning Source | Key Insight | How Applied |
|-----------------|-------------|-------------|
| `docs/learnings/critical-patterns.md` (atomic file replacement) | v1 uses `statSync` (follows symlinks) + cross-FS `mkdtempSync` — both bugs | `core/etchosts/atomic.go` uses `os.Lstat` and `os.CreateTemp(filepath.Dir(target), …)`; perms `& 0o0777 &^ 0o022`; chown swallows `unix.EPERM` only |
| `docs/learnings/critical-patterns.md` (smoke tests target compiled binary) | v1 reexec broke twice when tests ran against source instead of the compiled binary | `go/test/e2e/smoke_test.go` does `go build` then exec; release CI smokes each cross-compiled artifact |
| `docs/learnings/critical-patterns.md` (three-level artifact verification) | v1 Phase 1A shipped components that passed unit tests but were never wired into the integration root | Each bead's verification criteria include L1 (file exists), L2 (substantive impl with tests), L3 (wired into `cmd/hostie/main.go` or registered Cobra command or rendered in TUI tree) |
| `docs/learnings/critical-patterns.md` (hands-on UAT) | Five-specialist review returned PASS; UAT then found 5 P1s | Phase 4 (TUI) and the cutover commit both require manual UAT against the same checklist as v1 Phase 1B before GATE 3 |
| `docs/learnings/critical-patterns.md` (one renderer, one parser) | v1 has two `extractManagedBlock` variants + two YAML serialization call sites | `core/etchosts/marker.go` has one extractor; `core/yaml/` is the sole YAML I/O entry point; all `~/.hosts` reads/writes funnel through `core/fileio/` which calls into `core/yaml/` |
| `docs/learnings/critical-patterns.md` (parameterize malformed-input tests) | v1 has `test.each` with 5 malformed marker-block fixtures | Ported verbatim to `core/etchosts/marker_test.go` as table-driven `t.Run` subtests; expand freely for new cases |
| `docs/learnings/20260525-hostie-phase-1b.md` (sudo reexec broke twice) | First break: `Bun.argv[0]` was virtual `/$bunfs/root/…`; fix used `realpathSync(process.execPath)`. Second break: tests ran from source so the virtual path never surfaced | Go equivalent: `os.Executable()` then `filepath.EvalSymlinks` for re-exec; smoke test exercises the compiled binary's reexec path with a tempdir-as-`/etc/hosts` that returns `EACCES` |
| `docs/learnings/20260525-hostie-phase-1b.md` (MoveToGroupModal Esc flake) | One TUI test was flaky on Esc handling | Modal spike (HIGH-risk above) explicitly validates Esc routing on `ConfirmModal` before the modal pattern fans out; flake taxonomy documented in `tui/components/components_test.go` header |

---

## 8. Open Questions for Validating

> Items the validating skill's plan-checker and spike phase will address. None block bead decomposition.

- [ ] **Score-parity spike (HIGH)** — Define the fixture query corpus for fuzzy-search score parity. Suggested: 50 `~/.hosts` fixtures × 10 queries each (mix of exact, prefix, infix, typo, alias-only, IP-only, group-only). Equality predicate: top-5 result order matches fuse.js exactly. Tolerance for ties: undefined → must be defined during the spike.
- [ ] **YAML round-trip semantics spike (MEDIUM)** — Define the equality predicate: deep struct equality on `HostsFile` after parse→serialize→parse, NOT byte equality. Confirm yaml.v3 quoting decisions on edge cases (numeric-looking hostnames, hostnames with `:`, multi-line comments).
- [ ] **`tea.ExecProcess` smoke spike (HIGH)** — Validate on macOS Terminal, iTerm2, Alacritty, Linux gnome-terminal. Document any terminal-specific quirks (cursor restoration, altscreen exit/re-enter, color reset).
- [ ] **`__apply-privileged` threat model (HIGH)** — Write `docs/go-migration/threat-model.md`. Subcommand contract: (a) accepts only `--payload-path=<file>` and `--owner-uid=<uid>`; (b) rejects if path not under `os.TempDir()`; (c) rejects if `Lstat(path).Sys().Uid != owner-uid`; (d) rejects if mode != `0600`; (e) rejects if path is a symlink; (f) re-parses payload as managed-block bytes and rejects if invalid; (g) unlinks tempfile on every exit path via `defer` + `signal.Notify(SIGINT, SIGTERM)`.
- [ ] **Size measurement (MEDIUM)** — After Phase 1 bootstrap, do a `go build -trimpath -ldflags="-s -w"` on `cmd/hostie` stub with all four Charm deps imported but unused. Compare against the 18 MB ceiling and 10 MB aspirational. Document in `docs/go-migration/size-budget.md`. If first measurement > 15 MB, escalate dep choice (revisit Option C above). → Measured and documented in [size-budget.md](size-budget.md) (Phase 1 bootstrap, run #26390304200; all platforms 2.50–2.57 MB, well under both ceilings).
- [ ] **Golden harness v1-binary pinning (LOW)** — Confirm v1 release tag/SHA to pin (latest v1.x release on `main`). Decide checksum verification (SHA-256 against published `.sha256`).
- [ ] **`staticcheck` policy (LOW)** — Advisory at first, mandatory once Phase 2 settles. Pin staticcheck version in CI to avoid noise from rule changes.
- [ ] **Cobra version pin (LOW)** — Pin `spf13/cobra` to a specific minor version in `go.mod` so completion script outputs stay stable across our own releases.
