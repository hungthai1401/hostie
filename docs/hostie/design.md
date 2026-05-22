# Hostie — Design

**Feature slug:** hostie
**Date:** 2026-05-22
**Brainstorming session:** complete
**Scope:** Standard

---

## Feature Boundary

Hostie is a TUI-primary CLI for managing `/etc/hosts` entries via a user-owned YAML source file (`~/.hosts`). Users organize entries into nested groups, edit/toggle them in a rich TUI (or via CLI subcommands), and apply the rendered result into a managed block of `/etc/hosts`. The TUI is the canonical interface; CLI subcommands exist for scripting.

**Domain type(s):** ORGANIZE (nested group data model), SEE (TUI), CALL (CLI subcommands)

**Out of scope for v1:** Import from `/etc/hosts`, wildcard entries, watch + auto-apply, remote sync / git-backed source, multiple source files / profiles, hooks (pre/post-apply scripts), Windows support, undo history beyond single snapshot, theme/color configuration, export to other formats (Caddy, dnsmasq, Ansible).

---

## Locked Decisions

These are fixed. Planning must implement them exactly. No creative reinterpretation.

### Core Semantics
- **D1**: "Host alias" = hosts-file entry (hostname mapped to IP, written to a hosts file). Classic `/etc/hosts` management.
  *Rationale: Standard Unix hosts-file use case, not SSH-style nicknames.*

- **D2**: User-owned source file at **`~/.hosts`** is the source of truth; an apply/sync command writes the merged result to `/etc/hosts` (requires sudo only at apply time). Manual edits to `/etc/hosts` outside the managed block are preserved.
  *Rationale: User-writable source avoids constant sudo; managed-block pattern is standard (used by Ansible blockinfile, hosts Ruby gem).*

### Data Format
- **D3**: `~/.hosts` uses **YAML** as the source of truth. Structured groups with nesting, per-entry metadata (enabled flag, comment). The apply command renders this YAML into hosts-file format inside the managed block of `/etc/hosts`.
  *Rationale: YAML supports nested groups naturally, better for metadata than embedded comments in hosts-file format.*

- **D4**: **Groups are labeling/organization only.** Every enabled entry in any group is written to `/etc/hosts` on apply. Disabling happens at the entry level (`enabled: false`), not the group level. Groups exist to organize the list, filter listings, and structure the TUI — not to gate what gets applied.
  *Rationale: Simpler model; no group-level enable/disable state to track. User can still disable individual entries.*

