# Phase 1 Contract: hostie

## Entry State

- Empty repository (greenfield project)
- Bun 1.1.42+ installed
- User has sudo access for `/etc/hosts` writes
- `/etc/hosts` exists and is readable

## Exit State

1. **Functional TUI**: User can launch `hostie` and see a working terminal interface with:
   - Group tree sidebar (collapsible, navigable with j/k)
   - Entry list main panel (shows hostname, IP, aliases, enabled state)
   - Status bar (shows current mode, help hint)
   - All keybindings work (j/k nav, Space toggle, e edit, d delete, a add, g group menu, m move, / search, Enter/Ctrl+S apply, ? help, q quit)
   - Modals for entry editor, group creator, confirmations

2. **Functional CLI**: All subcommands work:
   - `hostie add <ip> <hostname> [aliases...]` — adds entry to ~/.hosts
   - `hostie rm <hostname>` — removes entry
   - `hostie enable <hostname>` — enables entry
   - `hostie disable <hostname>` — disables entry
   - `hostie list [--json]` — lists all entries
   - `hostie apply [--dry-run]` — applies ~/.hosts to /etc/hosts
   - `hostie group create <name>` — creates group
   - `hostie group add <group> <hostname>` — adds entry to group
   - `hostie completion <shell>` — outputs completion script
   - `hostie version` — shows version

3. **Data persistence**: 
   - `~/.hosts` YAML file is created on first run if missing
   - CRUD operations persist to `~/.hosts`
   - File format matches design.md schema (version: 1, groups: [...])

4. **Apply mechanism works**:
   - Reads `~/.hosts`, renders enabled entries to hosts-file format
   - Writes to `/etc/hosts` between `# BEGIN HOSTIE` / `# END HOSTIE` markers
   - Preserves content outside managed block
   - Re-execs with sudo if EACCES (D10)
   - Atomic writes via temp+rename (D11)
   - Idempotent (only writes if changed) (D12)

5. **Validation enforced**:
   - RFC 952/1123 hostname validation
   - IPv4/IPv6 address validation
   - No duplicate hostnames within enabled entries
   - Kebab-case group names

6. **Tests pass**:
   - Unit tests for domain validators, core YAML I/O, rendering, apply logic
   - Integration tests for CLI subcommands
   - Manual TUI checklist completed

7. **Compiled binaries**:
   - `bun build --compile` produces working binary for host platform (darwin-arm64)
   - Binary runs without Bun installed

## Demo Story

1. User runs `hostie` for the first time
2. TUI launches, shows empty state with "No entries yet. Press 'a' to add one."
3. User presses `a`, modal appears for adding entry
4. User fills in: IP=192.168.1.100, hostname=devserver.local, aliases=devserver
5. Entry appears in list, enabled by default
6. User presses `g`, creates group "development"
7. User presses `m`, moves devserver.local to "development" group
8. Group tree shows "development" with 1 entry
9. User presses Enter to apply
10. Tool prompts for sudo password (if not already elevated)
11. Success message: "Applied 1 entry to /etc/hosts"
12. User runs `ping devserver.local` in another terminal — resolves to 192.168.1.100
13. User presses `q` to quit

## Unlocks

- Users can manage `/etc/hosts` without manually editing the file
- Groups provide organization for large host lists
- TUI provides visual feedback and prevents syntax errors
- CLI enables scripting and automation
- Managed block pattern allows coexistence with manual entries

## Pivot Signals

1. **Ink performance unacceptable**: If TUI is sluggish with >100 entries, consider switching to a native TUI library (blessed, terminal-kit) or optimizing rendering.

2. **Sudo re-exec fails in common scenarios**: If re-exec pattern breaks in CI, Docker, or other non-interactive contexts, pivot to requiring `sudo hostie` upfront.

3. **Bun compile binary size >50MB**: If compiled binary is too large for distribution, consider splitting into separate TUI/CLI binaries or using a different bundler.

4. **Cross-platform testing reveals OS-specific bugs**: If darwin/linux differences are significant, consider platform-specific code paths or dropping support for one platform initially.

5. **YAML parsing performance issues**: If large `~/.hosts` files (>1000 entries) cause noticeable lag, pivot to JSON or a binary format.

6. **Managed block conflicts with existing tools**: If users report conflicts with other tools that modify `/etc/hosts`, consider alternative strategies (separate file + symlink, systemd-resolved integration, etc.).
