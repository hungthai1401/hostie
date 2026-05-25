# Hostie TUI — Manual Testing Checklist

Manual smoke / acceptance checklist for the `hostie` TUI. Automated tests in
`src/tui/**/__tests__` cover units and integrations; this document covers the
end-to-end interactive surface that is impractical to automate (rendering,
keyboard timing, filesystem persistence, terminal redraws).

## How to use

1. Check out the branch under test and build the binary:
   ```bash
   npm ci
   npm run build
   ```
2. Back up the real hosts file before running destructive scenarios:
   ```bash
   cp ~/.hosts ~/.hosts.bak.$(date +%s)
   ```
3. Launch the TUI:
   ```bash
   ./bin/hostie     # or: node dist/cli.js
   ```
4. Walk through each section. Mark each row **PASS** / **FAIL** / **N/A** and
   capture screenshots or terminal recordings for failures.
5. On completion, restore the hosts file if needed:
   ```bash
   mv ~/.hosts.bak.<timestamp> ~/.hosts
   ```

> **Convention:** "Selected" means the row with the highlighted/inverted
> background. "Sidebar" = left group tree. "Main" = right entry list.

---

## 1. Launch & initial render

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 1.1 | Start TUI with a populated `~/.hosts` | Sidebar shows top-level groups, main pane shows entries, status bar visible at bottom | |
| 1.2 | Start TUI with **no** `~/.hosts` file | App starts, empty sidebar, empty main pane, no crash; file is created lazily on first write | |
| 1.3 | Start TUI with an **empty** `~/.hosts` (`{ "version": 1, "groups": [] }`) | Empty-state hint visible (e.g. "No entries — press `a` to add"), no crash | |
| 1.4 | Resize terminal smaller (≤ 60 cols) | Layout reflows without truncation crashes; long entries are clipped, not overflowed | |
| 1.5 | Resize terminal larger | Layout grows, no stale rendering artifacts | |

**Pass criteria:** No exceptions in stderr, no React/ink reconciler warnings,
status bar always rendered.

---

## 2. Navigation keybindings (normal mode)

| # | Key | Context | Expected | Pass/Fail |
|---|-----|---------|----------|-----------|
| 2.1 | `j` | Main pane focus, mid-list | Selection moves down one entry | |
| 2.2 | `j` | Main pane focus, last entry | Selection wraps to first entry | |
| 2.3 | `k` | Main pane focus, mid-list | Selection moves up one entry | |
| 2.4 | `k` | Main pane focus, first entry | Selection wraps to last entry | |
| 2.5 | `j` | Sidebar focus | Selection moves to next group (depth-first, includes nested) | |
| 2.6 | `k` | Sidebar focus | Selection moves to previous group | |
| 2.7 | `Tab` | Sidebar focused | Focus moves to main; first entry selected if none was | |
| 2.8 | `Tab` | Main focused | Focus moves to sidebar; first group selected if none was | |
| 2.9 | `Tab` repeated | Either focus | Focus toggles between sidebar/main cleanly with no flicker | |
| 2.10 | `j`/`k` | Empty list | No crash, no movement, no error | |

**Pass criteria:** Selection indicator follows expected target on every press;
no off-by-one; wrap behavior consistent with `useKeyboard.ts` contract.

---

## 3. Entry actions

| # | Key | Expected | Pass/Fail |
|---|-----|----------|-----------|
| 3.1 | `Space` on enabled entry | Entry flips to disabled (visual `#` prefix or dim style); `~/.hosts` updated on disk; dirty flag clears after write | |
| 3.2 | `Space` on disabled entry | Entry flips to enabled; persisted to disk | |
| 3.3 | `Space` with no selection | No-op, no crash | |
| 3.4 | `d` on selected entry | Confirmation modal opens with message "Delete this entry?" | |
| 3.5 | `d` → `Enter` / `y` | Entry removed; selection moves to next entry (or previous if last); file persisted | |
| 3.6 | `d` → `Esc` / `n` | Modal closes, entry preserved, selection unchanged | |
| 3.7 | `e` on selected entry | Entry editor modal opens pre-filled with current IP/hostname/comment/enabled | |
| 3.8 | `a` (add) in a selected group | Entry editor modal opens with blank fields; on save the new entry appears in the selected group | |
| 3.9 | `m` (move) on selected entry | Move-to-group modal opens listing all groups; selecting a target moves the entry and persists | |
| 3.10 | `Ctrl+S` | Force-save: `~/.hosts` rewritten; status bar briefly shows "Saved"; dirty flag cleared | |

**Pass criteria:** All persistence steps survive a process restart (relaunch
the TUI and confirm the change is still present in `~/.hosts`).

---

## 4. Group actions

