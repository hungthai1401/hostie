# Learnings: hostie go-migration Phase 4 (TUI port)

**Date:** 2026-05-26
**Feature:** go-migration (epic hosts-cli-go-migration-epic-l54)
**Duration:** 1 phase, 20 beads + 5 P1 review beads + 4 inline-fix commits, 1 review round (5 specialists) + 1 re-review pass

## What Worked Well

- **3-Level artifact verification caught two unwired keybindings (`?` and `Enter`)** that all 5 specialist reviewers missed in Phase 1 of the review. Both were advertised in HelpModal and listed in the phase-4 contract keybind table, but absent from `update.go`'s root key router. The L3 "wired" check — grepping the integration layer for the symbol — is what surfaced them; the substantive-but-unwired pattern from Phase 1A repeated itself two phases later.
- **Inline P1 fix loop (3 waves, 4 commits, all 5 P1s closed in one session)** preserved architectural context. The OnyxBear pass (P1-A) in particular required understanding both the threat model and the existing `apply.Runner` API; doing it inline rather than batching meant the fixer still had the review reasoning in working memory.
- **Conformance pin for two parser/renderer paths** — `TestApply_DirectAndSudoPathsProduceIdenticalEtc` runs the same fixture through (a) direct `Runner.Apply` and (b) `PrepareSudoHandoff → ValidatePayloadFile → ReplaceManagedBlock → WriteEtcHosts`, asserting byte-equal output. This pin is what allowed P1-A's architectural refactor (moving merge across the privilege boundary) to land without fear of drift.
- **Splitting tea-facing helpers from pure exec.Cmd builders** (P1-B): `apply/sudo_cmd.go` keeps `BuildSudoCmd` (pure stdlib); the tea-aware `SudoApplyCmd`/`SudoFinishedMsg` moved to `tui/app/sudo_exec.go`. `go list -deps ./internal/apply/...` now shows zero bubbletea — Layer 4 is clean. Architectural win even though single-binary size didn't change.
- **Re-review of just the patch (1636 lines via focused subagent)** rather than re-spawning all 5 specialists was the right cost/value tradeoff after a targeted fix loop. Verdict APPROVED in one pass with 4 P3 nits captured.

## What Didn't Work

- **`br create --external-ref` rejects duplicates per epic**, so the Phase 1B pattern of `--external-ref hosts-cli-379:<letter>` doesn't generalize when reviews produce many off-epic beads. Pivoted to labels (`epic-hosts-cli-go-migration,review-p2,source-*`) for traceability. The label scheme works but is less queryable than external-ref would have been.
- **Spike findings filed under the spike's own working directory** (`.spikes/go-migration/sudo-spike-asr/FINDINGS.md`) rather than the contract-specified location (`.spikes/go-migration/p4-sudo-handoff/FINDINGS.md`). Phase 2 artifact verification caught this as a clause-11 mismatch. Workers should be reminded that contract paths are authoritative when the spike was renamed mid-flight.
- **Production globals as test seams** (`apply.ETC_HOSTS_PATH`) leaked from production code into test wiring. It's a layering smell that allowed `apply_privileged_test.go` to be written quickly but blocks `t.Parallel`, and one of the new conformance tests mutates the same global. Captured as a P2 follow-up; the next refactor should move to constructor injection or an interface field.
- **`os.Exit` from a SIGINT handler in `writePayload`** bypasses tea's terminal restore. Found in review (P2 `writepayload-sigint-os-exit-pdqc`). Pattern is generic enough it belongs in critical-patterns.
- **`rtk git diff` wrapper outputs only `--stat`,** which silently hid the actual changes during one review iteration; had to fall back to `/usr/bin/git diff` directly. Worth documenting in AGENTS.md or the rtk wrapper itself.

## Surprises

- **Single-binary size didn't change after removing bubbletea from `apply/`** (8.8 MB → 8.8 MB). The TUI is still linked into the same binary via the `tui` package; the Layer 4 boundary fix is purely architectural / test-isolation. Reviewers asked twice "did it not work?" — the answer is that the win is structural, not size-driven, and that's a fine outcome.
- **A user-deferred UAT (bead `hosts-cli-go-mig-p4-uat-acz`)** is now the only thing blocking epic close. Phase 1B taught us UAT routinely finds 5 P1s in 20 minutes; deferring it is a known risk we're consciously accepting in exchange for moving to Phase 5 sooner. Capture this trade-off rather than pretending the deferral is free.
- **The privileged side never had merge tests** until P1-D added 10–13 cases. Production code had been live (well, behind sudo) since `bb54076` with zero coverage for owner-uid validation, missing markers, duplicate BEGIN, etc. A reminder that "the privileged path is the hardest to exercise" maps directly to "it's the path most likely to be untested" — defensive scaffolding (fake-uid+1, in-process temp `/etc/hosts`) should be standing infrastructure, not invented per-fix.

## Critical Patterns

1. **Privilege-Boundary Re-Derivation — Never Shape Payloads Upstream of Sudo.** Anything the privileged child does (merge, validate, normalize, atomic-write) must re-derive from minimal inputs read under root. Upstream (unprivileged) processes hand off only marker-delimited payloads with strict invariants; the privileged side re-reads the target file and re-runs the merge. Skipping this means an attacker who controls the unprivileged process controls what root writes. (From P1-A `hosts-cli-review-p1-sudo-merge-boundary-bre`.)

2. **Tests that assert no-op for unwired features lock in L3 failures.** A test that says "pressing `?` is a no-op" silently certifies the broken state when `?` is *supposed* to open Help. Tests at the integration layer must encode intended behavior, not observed behavior. Pair with the existing three-level verification: L3 ("wired") plus test-side L3 ("test asserts the wire reaches the destination"). (From P1-C / P1-E `?` and `Enter`.)

3. **Bubble Tea (and any TUI runtime) must not `os.Exit` from signal handlers.** Signal-driven shutdown must route through `tea.Quit` (or the framework's equivalent) so the terminal is restored, goroutines drain, and any deferred cleanup runs. `os.Exit` skips all of it and leaves a corrupted terminal plus leaked tempfiles. (From P2 `writepayload-sigint-os-exit-pdqc`.)

4. **Production globals as test seams are a layering smell.** `var fn = realImpl` rewired by tests creates production/test divergence, blocks `t.Parallel`, and confuses readers about which value is canonical. Prefer constructor injection, interface fields, or `t.Cleanup`-scoped fakes wired through explicit options. If a global must exist for ergonomics, gate test mutation through a `SetForTest(t)` helper that fails outside tests. (From P2 `production-globals-test-seams-vd4m`.)
