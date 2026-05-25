# Hostie

A modern `/etc/hosts` manager with both a TUI and a CLI, backed by a single YAML source of truth at `~/.hosts`.

Hostie keeps your host entries organized in hierarchical groups, lets you toggle entries on and off without deleting them, and renders the resulting `/etc/hosts` file deterministically.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [TUI Usage](#tui-usage)
- [CLI Usage](#cli-usage)
- [Configuration: `~/.hosts` YAML Format](#configuration-hosts-yaml-format)
- [Shell Completion](#shell-completion)
- [Exit Codes](#exit-codes)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [License](#license)

## Overview

Hostie solves three pain points with `/etc/hosts`:

1. **Structure** — Entries are stored in `~/.hosts` as YAML with hierarchical groups (e.g., `work/prod`, `personal/lab`), aliases, and comments.
2. **Safety** — Changes are made in `~/.hosts` first and only written to `/etc/hosts` with an explicit `hostie apply`. A `--dry-run` mode shows the exact diff before any write.
3. **Speed** — A keyboard-driven TUI for interactive editing, and a scriptable CLI with stable exit codes for automation.

The CLI and TUI operate on the same YAML file, so you can mix workflows without losing state.

## Installation

### Option 1: Download a Prebuilt Binary

Prebuilt single-file binaries are published on the GitHub Releases page for macOS and Linux:

```bash
# macOS (Apple Silicon example)
curl -L -o hostie https://github.com/<owner>/hosts-cli/releases/latest/download/hostie-darwin-arm64
chmod +x hostie
sudo mv hostie /usr/local/bin/hostie
```

Verify the install:

```bash
hostie version
```

### Option 2: Install via npm

```bash
npm install -g hostie
# or
bun add -g hostie
```

### Option 3: Build From Source

Hostie is built with [Bun](https://bun.sh).

```bash
git clone https://github.com/<owner>/hosts-cli.git
cd hosts-cli
bun install
bun run build       # produces a compiled single-file binary in ./dist
./dist/index        # run the built binary
```

To run directly without compiling:

```bash
bun run dev         # launches the TUI from source
```

## Quick Start

```bash
# 1. Add an entry
hostie add 192.168.1.10 api.local --group work --alias api --comment "Staging API"

# 2. Inspect the YAML state
hostie list

# 3. Preview what would be written to /etc/hosts
hostie apply --dry-run

# 4. Apply the changes (requires sudo to write /etc/hosts)
sudo hostie apply
```

Or just launch the TUI and edit interactively:

```bash
hostie
```

## TUI Usage

Run `hostie` (with no arguments) to open the interactive terminal UI. The TUI has a sidebar (groups) and a main pane (entries), with modals for editing, confirmations, and help.

### Keybindings

| Key       | Context        | Action                                            |
|-----------|----------------|---------------------------------------------------|
| `j`       | Normal         | Move selection down (entries or groups)           |
| `k`       | Normal         | Move selection up (entries or groups)             |
| `Tab`     | Normal         | Switch focus between sidebar and main pane        |
| `Space`   | Normal         | Toggle the selected entry enabled / disabled      |
| `d`       | Normal         | Delete the selected entry (confirmation modal)    |
| `a`       | Normal         | Add a new entry to the current group              |
| `e`       | Normal         | Edit the selected entry                           |
| `g`       | Normal         | Group operations (create / move-to-group)         |
| `m`       | Normal         | Move the selected entry to a different group      |
| `/`       | Normal         | Start fuzzy search across hostnames, aliases, IPs |
| `Enter`   | Modal / Search | Confirm action or commit search                   |
| `?`       | Any            | Show the help modal with all keybindings          |
| `q`       | Normal         | Quit the TUI                                      |
| `Esc`     | Modal / Search | Cancel modal or exit search mode                  |
| `Ctrl+C`  | Any            | Exit the application                              |

### Modals

- **Entry Editor** — Use `Tab` / `Shift+Tab` to move between fields, `Space` to toggle the enabled checkbox, `Ctrl+U` to clear a field, `Enter` to save, `Esc` to cancel.
- **Confirmation** — Press `y` / `n` for quick yes/no, or use `← →` to navigate buttons and `Enter` to confirm.
- **Help** — Press `?` from anywhere to open; press `?` or `Esc` to close.

### Search

Press `/` and start typing. Search is fuzzy and ranks matches across hostname (weight 2), aliases (1.5), IP (1), and group path (0.5). Press `Enter` to jump to the top match, or `Esc` to leave search mode.

### Persistence

The TUI writes changes back to `~/.hosts` automatically (e.g., when toggling or deleting entries). Writing to `/etc/hosts` is **never** automatic — run `hostie apply` to push the YAML state to the live hosts file.

## CLI Usage

The CLI is non-interactive and scriptable. All commands operate on `~/.hosts` unless otherwise noted.

### `hostie add <ip> <hostname> [options]`

Add a new host entry.

```bash
hostie add 192.168.1.10 api.local
hostie add 10.0.0.50 db.prod --group work/prod --alias db --comment "Prod DB"
hostie add 127.0.0.2 staging.local --disabled
hostie add fe80::1 router.lan --alias gateway --alias gw
```

Options:

- `--group <path>` — Place the entry inside a group path, e.g., `work/prod`.
- `--alias <name>` — Additional alias. May be specified multiple times.
- `--disabled` — Create the entry in the disabled state.
- `--comment <text>` — Free-form comment stored alongside the entry.

### `hostie rm <hostname-or-id>`

Remove an entry by hostname or ULID.

```bash
hostie rm api.local
hostie rm 01HQXYZABC123...
```

### `hostie enable <hostname>` / `hostie disable <hostname>`

Toggle an existing entry without deleting it.

```bash
hostie disable api.local
hostie enable api.local
```

### `hostie list [options]`

List entries.

```bash
hostie list                         # human-readable tree
hostie list --group work/prod       # filter by group
hostie list --json                  # machine-readable output
```

### `hostie apply [options]`

Render the YAML state and write it to `/etc/hosts`.

```bash
hostie apply --dry-run              # show diff, write nothing
sudo hostie apply                   # write /etc/hosts
```

`apply` preserves any non-hostie-managed content in `/etc/hosts` by writing only within hostie's marker block. The `--dry-run` flag prints the exact diff that would be applied.

### `hostie group <subcommand>`

Manage groups.

```bash
hostie group add work/prod                  # create a (possibly nested) group
hostie group rm work/legacy                 # remove an empty group
hostie group rm work/legacy --force         # remove even if non-empty
hostie group list                           # print the group tree
hostie group mv work/prod ops/prod          # rename or move a group
```

### `hostie completion <shell>`

Print a shell completion script. See [Shell Completion](#shell-completion).

```bash
hostie completion bash
hostie completion zsh
hostie completion fish
```

### `hostie version`

Print the installed version.

```bash
hostie version
# hostie version 0.1.0
```

## Configuration: `~/.hosts` YAML Format

Hostie stores all state in `~/.hosts`. The file is created on first use and can be edited by hand — hostie will round-trip it through its YAML parser on the next command.

```yaml
version: 1
groups:
  - name: work
    entries:
      - id: 01HQXYZABC1234567890ABCDEF
        ip: 192.168.1.100
        hostname: api.work
        aliases: [api]
        enabled: true
        comment: "Production API server"
    groups:
      - name: prod
        entries:
          - id: 01HQXYZABC1234567890ABCDEG
            ip: 10.0.0.50
            hostname: db.prod.work
            aliases: []
            enabled: false
            comment: ""
  - name: personal
    entries: []
    groups: []
```

Field reference:

| Field      | Type            | Notes                                                                 |
|------------|-----------------|-----------------------------------------------------------------------|
| `version`  | integer         | Schema version. Currently `1`.                                        |
| `groups`   | array of Group  | Top-level groups. Groups may nest arbitrarily.                        |
| `name`     | string          | Group name. Must be unique among siblings.                            |
| `entries`  | array of Entry  | Entries directly in this group.                                       |
| `id`       | ULID string     | Stable identifier. Auto-generated; do not edit unless you know why.   |
| `ip`       | string          | IPv4 or IPv6 address.                                                 |
| `hostname` | string          | Primary hostname. Must be unique across the entire file.              |
| `aliases`  | array of string | Additional names. Each must be unique across the file.                |
| `enabled`  | boolean         | If `false`, the entry is skipped when writing `/etc/hosts`.           |
| `comment`  | string          | Optional. Rendered as a `#` comment in `/etc/hosts`.                  |

## Shell Completion

Hostie ships completion scripts for bash, zsh, and fish. Each shell's script can be sourced directly or installed into the user's completion directory.

### Bash

```bash
# One-off (current shell)
source <(hostie completion bash)

# Persistent
hostie completion bash > ~/.local/share/bash-completion/completions/hostie
```

### Zsh

Ensure `fpath` includes a user-writable directory and `compinit` is loaded:

```bash
hostie completion zsh > "${fpath[1]}/_hostie"
# Then restart your shell or run: compinit
```

### Fish

```bash
hostie completion fish > ~/.config/fish/completions/hostie.fish
```

Completions cover commands, subcommands, and dynamic values where applicable (e.g., hostnames for `hostie rm` / `enable` / `disable`).

## Exit Codes

All CLI commands use standardized exit codes for scripting and automation.

| Code | Meaning            | Examples                                                                            |
|------|--------------------|-------------------------------------------------------------------------------------|
| `0`  | Success            | Command completed successfully                                                      |
| `1`  | Validation Error   | Invalid IP, duplicate hostname, hostname not found, invalid group name              |
| `2`  | I/O Error          | File not found, disk full, corrupted YAML, read/write failure on `~/.hosts`         |
| `3`  | Permission Error   | Cannot write to `/etc/hosts` (needs `sudo`)                                         |

### Examples

```bash
# Check if a command succeeded
hostie add 192.168.1.10 test.local
if [ $? -eq 0 ]; then
  echo "Entry added successfully"
fi

# Detect validation errors
hostie add invalid-ip test.local
if [ $? -eq 1 ]; then
  echo "Validation failed - check your input"
fi

# Handle permission errors with automatic sudo retry
hostie apply
if [ $? -eq 3 ]; then
  echo "Permission denied - retrying with sudo"
  sudo hostie apply
fi

# Dispatch on every exit code
hostie list
case $? in
  0) echo "Success" ;;
  1) echo "Validation error" ;;
  2) echo "I/O error - check file permissions" ;;
  3) echo "Permission error" ;;
esac
```

## Troubleshooting

### "Permission denied" when running `hostie apply`

Writing to `/etc/hosts` requires root on macOS and Linux. Run:

```bash
sudo hostie apply
```

Hostie itself does not require root for any other command — only `apply` writes to `/etc/hosts`.

### Changes to `/etc/hosts` don't seem to take effect

The OS and individual applications cache DNS resolutions. Flush the cache:

```bash
# macOS
sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder

# Linux (systemd-resolved)
sudo resolvectl flush-caches

# Linux (nscd)
sudo systemctl restart nscd
```

Browsers (especially Chrome) maintain their own DNS cache. Either restart the browser or visit `chrome://net-internals/#dns` and click "Clear host cache".

### `~/.hosts` is corrupted or unparseable

Hostie exits with code `2` and a parser error. The file is plain YAML — open it in your editor and fix the syntax, or restore from your last backup. Hostie does not currently auto-backup, but `~/.hosts` is small and Git-friendly; consider tracking it in a personal dotfiles repo.

### Duplicate hostname error

Each hostname (and each alias) must be unique across the entire `~/.hosts` file. If `hostie add` fails with exit code `1`, use `hostie list` to find the existing entry and either remove it or pick a different name.

### TUI looks broken or characters are missing

The TUI uses Unicode box-drawing characters and ANSI colors. Use a modern terminal (iTerm2, Alacritty, WezTerm, Kitty, Windows Terminal) and a font with good Unicode coverage. If colors are unreadable, your terminal's color scheme may be overriding the defaults.

### Resetting `/etc/hosts`

Hostie only manages content within its marker block in `/etc/hosts`. To remove hostie-managed entries, delete the marker block manually or restore the default `/etc/hosts` from your OS.

## Development

```bash
bun install
bun test                 # run the test suite
bun run typecheck        # tsc --noEmit
bun run dev              # run TUI from source
bun run build            # produce compiled binary in ./dist
```

The project layout:

```
src/
  cli/         # commander-based CLI parser and subcommand handlers
  tui/         # Ink + React TUI (store, components, hooks)
  core/        # YAML I/O, /etc/hosts rendering, file-io
  domain/      # types, validators, ULID generation
```

Issue tracking and dependency-aware task graph live in `.beads/`; agent operating docs in `AGENTS.md`.

## License

MIT
