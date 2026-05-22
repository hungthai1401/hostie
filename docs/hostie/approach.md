# Approach: hostie

**Feature:** hostie  
**Date:** 2026-05-22  
**Status:** Ready for validation

---

## 1. Gap Analysis

| Dimension | Current State | Required State | Gap |
|-----------|---------------|----------------|-----|
| **Codebase** | Empty repository (greenfield) | Working TUI+CLI tool with full CRUD | Everything needs to be built from scratch |
| **Data model** | None | YAML-based nested group structure with Entry/Group types | Define TypeScript types, YAML schema, validation rules |
| **File I/O** | None | Atomic read/write for `~/.hosts` and `/etc/hosts` with managed block pattern | Implement YAML parser integration, atomic write (temp+rename), managed block extraction/injection |
| **TUI** | None | Full-featured React-based terminal UI with keyboard navigation, modals, search | Build ink-based React app with GroupTree, EntryList, modals, keybindings, Zustand state |
| **CLI** | None | Scriptable subcommands (add/rm/list/apply/group/etc.) | Implement commander-based CLI with 10+ subcommands |
| **Apply mechanism** | None | Sudo re-exec, idempotent writes, DNS cache flush | Implement privilege escalation, diff detection, platform-specific cache flush |
| **Validation** | None | RFC-compliant IP/hostname validation, duplicate detection | Implement validators per RFC 952/1123/791/4291 |
| **Distribution** | None | Single compiled binary for 4 platforms (darwin/linux × x64/arm64) | Set up `bun build --compile` for each target, CI pipeline |
| **Shell completion** | None | Dynamic bash/zsh/fish completions for hostnames/groups | Implement completion script generation with dynamic value completion |

**Summary:** This is a complete greenfield build. No existing code to integrate with or refactor. All components must be built from the ground up following the locked decisions in design.md.

---

## 2. Recommended Approach

### 2.1 Architecture: Monolithic Bun App (D18)

Follow the layered architecture from design.md with strict dependency boundaries:

```
src/
  domain/     # Pure types, validators, ID generation (no I/O)
  core/       # File I/O, rendering, applying (pure functions)
  cli/        # Subcommand handlers (thin wrappers)
  tui/        # ink + React app (UI layer)
  index.ts    # Entry point (dispatch to CLI or TUI)
```

**Dependency flow:** `index.ts` → `cli/` or `tui/` → `core/` → `domain/`  
**Key constraint:** `domain/` and `core/` have zero UI dependencies; `cli/` and `tui/` never import each other.

### 2.2 Technology Stack (D5)

- **Runtime:** Bun 1.1.42 (native TypeScript support, built-in test runner, `--compile` flag)
- **TUI framework:** `ink` (corrected from design.md's `@opentui/react` which doesn't exist; discovery.md confirmed `ink` is the standard React-based TUI framework)
- **CLI parsing:** `commander` (mature, simple, supports nested subcommands and help generation)
- **State management:** `zustand` (as specified in design.md)
- **Fuzzy search:** `fuse.js` (pure JS, no external binary dependency)
- **YAML:** `yaml` by eemeli (spec-compliant, zero dependencies, TypeScript support)
- **ULID:** `ulid` (canonical implementation, lexicographically sortable)

### 2.3 Implementation Order (Bottom-Up Layering)

**Phase 1: Domain layer (pure logic, fully testable)**
1. `domain/types.ts` — Entry, Group, HostsFile types
2. `domain/id.ts` — ULID generation wrapper
3. `domain/validate.ts` — IP/hostname/group validators (RFC 952/1123/791/4291)

**Phase 2: Core layer (file I/O, rendering)**
4. `core/yaml.ts` — loadHostsFile/saveHostsFile with atomic writes
5. `core/render.ts` — renderManagedBlock (YAML → hosts file format)
6. `core/etchosts.ts` — readEtcHosts, writeManagedBlock (preserve outside-marker content)
7. `core/diff.ts` — diff(currentBlock, nextBlock) for dry-run output
8. `core/apply.ts` — orchestrate render + write + sudo re-exec (D10)

**Phase 3: CLI layer (scriptable interface)**
9. `cli/index.ts` — commander setup, subcommand registration
10. `cli/commands/` — implement all subcommands (add/rm/list/enable/disable/group/apply/completion/version)

