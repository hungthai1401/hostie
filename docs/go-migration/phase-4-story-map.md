# Story Map: Go Migration — Phase 4 (TUI Port)

## Plan

Migrate hostie from Bun/TypeScript to Go to drive the compiled binary from 61.20 MB down to ≤18 MB, while preserving every parity surface from v1 and adding auto-apply on mutation.

---

## Phase: TUI port

Port the v1 Ink/zustand TUI to Bubble Tea / Bubbles / Lipgloss under `go/internal/tui/`, wire TUI mutations through `apply.Runner` with sudo TTY handoff via `tea.ExecProcess`, ship the v1 keybinding surface and modal flow at parity (D5/D15), and gate exit on hands-on UAT — so Phase 5 (Cutover) can delete `src/` without regressing the user's default interaction surface.

---

### Story 1: Store + search foundation — pure data + algorithm, teatest-independent

**Purpose:** A Phase 4 worker (or future maintainer) can `import "github.com/hungthai1401/hostie/go/internal/tui/store"` and `.../tui/search"` and get the same state shape, the same mutation semantics, and the same weighted ranking that v1's zustand store + fuse.js delivered — with the score-parity spike resolved before any TUI rendering code lands.

**Why Now:** Store and search have zero dependencies on Bubble Tea, Lipgloss, or any TUI component. Every other Phase 4 story imports them: Layout/EntryList/GroupTree render `store.State`; the root `Model` composes `*store.State` + `*search.Engine`; mutation handlers in Story 6 mutate the store and re-query search. Without these two packages, nothing else compiles and the score-parity spike cannot be retired early.

**Contributes To:** Exit-state clauses 1 (store), 2 (search), 11 (HIGH-risk search score-parity spike).

**Creates:**
- `go/internal/tui/store/state.go` — `State` struct + `sync.RWMutex` + mutation methods equivalent to v1 zustand actions
- `go/internal/tui/store/state_test.go` — ≥17 table-driven cases (port of `store.test.ts` / `store.integration.test.ts`)
- `go/internal/tui/search/weighted.go` — sahilm/fuzzy + weighted aggregator (host×2/alias×1.5/IP×1/group×0.5 per D8)
- `go/internal/tui/search/weighted_test.go` — ≥11 unit cases + score-parity fixture corpus
- `.spikes/go-migration/p4-search-parity/` — fixture corpus, v1 fuse.js capture, FINDINGS.md with tie-break rule

**Unlocks:** Stories 2 (layout components consume store), 3 (root Model composes both), 6 (mutations write through store), 8 (search mode in app/update.go calls into search.Engine).

