# Discovery — go-migration

**Feature:** go-migration
**Phase:** writing-plans → discovery
**Date:** 2025-11-22

---

## Institutional Learnings

Read in full: `docs/learnings/critical-patterns.md` + `docs/learnings/20260525-hostie-phase-1b.md`. Key insights directly relevant to the Go port:

1. **Atomic file replacement requires lstat + same-FS tempdir + permission clamping.** v1 uses `statSync` (follows symlinks) — must switch to `lstat` in Go. v1 also uses `mkdtempSync(tmpdir())` (cross-FS, EXDEV risk on Linux) — must switch to `os.CreateTemp(filepath.Dir(target), …)` to stay on the same filesystem as `/etc/hosts`. Perms must be clamped `& 0o0777 & ~0o022`, not just `& 0o7777`.
2. **Smoke tests target the compiled binary, not source.** Bun's `--compile` changed argv shape and module resolution; v1 reexec broke twice because tests ran against source. Go equivalent: integration tests must `go build` then exec the binary, never `go run` from source. Release CI smokes each cross-compiled artifact.
3. **Three-level artifact verification (exists / substantive / wired).** v1 Phase 1A shipped components that passed tests but were never wired into the integration root (`cli/index.ts` dispatched nothing; `tui/index.tsx` rendered nothing). For every D-id, verify L1+L2+L3.
4. **Hands-on UAT is non-negotiable.** Five-specialist review returned PASS_WITH_P2; UAT then found 5 P1s in 20 minutes (permission clamp, missing TUI keys, missing `/` handler, too-fuzzy search defaults, broken sudo reexec). Walk every decision manually.
5. **One renderer, one parser.** v1 already has two `extractManagedBlock` implementations (strict in `apply.ts`, lenient in `etchosts.ts`) and two YAML serialization call sites with different options. The Go port collapses both to single sources.
6. **Parameterize malformed-input tests** with table-driven `t.Run` subtests in Go (`test.each` equivalent). v1 has exactly one such table (`tests/core/apply.test.ts:243`, 5 cases). Port verbatim; add cases freely.

---

## Architecture Snapshot (Agent A findings — verbatim summary)

Full report retained in session memory. Key facts the planner needs:

### CLI surface (commander, in `src/cli/index.ts`)
| Command | Args | Options | Handler |
|---|---|---|---|
| (empty) | — | — | `renderTUI` |
| `add <ip> <hostname>` | ip, hostname | `--group <path>`, `--alias` (repeatable), `--disabled`, `--comment` | `addCommand` |
| `rm <target>` | hostname | — | `rmCommand` |
| `enable <hostname>` | hostname | — | `enableCommand` |
| `disable <hostname>` | hostname | — | `disableCommand` |
| `list` | — | `--group` (parsed-but-unused), `--json` | `listCommand` |
| `apply` | — | `--dry-run` | `applyCommand` |
| `group add <path>` | path (slash-separated) | — | `groupCreateCommand` (splits into parent + leaf) |
| `group rm/list/mv` | — | — | **unimplemented** in v1 (returns "not implemented yet", exit 1) |
| `completion <shell>` | bash/zsh/fish | — | hardcoded scripts (`grep` + `cut`/`string match`-based) |
| `version` | — | — | reads `package.json` (drift from parser's hardcoded `0.1.0`) |

### Exit codes (src/cli/exit-codes.ts)
`SUCCESS=0`, `VALIDATION=1`, `IO_ERROR=2`, `PERMISSION=3`.

### Marker block (literal strings, replicated in 3 files)
`# BEGIN HOSTIE` / `# END HOSTIE`. On-disk format from `apply.ts`: `${BEGIN}\n${block}\n${END}` (no padding). Dry-run preview from `render.ts`: `# BEGIN HOSTIE\n\n${content}\n\n# END HOSTIE` (padded, divergent — only used for preview). Per-group header: `# group: <path>`. Disabled entries: `# <ip> <hostname> …`.

### YAML config
`src/core/yaml.ts`: `YAML.stringify(data, { indent: 2, lineWidth: 0, minContentWidth: 0 })`. **Inconsistency to fix in Go port**: `src/core/file-io.ts:writeHostsFile` calls `yaml.stringify(data)` with library defaults (no options) and `readHostsFile` parses without schema validation. Go port must funnel all `~/.hosts` I/O through the one `yaml` package.

### Fuse.js config (src/tui/hooks/useSearch.ts)
```js
{
  keys: [
    { name: "entry.hostname",  weight: 2   },
    { name: "entry.aliases",   weight: 1.5 },
    { name: "entry.ip",        weight: 1   },
    { name: "groupPathString", weight: 0.5 },
  ],
  threshold: 0.3,
  includeScore: true,
  minMatchCharLength: 2,
  ignoreLocation: true,
}
```
Results sliced to top 10.

### TUI keybindings (normal mode, src/tui/hooks/useKeyboard.ts)
`/` search • `Tab` swap focus • `j`/`k` nav (wrapping) • `Space` toggle+persist • `d` delete (with confirm)+persist • `Enter`/`Ctrl+S` apply (with confirm) • `?` help • `q` quit (confirm if dirty) • `a` add modal • `e` edit modal • `g` group-create modal • `m` move-to-group modal.

Search mode: `Esc` clears, `Enter` keeps query as filter, `Backspace`/`Delete` pop char, printable chars append.

### Domain validators (src/domain/validators.ts)
- Hostname: RFC 952/1123 — max 255; label 1–63; first+last alphanumeric; body `[a-zA-Z0-9-]`; no `..`; no leading/trailing `.`.
- IPv4: 4 octets 0–255, no leading zeros (except `0`).
- IPv6: rejects `:::`, rejects multiple `::`, exactly 8 groups when uncompressed, each group 1–4 hex.
- `validateNoDuplicates`: enabled-only, case-insensitive, hostname+alias collision.

### Atomic write (src/core/etchosts.ts:writeEtcHosts)
- `statSync` (NOT lstat — bug to fix in Go port)
- `mode = (stat.mode & 0o0777) & ~0o022`
- `mkdtempSync(join(tmpdir(), "hostie-"))` (cross-FS — bug to fix; use `dirname(target)`)
- `writeFileSync` → `chmodSync(originalMode)` → `chownSync(uid, gid)` (swallow EPERM) → `renameSync`
- On error: best-effort `unlinkSync(tempFile)`

### Sudo re-exec (src/core/apply.ts:reexecWithSudo, v1)
- Triggers on `EACCES` from `writeEtcHosts`
- `argv0 = realpathSync(process.execPath)` (NOT `Bun.argv[0]` — virtual `/$bunfs/root/…` path won't exec under sudo)
- `Bun.spawn(["sudo", argv0, ...Bun.argv.slice(2)], { stdio: "inherit" })`
- Errors if already root

### Test count
**~424 tests** total — 102 under `tests/`, 322 co-located under `src/**/__tests__/`. Note: design.md mentioned 432, which was the count after Phase 1B. Discrepancy ≤8, doesn't change planning.

### Two block-rendering shapes (potential bug locus)
`render.ts:wrapManagedBlock` pads with blank lines. `apply.ts:renderManagedBlock` does not. They are used in different paths (preview vs disk). Go port should produce **one shape** (the disk shape), use it for preview too, and update the existing v1 test if it depends on padding. Document as `--dry-run` output change in 2.0.0 release notes.

---

## Constraints + Reusable Patterns (Agent B findings — verbatim summary)

Full report in session memory. Key facts:

### Toolchain (current)
- No Bun version pin (CI uses `latest`).
- TypeScript ^5.7.3, `strict: true`, `target/module: esnext`, `moduleResolution: bundler`.
- No lint/format config (no biome, eslint, prettier).
- No vitest config; tests are `bun:test`.
- CI: `.github/workflows/ci.yml` matrix `[ubuntu-latest, macos-latest]` — typecheck, test, build. Build artifact uploaded with `actions/upload-artifact@v4`.
- Release: tag `v*` → 4-target Bun build matrix → SHA-256 → `softprops/action-gh-release@v2`. No code signing, no Homebrew, no npm publish.

### Atomic write — exact code path (cited in Architecture section above)
Lives in `src/core/etchosts.ts:writeEtcHosts`. Test file: `src/core/__tests__/etchosts.test.ts` (19 cases) and `tests/core/apply.test.ts` (14 cases). Mock pattern uses `spyOn(fs, …)` extensively — Go port can't replicate this and must use real tempdirs for tests (no fs mock layer in stdlib).

### Test mocking (key patterns to replace in Go)
- `~/.hosts` redirection: v1 uses `spyOn(fileIo, "readHostsFile")` + `redirectsToTempPath` because `os.homedir()` caches in Bun. **In Go this is trivial** — `os.UserHomeDir()` reads `HOME` env at call time; set `HOME=<tmpdir>` in the test and the redirect is automatic.
- `/etc/hosts`: v1 spies on all `fs` operations. **In Go**, parametrize `etcHostsPath` (default `"/etc/hosts"`) and tests pass a tempdir path. The `__apply-privileged` codepath only runs when `etcHostsPath == "/etc/hosts"` AND `os.Access` fails — tests stay in unprivileged mode.
- `test.each` → Go `t.Run` with a slice of struct fixtures.

### Release workflow rewrite scope
Full workflow quoted in session memory. Need to swap the Bun setup + 4-target build for `actions/setup-go@v5` + `GOOS`/`GOARCH` matrix; everything else (checkout, rename, SHA-256, upload, release) is reusable as-is.

---

## External Research (Agent C findings — full report at `docs/go-migration/go-port-feasibility.md`)

Summary table (from report §7):

| Concern | Verdict | Confidence |
|---|---|---|
| ≤18 MB hard cap | Met (expect 10–15 MB) | High |
| ≤10 MB aspirational | Likely missed without dropping a dep | Medium |
| Bubble Tea TTY handoff (sudo) | `tea.ExecProcess` covers it natively | High |
| Sudo + fd payload (original D12) | Not portable without sudoers `closefrom_override` — **D12 amended** | High |
| sahilm/fuzzy weighted multi-field | Caller-implemented aggregator, ~40 lines | High |
| yaml.v3 comment + order round-trip | OK via `yaml.Node`; anchors + exact bytes are not | High |
| Cobra completion byte-stability | Not promised; regenerate at install time | High |

### Decisions surfaced for the planner
- **D12 amended** in design.md: `0600` tempfile path replaces fd-passing. ✅ resolved.
- **D2 aspirational ≤10 MB:** research says likely missed without dropping Bubbles. Recommend: pursue 10 MB only if it can be hit without dropping deps; otherwise accept the 10–15 MB band and document the actual size in the 2.0.0 release notes.
- **yaml.v3 round-trip:** byte-stable round-trip is NOT a realistic goal. The parity contract (D15) already says "existing `~/.hosts` files round-trip" — interpret as *semantic* (parse→serialize→parse fixed point), not *byte-stable*. Add an explicit note to the parity harness so it doesn't fail on whitespace.
- **Cobra completions:** snapshot-testing the script bytes is fragile across Cobra minor versions. Instead, snapshot the **behavior** (run completion against fixture inputs, assert outputs) and pin Cobra to a specific minor version in `go.mod`.
- **sahilm/fuzzy weighting:** confirmed implementable; ~40 LOC aggregator in `go/internal/tui/search/weighted.go`. Spike score-parity against fuse.js fixture corpus as HIGH-risk in validating.
- **Bubble Tea + sudo:** `tea.ExecProcess` is the canonical primitive. No custom TTY handoff code needed. Reduces Phase 4 risk significantly.

### Open items for spikes / planning
1. Confirm stripped binary size for `gum` darwin-arm64 via `gh release download` + `ls -l` — turns the 10–15 MB band into a single number.
2. macOS `sudo -V` capability probe — only relevant for the (now-rejected) fd path; **moot after D12 amendment**.
3. `tea.Suspend` semantics under `Ctrl-Z` in altscreen — only relevant if we add backgrounding (not in scope per D10).

---

## Open Questions for the Planner

None block decomposition. All are deferred-to-validation spikes:

- [ ] **Score-parity for weighted fuzzy** — fuse.js vs sahilm/fuzzy ranking on a fixed query corpus (top-5 order). Spike in validating.
- [ ] **YAML round-trip semantics** — confirm parse→serialize→parse fixed point on the v1 fixture corpus. Define exact equality predicate (deep struct equality, not byte equality).
- [ ] **Sudo re-exec + Bubble Tea TTY** — `tea.ExecProcess` smoke against macOS Terminal, iTerm2, Alacritty. HIGH-risk spike.
- [ ] **`__apply-privileged` threat model** — write up the input-validation contract: subcommand only accepts a tempfile path argument that (a) lives under `$TMPDIR`, (b) is owned by `getuid()` of the parent (passed as a second argv for verification), (c) is `0600`, (d) is unlinked on every exit path. Document in `docs/go-migration/threat-model.md` during validating.