**Phase 4: TUI layer (interactive interface)**
11. `tui/state/store.ts` — Zustand store with HostsFile + UI state
12. `tui/components/` — GroupTree, EntryList, modals (EntryEditor, EntryCreator, GroupCreator, ConfirmDialog)
13. `tui/components/SearchBar.tsx` — fuzzy search with fuse.js
14. `tui/App.tsx` — root layout, keybindings, modal orchestration
15. `tui/hooks/` — useHostsFile (load/save bridge), useApply (sudo re-exec)

**Phase 5: Entry point + distribution**
16. `src/index.ts` — dispatch logic (argv has subcommand → CLI, else → TUI)
17. `bun build --compile` for 4 targets (darwin-arm64, darwin-x64, linux-x64, linux-arm64)
18. Shell completion scripts (bash/zsh/fish) with dynamic hostname/group completion

**Parallelization opportunities:**
- Phase 1 (domain) must complete first (foundation)
- Phase 2 (core) depends on domain but can be built in parallel across files
- Phase 3 (CLI) and Phase 4 (TUI) can be built **in parallel** after Phase 2 completes (they both depend on core but not on each other)
- Phase 5 (entry point) requires both CLI and TUI to be complete

### 2.4 Key Architectural Decisions

**Decision 1: Managed block pattern (D8)**
- Use `# BEGIN HOSTIE` / `# END HOSTIE` markers in `/etc/hosts`
- Preserve all content outside markers verbatim
- First apply appends block; subsequent applies replace block contents only
- Implementation: `core/etchosts.ts` extracts block via regex, replaces, writes atomically

**Decision 2: Atomic writes everywhere (D11, D12)**
- All writes to `~/.hosts` and `/etc/hosts` use temp file + rename pattern
- Guarantees: partial writes never corrupt files; concurrent edits → last writer wins (acceptable for single-user tool)
- Implementation: `fs.writeFileSync(tmpPath); fs.renameSync(tmpPath, finalPath)`

**Decision 3: Sudo re-exec pattern (D10)**
- `apply` command checks write permission to `/etc/hosts`
- On EACCES: re-exec self with `sudo` (no setuid binary, no daemon)
- Implementation: `core/apply.ts` catches EACCES, calls `Bun.spawn(['sudo', process.argv[0], 'apply'])`

**Decision 4: Idempotent apply (D13)**
- Only write `/etc/hosts` if rendered managed block differs from current block
- Exit 0 if no changes needed
- Implementation: `core/diff.ts` compares blocks; skip write if identical

**Decision 5: TUI-primary interface (D6)**
- Bare `hostie` command launches TUI (default behavior)
- CLI subcommands exist for scripting but are secondary
- Implementation: `src/index.ts` checks `process.argv.length > 2` → CLI, else → TUI

**Decision 6: Groups are organizational only (D4)**
- Every enabled entry in any group is written to `/etc/hosts` on apply
- No group-level enable/disable state
- Disabling happens at entry level (`enabled: false`)
- Implementation: `core/render.ts` flattens all groups, filters by `entry.enabled`, renders

**Decision 7: Validation blocks saves (D14)**
- Validate IP (IPv4/IPv6), hostname (RFC 952/1123), no duplicates
- TUI: show errors inline, block save
- CLI: print to stderr, exit 1
- Implementation: `domain/validate.ts` returns `ValidationError[]`; UI layers check before calling `core/yaml.saveHostsFile`

### 2.5 Mapping Locked Decisions to Implementation

