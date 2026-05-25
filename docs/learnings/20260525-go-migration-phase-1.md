# Learnings: Go Migration — Phase 1 Bootstrap

**Date:** 2026-05-25
**Feature:** go-migration (Phase 1 of 5)
**Duration:** 1 session · 6 swarmed beads + 4 inline-fix beads + 1 epic · 10 commits on `feature/go-migration`

## Context

Phase 1 scope was deliberately narrow: stand up `go/go.mod` with all primary Charm-stack deps pinned, a do-nothing `cmd/hostie/main.go` that *imports* every dep (so the linker pulls them in and size measurement is realistic), Go CI lanes on 4 platforms, a size-check job enforcing the D2 ≤18 MB ceiling, and a `size-budget.md` recording actuals. No business logic. No user-visible Go functionality. The point was to **prove the size budget before sinking effort into a port.**

Outcome: 2.50–2.57 MB across 4 platforms — 15.4 MB headroom against the ≤18 MB ceiling. Pivot signal (drop Charm if oversize) is dormant. Phases 2–4 cleared to proceed.

## What Worked Well

- **Stub-with-real-imports as a size-budget probe.** Importing every primary dep from a do-nothing `main.go` (via `_ = pkg.Symbol` assignments) made the first measurement realistic. If we'd measured a true empty `main()`, the 2.5 MB number would have been meaningless and the pivot decision deferred to Phase 2 when far more code is at stake.
- **Pivot signal defined *before* the measurement.** D2's ≤18 MB hard / ≤10 MB aspirational thresholds were locked in design.md with a written escape hatch (drop Charm, try Cobra-only TUI-less binary). When the measurement came in green, the decision to proceed was mechanical — no debate.
- **Parallel CI lanes (v1 Bun + v2 Go) instead of replacement.** Per D6, the `go/` tree lives alongside `src/` until Phase 5 cutover. Phase 1 CI added Go jobs without touching Bun jobs. Zero risk of accidentally breaking the still-shipping v1 while bootstrapping v2.
- **Three-level verification extended to CI jobs and dep imports.** Reviewing's Phase 2 didn't just confirm files exist — it verified each pinned dep was actually *imported* in main.go (otherwise `go.mod` entries are decorative and won't show up in the binary size measurement), and each CI job is actually *wired* into the right workflow's `jobs:` map with correct `needs:` dependencies.
- **Inline-fix loop for top-severity P2s after GATE 3.** Rather than deferring all 8 P2 findings, we fixed the 4 with security or correctness impact (tag-injection, release-coupling, cobra-version, action-pinning) in the same session. Build/test/vet stayed green throughout. The remaining 4 P2 (ci-job-sprawl, artifact-prefix, test-contract, stat-portability) are now scheduled cleanly into Phase-2-prep — not lost.

## What Didn't Work

- **First-draft release.yml had a tag-ref injection vector.** Interpolating `${{ github.ref_name }}` directly into a `-ldflags` shell argument means a tag literal like `v1.0.0; rm -rf /` would have hit the shell on a workflow that holds `contents: write`. The same line also produced a `vv1.0.0` format bug because the tag's leading `v` wasn't stripped. The fix (env-pass via `env:` + `${REF_NAME#v}`) is now a critical pattern. Lesson: **any time GitHub Actions context values flow into a shell, default to `env:` and never interpolate.**
- **Third-party Actions started life pinned to floating major tags (`@v4`, `@v5`).** This is the GitHub Actions default everyone copies, and it's wrong for any repo that ships binaries. A compromised maintainer can rewrite `v4` to point at a malicious commit and the release workflow would silently use it. Fixed by SHA-pinning all 7 actions with semver comments + `.github/dependabot.yml` for automated PR-bumped updates. Lesson: **floating major-tag refs are fine for ad-hoc scripts, not for release pipelines.**
- **Cobra `--version` was hand-rolled with `BoolVar("version", ...)` + a `RunE` branch.** Worked, but blocks Phase 3 from ever introducing a `version` subcommand (Cobra would generate a colliding auto-handler). Native `cobra.Command.Version` + `SetVersionTemplate` is the supported path and produces byte-identical output. Lesson: **don't reinvent framework primitives, even for trivial cases — the collision will surface 3 phases later when you've forgotten why.**
- **Go release jobs initially ran on every push.** Cross-compiling 4 platforms on every commit to `feature/go-migration` was wasted CI minutes (release artifacts are only consumed on tag pushes). Fixed by tag-gating with `if: startsWith(github.ref, 'refs/tags/v')` and adding those jobs to `release.needs:` so an oversized binary blocks publication. Lesson: **release-only jobs need a tag gate from day one — adding them later means weeks of paid CI minutes spent on artifacts no one will ever download.**
- **GOPATH-style cache leakage into the working tree.** A worker's `go build` or `go install` ran with `GOPATH=$PWD`, dropping `/bin/gopls`, `/pkg/mod/`, and a stray `go/hostie` binary at the repo root. They didn't get committed (caught at review), but they would have on a less-careful run. Fixed retroactively in `.gitignore`. Lesson: **the first `.gitignore` entry of any new Go subdir should be the GOPATH-style cache paths, before the first `go` command runs.**

## Surprises

- **2.50 MB came in 7× under the ≤18 MB ceiling.** Pre-spike estimates from the feasibility doc projected 8–12 MB with all of Charm. The actual number being a quarter of that means the pivot signal is effectively dormant for the entire migration — there is enormous headroom for Phase 4's TUI components. The aspirational ≤10 MB target is now plausible as the *hard* ceiling for v2.0.0, not just aspirational.
- **bubbletea v1.3.6 pulls in a transitive `github.com/atotto/clipboard` via `bubbles/textinput`.** Not in our direct deps list, not in design.md's threat model. Worth a Phase-2-prep audit since clipboard libs typically shell out to platform binaries (`pbcopy`/`xclip`/`wl-copy`) and that's a behavior surface we hadn't considered.
- **bubbletea v2 was released mid-Phase-1 planning, and we'd quietly pinned to v1.x.** The v2 API is breaking (different Msg dispatch). Discovery of this drove a deliberate decision to add `bubbletea` to dependabot's *major-ignore* list so weekly bumps don't surprise us into a half-migrated state. The right answer was to pivot the design doc to explicitly call out "v1.x line, NOT v2" — which we did.
- **br tried to block closing fixed review beads.** The review beads were created with `blocks: hosts-cli-go-migration-epic-l54` (meaning the bead blocks the epic). `br close` interpreted "open dependencies exist" as a block in the wrong direction. `--force` was needed. Lesson: br's dependency-direction semantics around `blocks:` are non-obvious; document the workaround if it recurs.

## Critical Patterns

Three patterns are broad enough to promote to `critical-patterns.md`:

1. **GitHub Actions context values are untrusted input — pass via `env:`, never interpolate into shell.**
2. **Migration-era CI has two surfaces — application contracts AND cutover contracts — both need explicit gates.**
3. **Three-level artifact verification (exists / substantive / wired) extends to CI jobs and dep imports, not just source files.**

## Process Notes

- **Brainstorming → writing-plans → validating chain worked exactly as designed for a tiny scoped phase.** The temptation with a 6-bead phase is to skip straight to implementation. We didn't, and the upfront design.md lock on D2's pivot signal is what made the green-light decision mechanical.
- **GATE 3 inline-fix loop saved a review-then-rework cycle.** The reviewing skill produced 8 P2 findings; we triaged in-session, fixed the top 4, deferred the rest with bead handoffs. Without the inline option, all 8 would have gone to a new round of beads and a follow-up swarm.
- **`go.mod` Go-directive needed bumping (1.22 → 1.23) because bubbletea v1.3.6 requires it.** Worth checking the *transitive* Go version requirement for every primary dep before pinning, to avoid this kind of round-trip in tighter phases.
- **The 4 deferred P2 + 10 P3 review beads correctly went to external-ref off-epic** (e.g., `hosts-cli-go-migration-epic-l54:ci-job-sprawl`) rather than into the active phase. Keeps Phase 1 closeable without losing the work.
