# Phase 4 Contract: Go Migration — TUI Port

## Entry State

- Phase 3 exit state holds: all 9 CLI commands ported under `go/internal/cmd/` (`add`, `rm`, `enable`, `disable`, `list`, `apply`, `group`, `completion`, plus hidden `__apply-privileged`); `apply.Runner` orchestrates write → render → /etc/hosts replace; auto-apply wired through every mutator; `--dry-run` on every mutator; 286 tests across 8 Go packages green; golden harness covers `yaml-roundtrip`, `list --json`, `apply --dry-run`; threat model published at `docs/go-migration/threat-model-apply-privileged.md`.
- Binary size measured at 4.48 MB on darwin-arm64 — 13.52 MB below the 18 MB hard ceiling, 5.52 MB below the 10 MB aspirational target.
- v1 source tree (`src/`, `tests/`, `build.ts`, `package.json`, `bun.lock`, `tsconfig.json`) **still untouched** since Phase 1 baseline (`git diff main..feature/go-migration -- src/ tests/` empty per D6).
- Branch `feature/go-migration` active; tree clean.
- Charm stack deps already in `go.mod` from Phase 1: `bubbletea v1.3.6`, `bubbles`, `lipgloss`, `sahilm/fuzzy` — none yet imported from any non-test Go file. Transitive `github.com/atotto/clipboard` audit recorded as TUI-gated only (not reachable from `core/`).
- v1 TUI source under `src/tui/` is the parity reference: App.tsx (267L), store.ts (352L zustand), hooks/useKeyboard.ts (349L), hooks/useSearch.ts (~80L), 9 components (Layout, GroupTree, EntryList, StatusBar, ConfirmModal, HelpModal, EntryEditorModal, GroupCreatorModal, MoveToGroupModal). 80 v1 TUI test cases across 13 files (1,409 LOC) form the parity baseline for ported test coverage.
- `apply.Runner` signature locked: `Apply(hostsPath string, dryRun bool) (*ApplyResult, error)` and `ApplyFromFile(path string) (*ApplyResult, error)`. TUI consumes these unchanged — Phase 4 does not modify `go/internal/apply/`.
- `go` toolchain ≥ 1.23.0 (bubbletea v1.3.6 transitive minimum) installed.
- Fuse.js score fixture corpus to be defined by Story 1 spike — no pre-existing corpus carried in.

---

## Exit State

### TUI packages (all under `go/internal/tui/`)

1. **`store/state.go`** ports the v1 zustand store to a plain Go struct guarded by `sync.RWMutex` (D8):
   - Fields equivalent to v1: `HostsFile *domain.HostsFile`, `SelectedEntryID string`, `SelectedGroupPath []string`, `SearchQuery string`, `Mode StoreMode` (`Normal|Search|Edit|Modal`), `Dirty bool`, `Modal *ModalState`, `StatusMessage *StatusMessage`.
   - Mutating methods equivalent to v1 actions: `LoadHostsFile`, `SelectEntry`, `SelectGroup`, `SetSearchQuery`, `SetMode`, `MarkDirty/ClearDirty`, `AddEntry`, `UpdateEntry`, `DeleteEntry`, `ToggleEntry`, `MoveEntry`, `AddGroup`, `OpenModal/CloseModal`, `SetStatusMessage/ClearStatusMessage`.
   - Helpers ported verbatim: `findAndUpdateEntry`, `findGroupByPath`, `addEntryToGroup`, `addGroupToPath` — recursive group walk semantics match v1.
   - **≥ 17 table-driven test cases ported** from v1 `store.test.ts` / `store.integration.test.ts` (covers add/update/delete/toggle/move across nested groups, mode transitions, modal open/close stack).

2. **`search/weighted.go`** ports v1 fuzzy search with the locked D8 weight table:
   - Uses `github.com/sahilm/fuzzy` for per-field match scoring.
   - Per-entry aggregate = `host×2 + alias×1.5 + IP×1 + group×0.5` (D8 weights, locked).
   - Returns results sorted by aggregate score descending; tie-break order documented in source comment and exercised by score-parity tests.
   - **Score-parity spike resolved** against a fuse.js fixture corpus (≥10 queries × ≥10 fixtures): top-5 result order byte-equal to v1 fuse.js output on every (fixture, query) pair. Tie-break rule defined and tested.
   - **≥ 11 ported test cases** (mirrors v1 `useSearch.test.ts` surface).