| # | Key | Expected | Pass/Fail |
|---|-----|----------|-----------|
| 4.1 | `g` with sidebar focused | Group creator modal opens, input focused | |
| 4.2 | Group creator: type name + Enter | New group appears as sibling of selected group (or root if no parent) | |
| 4.3 | Group creator: empty name + Enter | Validation error shown; modal stays open; no group created | |
| 4.4 | Group creator: duplicate name at same level | Validation error shown; no duplicate created | |
| 4.5 | Group creator: `Esc` | Modal closes, no group created | |
| 4.6 | Navigate into a nested group | Sidebar selection updates with indentation reflecting depth | |
| 4.7 | Create nested group inside selected group | Created under correct parent path | |

**Pass criteria:** New groups round-trip through `~/.hosts` (verify with
`cat ~/.hosts`).

---

## 5. Modals

### 5.1 Entry Editor Modal (`e` / `a`)

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 5.1.1 | Open with `e` on existing entry | Fields pre-filled (IP, hostname, comment, enabled checkbox) | |
| 5.1.2 | `Tab` / `Shift+Tab` | Cycles forward / backward through fields | |
| 5.1.3 | Type in IP field with invalid IP | Save blocked or validation message shown on submit | |
| 5.1.4 | Type valid IPv4 / IPv6 | Accepted on save | |
| 5.1.5 | Empty hostname + save | Validation error, modal stays open | |
| 5.1.6 | `Space` on the enabled checkbox | Toggles checked state | |
| 5.1.7 | `Ctrl+U` in a field | Clears the focused field | |
| 5.1.8 | `Backspace` | Deletes one character from focused field | |
| 5.1.9 | Save (Enter on submit) | Entry list updates immediately and `~/.hosts` is persisted | |
| 5.1.10 | `Esc` | Modal closes; no changes saved | |

### 5.2 Group Creator Modal (`g`)

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 5.2.1 | Open with `g` | Single text input focused, "Create group" title | |
| 5.2.2 | Typing | Characters appear in input | |
| 5.2.3 | Enter on non-empty name | Group created, modal closes | |
| 5.2.4 | Enter on empty name | Validation error, modal stays open | |
| 5.2.5 | `Esc` | Modal closes, no change | |

### 5.3 Move-to-Group Modal (`m`)

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 5.3.1 | Open with `m` on selected entry | Modal lists all groups (including nested with indentation) | |
| 5.3.2 | `j`/`k` | Navigation within group list | |
| 5.3.3 | Enter on target | Entry moves to target group; sidebar/main update; file persisted | |
| 5.3.4 | Select current group | No-op move (or disabled option) | |
| 5.3.5 | `Esc` | Modal closes, entry unchanged | |

### 5.4 Confirm Modal (`d`)

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 5.4.1 | Open with `d` | Modal shows message and Yes/No buttons | |
| 5.4.2 | `←` / `→` | Button focus toggles | |
| 5.4.3 | Enter on Yes (or `y`) | Confirms (delete proceeds) | |
| 5.4.4 | Enter on No (or `n`) | Cancels; modal closes; entry preserved | |
| 5.4.5 | `Esc` | Cancels; modal closes | |

### 5.5 Help Modal (`?`)

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 5.5.1 | Press `?` in normal mode | Help modal opens listing all categories: Navigation, Actions, Modals, Entry Editor, General | |
| 5.5.2 | Verify each documented binding renders | Every keybinding listed in `HelpModal.tsx` `KEYBINDINGS` appears | |
| 5.5.3 | Press `?` again | Modal closes | |
| 5.5.4 | Press `Esc` | Modal closes | |
| 5.5.5 | Press `?` while another modal is open | Should be ignored (mode != "normal") or queued — verify no crash | |

---

## 6. Quit

| # | Key | Expected | Pass/Fail |
|---|-----|----------|-----------|
| 6.1 | `q` in normal mode, no unsaved changes | App exits cleanly, terminal restored | |
| 6.2 | `q` with unsaved (dirty) changes | Confirmation prompt: "Unsaved changes — quit anyway? (y/n)" | |
| 6.3 | `q` confirm yes on dirty | App exits, changes discarded (or saved depending on Ctrl+S beforehand) | |
| 6.4 | `q` confirm no on dirty | Stays in app, dirty state preserved | |
| 6.5 | `Ctrl+C` anywhere | Immediate exit, terminal restored, no hung process | |
| 6.6 | `q` inside a modal | Should not exit; closes modal or is no-op (verify intended behavior) | |

**Pass criteria:** After exit, cursor visible, terminal echo restored, no
stray ANSI codes.

---

## 7. Search mode

| # | Step | Expected | Pass/Fail |
|---|------|----------|-----------|
| 7.1 | Press `/` | Mode switches to "search", input prompt visible in status bar | |
| 7.2 | Type query matching hostnames | Main pane filters in real time | |
| 7.3 | Type query matching IPs | Main pane filters in real time | |
| 7.4 | Type query with **no matches** | Empty state shown ("No matches"), no crash | |
| 7.5 | Press Enter | Commits search; mode returns to normal; filter remains | |
| 7.6 | Press `Esc` | Clears search and returns to normal mode; full list restored | |
| 7.7 | Backspace through query | Filter updates per keystroke | |
| 7.8 | Search while on nested group | Search scope reflects design decision (global or scoped) — verify documented behavior | |