**Done Looks Like:** `cd go && go test ./internal/tui/store/... ./internal/tui/search/... -count=1 -race -v` runs ≥28 named subtests, all green; score-parity test compares ≥10 queries × ≥10 fixtures top-5 results against captured fuse.js JSON, byte-equal on every pair; `staticcheck ./internal/tui/store/... ./internal/tui/search/...` zero findings; `.spikes/go-migration/p4-search-parity/FINDINGS.md` committed with tie-break rule.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-store-state-2c7` | Port tui/store/state.go (plain struct + sync.RWMutex, all v1 zustand actions) | Mutation semantics match v1; recursive group helpers (`findAndUpdateEntry`, `findGroupByPath`, `addEntryToGroup`, `addGroupToPath`) ported verbatim; ≥17 table-driven cases |
| `hosts-cli-go-mig-p4-search-spike-khb` | Score-parity spike: sahilm/fuzzy aggregator vs fuse.js top-5 | HIGH-risk; capture v1 fuse.js output on fixture corpus first, then iterate aggregator + tie-break until top-5 byte-equal; commit FINDINGS.md |
| `hosts-cli-go-mig-p4-search-weighted-r1b` | Port tui/search/weighted.go (Engine with weighted aggregator + tie-break) | Implements decision from spike bead; ≥11 unit cases mirroring v1 useSearch.test.ts; embeds Critical Pattern "Parameterize Malformed-Input Tests" for empty-query / no-match / unicode cases |

---

### Story 2: Layout + non-modal components — render-only, no event loop

**Purpose:** A worker can `lipgloss.Render(Layout(...).View())` against a static `store.State` snapshot and see the 3-pane v1 frame (Sidebar / Main / StatusBar) with the GroupTree rendered into Sidebar and the EntryList rendered into Main — with no Bubble Tea program running. Pure render functions, easy to snapshot-test, decoupled from `Update`.

**Why Now:** Story 3 (root Model skeleton) composes Layout to render its `View()`. Layout needs GroupTree, EntryList, StatusBar already implemented to render anything beyond empty frames. None of these four components route key events, so they're independent of Story 4's modal spike. They depend only on `store.State` from Story 1.

**Contributes To:** Exit-state clause 3 (4 of the 9 components: Layout, GroupTree, EntryList, StatusBar).

**Creates:**
- `go/internal/tui/components/layout.go` — Sidebar/Main/StatusBar Lipgloss frame; widths from `tea.WindowSizeMsg`
- `go/internal/tui/components/grouptree.go` — recursive tree renderer over `domain.HostsFile.Groups`
- `go/internal/tui/components/entrylist.go` — table renderer over active group's entries
- `go/internal/tui/components/statusbar.go` — bottom row: dirty marker, mode, search query, status message
- `go/internal/tui/components/components_layout_test.go` — teatest snapshots for each (4 component test files at minimum)

**Unlocks:** Story 3 (root Model `View()` composes Layout); Story 8 (search mode renders into StatusBar input + filters EntryList).

**Done Looks Like:** `go test ./internal/tui/components/... -run 'Layout|GroupTree|EntryList|StatusBar' -count=1 -v` green; teatest golden snapshots match committed `.golden` files for fixtures covering empty state, single group, nested groups, selected entry, search-active state; render-only — no `tea.Cmd` paths exercised.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-comp-layout-3k0` | Implement Layout component (3-pane Lipgloss frame, WindowSizeMsg-driven widths) | Mirrors v1 Layout.tsx; widths/styling match v1 visual output; teatest snapshot |
| `hosts-cli-go-mig-p4-comp-grouptree-tag` | Implement GroupTree (recursive tree renderer, selection highlight) | Mirrors v1 GroupTree.tsx; indentation + expand glyph match v1; teatest snapshot covers nested groups |
| `hosts-cli-go-mig-p4-comp-entrylist-yl8` | Implement EntryList (table renderer, enabled/disabled/selected styling) | Mirrors v1 EntryList.tsx; per-row style table matches v1; teatest snapshot |
| `hosts-cli-go-mig-p4-comp-statusbar-unj` | Implement StatusBar (dirty/mode/query/status with TTL) | Mirrors v1 StatusBar.tsx; StatusMessage TTL implemented; teatest snapshot covers all 4 zones |

---

### Story 3: Root Model / Update / View skeleton — boots, renders, handles navigation

**Purpose:** A worker can run `go run ./cmd/hostie` (no subcommand) and see the v1 3-pane Layout, navigate the EntryList with `j`/`k` (with wrap), swap focus between Sidebar and Main with `Tab`, and quit with `q` — with **no mutations, no modals, no search, no apply** yet. This is the smallest boot-to-render-to-quit cycle.

**Why Now:** Story 4 (modal spike) needs a root program to dispatch the modal Cmd into. Stories 5–8 all extend `Update` and `View`; without a skeleton, every later story would need to bootstrap the program itself. Wiring `cmd/hostie` to drop into the TUI on no-args also resolves the long-running "No CLI surface yet (TUI)" debt from Phase 2/3.

**Contributes To:** Exit-state clauses 4 (app/model.go + app/update.go + app/view.go), 7 (cmd/hostie launches TUI on no subcommand).

