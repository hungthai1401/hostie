# Critical Patterns

Promoted learnings that apply broadly across features. Append only — never overwrite.

## Atomic File Replacement Requires lstat + Same-FS Tempdir + Permission Clamping (from hostie, 2026-05-25)
Privileged file writes via temp+rename must use `lstatSync` (not `statSync` — don't follow symlinks into other files), create the temp file in `dirname(target)` (not `os.tmpdir()` — avoid EXDEV cross-filesystem failures on Linux), and *clamp* preserved modes with `& 0o0777 & ~0o022` (not just `& 0o7777` — drop world-write, setuid, setgid, sticky). For privileged files, "preserve perms" is the wrong default; always actively normalize.

## Smoke Tests Target the Compiled Binary, Not the Source (from hostie, 2026-05-25)
`bun build --compile` changes module resolution (virtual `/$bunfs/root/...` paths), argv shape (extra virtual script element between exec and user args), and runtime FS layout. Tests against source (`bun run src/index.ts`) will pass while the compiled binary fails on real users. Always exercise the compiled artifact, and always force rebuild — never `if (!exists(binary)) build()` shortcuts that silently reuse stale builds.

## Three-Level Artifact Verification Is Non-Negotiable (from hostie, 2026-05-25)
For every locked decision in design.md: (L1) does the file/component exist, (L2) is it substantive (not a stub or `return null`), (L3) is it imported and called by the integration layer (CLI dispatch / TUI composition root / route handler)? Skipping L3 ships entire features as no-ops. Beads passing tests + reviews can still be fully unwired.

## Hands-On UAT Finds What Automated Review Misses (from hostie, 2026-05-25)
A 5-specialist code review with PASS verdicts is not a substitute for human UAT against design.md decisions. UAT routinely finds: wired-but-default-misconfigured behavior (search too fuzzy), wired-but-unkeybound features (modals with no entry path), permission/environment surprises (system file shipping world-writable), and toolchain-specific runtime breakage (compiled-binary argv). Walk every D-id; do not skip the human gate.

## One Renderer, One Parser — Share or Pin (from hostie, 2026-05-25)
Any custom file-format logic (managed blocks, config sections, fenced regions) must have exactly one serializer and one parser. If duplication is unavoidable for layering reasons (e.g., a strict and a lenient variant), pin both with a conformance test that feeds identical fixtures through both and asserts agreement. Otherwise they will drift silently and the two paths will write/read mutually incompatible artifacts.

## Parameterize Malformed-Input Tests with `test.each` (from hostie, 2026-05-25)
When a spec enumerates N invalid input shapes that must be rejected, encode all N as a parameterized table at the highest integration layer available (not just at the unit-parser layer). Asserting only a subset leaves a future refactor free to silently break the others while every test still passes.

## GitHub Actions Context Values Are Untrusted Input — Pass via `env:`, Never Interpolate (from hostie go-migration Phase 1, 2026-05-25)
Any time a GitHub Actions context value (`github.ref_name`, `github.head_ref`, `github.event.*`, PR titles/bodies, issue text, branch names) flows into a `run:` block, default to passing it through `env:` and referencing `$VAR` in the shell — *never* interpolate `${{ ... }}` directly into shell text. An attacker controls tag/branch names and PR metadata; a tag literal like `v1.0.0; rm -rf /` becomes shell on a workflow holding `contents: write`. The `env:` indirection sandboxes the value as a single argument. Apply universally to release pipelines, even when "we control the tags" — repos get forked, permissions get widened, conventions get forgotten.

## Migration-Era CI Has Two Surfaces — Application Contracts AND Cutover Contracts (from hostie go-migration Phase 1, 2026-05-25)
During any port/migration that ships old + new in parallel (Bun + Go, JS + WASM, REST + gRPC), CI has two independent contract surfaces: (1) the **application contract** — does the new code build, test, vet — and (2) the **cutover contract** — release-only jobs that gate publication of the new artifact. Both need explicit `if:` gates from day one. Release-only jobs without a tag gate (`if: startsWith(github.ref, 'refs/tags/v')`) burn CI minutes producing artifacts no one downloads, *and* fail to block release if their job isn't in `release.needs:`. Wire both from the first commit, not when the cost or risk becomes obvious.

## Three-Level Artifact Verification Extends to CI Jobs and Dependency Imports (from hostie go-migration Phase 1, 2026-05-25)
The three-level verification (L1 exists / L2 substantive / L3 wired) applies to more than source files. **CI jobs**: L1 the job exists in the workflow, L2 its steps actually do work (not `echo "TODO"`), L3 it's referenced in `needs:` by every dependent job that gates on it. **Dependency manifests**: L1 the dep is in `go.mod`/`package.json`, L2 it's pinned (not a floating range), L3 it's actually `import`/`require`d somewhere in code that ships — otherwise size measurements and security audits both lie. Decorative deps and orphan CI jobs pass file-existence reviews but produce a system that misbehaves at runtime or release time.

## Privilege-Boundary Re-Derivation — Never Shape Payloads Upstream of Sudo (from hostie go-migration Phase 4, 2026-05-26)
Anything the privileged child does (merge, validate, normalize, atomic-write) must re-derive from minimal inputs read under root. Upstream (unprivileged) processes hand off only marker-delimited payloads with strict invariants — exactly-one BEGIN, exactly-one END, BEGIN-before-END, no preamble/suffix — enforced by the privileged side's own validator. The privileged child then re-reads the target file under root and re-runs the merge locally. Skipping this means an attacker who controls the unprivileged process controls what root writes. Pair with a conformance test that pushes identical fixtures through the direct path and the sudo-handoff path and asserts byte-equal output.

## Tests That Assert No-Op for Unwired Features Lock In L3 Failures (from hostie go-migration Phase 4, 2026-05-26)
A test that says "pressing `?` is a no-op" silently certifies the broken state when `?` is *supposed* to open Help. Tests at the integration layer must encode the intended end-to-end behavior, not the currently observed behavior. This is the test-side corollary of three-level verification: every L3 "is it wired" claim needs a matching test that asserts the wire reaches the destination, not just that the source-side dispatcher exists. Especially dangerous when refactors collapse a "no-op for unknown keys" test that quietly enumerates known keys — adding to that list means *removing* coverage.

## Bubble Tea Processes Must Not os.Exit From Signal Handlers (from hostie go-migration Phase 4, 2026-05-26)
Any TUI runtime (Bubble Tea, Ink, blessed, ratatui, etc.) installs terminal-state save/restore around the program. Signal-driven shutdown must route through the framework's quit channel (`tea.Quit`, `program.Exit()`, etc.) so the terminal mode is restored, goroutines drain, and deferred cleanup runs. `os.Exit` from a SIGINT handler skips all of it and leaves the user with a corrupted terminal plus any leaked tempfiles the program was about to clean up. Generalizes: never `os.Exit` from inside a process that owns terminal state or holds cleanup defers.

## Production Globals as Test Seams Are a Layering Smell (from hostie go-migration Phase 4, 2026-05-26)
`var fn = realImpl` (or `var PATH = "/etc/hosts"`) rewired by tests creates production/test divergence, blocks `t.Parallel`, and confuses readers about which value is canonical. Prefer constructor injection, interface fields, or `t.Cleanup`-scoped fakes wired through explicit options. If a global must exist for ergonomics, gate test mutation through a `SetForTest(t *testing.T)` helper that calls `t.Helper()` + `t.Cleanup` to restore and fails outside tests. Production code should not contain `// for tests` toggles.