| Decision | Implementation Location | Notes |
|----------|------------------------|-------|
| D1 (hosts-file semantics) | `core/render.ts` | Render entries as `IP hostname [aliases]` format |
| D2 (user-owned source) | `core/yaml.ts`, `core/apply.ts` | Load from `~/.hosts`, apply to `/etc/hosts` with sudo only at apply time |
| D3 (YAML format) | `domain/types.ts`, `core/yaml.ts` | HostsFile type, YAML parse/stringify |
| D4 (groups are labels) | `core/render.ts` | Flatten all groups, no group-level filtering |
| D5 (TypeScript + Bun + ink) | All files, `package.json` | Use ink (not `@opentui/react`), Bun runtime |
| D6 (TUI primary) | `src/index.ts` | Default to TUI, CLI via subcommands |
| D7 (tool name) | `package.json`, binary output | Name: `hostie` |
| D8 (managed block) | `core/etchosts.ts` | `# BEGIN HOSTIE` / `# END HOSTIE` markers |
| D9 (full CRUD in TUI) | `tui/components/`, `tui/state/store.ts` | All CRUD actions + search + apply |
| D10 (sudo re-exec) | `core/apply.ts` | Catch EACCES, re-exec with `sudo` |
| D11 (atomic writes) | `core/yaml.ts`, `core/etchosts.ts` | Temp file + rename pattern |
| D12 (idempotent apply) | `core/apply.ts`, `core/diff.ts` | Compare blocks, skip write if identical |
| D13 (validation) | `domain/validate.ts` | RFC-compliant validators, duplicate detection |
| D14 (exit codes) | All CLI commands | 0=success, 1=validation, 2=I/O, 3=permission |
| D18 (monolithic app) | `src/` structure | Single repo, folder-level boundaries |
| D19 (shell completion) | `cli/commands/completion.ts` | Generate bash/zsh/fish scripts, dynamic completions |

---

## 3. Alternatives Considered

### Alternative 1: Separate TUI and CLI binaries
**Rejected.** Would require two build targets, duplicate core logic, or a shared library. Monolithic app (D18) is simpler: single binary, single entry point, dispatch based on argv. TUI and CLI share the same `core/` layer with zero duplication.

### Alternative 2: JSON instead of YAML for `~/.hosts`
**Rejected.** D3 locks YAML as the format. YAML is more human-friendly for manual edits (comments, no quotes, better nesting). JSON would be valid but contradicts the locked decision.

### Alternative 3: Group-level enable/disable state
**Rejected.** D4 explicitly states groups are organizational only. Adding group-level state would complicate the model (what happens when a group is disabled but an entry is enabled?). Simpler to disable at entry level only.

### Alternative 4: Native TUI library (e.g., `blessed`, `terminal-kit`)
**Rejected.** D5 specifies React-based TUI. Discovery.md confirmed `ink` is the correct library (design.md's `@opentui/react` doesn't exist). React component model gives better composability and state management than imperative TUI libraries.

### Alternative 5: Setuid binary or daemon for sudo-less writes
**Rejected.** D10 specifies sudo re-exec pattern. Setuid binaries are a security risk (must be audited, can be exploited). Daemon adds complexity (lifecycle management, IPC). Sudo re-exec is standard, transparent, and secure.

### Alternative 6: File locking for concurrent edits
**Rejected.** D12 accepts last-writer-wins for concurrent edits (single-user tool). File locking adds complexity (lock files, stale locks, deadlock handling) for a rare edge case. Atomic writes prevent corruption; concurrent edits are acceptable.

### Alternative 7: Backup files on every write
**Rejected.** D8 explicitly states "no automatic backup". Users can use git or manual backups if desired. Automatic backups clutter the filesystem and add complexity (rotation, cleanup).

---

## 4. Risk Map