**Creates:**
- `go/internal/tui/app/model.go` — `Model` struct composing `*store.State`, focus-pane enum, window size
- `go/internal/tui/app/update.go` — `tea.WindowSizeMsg`, `tea.KeyMsg` for `j`/`k`/`Tab`/`q` only
- `go/internal/tui/app/view.go` — composes Layout from Story 2
- `go/internal/tui/app/app_test.go` — teatest end-to-end: boot, send j×3, send Tab, send q, assert final view
- `go/cmd/hostie/main.go` (modify) or `go/internal/cmd/root.go` (modify) — invoke `app.Run()` when called with no subcommand

**Unlocks:** Story 4 (modal spike dispatches via this Update loop); Story 6 (mutation handlers extend this Update); Story 8 (search mode extends this Update).

**Done Looks Like:** `./bin/hostie` (built from source after this story) boots into the 3-pane TUI on an empty `~/.hosts` and a populated one; `j`/`k`/`Tab`/`q` work as specified; `go test ./internal/tui/app/... -run TestSkeletonNavigation -count=1` green; existing CLI commands (`add`, `rm`, etc.) continue to work unchanged.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-app-skeleton-kgg` | Implement Bubble Tea Model/Update/View skeleton (navigation only) | j/k with wrap, Tab focus swap, q quit; no modals/mutations/search; teatest end-to-end |
| `hosts-cli-go-mig-p4-app-wire-i6k` | Wire cmd/hostie to launch TUI on no subcommand | Modifies root.go; verifies all existing subcommands still work; integration test covers `hostie` (no args) → TUI boot |

---

### Story 4: Modal pattern spike — ConfirmModal end-to-end as the reference implementation

**Purpose:** Prove the Bubble Tea modal pattern (open via key → route Esc/Enter/y/n → close → return result via `tea.Cmd`) on ONE modal end-to-end before the remaining 4 modals fan out. Establishes the boilerplate that Stories 5 and 6 will both inherit; catches the "Ink-declarative → Elm-loop is non-trivial" HIGH risk before it compounds across 5 modals.

**Why Now:** Listed as HIGH-risk #1 in approach.md §4. Story 5 (remaining modals) MUST inherit the spike's pattern; doing 5 modals in parallel before the pattern is proven multiplies blast radius by 5×. The spike is the cheapest way to learn whether a thin `modal.Routable` interface helps, whether per-modal `tea.Cmd` plumbing is acceptable, and whether focus-stack management needs centralization. v1's known `MoveToGroupModal` Esc flake is the canary — if the pattern handles Esc deterministically on `ConfirmModal`, the pattern is sound.

**Contributes To:** Exit-state clauses 3 (1 of 9 components: ConfirmModal), 11 (HIGH-risk modal pattern spike).

**Creates:**
- `go/internal/tui/components/confirm_modal.go` — modal with title, message, `y`/`n`/`Enter`/`Esc` routing
- `go/internal/tui/components/confirm_modal_test.go` — teatest covering open → key route → close → result Cmd; ≥1 case explicitly exercises Esc determinism (≥100 runs in a `for` loop or `-count=100`)
- `go/internal/tui/app/update.go` (extend) — modal-stack handling, ModalResultMsg routing
- `go/internal/tui/app/view.go` (extend) — overlay rendering when modal is open
- `.spikes/go-migration/p4-modal-pattern/FINDINGS.md` — decision: keep per-modal Cmd plumbing as-is, OR introduce `modal.Routable` interface, OR fall back to non-modal flow for low-value cases

**Unlocks:** Story 5 (the 4 remaining modals follow this pattern); Story 6 (delete/quit mutations route through ConfirmModal first).

**Done Looks Like:** `go test ./internal/tui/components/... -run TestConfirmModal -count=100` green (zero flakes recorded); teatest verifies Esc closes modal with `Cancelled` result, `y`/`Enter` closes with `Confirmed`, `n` closes with `Cancelled`; FINDINGS.md committed with the chosen pattern documented; modal overlay renders above Layout in `View()`.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-modal-spike-xh5` | Modal pattern spike: ConfirmModal end-to-end with teatest determinism harness | HIGH-risk; ≥100-run Esc determinism test; FINDINGS.md commits the chosen pattern (vanilla / Routable interface / non-modal fallback) |

