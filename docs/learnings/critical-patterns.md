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