---

## 8. Edge cases

| # | Scenario | Expected | Pass/Fail |
|---|----------|----------|-----------|
| 8.1 | **Empty hosts file** (zero groups) | Sidebar/main render empty state; `a`/`g` still work to bootstrap | |
| 8.2 | **Large list** (≥ 1,000 entries) | Initial render < 2s; `j`/`k` navigation stays responsive (no perceptible lag) | |
| 8.3 | **Deeply nested groups** (≥ 5 levels) | Sidebar shows indentation; navigation reaches deepest group; create-group works at depth | |
| 8.4 | **Very long hostname / comment** (> terminal width) | Truncated with ellipsis, no wrap-induced misalignment | |
| 8.5 | **Unicode / emoji in comments** | Renders correctly; selection width still aligns | |
| 8.6 | **Duplicate hostnames across groups** | Both render; toggle affects only the selected one | |
| 8.7 | **Read-only `~/.hosts`** | Write attempt surfaces an error in status bar; no crash; in-memory state still mutable | |
| 8.8 | **Concurrent external edit** (edit `~/.hosts` in another editor mid-session) | App continues with in-memory copy; next save warns or wins-last (verify D-prefixed decision) | |
| 8.9 | **Search with no matches** | "No matches" banner; `j`/`k` no-op; Esc restores | |
| 8.10 | **Toggle then quit without Ctrl+S** | Disk reflects the toggle (auto-persisted) — verify with `cat ~/.hosts` | |
| 8.11 | **Delete last entry in a group** | Group remains, becomes empty; selection moves to next available entry or clears | |
| 8.12 | **Move entry into its current group** | No duplication; entry unchanged | |

---

## 9. End-to-end scenarios

### Scenario A — First-run bootstrap

1. Remove `~/.hosts` (after backup).
2. Launch hostie.
3. Press `g`, name the group `local`, Enter.
4. Press `a`, fill IP `127.0.0.1`, hostname `dev.local`, enabled = true, save.
5. Quit with `q`.
6. **Expected:** `~/.hosts` exists, contains one group `local` with one
   enabled entry `127.0.0.1 dev.local`.

### Scenario B — Toggle workflow

1. Launch with a populated file.
2. Navigate to an enabled entry, press `Space` — confirm disabled.
3. `Ctrl+C` to exit.
4. Relaunch — confirm the entry is still disabled.
5. `Space` again — confirm enabled, persisted.

### Scenario C — Edit and move

1. Select an entry, press `e`, change hostname, save.
2. Press `m`, move to a different group.
3. Verify entry appears in target group with the new hostname.
4. Verify it no longer appears in the source group.

### Scenario D — Delete with selection follow-through

1. Select the second of three entries.
2. Press `d`, confirm.
3. **Expected:** Entry removed; the entry that was third is now selected
   (becomes the second).
4. Press `d` on the last entry, confirm.
5. **Expected:** Selection moves to the previous (now-last) entry.

### Scenario E — Help discoverability

1. Launch hostie cold.
2. Press `?`.
3. **Expected:** Help modal lists every binding used in scenarios A–D and the
   labels match exactly what the binding does.

### Scenario F — Stress

1. Generate a hosts file with 1,000 entries across 10 groups, 3 levels deep:
   ```bash
   node scripts/gen-fake-hosts.mjs > ~/.hosts   # or hand-craft
   ```
2. Launch hostie.
3. Hold `j` for 5 seconds — selection scrolls smoothly, no dropped frames.
4. `/` then type — filter responsive.
5. `Ctrl+S` — save completes < 500ms.

---

## 10. Cleanup / regressions

| # | Check | Expected | Pass/Fail |
|---|-------|----------|-----------|
| 10.1 | All listed bindings exercised | Every row above has Pass/Fail/N/A | |
| 10.2 | No stderr noise during session | Run with `2>err.log`; file is empty or only contains expected info | |
| 10.3 | Terminal restored on exit | `stty sane` not required after quitting | |
| 10.4 | `~/.hosts` is valid JSON after all scenarios | `jq . ~/.hosts` succeeds | |
| 10.5 | Restore backup | `~/.hosts.bak.*` restored if needed | |

---

## Reporting

For each FAIL row, file a bug with:

- TUI version / commit hash (`git rev-parse --short HEAD`)
- Terminal emulator + size (cols × rows)
- Node version (`node -v`)
- Steps to reproduce (from the row above)
- Expected vs actual
- Screenshot or asciinema recording when possible

File via `br create --title="tui: <summary>" --type=bug --priority=1` and link
to the failing row (e.g. "Checklist row 5.1.5").