---

### Story 5: Remaining modals (Help, EntryEditor, GroupCreator, MoveToGroup) — fan out on the proven pattern

**Purpose:** Port the 4 remaining v1 modals using the pattern Story 4 ratified. Each modal is independent of the others (no inter-modal data flow) and reuses the modal-stack infrastructure already in `update.go`/`view.go`. The previously-flaky `MoveToGroupModal` Esc test passes deterministically — the v1 flake does not survive the port.

**Why Now:** Story 6 (mutations) needs every modal already implemented to wire keys `a`/`e`/`g`/`m`/`?` to their respective modals. Story 4 must complete first (pattern); Stories 1 + 2 + 3 must complete first (store, validation, root program). Within Story 5 the 4 modals can land in parallel.

**Contributes To:** Exit-state clause 3 (4 of 9 components: HelpModal, EntryEditorModal, GroupCreatorModal, MoveToGroupModal), part of clause 9 (test parity, including the previously-flaky case).

**Creates:**
- `go/internal/tui/components/help_modal.go` + test — keybind reference; closes on `?` or `Esc`
- `go/internal/tui/components/entry_editor_modal.go` + test — form with field navigation (Tab cycling), inline validation against `domain.Validate*`
- `go/internal/tui/components/group_creator_modal.go` + test — single text-input form; non-empty + uniqueness validation
- `go/internal/tui/components/move_to_group_modal.go` + test — list of groups for selected entry; deterministic Esc routing (≥100-run test)

**Unlocks:** Story 6 (every key in `a`/`e`/`g`/`m`/`?` now has a target modal to dispatch).

**Done Looks Like:** `go test ./internal/tui/components/... -run 'Help|EntryEditor|GroupCreator|MoveToGroup' -count=1 -v` green; `MoveToGroupModal` Esc test run with `-count=100` zero flakes; teatest snapshots match v1 visual output for HelpModal full keybind list; EntryEditorModal validation rejects bad hostname/IP with the same error strings as `domain.Validate*`.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-modal-help-i5i` | Port HelpModal (full keybind reference) | Closes on `?` or `Esc`; teatest snapshot matches v1 |
| `hosts-cli-go-mig-p4-modal-editor-n6t` | Port EntryEditorModal (form, field nav, inline validation) | Tab cycles fields; validates via `domain.Validate*`; covers add + edit modes |
| `hosts-cli-go-mig-p4-modal-groupcreator-zed` | Port GroupCreatorModal (text input, non-empty + uniqueness) | Validation matches v1 group rules |
| `hosts-cli-go-mig-p4-modal-movetogroup-9dl` | Port MoveToGroupModal with deterministic Esc routing | Previously-flaky v1 case; ≥100-run Esc test must be zero-flake |

---

### Story 6: Mutations + auto-apply — wire Space/d/a/e/g/m through apply.Runner via tea.Cmd

**Purpose:** Every TUI mutation path (toggle, delete, add, edit, create-group, move-to-group) writes `~/.hosts` and dispatches `apply.Runner.Apply` as a `tea.Cmd`. The StatusBar reflects the apply outcome. Apply failures keep the YAML write (D13). There is no TUI `--dry-run` (D14).

**Why Now:** This is where Phase 4 transitions from "renders v1" to "behaves like v1 + auto-applies". Requires Story 1 (store mutations), Story 3 (Update loop to extend), and Story 5 (modals already in place to open). Sudo handoff (Story 7) is split out because it has its own HIGH-risk profile and depends on terminal-specific behavior.

**Contributes To:** Exit-state clause 5 (TUI mutations auto-apply via apply.Runner; YAML kept on apply failure; no TUI dry-run).

**Creates:**
- `go/internal/tui/app/mutations.go` — keybind handlers for Space (toggle), d (delete with confirm), a (entry-creator open), e (entry-editor open), g (group-creator open), m (move-to-group open)
- `go/internal/tui/app/apply_cmd.go` — `applyCmd(hostsPath)` returns `tea.Cmd` that runs `apply.Runner.ApplyFromFile` on a goroutine and returns `ApplyResultMsg`
- `go/internal/tui/app/update.go` (extend) — handle ModalResultMsg → mutate store → dispatch applyCmd → handle ApplyResultMsg → update StatusBar
- `go/internal/tui/app/app_test.go` (extend) — teatest covering full Space/d flows including ApplyResultMsg surface in StatusBar; apply-failure case asserts YAML kept

**Unlocks:** Story 7 (apply path now exists for sudo handoff to extend); Story 8 (search results can be acted upon via mutation keys).

**Done Looks Like:** `go test ./internal/tui/app/... -run 'TestMutation|TestApplyResult' -count=1 -v` green; teatest flow `boot → j → Space → ApplyResultMsg(success) → StatusBar reflects` works; teatest flow `boot → d → ConfirmModal(y) → ApplyResultMsg(failure) → YAML still written, StatusBar shows failure` works; manual smoke: launch TUI, toggle an entry, verify `~/.hosts` and the managed `/etc/hosts` block both update (sudo prompt handled by Story 7's path or current `CanWriteEtcHosts` fallback).

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-app-mutations-9fk` | Wire Space/d/a/e/g/m keybinds + ModalResult routing in Update | Each key opens correct modal or directly mutates; ModalResultMsg drives store mutations |
| `hosts-cli-go-mig-p4-app-applycmd-91r` | Implement applyCmd tea.Cmd + ApplyResultMsg → StatusBar plumbing | D11 auto-apply; D13 YAML kept on failure; teatest covers both success + failure paths |

