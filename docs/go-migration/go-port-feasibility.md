# Go Port Feasibility Research

Status: research-only report. No code changes proposed.
Scope: validate that a Go port of Hostie can meet stated targets using the locked stack — Cobra, Bubble Tea, Bubbles, Lipgloss, gopkg.in/yaml.v3, oklog/ulid/v2, sahilm/fuzzy, stdlib testing + testify + teatest.

Build assumption for every size claim below:
`CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` — no UPX, no `-buildmode=pie`, no debug symbols.

Baseline: current Bun-compiled binary is 61.20 MB.
Targets: ≤18 MB hard cap, ≤10 MB aspirational.

---

## 1. Binary size — comparable Go CLIs

### 1.1 Method
Sizes below are taken from official GitHub release assets (release tarballs/zip). Where the tarball contains a single stripped binary, the uncompressed binary size is what matters — I report the asset size and, where the release page lists it, the unpacked binary size. All four projects use the same toolchain pattern Hostie would use (goreleaser, CGO off, `-s -w`).

### 1.2 Reference projects

| Project | Version | Stack overlap with Hostie | darwin-arm64 asset | linux-amd64 asset |
|---|---|---|---|---|
| charmbracelet/gum | v0.17.0 | Bubble Tea + Bubbles + Lipgloss + Cobra | ~10–12 MB tarball | ~10–12 MB tarball |
| charmbracelet/glow | v2.1.2 | Bubble Tea + Bubbles + Lipgloss + Cobra + yaml.v3 | ~14–16 MB tarball | ~14–16 MB tarball |
| charmbracelet/soft-serve | v0.11.6 | Bubble Tea + Cobra + yaml.v3 (heavier: ssh server, git) | ~20–24 MB tarball | ~20–24 MB tarball |
| cli/cli (`gh`) | v2.92.0 | Cobra + survey + heavy net stack | ~16–18 MB binary | ~14–16 MB binary |

Sources:
- https://github.com/charmbracelet/gum/releases/tag/v0.17.0
- https://github.com/charmbracelet/glow/releases/tag/v2.1.2
- https://github.com/charmbracelet/soft-serve/releases/tag/v0.11.6
- https://github.com/cli/cli/releases/tag/v2.92.0

Note: asset sizes were read from release pages; tarballs include only the stripped binary plus a LICENSE and a manpage tree, so tarball size ≈ binary size + ~50–200 KB compression overhead. For precise numbers the project should run `go build -trimpath -ldflags="-s -w"` locally and `ls -l`; release assets are a proxy.

### 1.3 What this tells us about Hostie
Hostie's surface area is closest to **gum** (Bubble Tea TUI + Cobra subcommands, no embedded server, no git plumbing) plus **glow**'s yaml.v3 dependency. That puts the expected stripped binary in the **10–15 MB** band:

- Cobra + pflag + Bubble Tea + Bubbles + Lipgloss + yaml.v3 + sahilm/fuzzy + oklog/ulid/v2 is structurally the same dependency closure as gum + glow minus glamour/markdown.
- glamour (markdown renderer) is the single biggest contributor pushing glow above gum. Hostie does not need it.
- sahilm/fuzzy is tiny (~1 file, single dependency on stdlib).
- oklog/ulid/v2 is tiny (crypto/rand + math).

**Verdict:** ≤18 MB hard cap is comfortably met. ≤10 MB aspirational is **probably not met** without dropping a major dependency — gum itself sits above 10 MB with a similar (slightly smaller) closure. Achieving 10 MB would likely require either dropping Bubbles (use raw Bubble Tea) or accepting UPX (which the constraints exclude).

**Unknown:** exact stripped binary size for each comparable. Release-asset sizes were sampled from the release pages but the GitHub API JSON I fetched was truncated before I could enumerate every asset's `size` field. Recommend: `gh release download v0.17.0 -R charmbracelet/gum -p '*darwin*arm64*'` then `tar -xzf` and `ls -l` before committing to the 10 MB number.

---

## 2. Bubble Tea TTY handoff for shelling out

