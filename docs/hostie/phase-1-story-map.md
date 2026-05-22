# Story Map: hostie

## Plan
Enable users to manage `/etc/hosts` entries through a visual TUI and scriptable CLI, with changes persisted to a YAML source file.

## Phase: Foundation & Implementation

### Story 1: Data Foundation
**Purpose:** Users' host entries are represented as typed data structures with validation.
**Contributes To:** Exit state #3 (data persistence), #5 (validation enforced)
**Unlocks:** All other stories (nothing can be built without the data model)

**Beads:**
- `hosts-cli-379.1` — Define core types (Entry, Group, HostsFile)
- `hosts-cli-379.2` — Implement hostname validation (RFC 952/1123)
- `hosts-cli-379.3` — Implement IP address validation (IPv4/IPv6)
- `hosts-cli-379.4` — Implement ULID generation for entry IDs
- `hosts-cli-379.5` — Implement duplicate hostname detection

### Story 2: File Persistence
**Purpose:** Users' entries are saved to and loaded from `~/.hosts` in YAML format.
**Contributes To:** Exit state #3 (data persistence)
**Unlocks:** CLI and TUI can read/write user data

**Beads:**
- `hosts-cli-379.6` — Implement YAML serialization (HostsFile → YAML string)
- `hosts-cli-379.7` — Implement YAML deserialization (YAML string → HostsFile)
- `hosts-cli-379.8` — Implement ~/.hosts file I/O (read, write, create if missing)
- `hosts-cli-379.9` — Implement schema version handling (v1 only for now)

### Story 3: Hosts File Rendering
**Purpose:** Enabled entries are rendered to valid `/etc/hosts` format.
**Contributes To:** Exit state #4 (apply mechanism works)
**Unlocks:** Apply logic can generate output

**Beads:**
- `hosts-cli-379.10` — Implement hosts-file format renderer (Entry → "IP hostname aliases...")
- `hosts-cli-379.11` — Implement managed block wrapper (add BEGIN/END markers)
- `hosts-cli-379.12` — Implement enabled-only filtering

### Story 4: Apply Mechanism
**Purpose:** Users can apply their `~/.hosts` changes to `/etc/hosts` safely.
**Contributes To:** Exit state #4 (apply mechanism works)
**Unlocks:** Core functionality is complete

**Beads:**
- `hosts-cli-379.13` — Implement /etc/hosts reader (parse existing file)
- `hosts-cli-379.14` — Implement managed block extraction (find BEGIN/END, preserve outside content)
- `hosts-cli-379.15` — Implement managed block replacement (splice new content between markers)
- `hosts-cli-379.16` — Implement atomic write (temp file + rename)
- `hosts-cli-379.17` — Implement idempotency check (only write if changed)
- `hosts-cli-379.18` — Implement sudo re-exec (detect EACCES, re-run with sudo)

### Story 5: CLI Commands
**Purpose:** Users can manage entries via command-line for scripting and automation.
**Contributes To:** Exit state #2 (functional CLI)
**Unlocks:** Scriptable workflows

**Beads:**
- `hosts-cli-379.19` — Implement CLI argument parser (commander setup)
- `hosts-cli-379.20` — Implement `hostie add` subcommand
- `hosts-cli-379.21` — Implement `hostie rm` subcommand
- `hosts-cli-379.22` — Implement `hostie enable` subcommand
- `hosts-cli-379.23` — Implement `hostie disable` subcommand
- `hosts-cli-379.24` — Implement `hostie list` subcommand (with --json flag)
- `hosts-cli-379.25` — Implement `hostie apply` subcommand (with --dry-run flag)
- `hosts-cli-379.26` — Implement `hostie group create` subcommand
- `hosts-cli-379.27` — Implement `hostie group add` subcommand
- `hosts-cli-379.28` — Implement `hostie completion` subcommand
- `hosts-cli-379.29` — Implement `hostie version` subcommand
- `hosts-cli-379.30` — Implement exit codes (0=success, 1=validation, 2=I/O, 3=permission)

### Story 6: TUI Foundation
**Purpose:** Users see a working terminal interface with navigation and layout.
**Contributes To:** Exit state #1 (functional TUI)
**Unlocks:** Interactive visual management

**Beads:**
- `hosts-cli-379.31` — Set up Ink + React + Zustand
- `hosts-cli-379.32` — Implement app state store (Zustand)
- `hosts-cli-379.33` — Implement main layout (sidebar + main + status bar)
- `hosts-cli-379.34` — Implement group tree component (collapsible, navigable)
- `hosts-cli-379.35` — Implement entry list component (hostname, IP, aliases, enabled)
- `hosts-cli-379.36` — Implement status bar component (mode, help hint)
- `hosts-cli-379.37` — Implement keyboard navigation (j/k, focus management)

### Story 7: TUI Interactions
**Purpose:** Users can perform all CRUD operations through the TUI.
**Contributes To:** Exit state #1 (functional TUI)
**Unlocks:** Full visual workflow

**Beads:**
- `hosts-cli-379.38` — Implement entry editor modal (add/edit form)
- `hosts-cli-379.39` — Implement group creator modal
- `hosts-cli-379.40` — Implement confirmation modal (delete, apply)
- `hosts-cli-379.41` — Implement fuzzy search (fuse.js integration)
- `hosts-cli-379.42` — Implement toggle enabled (Space key)
- `hosts-cli-379.43` — Implement delete entry (d key)
- `hosts-cli-379.44` — Implement move to group (m key)
- `hosts-cli-379.45` — Implement apply action (Enter/Ctrl+S)
- `hosts-cli-379.46` — Implement help modal (? key)
- `hosts-cli-379.47` — Implement quit (q key)

### Story 8: Entry Point & Distribution
**Purpose:** Users can install and run `hostie` as a single binary.
**Contributes To:** Exit state #7 (compiled binaries)
**Unlocks:** Distribution and installation

**Beads:**
- `hosts-cli-379.48` — Implement main entry point (route to TUI or CLI based on args)
- `hosts-cli-379.49` — Create package.json with dependencies and scripts
- `hosts-cli-379.50` — Create tsconfig.json for Bun
- `hosts-cli-379.51` — Implement build script (bun build --compile)
- `hosts-cli-379.52` — Create shell completion scripts (bash, zsh, fish)
- `hosts-cli-379.53` — Write README with installation and usage instructions

### Story 9: Testing & Quality
**Purpose:** Core functionality is verified through automated tests.
**Contributes To:** Exit state #6 (tests pass)
**Unlocks:** Confidence in correctness

**Beads:**
- `hosts-cli-379.54` — Write unit tests for domain validators
- `hosts-cli-379.55` — Write unit tests for YAML I/O
- `hosts-cli-379.56` — Write unit tests for hosts-file rendering
- `hosts-cli-379.57` — Write unit tests for managed block extraction/replacement
- `hosts-cli-379.58` — Write unit tests for apply logic
- `hosts-cli-379.59` — Write integration tests for CLI subcommands
- `hosts-cli-379.60` — Create manual TUI testing checklist
- `hosts-cli-379.61` — Set up CI (type check, lint, test, build)