---

### Story 7: Sudo TTY handoff via tea.ExecProcess — smoked on 4 terminals

**Purpose:** When `apply.CanWriteEtcHosts()` returns false, the TUI uses `tea.ExecProcess` to release the TTY, run `sudo hostie __apply-privileged ...`, and re-acquire the TTY cleanly. Validated on macOS Terminal, iTerm2, Alacritty, and Linux gnome-terminal. Terminal-specific quirks documented.

**Why Now:** Listed as HIGH-risk #3 in approach.md §4. Depends on Story 6's apply Cmd path existing. Pivots are terminal-specific (cursor restoration, altscreen exit/re-enter, color reset), so the spike must run on real terminals — not in CI. Splitting from Story 6 keeps the mutation logic testable in CI while isolating the manual-validation cost here.

**Contributes To:** Exit-state clauses 6 (sudo TTY handoff via tea.ExecProcess), 11 (HIGH-risk sudo handoff spike).

**Creates:**
- `go/internal/tui/app/apply_cmd.go` (extend from Story 6) — branch on `CanWriteEtcHosts`; if false, use `tea.ExecProcess` to invoke `sudo <exe> __apply-privileged --payload-path=... --owner-uid=...`
- `go/internal/apply/sudo_cmd.go` — helper to construct the sudo command line (reuses payload-file machinery from `apply/privilege.go`); ≥1 unit test for the command-line construction
- `.spikes/go-migration/p4-sudo-handoff/FINDINGS.md` — per-terminal smoke matrix (Terminal/iTerm2/Alacritty/gnome-terminal × clean handoff/cursor restore/altscreen restore/color reset); quirks + agreed mitigation per row
- `docs/go-migration/threat-model-apply-privileged.md` (append) — short section on TUI sudo handoff if any quirk affects the threat model

**Unlocks:** Story 8 (post-search mutations can trigger sudo and return cleanly); Phase 5 (Cutover can rely on TUI sudo working across the supported terminals).

