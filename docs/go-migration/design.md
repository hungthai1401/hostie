# Go Migration — Design

**Feature slug:** go-migration
**Date:** 2025-11-22
**Brainstorming session:** complete
**Scope:** Deep

---

## Feature Boundary

Rewrite hostie from Bun/TypeScript (Ink + commander + zustand + fuse.js) to a single statically-linked Go binary using the Charm stack (Cobra + Bubble Tea + Bubbles + Lipgloss), to drive the compiled binary down from **61.20 MB** to ≤18 MB (aspirational ≤10 MB) while preserving the `~/.hosts` schema, the `/etc/hosts` marker-block contract, exit-code taxonomy, TUI keybindings, and fuzzy-search weighting. One scoped behavior change ships with the rewrite: every mutating command (CLI and TUI) auto-applies to `/etc/hosts`, escalating via `sudo` when necessary, with `--dry-run` as the only opt-out. Ships as **2.0.0** via GitHub Releases only; npm distribution is dropped.

**Domain type(s):** ORGANIZE + RUN + CALL + SEE

---

## Locked Decisions

### Migration shape
- **D1** Full rewrite — CLI + TUI both in Go, single binary. No partial port, no FFI bridge.
- **D6** Greenfield Go module under `go/`, parallel to existing TS in `src/`. CI runs both lanes until cutover. Cutover = single commit deleting `src/`, `build.ts`, `bun.lock`, `package.json`; move `go/*` to repo root.
- **D7** GitHub Releases only. Matrix: darwin-arm64, darwin-x64, linux-x64, linux-arm64. Drop npm package. Version bumps to **2.0.0**.

### Build & size
- **D2** Build with `go build -trimpath -ldflags="-s -w"`. No UPX. **Hard ceiling: 18 MB** per platform; **aspirational: ≤10 MB**. CI asserts ceiling on every release build.

### Libraries
- **D3** TUI: Bubble Tea + Bubbles + Lipgloss.
- **D4** CLI: Cobra. Replaces commander and the hand-rolled `completion` command (Cobra generates bash/zsh/fish/powershell completions).
- **D8** Standard Go libs elsewhere: `gopkg.in/yaml.v3`, `github.com/oklog/ulid/v2`, `github.com/sahilm/fuzzy` wrapped with a weighted scorer (host×2 / alias×1.5 / IP×1 / group×0.5). State: plain structs + `sync.RWMutex`. Tests: stdlib `testing` + `testify` + `teatest`.

### Parity contract
- **D5** *(amended by D15)* Strict schema, file-format, and exit-code parity with v1.
- **D15** *(amends D5)* Parity surfaces that remain strict: `~/.hosts` YAML schema (existing files round-trip), `list --json` output, `/etc/hosts` marker block format, exit codes `0` ok / `1` validation / `2` I/O / `3` permission, atomic-write semantics. The **only** intentional CLI behavior changes are those introduced by D11–D14 (auto-apply, sudo escalation, apply-failure exit codes on mutating commands, `--dry-run` extended to all mutators). Cross-binary golden harness (D9) is scoped to YAML round-trip + `list --json` + `apply --dry-run` only; it does **not** assert exit codes for mutating commands.
- **D9** Port all 432 TS tests 1:1 to Go. Cross-binary golden harness runs the v1 TS binary and the new Go binary against a fixture corpus and diffs the D15 parity surfaces. CI fails on divergence. Harness lives under `go/test/golden/` and survives cutover (the v1 binary is pinned in CI as a tarball reference).

### Auto-apply on mutation
- **D11** Every mutation auto-applies to `/etc/hosts` after the `~/.hosts` write succeeds.
  - CLI: `add`, `rm`, `enable`, `disable`, `group add`, `group rm`, `group mv`.
  - TUI: toggle (`Space`), delete (`d`), add (`a`), edit (`e`), all group operations.
  - `hostie apply` remains as the manual re-render command.
