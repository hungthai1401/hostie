# Learnings: hostie /etc/hosts TUI+CLI Manager (Phase 1B)

**Date:** 2026-05-25
**Feature:** hostie (epic hosts-cli-379)
**Duration:** 1 phase (1B), ~14 beads delivered, ~20 review beads created, 3 review rounds

## What Worked Well

- **Swarming parallel rounds** — 6 workers in round 1, 2 in round 2, 1 in round 3. Conflicts surfaced cleanly via `[BLOCKED]` markers + reservations; one file conflict (apply.ts between .64/.65) resolved by serializing without ceremony.
- **3-Level artifact verification (exists/substantive/wired)** caught the entire "infrastructure built but never imported" class of gaps in Phase 1A (cli/index.ts never dispatched; tui/index.tsx never composed). Without L3, ./dist/hostie would have shipped as a no-op.
- **Inline P1 fix loop during UAT** — each UAT failure (D10 reexec, D12 perms, D9 keybindings, D19 search) was filed as a P1 bead and fixed in the same session. Tight loop preserved context and design intent.
- **External-ref pattern for non-blocking review beads** — P2/P3 created with `external_ref=hosts-cli-379:<letter>` instead of `--parent` kept the epic-close path uncluttered while preserving traceability.
- **`test.each` for malformed-input shapes** — replacing 2 explicit assertions with a 5-case parameterized table covered the full safety contract in fewer lines.

## What Didn't Work

- **`Bun.argv` semantics in `bun build --compile` are non-obvious.** Two iterations on reexec (.72 round 1 fixed argv[0], round 2 had to also `slice(2)` instead of `slice(1)`) because the compiled binary's argv includes a `/$bunfs/root/<name>` virtual script path between the exec and user args. Should have tested against the compiled binary, not the source, first.
- **Phase 1A "implementation complete" was a lie** — code-quality/architecture review caught it, but the brainstorming-phase definition of "done" did not require the integration layer to be wired. Phase 1B existed entirely to compose the parts.
- **Fuse.js defaults are too fuzzy for short tokens.** Default `threshold: 0.4 + minMatchCharLength: 1` matched "two" against "mover.local" via single-char overlap. Needed tightening to 0.3 + 2.
- **`statSync` quietly follows symlinks.** Reviewer caught this but pre-UAT; it never broke UAT because `/etc/hosts` isn't usually a symlink. Easy class of bug to ship.
- **D5 doc drift** — design.md locks `@opentui/react`; implementation uses `ink`. No ADR/amendment was written when the swap happened. Still open (P3 .d5-doc-drift-iqc).

## Surprises

- **/etc/hosts shipping at mode 0o646 on macOS** — the system file was world-writable by default on the UAT host. `stats.mode & 0o7777` faithfully preserved that. Mask had to actively *clamp* (`& ~0o022`), not just preserve. "Preserve perms" is the wrong default for any privileged file.
- **5 P1 beads created during UAT** — even after a clean review with PASS_WITH_P2 verdicts from all 5 specialists, hands-on use found half-a-dozen gaps the automated review missed (wired-but-default-fuzzy search, wired-but-no-keybinding modals, system file with permissive ambient mode). UAT is irreplaceable.
- **The pre-existing MoveToGroupModal Escape flake was already present** on the merge base — never introduced or touched by Phase 1B. It survived every review, every gate.

## Critical Patterns

1. **Atomic file replacement requires lstat + same-FS tempdir + permission clamping** — Privileged file writes via temp+rename must (a) `lstatSync` not `statSync` (don't follow symlinks into `/etc/shadow`), (b) `mkdtempSync` in `dirname(target)` not `os.tmpdir()` (avoid EXDEV), (c) mask preserved modes with `& 0o0777 & ~0o022` not `& 0o7777` (drop world-write + setuid/setgid/sticky), (d) tolerate EPERM on chown only, re-throw all other errno.

2. **Smoke tests target the compiled binary, not the source.** `bun build --compile` changes module resolution (`/$bunfs/root/...`), argv shape (extra virtual script element), and runtime FS layout. Tests against `bun run src/index.ts` will pass while `./dist/binary` fails. Always exercise the compiled artifact, and always force rebuild (no `if !exists` shortcuts).

3. **Three-level artifact verification (exists / substantive / wired) is non-negotiable.** Phase 1A had all components built with passing tests, yet the CLI dispatched nothing and the TUI rendered nothing. Without checking L3 (imported and called by the integration layer), entire features ship as no-ops.

4. **Hands-on UAT finds what automated review misses.** Five specialist agents + a synthesizer returned PASS_WITH_P2 on Phase 1B. UAT then found 5 P1s in ~20 minutes: a permission-mode clamp, a TUI without add/edit/group keys, a `/` key with no handler, a search with too-fuzzy defaults, a sudo reexec broken in the compiled binary. Skip UAT and these ship.

## Process Notes

- Inline-fix loop within UAT (file P1 bead → patch → test → close → continue) was much faster than batching UAT failures and re-spawning workers. Keep this pattern for any feature where the human is already in the loop.
- `br create --external-ref <epic>:<letter>` requires a unique value per bead — used `:<feature-letter>` suffix to allow many beads to traceback to the same epic without colliding.
- Swarm round 1 dispatch slim-context prompts (just bead ID + paths to design.md / approach.md) worked well; no worker hallucinated additional scope.
- Worker `[BLOCKED]` on file conflict → orchestrator simply waited for the holder to finish and dispatched a round-2 worker. No coordination overhead.