**Done Looks Like:** On each of 4 terminals: operator boots TUI, makes a mutation requiring sudo, sees sudo prompt in the same terminal, enters password, returns to TUI in the correct state (altscreen, cursor at expected position, colors intact); FINDINGS.md row marked PASS for each (terminal, check) cell, or PASS-WITH-NOTE plus mitigation; `apply/sudo_cmd.go` unit tests green; threat-model addendum committed if needed.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-sudo-spike-asr` | Sudo TTY handoff spike: tea.ExecProcess across 4 terminals | HIGH-risk; manual validation on macOS Terminal, iTerm2, Alacritty, Linux gnome-terminal; FINDINGS.md commits per-terminal matrix |
| `hosts-cli-go-mig-p4-sudo-wire-jpr` | Wire sudo branch in apply_cmd.go + extract apply/sudo_cmd.go helper | Branches on CanWriteEtcHosts; reuses payload-file machinery from apply/privilege.go; unit test covers command-line construction |

---

### Story 8: Search mode + hands-on UAT — end-to-end parity, HARD-GATE on exit

**Purpose:** Pressing `/` enters search mode, the StatusBar input accepts keystrokes, the EntryList in the main pane filters live via `search.Engine`, `Esc` restores prior selection, `Enter` accepts the top result. Then: a human walks the v1 keybinding checklist end-to-end against the compiled binary. UAT outcome is the final HARD-GATE per D5/D15 + critical-patterns.md "hands-on UAT is non-negotiable".

**Why Now:** Requires Story 1 (search.Engine), Story 3 (Update loop), Story 6 (mutations exist for "search → act" flows). UAT requires Story 7 (sudo handoff) so the apply path is real, not gated by `CanWriteEtcHosts`. UAT is last because it validates the integration of every earlier story; if UAT fails, the phase doesn't exit.

**Contributes To:** Exit-state clauses 8 (search mode integrates end-to-end), 9 (test parity including search), 10 (hands-on UAT HARD-GATE).

**Creates:**
- `go/internal/tui/app/search_mode.go` — Update branch for `Mode == Search`: route printable chars / Backspace / Esc / Enter; call `search.Engine.Query` on each keystroke; restore prior selection on Esc; accept top result on Enter
- `go/internal/tui/components/entrylist.go` (extend) — accept filtered entry list from search results; render filter highlight
- `go/internal/tui/app/app_test.go` (extend) — teatest: boot → / → type query → assert EntryList filtered → Esc → assert selection restored; second flow: type → Enter → top result selected
- `docs/go-migration/phase-4-uat-log.md` — UAT checklist + run results (PASS / OBSERVED ISSUES / decisions)

**Unlocks:** Phase 5 (Cutover). All parity surfaces (YAML round-trip, `list --json`, `apply --dry-run`, TUI keybindings) now have a green test or signed UAT log.

**Done Looks Like:** `go test ./internal/tui/app/... -run TestSearchMode -count=1 -v` green; teatest search flow filters EntryList in real time; manual UAT walks the full v1 keybind table on the compiled binary on macOS, including ≥1 sudo-required mutation end-to-end; `docs/go-migration/phase-4-uat-log.md` committed with PASS verdict (or with documented OBSERVED ISSUES + remediation plan if not first-pass PASS); compiled binary still ≤18 MB; size-budget.md updated with Phase 4 delta from 4.48 MB Phase 3 baseline.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p4-search-mode-n1c` | Implement search mode in app/update.go (`/` enter, live filter, Esc restore, Enter accept) | Routes search keys, calls search.Engine.Query, mutates EntryList input; teatest covers both Esc and Enter exit paths |
| `hosts-cli-go-mig-p4-uat-acz` | Hands-on UAT against v1 keybinding checklist + size-budget update | HARD-GATE; manual on macOS compiled binary; includes ≥1 sudo-required mutation; commits `phase-4-uat-log.md` + updated `size-budget.md` |
