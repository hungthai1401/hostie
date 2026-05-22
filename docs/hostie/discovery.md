# Discovery: hostie

## Institutional Learnings

No prior learnings for this domain.

## Runtime & Tooling Constraints

### Bun Runtime
- **Version:** 1.1.42
- **Compile support:** ✅ Confirmed via `bun build --help`
  - `--compile` flag available for creating standalone executables
  - `--target` flag supports: `bun`, `bun-linux-x64`, `bun-linux-arm64`, `bun-darwin-x64`, `bun-darwin-arm64`, `bun-windows-x64`
  - All four target platforms (darwin-arm64, darwin-x64, linux-x64, linux-arm64) are supported
- **TypeScript:** ✅ Native TypeScript support built-in (no separate `tsc` required for execution)

### TypeScript Tooling
- **Bun's built-in transpiler:** Available for `.ts` files
- **Type checking:** Bun does not perform type checking by default; recommend adding `bun x tsc --noEmit` to CI/pre-commit hooks if strict type validation is needed
- **tsconfig.json:** Should be created with `"module": "esnext"` and `"target": "esnext"` for Bun compatibility

### /etc/hosts File
- **Location:** `/etc/hosts` (standard on darwin/macOS)
- **Format:** Standard hosts file format (IP address, whitespace, hostname(s), optional comment)
- **Read access:** ✅ Readable without sudo (world-readable: `-rw-r--r--`)
- **Write access:** ❌ Requires `sudo` (owned by `root:wheel`)
- **Sample entry:** `127.0.0.1 localhost`

### Platform-Specific Notes
- **darwin (macOS):**
  - `/etc/hosts` changes require `sudo` for writes
  - DNS cache flush needed after modification: `sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder`
  - File must maintain proper permissions (644) to avoid system warnings
- **linux:**
  - `/etc/hosts` location identical
  - DNS cache behavior varies by distro (systemd-resolved, nscd, dnsmasq)
  - May need `sudo systemctl restart systemd-resolved` or equivalent

### Blockers & Warnings
- ⚠️ **Sudo requirement:** All write operations to `/etc/hosts` will require `sudo` elevation
  - Tool must handle privilege escalation gracefully
  - Consider prompting user once at start vs. per-operation
  - Validate sudo access before attempting writes
- ⚠️ **Cross-compilation:** Bun's `--compile` creates binaries for the *host platform* by default
  - Building for all four targets requires running `bun build --compile --target=<platform>` separately for each
  - Recommend CI/release pipeline for multi-platform builds (cannot cross-compile from darwin to linux on local machine)
- ⚠️ **DNS cache:** Tool should either:
  - Auto-flush DNS cache after writes (requires sudo + platform detection)
  - Warn user to flush manually
  - Provide `hostie flush` subcommand

## External Dependencies & Formats

### Ink (React-based TUI framework)

**Note:** Design.md specified `@opentui/react`, but research shows this should be `ink` (the standard React-based TUI framework). `@opentui/react` does not exist as a package. Proceeding with `ink` as the TUI framework.

