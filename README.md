# Hostie

A modern `/etc/hosts` manager with TUI and CLI interfaces, using YAML as the source of truth.

## Features

- **YAML Source of Truth**: Manage entries in `~/.hosts` with a clean, structured format
- **TUI Interface**: Interactive terminal UI for easy management
- **CLI Commands**: Full command-line interface for scripting and automation
- **Group Organization**: Organize entries into hierarchical groups
- **Enable/Disable**: Toggle entries without deleting them
- **Shell Completion**: Dynamic completions for bash, zsh, and fish

## Installation

```bash
bun install
bun run build
```

## Usage

### TUI Mode

Launch the interactive terminal UI:

```bash
hostie
```

### CLI Commands

#### Add Entry

```bash
hostie add <ip> <hostname> [options]

Options:
  --group <path>      Group path (e.g., work/prod)
  --alias <name>      Additional alias (can be specified multiple times)
  --disabled          Create entry as disabled
  --comment <text>    Optional comment
```

#### Remove Entry

```bash
hostie rm <hostname>
```

#### Enable/Disable Entry

```bash
hostie enable <hostname>
hostie disable <hostname>
```

#### List Entries

```bash
hostie list [options]

Options:
  --group <path>      Filter by group path
  --json              Output as JSON
```

#### Apply Changes

```bash
hostie apply [options]

Options:
  --dry-run           Show diff without writing
```

#### Manage Groups

```bash
hostie group add <path>         # Create a new group
hostie group rm <path>          # Remove a group
hostie group list               # List all groups
hostie group mv <path> <dest>   # Rename/move a group
```

#### Shell Completion

```bash
hostie completion bash          # Generate bash completion script
hostie completion zsh           # Generate zsh completion script
hostie completion fish          # Generate fish completion script
```

#### Version

```bash
hostie version
```

## Exit Codes

All CLI commands use standardized exit codes for scripting and automation:

| Code | Meaning | Examples |
|------|---------|----------|
| `0` | Success | Command completed successfully |
| `1` | Validation Error | Invalid IP address, duplicate hostname, hostname not found, invalid group name |
| `2` | I/O Error | File not found, disk full, corrupted YAML, read/write failure |
| `3` | Permission Error | Cannot write to `/etc/hosts` (need sudo) |

### Exit Code Examples

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

# Handle permission errors
hostie apply
if [ $? -eq 3 ]; then
  echo "Permission denied - retrying with sudo"
  sudo hostie apply
fi

# Detect I/O errors
hostie list
case $? in
  0) echo "Success" ;;
  1) echo "Validation error" ;;
  2) echo "I/O error - check file permissions" ;;
  3) echo "Permission error" ;;
esac
```

## Data Format

Entries are stored in `~/.hosts` as YAML:

```yaml
version: 1
groups:
  - name: work
    entries:
      - id: 01HQXYZ...
        ip: 192.168.1.100
        hostname: api.work
        aliases: [api]
        enabled: true
        comment: "Production API server"
    groups:
      - name: prod
        entries:
          - id: 01HQXYZ...
            ip: 10.0.0.50
            hostname: db.prod.work
            aliases: []
            enabled: false
```

## Development

```bash
# Run tests
bun test

# Run linter
bun run lint

# Build
bun run build
```

## License

MIT