3. **`components/` ports the 9 v1 components to Bubble Tea / Lipgloss**:
   - `layout.go` — top-level Lipgloss frame: sidebar / main / statusbar columns; widths derive from `tea.WindowSizeMsg`.
   - `grouptree.go` — recursive tree renderer over `domain.HostsFile.Groups`; honors `SelectedGroupPath`; indentation, expansion glyphs, highlight style all match v1 sidebar.
   - `entrylist.go` — table renderer over the active group's entries; honors `SelectedEntryID`; per-row styling (enabled/disabled/selected) matches v1.
   - `statusbar.go` — bottom row: dirty marker, current mode, search query, transient status messages (`StatusMessage` with TTL); style + position match v1.
   - `confirm_modal.go` — modal pattern reference implementation; routes `y`/`n`/`Enter`/`Esc`; returns a result via `tea.Cmd`.
   - `help_modal.go` — full keybind reference matching v1; closes on `?` or `Esc`.
   - `entry_editor_modal.go` — form with field navigation (Tab cycling), inline validation against `domain.Validate*`; covers both add and edit modes.
   - `group_creator_modal.go` — single text-input form; validates non-empty + uniqueness; submits via `tea.Cmd`.
   - `move_to_group_modal.go` — list of available groups for the selected entry; routes Esc deterministically (previously-flaky v1 test passes on the first run, every run).
   - Each component has its own teatest snapshot/behavior tests; combined coverage ≥ the 7 v1 component-test files (1,028 LOC).

4. **`app/model.go` + `app/update.go` + `app/view.go`** wire the Bubble Tea root program:
   - `Model` composes `*store.State` + `*search.Engine` + a `Layout` view; tracks window size, focused pane, modal stack.
   - `Update(tea.Msg) (tea.Model, tea.Cmd)` handles:
     - `tea.WindowSizeMsg` — propagates to Layout.
     - `tea.KeyMsg` — dispatches via the keybinding table (search-mode chars / Esc / Enter / Backspace; `/` enter search; `Tab` focus swap sidebar↔main; `j`/`k` navigation with wrap; `Space` toggle; `d` delete (with confirm); `Enter` or `Ctrl+S` apply; `?` help; `q` quit (confirm if dirty); `a` entry-creator; `e` entry-editor; `g` group-creator; `m` move-to-group).
     - Modal result messages — close modal, mutate store, dispatch follow-up `tea.Cmd`.
     - `ApplyResultMsg` — update status bar with success or failure detail.
   - `View() string` composes Lipgloss output: Layout below, modal overlay on top when `ModalState != nil`.
   - **No business logic in `View()`**; all mutations go through `Update`.

5. **TUI mutations auto-apply via `apply.Runner` dispatched as `tea.Cmd`** (D11):
   - Every mutation path that closes a modal or toggles state writes `~/.hosts` and then invokes `apply.Runner.Apply(...)` on a goroutine, returning an `ApplyResultMsg` to the Bubble Tea event loop.
   - On apply failure: YAML write is **kept** (D13); status bar reflects the failure; no rollback. Error text matches the v1 surface where v1 had equivalent error text.
   - **There is no TUI `--dry-run`** (D14): TUI mutations are always real. `--dry-run` remains the CLI-only opt-out.

6. **Sudo TTY handoff via `tea.ExecProcess`** for the `/etc/hosts` write step when `CanWriteEtcHosts()` returns false:
   - `tea.ExecProcess(cmd, callback)` releases the TTY, runs `sudo <hostie> __apply-privileged --payload-path=… --owner-uid=…`, re-acquires the TTY on return.
   - **Smoked manually on four terminals** before exit: macOS Terminal, iTerm2, Alacritty, Linux gnome-terminal. Quirks documented inline in `app/update.go` and (if material) in a short note in `docs/go-migration/threat-model-apply-privileged.md`.
   - Tempfile lifecycle inherits the Phase 3 contract (0600, $TMPDIR, owner-uid-validated, unlinked on every exit path including SIGINT/SIGTERM).

### Wiring + entry points

7. **`cmd/hostie/main.go` (or `cmd/root.go`) launches the TUI when invoked with no subcommand** — matching v1 behavior. Existing subcommands continue to work; `hostie` (no args) drops into the TUI program. Resolves the "No CLI surface yet" wiring in Phase 2/3.

8. **Search mode integrates end-to-end:**
   - Pressing `/` switches `store.Mode` to `Search` and focuses the status-bar input.
   - Keystrokes update `SearchQuery`; results from `search.Engine` filter the entry list in the main pane in real time.
   - `Esc` exits search mode and restores the previous selection; `Enter` accepts the top result and exits search mode.