- **D12** Privilege model: if the running process can write `/etc/hosts` directly (root, or appropriate ACL), do it silently. Otherwise, re-exec the binary under `sudo` for **only** the apply step, via a hidden `__apply-privileged` subcommand. macOS uses `sudo` (no `pkexec`). The `~/.hosts` write never requires sudo.
  - **Payload transport** *(revised after Phase 1 discovery)*: write the rendered payload to a `0600` tempfile in `$TMPDIR` (owned by invoking uid), pass the path as argv to `__apply-privileged`. Child reads, re-validates the bytes as a well-formed hostie-managed hosts block, performs the atomic write to `/etc/hosts`, then unlinks the tempfile (with `defer`-based cleanup on every error path including signals). Original design called for fd-passing over a pipe to keep the payload off disk; research (`docs/go-migration/go-port-feasibility.md` §3) confirmed that requires `sudo -C` + `Defaults closefrom_override` in `/etc/sudoers`, which is not portable on stock macOS or Linux. Tempfile is `0600`-mode, owned by invoking uid (not root), lives for milliseconds — payload never world-readable; argv contains only the path, not contents; tempfile is unlinked even on crash via signal handler.
- **D13** `~/.hosts` write and `/etc/hosts` apply are independent. YAML write is atomic and happens first. If the apply step fails (sudo denied, I/O error, `/etc/hosts` unwritable), keep the YAML change, print the error to stderr, exit `3` (permission) or `2` (I/O) per existing taxonomy. The user can re-run `hostie apply` later to reconcile. In the TUI, surface the error in the status line; never undo the user's edit.
- **D14** Sole opt-out is `--dry-run`, extended from `apply` to every mutating CLI command. With `--dry-run`: print the would-be YAML change and the would-be `/etc/hosts` diff, write **nothing** (no `~/.hosts`, no `/etc/hosts`). Without it: both writes run. No `--no-apply` flag, no env var. TUI has no `--dry-run` analog (interactive mutations always auto-apply).

### Scope discipline
- **D10** *(amended by D11)* Migration only — zero new features apart from the auto-apply behavior covered by D11–D15. Explicitly out: Windows support, mouse, Homebrew/AUR, auto-update, telemetry, schema v2, TUI redesign, npm shim package, plugin system.

### Agent's Discretion
None. Every decision is locked.

---

## Specific Ideas & References

- Target stack chosen for Go binary size: Cobra + Bubble Tea + Bubbles + Lipgloss + yaml.v3 + oklog/ulid/v2 + sahilm/fuzzy. Reference real-world strip-and-trim Go CLIs land in the 8–15 MB range with this stack.
- Search weighting must match v1 exactly: host×2 / alias×1.5 / IP×1 / group×0.5 (documented in README, currently implemented over fuse.js in `src/tui/` search code).
- Atomic-write semantics carry over the v1 critical pattern: `lstat` + same-directory tempfile + perms clamped `& 0o0777 & ~0o022`, then `rename`.

---

## Existing Code Context

Downstream agents: read these files before planning.

### Reusable Assets (logic to port, not reuse)
- `src/domain/types.ts`, `src/domain/validators.ts` (9.2 KB), `src/domain/id.ts` — port to `go/internal/domain/`.
- `src/core/yaml.ts` (4.8 KB) — port to `go/internal/core/yaml/`. Must produce byte-stable output where YAML allows.
- `src/core/render.ts` — port to `go/internal/core/render/`.
- `src/core/etchosts.ts` (6.5 KB) — port to `go/internal/core/etchosts/`. Holds the marker-block contract.
- `src/core/apply.ts` (11 KB) and `src/core/file-io.ts` — port to `go/internal/core/etchosts/atomic.go` + `go/internal/core/fileio/`. Atomic-write critical pattern lives here.
- `src/cli/exit-codes.ts` — port to `go/internal/cli/exitcodes/`.
- `src/cli/index.ts` + `src/cli/__tests__/` — re-express as Cobra commands under `go/internal/cli/commands/`. Tests port 1:1.
- `src/tui/App.tsx` (8.1 KB), `src/tui/store.ts` (8.1 KB zustand), `src/tui/components/`, `src/tui/hooks/` — re-express as Bubble Tea `Model`/`Update`/`View` under `go/internal/tui/`. Zustand store → plain struct + `sync.RWMutex` under `go/internal/tui/store/`.
- `tests/{cli,core,domain,tui}/__tests__/` (432 tests) — port 1:1 to `go/...` test files matching the new package layout.

