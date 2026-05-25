## Current
- **Skill:** swarming
- **Feature:** hostie
- **Epic:** hosts-cli-379
- **Phase:** phase-1B-rework (P1 fixes from reviewing)

## Workers
| Name | Status | Current Bead | Reserved Files |
|------|--------|--------------|----------------|
| (none dispatched yet) | — | — | — |

## Progress
- Phase 1: 61/61 beads closed.
- Reviewing surfaced 4 P1 blockers + 4 P2 gaps.
- Phase 1B: 8 new beads (.62–.69) created, deps wired.
  - .62 cli-wire (P1) — wire src/index.ts dispatch
  - .63 tui-compose (P1) — compose src/tui/index.tsx
  - .64 apply-chmod (P1) — preserve perms on rename
  - .65 apply-malformed (P1) — reject unbalanced markers
  - .66 apply-dedup (P2) — blocked by .64, .65
  - .67 test-validate-hostname (P2)
  - .68 test-sudo-reexec (P2)
  - .69 test-tui-integration (P2) — blocked by .63

## Next
1. Load swarming skill, dispatch workers on .62, .63, .64, .65, .67, .68 in parallel.
2. After unblock: .66 and .69.
3. Re-run reviewing skill (specialists + synthesizer).
4. Phase 2 verification + Phase 3 UAT + GATE 3.