### Test parity

9. **v1 TUI test surface is ported**:
   - 80 v1 `it()`/`test()` cases across 13 v1 test files form the baseline.
   - Ported coverage targets: ≥17 store cases (Story 1), ≥11 search cases (Story 1), ≥7 component test files (Stories 2 + 5), and end-to-end teatest coverage for keybindings + modal flow in `app/app_test.go`.
   - **Previously-flaky `MoveToGroupModal` Esc test passes deterministically** (run ≥100× without flake recorded in CI; see Pivot Signal below if it still flakes).
   - Total Phase 4 Go test count ≥ 80 (parity floor) and the regression Phase-3 test count of 286 does not decrease.

### Hands-on UAT (HARD-GATE before exit per D5/D15)

10. **Manual UAT against a v1 keybinding checklist** completes successfully on macOS before this phase exits:
    - Operator boots `go run ./cmd/hostie` (and once on the compiled `bin/hostie`).
    - Walks the full v1 keybind table: `/`, `Tab`, `j`/`k`, `Space`, `d`, `Enter`, `Ctrl+S`, `?`, `q`, `a`, `e`, `g`, `m`, `Esc`.
    - Exercises a sudo-required `apply` mutation end-to-end (toggle entry → modal confirm → sudo prompt → return to TUI → status bar updates).
    - UAT outcome appended to a fresh `docs/go-migration/phase-4-uat-log.md` (PASS / OBSERVED ISSUES / decisions taken).

### HIGH-risk spikes resolved (carry-in from validating)

11. The three Phase 4 HIGH-risk items from approach.md §4 are resolved with concrete artifacts before the corresponding bead closes:
    - **Modal pattern (`tui/components/confirm_modal.go`)**: spike under `.spikes/go-migration/p4-modal-pattern/` demonstrates a complete open → route keys → close → return result round-trip via teatest; FINDINGS.md commits the pattern that the remaining 4 modals will follow.
    - **Weighted search score parity (`tui/search/weighted.go`)**: spike under `.spikes/go-migration/p4-search-parity/` defines the fixture × query corpus, runs both v1 fuse.js and v2 implementations, asserts top-5 byte-equal; FINDINGS.md records the tie-break rule.
    - **Sudo TTY handoff (`tui/app/update.go` ExecProcess path)**: spike under `.spikes/go-migration/p4-sudo-handoff/` smoke-tested on the four terminals; FINDINGS.md records terminal-specific quirks and the agreed mitigation per quirk.

### Cross-cutting invariants

12. `go build ./...` + `go test ./...` + `go vet ./...` all green on the matrix `[ubuntu-latest, macos-latest]`.
13. `staticcheck` zero findings against `go/internal/tui/...`.
14. Binary size measured again after Phase 4: still ≤ 18 MB (no regression flag fires). Recorded in `docs/go-migration/size-budget.md` as a Phase 4 delta from the Phase 3 baseline (4.48 MB).
15. v1 source tree still untouched (`git diff main..feature/go-migration -- src/ tests/` still empty per D6).
16. All Phase 4 beads closed via `br close`; the migration epic `hosts-cli-go-migration-epic-l54` remains open until Phase 5 cutover.

---

## Demo Story

A developer pulls `feature/go-migration` after Phase 4 lands. They run `go build -o bin/hostie ./cmd/hostie && ./bin/hostie` — no subcommand, no flags. The terminal switches to altscreen; the v1 3-pane Layout appears: a Sidebar listing groups on the left, an EntryList showing the current group's entries in the middle, a StatusBar at the bottom showing mode `NORMAL`. They press `?` — the HelpModal overlays the screen with the full keybind reference; `Esc` dismisses it. They press `j` three times — the EntryList selection moves down with wrap. They press `Tab` — focus jumps to the Sidebar; `j`/`k` navigates groups; `Tab` again returns focus to the EntryList. They press `/` — the StatusBar input activates; they type `staging` and watch the EntryList filter live as `search.Engine` ranks matches by `host×2 + alias×1.5 + IP×1 + group×0.5`. `Esc` exits search mode and restores selection. They press `d` on a selected entry — a ConfirmModal asks "Delete entry foo.example.com?"; they press `y`. The TUI writes `~/.hosts`, dispatches `apply.Runner.Apply` as a `tea.Cmd`, surfaces a sudo prompt via `tea.ExecProcess` (TTY hands off cleanly, then re-acquires), and the StatusBar reflects `applied · 1 entry removed`. They open a second terminal and `cat ~/.hosts` — the entry is gone; `cat /etc/hosts` inside the managed block — also gone. They press `q`; the program quits cleanly back to the shell. Operator runs `cd go && go test ./internal/tui/... -count=1` — all store, search, component, and `app` teatest cases pass; the previously-flaky `MoveToGroupModal_Esc` case is among them and has zero recorded flakes. Size-check workflow on the latest commit: binary 5.2 MB — under both ceilings. The TUI is now the default invocation path; v1 source tree is still untouched.