### Established Patterns (carry forward — from `docs/learnings/critical-patterns.md`)
- **Atomic file replacement**: `lstat` + same-dir tempfile + perms clamped `& 0o0777 & ~0o022`. Re-implement in Go with `os.Lstat`, `os.CreateTemp(filepath.Dir(target), …)`, `f.Chmod(mode & 0o0777 &^ 0o022)`, `os.Rename`. Gate with parameterized test table.
- **Smoke tests hit the compiled binary** — Go integration tests must `go build` then exec the binary, not `go run` from source. The release CI lane runs the same smokes against each cross-compiled artifact.
- **Three-level artifact verification** (exists / substantive / wired) — applies to every ported component.
- **Hands-on UAT non-negotiable** — Phase 4 (TUI) and Phase 5 (cutover) both require manual UAT before GATE 3.
- **One renderer, one parser** — no duplicate YAML or hosts-file logic in the Go tree.
- **Parameterize malformed-input tests** — Go equivalent is table-driven `t.Run` subtests; port `test.each` cases verbatim.

### Integration Points
- `~/.hosts` — same path, same schema. v1 files must round-trip.
- `/etc/hosts` — same marker block format. Auto-apply (new) and explicit `apply` (existing) write through `go/internal/core/etchosts/`.
- Shell completions — Cobra generates them; we snapshot-test the output so the contents of `completions/` (if shipped in release artifacts) stay byte-stable where feasible.
- GitHub Actions release workflow (`.github/workflows/release.yml`) — replace the Bun cross-compile matrix with a Go `GOOS`/`GOARCH` matrix; keep tag-driven release semantics; add size-ceiling assertion step.

---

## Architecture

```
go/
  cmd/hostie/                main.go             // wires Cobra root + TUI launch when no args
  internal/
    domain/                  types.go validators.go id.go
    core/
      yaml/                  serialize.go parse.go
      render/                render.go
      etchosts/              marker.go atomic.go
      fileio/                readwrite.go
    apply/
      runner.go              // orchestrates YAML-write → render → /etc/hosts write
      privilege.go           // can-write probe; sudo re-invocation (subcommand "__apply-privileged")
      dryrun.go              // pure-compute path: build diff without touching disk
    cli/
      commands/              add rm enable disable list apply group completion version  (each gains --dry-run where mutating)
      exitcodes/             codes.go            // 0 ok / 1 validation / 2 I/O / 3 permission
    tui/
      app/                   model.go update.go view.go
      components/            sidebar mainpane editmodal confirmmodal helpmodal search
      store/                 state.go            // plain struct + sync.RWMutex; mutations dispatch apply.Runner asynchronously off the UI goroutine
      search/                weighted.go         // sahilm/fuzzy + host×2/alias×1.5/IP×1/group×0.5
  test/
    golden/                  fixtures/*.yaml harness_test.go   // runs v1 TS + v2 Go, diffs parity surfaces
go.mod
go.sum
```

### Privilege flow (auto-apply)

1. Command parses, validates input. Exit `1` on validation failure (no writes).
2. Atomic write to `~/.hosts` (lstat + same-dir tempfile + clamped perms).
3. Render new `/etc/hosts` content in memory.
4. Probe: can current process write `/etc/hosts`? (`os.OpenFile` for write + close, or `unix.Access`.) If yes → atomic write to `/etc/hosts`. Done. Exit `0`.
5. If no → re-exec `sudo $0 __apply-privileged --payload-path=<file>`; the hidden `__apply-privileged` subcommand opens the path (`0600` tempfile in `$TMPDIR`, owned by invoking uid), reads the rendered bytes, re-validates them as a well-formed hostie-managed hosts block, and performs the same atomic write. Tempfile is unlinked on exit (success and every failure path, including signal handlers).
6. On success: exit `0`. On failure at steps 4–5: exit `2` (I/O) or `3` (permission). YAML change from step 2 is retained.
7. TUI: same flow, but step 5 requires releasing the terminal. Use Bubble Tea's `tea.ExecProcess`/`Pause` to hand the TTY to `sudo`, restore the program on return. Status line shows pending → success/error. Errors do not undo the user's edit.

### Build & release

- Per platform: `CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o dist/hostie-$os-$arch ./cmd/hostie`
- CI asserts each artifact ≤ 18 MB; warns at > 10 MB.
- Release workflow tags `v2.0.0` and uploads four artifacts plus generated shell completions plus checksums.

### Migration plan (high-level — full bead decomposition is writing-plans' job)