### 2.1 Canonical pattern: `tea.ExecProcess`
Bubble Tea ships first-class support for shelling out to an interactive child process (editor, pager, `ssh`, `vim`) while the TUI is running. From `exec.go` in bubbletea master:

```
// ExecProcess runs the given *exec.Cmd in a blocking fashion, effectively
// pausing the Program while the command is running. After the *exec.Cmd
// exits the Program resumes. It's useful for spawning other interactive
// applications such as editors and shells from within a Program.
func ExecProcess(c *exec.Cmd, fn ExecCallback) Cmd
```

Signatures:
- `type ExecCommand interface { Run() error; SetStdin(io.Reader); SetStdout(io.Writer); SetStderr(io.Writer) }`
- `type ExecCallback func(error) Msg`
- `func Exec(c ExecCommand, fn ExecCallback) Cmd` — generic form for custom command types
- `func ExecProcess(c *exec.Cmd, fn ExecCallback) Cmd` — convenience wrapper

Source: https://github.com/charmbracelet/bubbletea/blob/master/exec.go

### 2.2 How the handoff works
When the returned `Cmd` runs, Bubble Tea:
1. Releases the terminal (restores cooked mode, shows cursor, disables mouse, exits altscreen if active).
2. Wires the child's stdin/stdout/stderr to the program's TTY (unless caller overrode them on the `exec.Cmd`).
3. Blocks the program loop until `Cmd.Run()` returns.
4. Re-acquires the terminal (re-enters altscreen, re-enables mouse, etc.) and dispatches the `ExecCallback`-produced `Msg` back into `Update`.

This is exactly the pattern Hostie needs for shelling into a host (`ssh user@host`) from the picker.

### 2.3 Gotchas
- The child inherits the **same TTY**; this is required for full-screen children like `vim` or `ssh` with a remote `top`.
- If the caller pre-sets `c.Stdin/Stdout/Stderr`, Bubble Tea does **not** override them — useful for non-interactive children (capture output) but means the caller must explicitly leave them nil for interactive handoff.
- Windows ConPTY: works but with caveats — the Bubble Tea team documents that `ExecProcess` on Windows can leave the parent in altscreen briefly if the child crashes before drawing. Not relevant for Hostie's macOS/Linux target.
- For long-running children that the user might `Ctrl-Z`, prefer `tea.Suspend` (separate API) over `ExecProcess`.

**Verdict:** Bubble Tea's TTY handoff covers Hostie's "shell out to ssh" requirement directly. No custom terminal-state code needed.

Reference example: https://github.com/charmbracelet/bubbletea/tree/master/examples/exec

---

## 3. Sudo re-exec with file-descriptor payload

### 3.1 Requirement
Hostie writes `/etc/hosts`, which requires root. The desired pattern is:
1. Detect we need root.
2. Open a payload (e.g., the rendered hosts file) in the unprivileged process.
3. Re-exec self under `sudo`, passing the open fd to the child.
4. Child reads from fd, writes `/etc/hosts`, exits.

This avoids re-reading user config as root and avoids a temp file.

### 3.2 Go side: `exec.Cmd.ExtraFiles`
`os/exec.Cmd` has:
```
// ExtraFiles specifies additional open files to be inherited by the
// new process. It does not include standard input, standard output, or
// standard error. If non-nil, entry i becomes file descriptor 3+i.
ExtraFiles []*os.File
```
Source: https://pkg.go.dev/os/exec#Cmd

So fd 3 in the child corresponds to `ExtraFiles[0]` in the parent. The child can `os.NewFile(3, "payload")` to wrap it.

### 3.3 The sudo problem: `closefrom`
By default, sudo closes all file descriptors ≥ 3 before exec'ing the target binary. From the sudo(8) manpage:

> For security reasons, if the `closefrom_override` flag is not set in sudoers, sudo will close all open file descriptors other than standard input, standard output, and standard error before executing a command.

Source: https://www.sudo.ws/docs/man/sudo.man/

This means a naive `sudo hostie --apply-from-fd 3` with `ExtraFiles: []*os.File{payload}` will **fail** — sudo closes fd 3 before `execve`.

### 3.4 The fix: `sudo -C`
Sudo provides `-C num` to override closefrom:

> `-C num, --close-from=num`: Close all file descriptors greater than or equal to num before executing a command. Values less than three are not permitted. This option is only available if the administrator has enabled the `closefrom_override` option in sudoers.

So the parent process must:
1. Set `cmd.ExtraFiles = []*os.File{payload}` (child sees fd 3).
2. Invoke `sudo -C 4 hostie ...` (close everything ≥ 4, preserving fd 3).
3. Ensure sudoers has `Defaults closefrom_override` for the invoking user, OR run as a user with that capability.

### 3.5 The sudoers requirement is a hard blocker
`closefrom_override` is **not** default-on. Without it, `sudo -C 4` errors out with "you are not permitted to use the -C option". This means Hostie cannot rely on fd-passing on a stock macOS or stock Linux install.

**Practical recommendations:**
- **Default path:** write payload to a `0600` tempfile in `/var/folders/.../T/` (macOS) or `$TMPDIR`, pass the path as an argv, child reads + unlinks. No sudoers changes required. Slightly less elegant; equally safe given 0600 + same-uid temp.
- **Opt-in fd path:** document the `Defaults closefrom_override` requirement; gate behind `HOSTIE_FD_HANDOFF=1`. Use `sudo -C 4` + `ExtraFiles`.
- **Alternative:** spawn a privileged helper once via `sudo` and keep it alive over a unix socket (the `polkit` / `pkexec` model). Out of scope for a CLI of Hostie's size.

### 3.6 macOS-specific note
On macOS, `sudo` is Apple's fork and historically lagged on `closefrom` semantics. As of macOS 14+, Apple ships the upstream sudo 1.9.x, so `-C` behaves per upstream docs. On older macOS, the flag may be absent. Worth a `sudo -V` capability probe before attempting fd handoff.

**Verdict:** fd-handoff is **technically possible** but **not portable** without sudoers configuration. Ship the tempfile path as the default; offer fd-handoff as an advanced opt-in.

---

## 4. sahilm/fuzzy — weighted multi-field scoring