---

## Unlocks

- **Phase 5 (Cutover) can begin.** Cutover deletes `src/` and the Bun toolchain; this requires that the Go binary fully replaces the v1 experience including the TUI. Without Phase 4, cutover would ship a regression (no TUI) on `main`.
- **The Go binary becomes the user's default surface.** Prior phases only added subcommands invoked with arguments; Phase 4 makes `hostie` (no args) functional, matching v1.
- **All four HIGH-risk items from approach.md §4 are retired** (modal pattern, search score parity, sudo TTY handoff in TUI context — the fourth, atomic write, was retired in Phase 2; sudo reexec in CLI context was retired in Phase 3). Phase 5 inherits zero open HIGH risks.
- **The migration parity contract (D5/D15) is end-to-end testable** for the first time: golden harness already covers `yaml-roundtrip`, `list --json`, `apply --dry-run` (Phases 2/3); UAT now covers TUI keybinding parity (Phase 4). All parity surfaces have a green test or signed UAT log.

---

## Pivot Signals

- **`tea.ExecProcess` cannot release/re-acquire the TTY cleanly on one or more of the four target terminals** — escalate before closing the sudo-handoff bead. The TUI mutation flow depends on this; if the TTY is permanently lost on, say, iTerm2, mutations would have to either disable themselves in TUI mode (violating D11 auto-apply) or fall back to a non-TTY sudo path (changes the threat model). Investigate `tea.ClearScreen` + `tea.ExitAltScreen` sequencing; do not paper over with a hard refresh.
- **Score-parity spike cannot reach top-5 byte-equal against fuse.js on the fixture corpus** — escalate. D8 locks the weight table to match v1 perceived ranking; if `sahilm/fuzzy` aggregate scoring genuinely cannot reproduce fuse.js order even after tie-break tuning, the design.md must be amended (revisit alternate fuzzy libraries: `lithammer/fuzzysearch`, `junegunn/fzf` algorithm port) before continuing.
- **Modal pattern spike on `ConfirmModal` reveals that Bubble Tea modal routing requires substantially more boilerplate than the v1 Ink declarative model** (e.g., per-modal `tea.Cmd` plumbing, focus-stack management that Ink handles implicitly) — escalate. The 4 remaining modals (Help, EntryEditor, GroupCreator, MoveToGroup) inherit the pattern; if it costs 2× the per-modal LOC of v1, the phase scope balloons and either component count or test coverage will slip. Decision: keep pattern (and accept the cost), introduce a thin `modal.Routable` interface to share the boilerplate, or fall back to a non-modal flow for low-value modals (Help could become a separate pane).
- **`MoveToGroupModal` Esc flake reappears in Go port despite a deterministic Bubble Tea event loop** — escalate. The v1 flake was attributed to Ink scheduler races; if it survives the port, the root cause is in the modal-routing logic itself, not the framework. Block on root-cause before exit.
- **Phase 4 binary size > 10 MB aspirational ceiling** (still under 18 MB hard) — record-as-info, no escalation. Charm stack import surface is large; some growth from 4.48 MB Phase 3 baseline is expected.
- **Phase 4 binary size > 18 MB hard ceiling** (very unlikely given 13.52 MB headroom) — escalate per D2. Audit Charm dep transitive growth (`go mod why -m <package>`); consider stripping unused `bubbles` subpackages.
- **Hands-on UAT discovers a P0 or P1 keybinding/visual divergence from v1 that cannot be fixed inside the current bead set** (e.g., terminal rendering glitch on a specific platform, modal focus order that v1 implicitly defined and v2 mis-orders) — escalate. UAT is a HARD-GATE per critical-patterns.md; if it fails, the phase does not exit. Add a remediation bead and re-run UAT.
- **`atotto/clipboard` becomes reachable from TUI code paths in unexpected ways** (e.g., a `bubbles/textinput` action that auto-copies on yank) — surface during the Story 4 modal spike. If clipboard exec is reachable, document and decide whether to disable the feature (suppress via styling), accept the threat-model exposure (cleanup CLI path is still gated), or vendor a stripped `textinput` fork.