1. **Bootstrap** — `go/` module, CI lane (`go build`, `go test`, `go vet`, `staticcheck`), size-ceiling job.
2. **Core port** — domain, yaml, render, etchosts atomic write, fileio. Port matching TS tests; stand up cross-binary golden harness against v1.
3. **CLI port** — Cobra root + all subcommands incl. completion. Exit-code parity tests. **`apply.Runner` lands here**, including dry-run plumbing on all mutators.
4. **TUI port** — Bubble Tea model, sidebar/mainpane, all modals, search, store. Teatest coverage matching the v1 Ink testing-library coverage. **Sudo-handoff spike runs here as a HIGH-risk item.**
5. **Cutover** — when golden harness + ported tests are green on every platform: single commit deleting `src/`, `build.ts`, `bun.lock`, `package.json`; move `go/*` to repo root; update README/AGENTS/CLAUDE; tag **2.0.0** with release notes covering the npm removal and the auto-apply behavior change.

### Risks

- **Search weighting parity** — fuse.js and sahilm/fuzzy score differently. Golden test asserts exact top-5 order on a fixed query corpus.
- **YAML round-trip drift** — yaml.v3 may differ from `yaml` (npm) on quoting. Acceptable if parse→serialize→parse is a fixed point; harness asserts this on the fixture corpus.
- **TUI focus/modal porting** — Ink/React's declarative composition doesn't map 1:1 to Bubble Tea's Elm loop. Longest phase. Teatest is the main safety net.
- **Re-exec-under-sudo + TUI terminal handoff** — Bubble Tea + `sudo` over one TTY is the trickiest piece. **Spike early as HIGH-risk in writing-plans.** Validate on macOS Terminal, iTerm2, Alacritty, and Linux gnome-terminal.
- **Self-re-exec safety** — privileged subcommand must reject any input that isn't a valid rendered hosts block. Threat model documented in writing-plans. Payload only via inherited fd; never argv, never tempfile.
- **Double sudo prompts in scripts** — back-to-back `hostie add` calls each prompt unless sudo's timestamp cache is warm. Documented behavior; acceptable.
- **`apply --dry-run` vs new mutator `--dry-run`** — the diff produced by `hostie add foo --dry-run` must match what `hostie apply --dry-run` would produce immediately after `hostie add foo` (without dry-run). Asserted by the golden harness.
- **Privileged-write learnings carry-over** — re-implement lstat + same-dir tempfile + clamp in Go; parameterized test table per critical-patterns.md.

---

## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `docs/learnings/critical-patterns.md` — atomic file replacement; smoke tests on compiled binary; three-level artifact verification; hands-on UAT; one renderer one parser; parameterize malformed-input tests.
- `README.md` — CLI surface, TUI keybindings, exit codes, YAML schema, marker block (the parity contract).
- `src/core/etchosts.ts` — current marker block implementation; the format new Go code must produce byte-for-byte.
- `src/core/yaml.ts` — current YAML serializer; round-trip target.

---

## Outstanding Questions

### Resolve Before Planning
None.

### Deferred to Planning
- [ ] Exact mechanism for Bubble Tea ↔ sudo TTY handoff (`tea.ExecProcess` vs manual `Pause`/`Resume`) — spike during Phase 4.
- [ ] Whether the v1 TS binary used by the golden harness is built in-CI or stored as a release-tagged tarball pinned by SHA — investigate during Phase 1.
- [ ] Whether `staticcheck` runs as a CI gate or advisory — decide during Phase 1 bootstrap.
- [ ] Snapshot format for Cobra-generated shell completions (golden file vs runtime regeneration check) — decide during Phase 3.

---

## Deferred Ideas

- Windows support — out of scope; would need separate /etc/hosts equivalent + privilege model.
- Mouse support in TUI — out of scope.
- Homebrew / AUR packaging — out of scope; GitHub Releases only for 2.0.0.
- Auto-update — out of scope.
- Telemetry — out of scope.
- Schema v2 — out of scope; v1 schema is the contract.
- TUI visual redesign — out of scope; keybindings and layout preserved.
- npm shim package wrapping the Go binary — out of scope; npm distribution is dropped entirely.
- Plugin system — out of scope.
- `--no-apply` flag / `HOSTIE_NO_APPLY` env var — explicitly rejected (D14). Only `--dry-run` opts out.
- `HOSTIE_LEGACY=1` v1-compat mode — explicitly rejected (D15). 2.0.0 is a clean break for the auto-apply behavior.

---

## Handoff Note

design.md is the single source of truth for this feature.

- **writing-plans** reads: locked decisions, code context, canonical refs, deferred-to-planning questions
- **validating** reads: locked decisions (to verify plan-checker coverage)
- **executing-plans** reads: locked decisions (to honor during implementation)
- **reviewing** reads: locked decisions (for UAT verification)

Decision IDs (D1–D15) are stable. Reference them by ID in all downstream artifacts.