| Component | Risk Level | Reason | Verification Strategy |
|-----------|-----------|--------|----------------------|
| **domain/types.ts** | LOW | Pure TypeScript types, no I/O | Unit tests (type instantiation, serialization) |
| **domain/validate.ts** | MEDIUM | Complex RFC validation logic (IPv4/IPv6, hostname rules) | Comprehensive unit tests with RFC edge cases |
| **domain/id.ts** | LOW | Thin wrapper around `ulid` library | Unit tests (uniqueness, sortability) |
| **core/yaml.ts** | MEDIUM | YAML parsing (external dep), atomic write logic | Unit tests (round-trip, atomic write, error cases) |
| **core/render.ts** | LOW | Pure function (HostsFile → string) | Unit tests (group comments, disabled entries, aliases) |
| **core/etchosts.ts** | HIGH | Writes to `/etc/hosts` (system file), managed block extraction | Unit tests with temp files + manual verification on real `/etc/hosts` |
| **core/diff.ts** | LOW | Pure function (string comparison) | Unit tests (added/removed/changed detection) |
| **core/apply.ts** | HIGH | Orchestrates sudo re-exec, idempotency, error handling | Unit tests with mocks + manual verification with sudo |
| **cli/index.ts** | LOW | Thin commander setup | Integration tests (subcommand dispatch) |
| **cli/commands/*.ts** | MEDIUM | 10+ subcommands, each with validation and file mutations | Integration tests per subcommand with temp fixtures |
| **tui/state/store.ts** | MEDIUM | Zustand state management, dirty flag logic | Unit tests (state transitions, dirty flag) |
| **tui/components/*.tsx** | MEDIUM | ink components (external dep), keyboard input handling | Manual testing (ink rendering is trusted) |
| **tui/App.tsx** | MEDIUM | Keybindings, modal orchestration, layout | Manual testing checklist |
| **tui/hooks/useApply.ts** | HIGH | Sudo re-exec from TUI context | Manual testing with sudo prompt |
| **Shell completion** | MEDIUM | Dynamic completion (hostname/group from `~/.hosts`) | Manual testing in bash/zsh/fish |
| **Bun compile** | HIGH | External dependency (Bun toolchain), multi-platform builds | CI pipeline for 4 targets, manual verification per platform |
| **DNS cache flush** | HIGH | Platform-specific commands (darwin vs linux), requires sudo | Manual verification on macOS and Linux |

**HIGH-risk items requiring spikes or extra validation:**
1. **core/etchosts.ts** — Managed block extraction with malformed input (missing END marker, nested markers)
2. **core/apply.ts** — Sudo re-exec flow (interactive vs non-interactive, already-root case)
3. **Bun compile** — Verify all 4 targets build and run (cannot cross-compile; need CI or VMs)
4. **DNS cache flush** — Platform detection and correct flush commands (macOS: `dscacheutil + killall mDNSResponder`, Linux: `systemd-resolved` or `nscd`)

---

## 5. Proposed File Structure

```
hostie/
  src/
    domain/
      types.ts              # Entry, Group, HostsFile, ValidationError types
      validate.ts           # validateIP, validateHostname, validateGroup, validateFile
      id.ts                 # ULID wrapper: generateId()
    core/
      yaml.ts               # loadHostsFile, saveHostsFile (atomic write)
      render.ts             # renderManagedBlock(file): string
      etchosts.ts           # readEtcHosts, writeManagedBlock, extractManagedBlock
      apply.ts              # apply(file, etcHostsPath, options): ApplyResult
      diff.ts               # diff(currentBlock, nextBlock): DiffResult
    cli/
      index.ts              # Commander setup, subcommand registration
      commands/
        add.ts              # hostie add <ip> <hostname> [options]
        rm.ts               # hostie rm <hostname-or-id>
        list.ts             # hostie list [--group] [--json]
        enable.ts           # hostie enable <hostname>
        disable.ts          # hostie disable <hostname>
        group.ts            # hostie group {add,rm,mv,list}
        apply.ts            # hostie apply [--dry-run]
        edit.ts             # hostie edit <hostname> (opens $EDITOR)
        completion.ts       # hostie completion {bash,zsh,fish}
        version.ts          # hostie version
    tui/
      App.tsx               # Root component: layout + keybindings
      components/
        GroupTree.tsx       # Recursive group tree with keyboard nav
        EntryList.tsx       # Entry list for selected group
        EntryEditor.tsx     # Modal: edit entry form
        EntryCreator.tsx    # Modal: add entry form
        GroupCreator.tsx    # Modal: add group form
        SearchBar.tsx       # Fuzzy search overlay
        StatusBar.tsx       # Dirty flag, validation errors, apply status
        ConfirmDialog.tsx   # Confirmation for destructive actions
      state/
        store.ts            # Zustand store: HostsFile + UI state + actions
        keybindings.ts      # Centralized keymap
        search.ts           # Fuzzy search with fuse.js
      hooks/
        useHostsFile.ts     # Load/save bridge to core/yaml
        useApply.ts         # Apply with sudo re-exec
    index.ts                # Entry point: dispatch to CLI or TUI
  completions/
    hostie.bash             # Bash completion script
    hostie.zsh              # Zsh completion script
    hostie.fish             # Fish completion script
  tests/
    domain/
      validate.test.ts
      id.test.ts
    core/
      yaml.test.ts
      render.test.ts
      etchosts.test.ts
      diff.test.ts
      apply.test.ts
    cli/
      commands/
        add.test.ts
        rm.test.ts
        list.test.ts
        # ... (one per subcommand)
    tui/
      state/
        store.test.ts
  package.json
  tsconfig.json
  README.md
  .gitignore
```

**New files to create:** All files above (greenfield project).

**Key directories:**
- `src/domain/` — 3 files (types, validate, id)
- `src/core/` — 5 files (yaml, render, etchosts, apply, diff)
- `src/cli/` — 1 index + 10 command files
- `src/tui/` — 1 App + 8 components + 1 store + 2 hooks + 2 utilities
- `completions/` — 3 shell scripts
- `tests/` — mirrors `src/` structure

**Total new files:** ~40 implementation files + ~20 test files + 3 completion scripts + 4 config files (package.json, tsconfig.json, README.md, .gitignore) = **~67 files**.

---

## 6. Dependency Order

### Layer 1: Domain (no dependencies)
- `domain/types.ts`
- `domain/id.ts`
- `domain/validate.ts`

**Parallelizable:** All 3 files can be built in parallel.

### Layer 2: Core (depends on domain)
- `core/yaml.ts` (depends on domain/types)
- `core/render.ts` (depends on domain/types)
- `core/etchosts.ts` (depends on domain/types)
- `core/diff.ts` (no domain dependency, pure string diff)
- `core/apply.ts` (depends on core/yaml, core/render, core/etchosts, core/diff)

**Parallelizable:** yaml, render, etchosts, diff can be built in parallel. Apply must wait for all 4.

### Layer 3a: CLI (depends on core)
- `cli/index.ts` (depends on cli/commands/*)
- `cli/commands/*.ts` (each depends on core/yaml, core/apply, domain/validate)

**Parallelizable:** All command files can be built in parallel. Index waits for commands.

### Layer 3b: TUI (depends on core, parallel with CLI)
- `tui/state/store.ts` (depends on domain/types)
- `tui/state/search.ts` (depends on domain/types)
- `tui/hooks/useHostsFile.ts` (depends on core/yaml, tui/state/store)
- `tui/hooks/useApply.ts` (depends on core/apply, tui/state/store)
- `tui/components/*.tsx` (depends on tui/state/store, tui/hooks/*)
- `tui/App.tsx` (depends on tui/components/*, tui/state/keybindings)

**Parallelizable:** store and search can be built in parallel. Hooks wait for store. Components wait for hooks. App waits for components.

### Layer 4: Entry point (depends on CLI + TUI)
- `src/index.ts` (depends on cli/index, tui/App)

**Parallelizable:** No parallelization (single file, waits for both CLI and TUI).

### Layer 5: Distribution (depends on entry point)
- Shell completion scripts (can be written in parallel with implementation)
- `bun build --compile` for 4 targets (must be done sequentially or in CI with matrix)

**Critical path:** domain → core → (CLI | TUI) → index → compile

**Estimated implementation order (by bead/task):**
1. Domain layer (types, validate, id) — 3 beads
2. Core layer (yaml, render, etchosts, diff, apply) — 5 beads
3. CLI layer (index + 10 commands) — 11 beads
4. TUI layer (store, hooks, 8 components, App) — 12 beads
5. Entry point + distribution (index, compile, completions) — 3 beads

**Total:** ~34 beads (can be further decomposed during planning).

---

## 7. Institutional Learnings Applied

**No prior learnings found.** This is the first implementation in this domain (hosts file management TUI).

**Learnings to capture during implementation:**
1. Ink component patterns (layout, keyboard input, modal orchestration)
2. Bun compile quirks (platform-specific issues, binary size, startup time)
3. Sudo re-exec patterns (interactive vs non-interactive, error handling)
4. Managed block extraction edge cases (malformed markers, concurrent edits)
5. DNS cache flush commands per platform (macOS, Linux distros)
6. Shell completion dynamic value generation (parsing `~/.hosts` efficiently)

**Post-implementation:** Compounding phase will extract critical patterns to `docs/learnings/critical-patterns.md`.

---

## 8. Open Questions for Validating

### Q1: Managed block extraction with malformed input
**Risk:** HIGH  
**Question:** How should `core/etchosts.ts` handle malformed managed blocks (missing END marker, nested BEGIN markers, corrupted content)?  
**Spike needed:** Yes — write spike to test edge cases:
- `/etc/hosts` with `# BEGIN HOSTIE` but no `# END HOSTIE`
- Multiple `# BEGIN HOSTIE` markers
- Content between markers that isn't valid hosts-file format
**Resolution:** Define error messages and recovery strategy (fail-safe: refuse to write, require manual repair).

### Q2: Sudo re-exec in non-interactive contexts
**Risk:** HIGH  
**Question:** How does sudo re-exec behave when stdin is not a TTY (e.g., in a script, CI, or cron job)?  
**Spike needed:** Yes — test `sudo hostie apply` in:
- Interactive terminal (expected: sudo password prompt)
- Non-interactive script (`hostie apply < /dev/null`)
- Already running as root (`sudo hostie apply` when already root)
**Resolution:** Detect TTY, provide clear error message if sudo fails in non-interactive context.

### Q3: Bun compile binary size and startup time
**Risk:** MEDIUM  
**Question:** What is the compiled binary size and startup time for the full app (ink + React + all dependencies)?  
**Spike needed:** Optional — build a minimal ink app with `bun build --compile`, measure size and startup time.  
**Resolution:** If binary is >50MB or startup is >500ms, consider lazy-loading TUI components or splitting CLI/TUI into separate binaries (contradicts D18, so only if critical).

### Q4: DNS cache flush platform detection
**Risk:** HIGH  
**Question:** How to reliably detect platform (macOS vs Linux distro) and run the correct DNS cache flush command?  
**Spike needed:** Yes — test on:
- macOS (darwin): `dscacheutil -flushcache && killall -HUP mDNSResponder`
- Linux with systemd-resolved: `systemctl restart systemd-resolved`
- Linux with nscd: `systemctl restart nscd`
- Linux with dnsmasq: `systemctl restart dnsmasq`
**Resolution:** Use `process.platform` for macOS. For Linux, check which service is running (`systemctl is-active`), fall back to warning message if none detected.

### Q5: Shell completion dynamic value generation performance
**Risk:** MEDIUM  
**Question:** Can shell completions parse `~/.hosts` fast enough for interactive completion (<100ms)?  
**Spike needed:** Optional — write completion script that parses `~/.hosts` YAML, measure time with 1000 entries.  
**Resolution:** If too slow, cache parsed hostnames in a separate file (e.g., `~/.config/hostie/completion-cache.txt`), regenerate on save.

### Q6: Ink component testing strategy
**Risk:** MEDIUM  
**Question:** How to test ink components without manual verification? (Ink renders to terminal, not DOM.)  
**Spike needed:** No — accept manual testing for TUI (per design.md testing strategy).  
**Resolution:** Unit test state logic (store, hooks), manual checklist for TUI rendering and keybindings.

### Q7: YAML schema version forward-compatibility
**Risk:** LOW  
**Question:** How to handle future schema versions (v2, v3) in `~/.hosts`?  
**Spike needed:** No — design decision.  
**Resolution:** Current implementation only supports `version: 1`. If `version` field is missing or >1, error with "Unsupported schema version". Future versions can add migration logic.

### Q8: Concurrent edits to `~/.hosts` from multiple hostie instances
**Risk:** LOW  
**Question:** What happens if two `hostie` TUI instances edit `~/.hosts` simultaneously?  
**Spike needed:** No — design decision (D12: last writer wins).  
**Resolution:** Atomic writes prevent corruption. Last writer wins. No file locking. Document in README as known limitation.

---

## Summary

**Approach:** Bottom-up layered implementation starting with domain (types, validators), then core (file I/O, rendering, apply logic), then CLI and TUI in parallel, finishing with entry point and distribution. Monolithic Bun app with strict dependency boundaries. Use ink (not `@opentui/react`) for TUI, commander for CLI, fuse.js for search, YAML library for parsing. Implement managed block pattern with sudo re-exec, atomic writes, and idempotent apply per locked decisions.

**Key risks:** Managed block extraction edge cases, sudo re-exec in non-interactive contexts, DNS cache flush platform detection, Bun compile multi-platform builds. All HIGH-risk items require spikes or manual verification during validation phase.

**Estimated scope:** ~40 implementation files, ~20 test files, ~34 beads. Critical path: domain → core → (CLI | TUI) → index → compile. CLI and TUI can be built in parallel after core is complete.

**Next step:** Validating phase will verify this approach covers all design.md decisions, run spikes for HIGH-risk items, and decompose into executable beads.