**Component Model:**
- Ink uses React components and JSX syntax for building terminal UIs
- Components render to terminal output instead of DOM
- Uses Yoga (Facebook's Flexbox engine) for layout in the terminal
- All elements are Flexbox containers by default
- Core components: `<Text>`, `<Box>`, `<Newline>`, `<Spacer>`, `<Static>`, `<Transform>`

**State Management:**
- Standard React hooks: `useState`, `useEffect`, `useContext`, etc.
- Ink-specific hooks: `useInput`, `useApp`, `useStdin`, `useStdout`, `useFocus`, `useCursor`
- `useInput(handler)` for keyboard input handling
- `useApp()` provides `exit()` method to terminate the app

**Event Handling:**
- Input events via `useInput(callback, options)`
- Callback receives `(input, key)` parameters
- Key object contains: `upArrow`, `downArrow`, `leftArrow`, `rightArrow`, `return`, `escape`, `ctrl`, `shift`, `meta`, etc.

**Known Gotchas:**
- All text must be wrapped in `<Text>` components (cannot render raw strings)
- `<Text>` cannot contain `<Box>` children (only text nodes and nested `<Text>`)
- Layout uses terminal rows/columns, not pixels
- App stays alive only while there's active work in event loop (timers, promises, stdin listeners)
- Must use `render()` from 'ink' package, not ReactDOM
- Requires Babel with React preset for JSX transformation

### YAML Library

**Recommendation:** `yaml` package by eemeli (https://github.com/eemeli/yaml)

**Features:**
- Supports both YAML 1.1 and YAML 1.2
- Zero external dependencies
- Works in Node.js and browsers
- TypeScript support (minimum TS 5.9)
- API: `parse(str)` and `stringify(value)`
- Handles comments and blank lines
- Passes all yaml-test-suite tests

**Usage:**
```typescript
import { parse, stringify } from 'yaml'
const data = parse(yamlString)
const yaml = stringify(data)
```

### ULID Library

**Recommendation:** `ulid` package (https://github.com/ulid/javascript)

**Features:**
- Lexicographically sortable unique identifiers
- 26-character string (vs UUID's 36 characters)
- Uses Crockford's base32 encoding
- Monotonic sort order support
- 128-bit compatibility with UUID
- Case insensitive, URL safe

**Usage:**
```typescript
import { ulid } from 'ulid'
const id = ulid() // "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// For monotonic IDs (same millisecond)
import { monotonicFactory } from 'ulid'
const ulid = monotonicFactory()
```

### /etc/hosts Format Specification

**RFC References:**
- RFC 952: DoD Internet Host Table Specification
- RFC 1123: Updates RFC 952 (relaxes first character to allow digits)

**Format Rules:**
- Each line: `<IP address> <hostname> [aliases...]`
- IP address: IPv4 dotted-decimal (e.g., `192.168.1.1`) or IPv6
- Hostname: up to 255 characters (63 per label)
- Characters: letters (a-z, A-Z), digits (0-9), hyphen (-), period (.)
- First character: letter or digit (RFC 1123 update)
- Last character: cannot be hyphen or period
- Case insensitive
- Comments: lines starting with `#`
- Blank lines: ignored

**Edge Cases:**
- Single-character names not allowed
- Hostnames with `-GATEWAY` or `-GW` suffix indicate gateway hosts
- Hostnames with `-TAC` suffix indicate TAC hosts (DoD)
- No spaces allowed within names
- Multiple aliases per line are space-separated

**Example:**
```
127.0.0.1       localhost
192.168.1.10    myserver.local myserver
# Comment line
::1             localhost
```

### Shell Completion

**Bun Support:**
- Bun does not have built-in shell completion generation
- Manual completion scripts required for bash/zsh/fish
- Common pattern: provide completion scripts in repo
- Completion scripts typically parse `--help` output or use hardcoded command lists

**Implementation Pattern:**
- Provide separate completion files: `completions/hostie.bash`, `completions/hostie.zsh`, `completions/hostie.fish`
- Document installation in README
- Use standard completion frameworks for each shell

### Known Limitations

**Ink:**
- No built-in form validation
- Terminal size changes require manual handling
- Limited to terminal capabilities (no mouse in all terminals)
- Performance can degrade with very large component trees

**YAML:**
- Large files can be memory-intensive
- No streaming parser in standard library

**ULID:**
- Requires monotonic factory for same-millisecond uniqueness
- Clock skew can affect sortability

**hosts file:**
- No standard for comments on same line as entries (implementation-dependent)
- IPv6 support varies by OS
- Maximum line length not standardized (typically 255-1024 chars)

## Dependency Analysis

### Production Dependencies

- **ink**: TUI framework (corrected from `@opentui/react` which does not exist). This is the primary UI framework for the terminal interface.

- **react**: Peer dependency for ink.

- **yaml**: YAML parsing and serialization for hosts file format. Alternatives: `js-yaml` (more popular but heavier), `yaml` (modern, spec-compliant, recommended).

- **ulid**: ULID generation for unique host identifiers. Alternatives: `ulidx` (faster), but `ulid` is the canonical implementation.

- **zustand**: State management as specified in design.md for managing application state in the TUI.

- **fuse.js**: Fuzzy search functionality. Alternatives: `fzf` (native binary, requires separate install), `fzy.js` (lighter but less features). Recommendation: `fuse.js` for pure JS portability and no external dependencies.

- **commander**: CLI argument parsing. Alternatives: `yargs` (more features, heavier), `cac` (lightweight), `clipanion` (type-safe). Recommendation: `commander` for maturity and simplicity.

### Development Dependencies

- **@types/react**: TypeScript definitions for React.

- **@types/node**: TypeScript definitions for Node.js APIs (even though using Bun, many APIs are compatible).

- **typescript**: TypeScript compiler (note: Bun has built-in TypeScript transpilation, but `tsc` may still be needed for type-checking in CI/development).

- **@typescript-eslint/eslint-plugin**: TypeScript-specific linting rules.

- **@typescript-eslint/parser**: ESLint parser for TypeScript.

- **eslint**: Code linting (optional but recommended for code quality).

- **prettier**: Code formatting (optional but recommended for consistency).

**Note on Bun built-ins:**
- Bun has a built-in test runner (`bun test`), so no need for Jest/Vitest
- Bun has built-in TypeScript transpilation, but keeping `typescript` as dev dependency for `tsc --noEmit` type checking is recommended
- Bun has built-in bundler, so no need for esbuild/webpack/rollup

## Key Findings

1. **TUI Framework Correction:** Design.md specified `@opentui/react`, but this package does not exist. The correct package is `ink`, the standard React-based TUI framework for Node.js/Bun.

2. **Sudo Handling:** All writes to `/etc/hosts` require sudo. The tool must implement graceful privilege escalation (D10 from design.md).

3. **DNS Cache Flushing:** After modifying `/etc/hosts`, DNS cache must be flushed for changes to take effect. This is platform-specific and requires sudo.

4. **Cross-Compilation Limitation:** Bun cannot cross-compile from darwin to linux. Multi-platform binaries require CI/release pipeline with platform-specific builds.

5. **Shell Completion:** Bun has no built-in completion generation. Manual completion scripts required for bash/zsh/fish.

6. **Fuzzy Search:** Recommend `fuse.js` over native `fzf` for portability (no external binary dependency).

7. **Type Checking:** Bun transpiles TypeScript but does not type-check. Add `bun x tsc --noEmit` to CI for strict validation.