### Technology Stack
- **D5**: **TypeScript on Bun + `@opentui/react`**. Distribution via `bun build --compile` to single binary. CLI argument parsing via a Bun-friendly library (likely `commander` or `citty`; agent's discretion). The same TUI components can be rendered by either the `tui` command or composed inside CLI prompts.
  *Rationale: React reconciler is the most mature OpenTUI binding (used in OpenCode production), gives composable components for the group tree + entry list, hooks for state (selection, search, edits), and reuse between the full TUI and any inline prompts. Bun's `--compile` removes the Node-runtime distribution problem.*

### Interface Model
- **D6**: **TUI primary, CLI as shortcuts.** Running the bare command opens the TUI by default. Subcommands (`add`, `list`, `rm`, `apply`, etc.) exist for scripting and quick one-shots but are secondary. The TUI is the canonical interface.
  *Rationale: User wants a rich interactive experience as the main workflow; CLI subcommands provide scriptability.*

- **D7**: Tool name is **`hostie`**. Binary command: `hostie`. (Will verify npm package availability during planning before publishing; if taken, fall back is `@<scope>/hostie`.)
  *Rationale: Short, friendly, playful, evokes "hosts".*

### Apply Mechanism
- **D8**: **Managed block with markers** (`# BEGIN HOSTIE` / `# END HOSTIE`). Content outside the markers is preserved verbatim. First apply appends the block; subsequent applies replace its contents only. No automatic backup, no diff prompt by default (a `--dry-run` flag will print the diff without writing).
  *Rationale: Standard pattern (Ansible blockinfile, hosts Ruby gem). Preserves manual edits outside the block. No backup clutter; users can use git or manual backups if desired.*

### TUI Feature Scope
- **D9**: **Full CRUD + search/filter inside the TUI** — browse the group tree, toggle entries, inline-edit IP/hostname/aliases, add new entries, add new groups, delete entries, move entries between groups, apply, and fuzzy-search across entries and groups (with filter-by-group).
  *Rationale: TUI-primary means it must be a real editor, not just a viewer. Full CRUD makes the TUI self-sufficient.*

### Defaults & Implementation Details
- **D10**: `apply` re-exec's itself with `sudo` if it can't write `/etc/hosts`. No setuid binary, no daemon. User sees the sudo prompt.

- **D11**: `~/.hosts` is the user's source-of-truth file. Hostie's own settings (theme, default flags, etc.) live in `~/.config/hostie/config.yaml` if needed; v1 may not need a config file at all.

- **D12**: Writes to `~/.hosts` and `/etc/hosts` are atomic — write-to-temp + rename within the same filesystem. Concurrent edits are not coordinated (single-user tool); last writer wins.

- **D13**: `apply` only writes `/etc/hosts` if the rendered managed block differs from what's already there (idempotent). Exit codes: 0 = no change or applied successfully, non-zero on failure.

- **D14**: On save, hostie validates: IP is well-formed (IPv4 or IPv6), hostname matches RFC 952/1123, no duplicate hostname within the same enabled set. Errors block save (in TUI) or exit non-zero (in CLI).

- **D17**: Everything in the deferred list is **out of scope for v1** (see Feature Boundary above).

### Architecture
- **D18**: **Approach 1 — monolithic Bun app** with `src/{cli,tui,core,domain}/` layout, single compiled binary.
  *Rationale: Single binary, single repo, single distribution. Folder-level boundaries give clean separation without workspace overhead. If `core` needs to become its own package later (deferred items), promoting `src/core/` to `packages/core/` is a mechanical move.*

### Shell Completion
- **D19**: **Shell completion support** for bash, zsh, fish. The CLI library must support completion script generation (e.g., `hostie completion bash`), or we implement it manually. Completions cover subcommands, flags, and dynamic values (hostname/group path completion from `~/.hosts`).
  *Rationale: Professional CLI tools provide completions; improves UX for power users.*

### Agent's Discretion
- **D15**: Agent picks CLI library between `citty`, `commander`, or `cac` during planning. Constraint: must work natively on Bun, support nested subcommands, and produce helpful `--help` output.

- **D16**: Agent picks fuzzy search library (`fuse.js`, `fzf`-style, etc.). Constraint: substring + fuzzy match, fast on ~1000 entries.

---

## Specific Ideas & References

- **OpenTUI**: User surfaced OpenTUI (https://github.com/anomalyco/opentui) as the TUI library. It's a native terminal UI core written in Zig with TypeScript bindings, used in production by OpenCode. React reconciler (`@opentui/react`) is the most mature binding.

- **Managed block pattern**: Standard pattern used by Ansible's `blockinfile` module and the `hosts` Ruby gem. Markers (`# BEGIN HOSTIE` / `# END HOSTIE`) delimit the managed section; content outside is preserved.

---

## Existing Code Context

No existing code — greenfield project. No git repo yet.

---

## Data Model

```typescript
// domain/types.ts
type Entry = {
  id: string;          // ULID; stable across edits, used for TUI selection & references
  ip: string;          // IPv4 or IPv6; validated on save
  hostname: string;    // primary hostname; RFC 952/1123
  aliases: string[];   // additional hostnames on the same line (default: [])
  enabled: boolean;    // default: true; false → emitted as a `#`-commented line
  comment?: string;    // optional trailing `# comment` in rendered output
};

type Group = {
  name: string;        // path-segment; e.g., "prod" inside "work/prod"
  entries: Entry[];    // entries directly in this group
  groups: Group[];     // nested subgroups (recursive)
};

type HostsFile = {
  version: 1;          // schema version for forward-compat
  groups: Group[];     // top-level groups (a synthetic "ungrouped" root holds loose entries if any)
};
```

**On-disk YAML shape (`~/.hosts`):**
```yaml
version: 1
groups:
  - name: work
    entries:
      - { id: 01J5..., ip: 10.0.1.5, hostname: jira.work }
    groups:
      - name: prod
        entries:
          - { id: 01J5..., ip: 10.0.2.10, hostname: db.prod.work }
          - { id: 01J5..., ip: 10.0.2.11, hostname: db-replica.prod.work, enabled: false }
```

**Rendering to `/etc/hosts`** (between `# BEGIN HOSTIE` / `# END HOSTIE`):
```
# BEGIN HOSTIE
# group: work
10.0.1.5  jira.work
# group: work/prod
10.0.2.10 db.prod.work
# 10.0.2.11 db-replica.prod.work
# END HOSTIE
```

**Validation rules (D14):**
- `ip` parses as IPv4 (RFC 791) or IPv6 (RFC 4291).
- `hostname` and each `alias` match `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$` (RFC 952/1123, total ≤ 253 chars).
- No duplicate hostname across enabled entries (across the whole file, not per group). Aliases also count.
- Group names: `[a-z0-9][a-z0-9-]*` (kebab-case, no slashes — nesting is structural, not in the name).

---

## Architecture

```
src/
  domain/           # Pure types and validators. No I/O. Reusable.
    types.ts          # Entry, Group, HostsFile
    validate.ts       # validateIP, validateHostname, validateGroup, validateFile (returns ValidationError[])
    id.ts             # ULID generator for Entry.id
  core/             # File I/O, rendering, applying. Pure functions where possible.
    yaml.ts           # loadHostsFile(path), saveHostsFile(path, file) — atomic write
    render.ts         # renderManagedBlock(file): string
    etchosts.ts       # readEtcHosts, writeManagedBlock — preserves outside-marker content; atomic
    apply.ts          # apply(file, etcHostsPath, { dryRun, sudo }): ApplyResult
    diff.ts           # diff(currentBlock, nextBlock): { added, removed, changed }
  cli/              # Subcommand handlers. Thin wrappers around core/.
    index.ts          # Entry point: parses argv, dispatches subcommand or launches TUI
    commands/
      add.ts          # hostie add <ip> <hostname> [--group=path/to/group] [--alias=...] [--disabled]
      rm.ts           # hostie rm <hostname-or-id>
      list.ts         # hostie list [--group=...] [--json]
      enable.ts       # hostie enable <hostname>
      disable.ts      # hostie disable <hostname>
      group.ts        # hostie group {add,rm,mv,list} ...
      apply.ts        # hostie apply [--dry-run]
      edit.ts         # hostie edit <hostname>  (opens $EDITOR on the entry)
      completion.ts   # hostie completion {bash,zsh,fish}
      version.ts
  tui/              # OpenTUI + React app.
    App.tsx           # Root; layout: sidebar (group tree) + main (entry list) + status bar
    components/
      GroupTree.tsx     # Recursive tree; keyboard nav
      EntryList.tsx     # Selected group's entries; toggle/edit/delete
      EntryEditor.tsx   # Inline edit modal for an entry
      EntryCreator.tsx  # Add-new modal
      GroupCreator.tsx  # Add-new-group modal
      SearchBar.tsx     # Fuzzy search across all entries+groups
      StatusBar.tsx     # Pending changes, save/apply hints, validation errors
      ConfirmDialog.tsx # Destructive-action confirmation
    state/
      store.ts          # Zustand or simple useReducer; holds HostsFile + UI state + dirty flag
      keybindings.ts    # Centralized keymap
      search.ts         # Fuzzy search implementation
    hooks/
      useHostsFile.ts   # Load + save bridge to core/yaml
      useApply.ts       # Trigger apply, handle sudo re-exec
  index.ts          # Single entry: if argv has subcommand → cli; else → tui
```

**Data flow:**
- Read: `cli` or `tui` → `core/yaml.loadHostsFile` → `HostsFile` object.
- Mutate: `tui` mutates in-memory `HostsFile` via store actions; `cli` subcommands construct the mutation and call `core/yaml.saveHostsFile`.
- Save: `core/yaml.saveHostsFile` writes `~/.hosts` atomically (temp file + rename).
- Apply: `core/apply.apply` reads `~/.hosts`, calls `render.renderManagedBlock`, then `etchosts.writeManagedBlock`. If write fails with EACCES, re-execs via `sudo` (D10).

**Boundaries:**
- `domain/` knows nothing of files or YAML — pure types and validators. Tested in isolation.
- `core/` knows nothing of UI — pure functions taking paths/strings, returning structured results. Tested in isolation.
- `cli/` and `tui/` both depend on `core/`, never on each other.
- The TUI is a React app over OpenTUI; no DOM, no Node-only APIs that Bun's runtime doesn't support.

**Distribution:**
- `bun build --compile --target=bun-darwin-arm64 src/index.ts -o dist/hostie` (and equivalents for `bun-darwin-x64`, `bun-linux-x64`, `bun-linux-arm64`).
- Published to npm as `hostie` (with scoped fallback `@<scope>/hostie` if name is taken — checked during planning).
- Install via `bun install -g hostie`, `npm i -g hostie`, or download prebuilt binary from GitHub Releases.

**Error handling:**
- Validation errors → structured `ValidationError[]` with field path; TUI surfaces inline, CLI prints and exits non-zero.
- I/O errors (file read/write) → wrapped in `HostsError` with context; CLI shows message + path, TUI surfaces in status bar.
- Sudo failure → distinct error path; instructions to re-run with `sudo` if non-interactive.

---

## TUI Interaction Model

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│ hostie                                      [?] Help  [q] Quit│
├──────────────┬──────────────────────────────────────────────┤
│              │                                               │
│  Groups      │  Entries in: work/prod                       │
│              │                                               │
│  ▼ work      │  ☑ 10.0.2.10  db.prod.work                   │
│    ▼ prod    │  ☐ 10.0.2.11  db-replica.prod.work           │
│      staging │  ☑ 10.0.2.15  api.prod.work  [api]           │
│  ▼ personal  │                                               │
│    dev       │  [a] Add  [e] Edit  [d] Delete  [Space] Toggle│
│              │  [/] Search  [Enter] Apply                    │
│              │                                               │
│              │                                               │
├──────────────┴──────────────────────────────────────────────┤
│ 3 entries • 2 enabled • Unsaved changes                      │
└──────────────────────────────────────────────────────────────┘
```

**Key bindings:**
- **Navigation:** `↑/↓` or `j/k` move selection; `Tab` switches focus between group tree and entry list; `←/→` or `h/l` collapse/expand groups.
- **Entry actions:** `Space` toggles enabled/disabled; `e` opens inline editor; `d` deletes (with confirmation); `a` opens add-entry modal.
- **Group actions:** `g` opens group menu (add/rename/delete group); `m` moves selected entry to a different group.
- **Search:** `/` opens fuzzy search bar; type to filter; `Esc` clears; `Enter` on result jumps to it.
- **Apply:** `Enter` (or `Ctrl+S`) saves `~/.hosts` and applies to `/etc/hosts` (prompts for sudo if needed).
- **Global:** `?` toggles help overlay; `q` quits (prompts if unsaved changes).

**Modals:**
- **Entry editor:** Form with fields for IP, hostname, aliases (comma-separated), enabled checkbox, comment. `Enter` saves, `Esc` cancels. Validation errors shown inline.
- **Entry creator:** Same form, pre-filled with current group. Creates new entry with generated ULID.
- **Group creator:** Single text input for group name (validates kebab-case). Creates under currently selected group or at root.
- **Confirmation dialog:** For destructive actions (delete entry, delete group with children). `y/n` or `Enter/Esc`.

**State management:**
- Zustand store holds: `file: HostsFile`, `dirty: boolean`, `selectedGroupPath: string[]`, `selectedEntryId: string | null`, `searchQuery: string`, `modal: null | { type, data }`.
- Actions: `loadFile`, `saveFile`, `addEntry`, `updateEntry`, `deleteEntry`, `toggleEntry`, `addGroup`, `deleteGroup`, `moveEntry`, `setSearch`, `openModal`, `closeModal`, `apply`.
- `dirty` flag set on any mutation; cleared on successful save. Status bar shows "Unsaved changes" when dirty.

**Search behavior (D16 — Agent's Discretion on library):**
- Fuzzy match on `hostname`, `aliases`, `ip`, and group path.
- Results ranked by match score; top 10 shown in a dropdown overlay.
- Selecting a result focuses that entry in the list and scrolls to it.

**Apply flow:**
1. User presses `Enter` or `Ctrl+S`.
2. If dirty: save `~/.hosts` first (validates; blocks if errors).
3. Call `core/apply.apply` with `{ dryRun: false, sudo: true }`.
4. If EACCES: show "Needs sudo" message, re-exec with `sudo hostie apply` (or prompt user to run manually if non-interactive).
5. On success: show "Applied ✓" in status bar for 2s.
6. On failure: show error in status bar; log to stderr.

---

## CLI Subcommands

All subcommands operate on `~/.hosts` and exit with status 0 on success, non-zero on failure. Validation errors print to stderr with field context.

**Core commands:**
```bash
hostie                          # Launch TUI (default behavior)
hostie add <ip> <hostname>      # Add entry; --group=path/to/group, --alias=name, --disabled, --comment="..."
hostie rm <hostname-or-id>      # Remove entry by hostname or ULID
hostie enable <hostname>        # Set enabled=true
hostie disable <hostname>       # Set enabled=false
hostie edit <hostname>          # Open entry in $EDITOR as YAML snippet; save on exit
hostie list [--group=...]       # List entries; --json for machine-readable output
hostie apply [--dry-run]        # Render and write to /etc/hosts; --dry-run prints diff without writing
```

**Group commands:**
```bash
hostie group add <path>         # Create group at path (e.g., work/staging)
hostie group rm <path>          # Delete group (fails if non-empty unless --force)
hostie group list               # List all groups as tree
hostie group mv <path> <dest>   # Rename/move group
```

**Completion commands:**
```bash
hostie completion bash          # Print bash completion script
hostie completion zsh           # Print zsh completion script
hostie completion fish          # Print fish completion script
```

**Utility commands:**
```bash
hostie version                  # Print version + Bun runtime version
hostie help [command]           # Show help
```

**Examples:**
```bash
# Add a new entry to work/prod group
hostie add 10.0.2.20 cache.prod.work --group=work/prod --alias=redis

# Disable an entry
hostie disable db-replica.prod.work

# List all entries in work group (recursive)
hostie list --group=work

# Dry-run apply to see what would change
hostie apply --dry-run

# Apply changes (will prompt for sudo if needed)
hostie apply
```

**Output format:**
- `list` default: table with columns `[✓/✗] IP  Hostname  Aliases  Group`.
- `list --json`: array of Entry objects with group path injected.
- `apply --dry-run`: unified diff of the managed block.
- Errors: `Error: <message>` to stderr, exit code 1 (validation) or 2 (I/O) or 3 (permission).

**Shell completion (D19):**
- `hostie completion bash/zsh/fish` generates completion scripts.
- Dynamic completions: `hostie rm <TAB>` completes with hostnames from `~/.hosts`; `hostie enable <TAB>` completes with disabled hostnames; `hostie list --group=<TAB>` completes with group paths.
- Installation documented in README (user copies script to shell-specific completion directory).

---

## Testing Strategy

**Unit tests (Bun's built-in test runner):**
- `domain/validate.test.ts` — all validation rules (IP formats, hostname RFC compliance, duplicate detection, group name constraints).
- `domain/id.test.ts` — ULID generation (uniqueness, sortability).
- `core/yaml.test.ts` — load/save round-trip, atomic write behavior, schema version handling.
- `core/render.test.ts` — managed block rendering (group comments, disabled entries, aliases).
- `core/etchosts.test.ts` — managed block extraction, preservation of outside-marker content, atomic write.
- `core/diff.test.ts` — diff output correctness (added/removed/changed).
- `core/apply.test.ts` — apply logic (idempotency, dry-run, error paths). Uses temp files, not real `/etc/hosts`.

**Integration tests:**
- `cli/commands/*.test.ts` — each subcommand with temp `~/.hosts` fixture; verify file mutations and stdout/stderr.
- `tui/state/store.test.ts` — state transitions (add/edit/delete/toggle/move); dirty flag behavior.

**Manual TUI testing checklist (pre-release):**
- Keyboard navigation (all bindings work).
- Modal flows (add/edit/delete with validation errors).
- Search (fuzzy match, result selection).
- Apply flow (save + sudo prompt if needed).
- Quit with unsaved changes (confirmation).

**No tests for:**
- OpenTUI rendering itself (trust the library).
- Actual `/etc/hosts` writes in CI (too invasive; manual verification on local dev machine).

**CI (GitHub Actions):**
- Run `bun test` on push.
- Build binaries for darwin-arm64, darwin-x64, linux-x64, linux-arm64.
- Attach binaries to release on tag push.

---

## Error Handling & Edge Cases

**Validation errors:**
- Invalid IP → `ValidationError: "ip" must be a valid IPv4 or IPv6 address`
- Invalid hostname → `ValidationError: "hostname" must match RFC 952/1123 (got: "foo_bar")`
- Duplicate hostname → `ValidationError: hostname "db.prod.work" already exists in group "work/staging"`
- Invalid group name → `ValidationError: group name must be kebab-case (got: "Work/Prod")`
- All validation errors include field path and current value; TUI shows inline, CLI prints to stderr and exits 1.

**File I/O errors:**
- `~/.hosts` missing on first run → create with empty template (`version: 1, groups: []`).
- `~/.hosts` corrupted YAML → show parse error with line number; TUI refuses to start, CLI exits 2.
- `~/.hosts` has unknown schema version → error: "Unsupported schema version X; this hostie supports version 1."
- Write failure (disk full, permission) → error with path and OS error; exit 2.

**Apply errors:**
- `/etc/hosts` missing → error: "Cannot find /etc/hosts; is this a Unix system?"
- `/etc/hosts` has no managed block → append block at end (first-time apply).
- `/etc/hosts` has malformed managed block (missing END marker) → error: "Managed block is corrupted; manual repair required."
- Write permission denied → re-exec with `sudo` (D10); if already running as root, fail with "Cannot write /etc/hosts even as root."
- Concurrent modification (another tool edited `/etc/hosts` between read and write) → detect via mtime check; warn and proceed (last-writer-wins; no locking).

**TUI edge cases:**
- Empty `~/.hosts` → show "No entries yet. Press [a] to add one."
- Delete last entry in a group → group remains (empty groups are valid).
- Delete group with children → confirmation: "Group 'work' has 2 subgroups and 5 entries. Delete all? [y/N]"
- Search with no results → show "No matches for 'xyz'."
- Unsaved changes on quit → confirmation: "You have unsaved changes. Quit anyway? [y/N]"

**CLI edge cases:**
- `hostie add` with duplicate hostname → validation error, exit 1.
- `hostie rm <nonexistent>` → error: "No entry found for 'foo.local'", exit 1.
- `hostie apply` with no changes → print "No changes to apply", exit 0.
- `hostie group rm <nonempty>` without `--force` → error: "Group 'work' is not empty. Use --force to delete.", exit 1.

**Atomicity guarantees (D12):**
- All writes (to `~/.hosts` and `/etc/hosts`) use temp file + rename within the same filesystem.
- If write fails mid-operation, original file is untouched.
- No file locking; concurrent edits from multiple `hostie` instances → last writer wins (acceptable for single-user tool).

---

## Success Criteria

**v1 is complete when:**

1. **Core functionality works:**
   - User can create/edit/delete entries and groups via TUI and CLI.
   - `apply` correctly writes the managed block to `/etc/hosts`, preserving outside-marker content.
   - Validation catches malformed IPs, hostnames, and duplicates before save.
   - Sudo re-exec works when `/etc/hosts` write needs elevation.

2. **TUI is usable:**
   - All keybindings work (navigation, toggle, edit, delete, search, apply, quit).
   - Modals render correctly (entry editor, group creator, confirmations).
   - Fuzzy search finds entries and groups; selecting a result jumps to it.
   - Status bar shows dirty state, validation errors, and apply results.
   - Unsaved changes prompt on quit.

3. **CLI is scriptable:**
   - All subcommands (`add`, `rm`, `enable`, `disable`, `list`, `apply`, `group`) work.
   - `--json` output is valid and parseable.
   - `--dry-run` shows diff without writing.
   - Shell completions generate for bash/zsh/fish and complete hostnames/groups dynamically.
   - Exit codes distinguish success (0), validation errors (1), I/O errors (2), permission errors (3).

4. **Distribution works:**
   - `bun build --compile` produces working binaries for darwin-arm64, darwin-x64, linux-x64, linux-arm64.
   - Published to npm as `hostie` (or scoped fallback).
   - README documents install, usage, TUI keybindings, CLI examples, completion setup.

5. **Tests pass:**
   - All unit tests green (`bun test`).
   - Integration tests cover CLI subcommands and TUI state mutations.
   - Manual TUI checklist completed on macOS and Linux.

---

## Deferred Ideas

Out-of-scope for v1 (captured for future consideration):

1. **Import from `/etc/hosts`** — one-time migration command to parse existing `/etc/hosts` entries into `~/.hosts`.
2. **Wildcard entries** — DNS-style wildcards like `*.dev.local`.
3. **Watch + auto-apply** — `hostie watch` mode that monitors `~/.hosts` and auto-applies on change.
4. **Remote sync / git-backed source** — cloud sync or git integration for `~/.hosts`.
5. **Multiple source files / profiles** — e.g., `~/.hosts.work`, `~/.hosts.home`, switch between them.
6. **Hooks** — pre-apply, post-apply scripts for custom workflows.
7. **Windows support** — manage `%SystemRoot%\System32\drivers\etc\hosts`.
8. **Undo history** — beyond a single snapshot; full undo/redo stack.
9. **Theme/color configuration** — customizable TUI colors and styles.
10. **Export to other formats** — Caddy, dnsmasq, Ansible inventory, etc.

---

## Handoff Note

design.md is the single source of truth for this feature.

- **writing-plans** reads: locked decisions, data model, architecture, CLI/TUI specs, deferred-to-planning questions
- **validating** reads: locked decisions (to verify plan-checker coverage)
- **executing-plans** reads: locked decisions (to honor during implementation)
- **reviewing** reads: locked decisions (for UAT verification)

Decision IDs (D1–D19) are stable. Reference them by ID in all downstream artifacts.