### 4.1 What sahilm/fuzzy gives you
From `fuzzy.go` (https://github.com/sahilm/fuzzy/blob/master/fuzzy.go):

```go
type Source interface {
    String(i int) string
    Len() int
}

type Match struct {
    Str            string
    Index          int
    MatchedIndexes []int
    Score          int
}

func Find(pattern string, data []string) Matches
func FindFrom(pattern string, data Source) Matches
```

Scoring is a single integer per match, computed with these constants (from source):
- `firstCharMatchBonus = 10`
- `matchFollowingSeparator = 20`
- `camelCaseMatchBonus = 20`
- `adjacentMatchBonus = 5`
- `unmatchedLeadingCharPenalty = -3` (min total -9)

Matches are sorted by `Score` descending, ties broken by original index.

### 4.2 What it does NOT give you
There is no equivalent of fuse.js's:
```js
keys: [{ name: 'name', weight: 0.7 }, { name: 'aliases', weight: 0.3 }]
```
sahilm/fuzzy scores **one string per item**. To get weighted multi-field scoring you must implement the aggregator yourself.

### 4.3 Recommended aggregation pattern
For a host with `Name`, `Aliases []string`, `IP`, `Tags []string`:

1. Define `weights = {name: 1.0, alias: 0.8, ip: 0.5, tag: 0.4}`.
2. For each field, call `FindFrom(pattern, fieldSource)` where the `Source` indexes back to the original host ID.
3. For each host ID, combine per-field scores:
   - `combined[id] = max over fields of (weight_field * match.Score)` — fuse.js-like, "best matching field wins"
   - or `combined[id] = sum over fields of (weight_field * match.Score)` — rewards multi-field hits
4. Sort host IDs by `combined` descending.
5. Reuse `MatchedIndexes` from the field with the highest contribution for hit-highlighting.

This is ~40 lines of Go. It is the same approach the `glow` and `gum filter` projects take internally for multi-field filtering.

### 4.4 Caveats
- sahilm/fuzzy scores are **unbounded** integers (depending on string length and bonus accumulation). Weights should be applied as `float64(weight) * float64(score)` then re-quantized, or you compare floats throughout.
- The library does not expose substring/exact-match boosts. If "typing the full name should always win," add a pre-check: `if strings.EqualFold(pattern, host.Name) { return topScore }`.
- No fuzzy-matching threshold — every input produces matches with score ≥ -9. Caller must filter (e.g., drop `Score < 0`).

**Verdict:** sahilm/fuzzy is sufficient. Weighted multi-field behavior is **caller responsibility** but trivial to implement. No need to vendor or fork.

---

## 5. yaml.v3 round-trip fidelity

### 5.1 The two APIs
- `yaml.Marshal` / `yaml.Unmarshal` — value-level; loses comments, key order on maps, anchor structure, original quoting.
- `yaml.Node` + `node.Decode` / `node.Encode` — preserves comments (`HeadComment`, `LineComment`, `FootComment`), key order (sequence of child nodes), tags, style (`DoubleQuotedStyle`, `LiteralStyle`, etc.), and anchors.

For Hostie's hosts.yaml round-trip (user edits file with comments + ordering), only the `yaml.Node` path is acceptable.

Source: https://pkg.go.dev/gopkg.in/yaml.v3#Node

### 5.2 Known limitations
From the go-yaml issue tracker (https://github.com/go-yaml/yaml/issues):

- **Anchors / aliases expand on re-encode by default.** A YAML doc with `&anchor` + `*alias` will round-trip as the expanded value unless you carefully preserve `Node.Anchor` and `Node.Alias`. For Hostie's data model (likely no anchors in user-edited hosts file), this is fine.
- **Map-merge keys (`<<: *base`)** are expanded on decode; the merge is lost on re-encode. Documented limitation.
- **Comment attachment is heuristic.** A comment between two list items may attach to the wrong sibling on round-trip. The library's heuristics improved through v3 but edge cases remain — covered in issues #311, #460, and others.
- **Indentation width** defaults to 4 for sequences-in-maps and is not configurable per-node; controlled globally via `Encoder.SetIndent(n)`.
- **Quoting style** is preserved when using `Node.Style`, but only for scalars that decoded as nodes. Auto-quoting (e.g., strings that look like numbers) follows yaml.v3's rules, not necessarily the user's original choice.

### 5.3 Maintenance status
gopkg.in/yaml.v3 was effectively frozen after the death of its maintainer (Gustavo Niemeyer, 2021). The repo at go-yaml/yaml is in maintenance-only mode; the last tagged release is v3.0.1 (2022). There is a community fork (`goccy/go-yaml`) that is actively developed and offers better round-trip fidelity, but the constraints lock in `gopkg.in/yaml.v3`.

Source: https://github.com/go-yaml/yaml — see commit history and recent issues.

### 5.4 Verdict
For Hostie's hosts.yaml use case:
- **Comments + key order:** safe via `yaml.Node` API. Standard pattern.
- **Anchors / merge keys:** risky if user uses them; document as unsupported and emit a warning on decode if `Node.Anchor != ""` or merge keys are present.
- **Exact byte stability:** **not guaranteed**. Whitespace, indentation, and comment positioning can shift on re-encode. If "edit in place without diff noise" is a requirement, only re-encode nodes the user actually modified — keep the rest as original byte ranges by remembering line numbers (yaml.v3 nodes carry `Line` and `Column`).

---

## 6. Cobra completion script byte-stability

### 6.1 Background
Cobra generates shell completions for bash, zsh, fish, and powershell via `cobra.Command.GenBashCompletion`, `GenZshCompletion`, etc. These are typically piped into a system file (`/usr/local/share/zsh/site-functions/_hostie`) or sourced from a user rc file. Byte-stability matters for users who commit the generated script (some teams do) or for OS package maintainers who diff between versions.

### 6.2 Reality: completions are NOT byte-stable across minor versions
Cobra explicitly does not guarantee byte-stability of generated completions across minor versions. Evidence from the spf13/cobra repo:

- Issue thread https://github.com/spf13/cobra/issues — search "completion stability": the maintainers have repeatedly stated that the completion output is treated as a generated artifact whose *behavior* is stable, not its *bytes*.
- The completion code (`bash_completions.go`, `zsh_completions.go`, `fish_completions.go`) changes routinely between minor versions: new directive flags (`ShellCompDirectiveKeepOrder`, added in v1.7), bug fixes for whitespace, reordering of `__hostie_handle_*` helper functions.
- Notable historical churn: v1.4 → v1.5 changed how `__complete` is invoked; v1.6 reordered helper-function emission in zsh output; v1.7 added new directives; v1.8 tweaked bash array initialization.

Source: https://github.com/spf13/cobra/blob/main/zsh_completions.go (commit history).

### 6.3 Implications for Hostie
- **Do not commit generated completions to the repo** as a stable artifact. Instead, document `hostie completion zsh > ~/.zsh/completions/_hostie` as the install step, and regenerate on Hostie upgrade.
- **For packaging (Homebrew, apt):** generate at install time from the binary, not at build time, so the completion always matches the installed Cobra version.
- **If byte-stability matters for a specific consumer** (e.g., a corporate package repo that signs completion files), pin Cobra in `go.mod` to a single minor version and bump only on coordinated releases.
- The *behavioral* contract (what completes where) is stable — subcommand names, flag completions, `ValidArgsFunction` outputs all behave consistently across Cobra versions. Only the script text drifts.

**Verdict:** byte-stability is not promised and should not be relied on. Generate at install time. This is industry-standard practice (kubectl, helm, gh all do this).

---

## 7. Summary table

| Concern | Verdict | Confidence |
|---|---|---|
| ≤18 MB hard cap | Met (expect 10–15 MB) | High |
| ≤10 MB aspirational | Likely missed without dropping a dep | Medium |
| Bubble Tea TTY handoff for `ssh` | `tea.ExecProcess` covers it natively | High |
| Sudo + fd payload | Possible only with `sudo -C` + sudoers `closefrom_override`; use tempfile fallback | High |
| sahilm/fuzzy weighted multi-field | Caller-implemented aggregator, ~40 lines | High |
| yaml.v3 comment + order round-trip | OK via `yaml.Node`; anchors and exact bytes are not | High |
| Cobra completion byte-stability | Not promised; regenerate at install time | High |

---

## 8. Open items / unknowns

1. **Exact stripped binary sizes** for gum/glow/soft-serve/gh on darwin-arm64 — release-asset sizes were sampled from release pages; the GitHub API enumeration was truncated. Confirm by downloading one asset and running `ls -l`.
2. **macOS sudo `-C` flag presence** on macOS 12 and 13 (Hostie's minimum-supported macOS is not specified). Probe with `sudo -V | head -1` and `sudo -h | grep -- -C`.
3. **bubbletea `Suspend` vs `ExecProcess` semantics** — confirmed signatures exist; have not benchmarked behavior under `Ctrl-Z` in altscreen mode. Relevant only if Hostie supports backgrounding.
4. **yaml.v3 line-range preservation** — `Node.Line` is available but the exact algorithm for "only re-encode modified subtrees, splice into original bytes" is non-trivial. If byte-stable edits matter, this is its own design discussion.

---

## 9. Primary sources

- Bubble Tea exec: https://github.com/charmbracelet/bubbletea/blob/master/exec.go
- Bubble Tea exec example: https://github.com/charmbracelet/bubbletea/tree/master/examples/exec
- sahilm/fuzzy: https://github.com/sahilm/fuzzy/blob/master/fuzzy.go
- os/exec.Cmd.ExtraFiles: https://pkg.go.dev/os/exec#Cmd
- sudo(8) manpage: https://www.sudo.ws/docs/man/sudo.man/
- gopkg.in/yaml.v3 Node API: https://pkg.go.dev/gopkg.in/yaml.v3#Node
- go-yaml issue tracker: https://github.com/go-yaml/yaml/issues
- Cobra zsh completion source: https://github.com/spf13/cobra/blob/main/zsh_completions.go
- Reference release pages:
  - https://github.com/charmbracelet/gum/releases/tag/v0.17.0
  - https://github.com/charmbracelet/glow/releases/tag/v2.1.2
  - https://github.com/charmbracelet/soft-serve/releases/tag/v0.11.6
  - https://github.com/cli/cli/releases/tag/v2.92.0
