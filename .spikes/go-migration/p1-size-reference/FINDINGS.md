# Spike: p1-size-reference

## Risk
Phase 1 pivot signal: if first stripped Go binary > 18 MB, abort Phase 2 and escalate to approach.md §3 Option C (drop `bubbles`). Need empirical confidence before sinking 6 beads of bootstrap work.

approach.md §8 explicitly asks for this measurement as a pre-Phase-1 check.

## Approach
1. Download a production Charm/Bubble Tea binary (`charmbracelet/gum` v0.16.0) for darwin-arm64 as the reference upper bound (gum exercises bubbletea + bubbles + lipgloss heavily — closest analog to hostie's TUI).
2. Build a minimal `cmd/hostie`-shaped stub matching the bead `hosts-cli-go-mig-p1-main-oy5` spec: all 7 primary deps imported (cobra real-use, others blank-used so the linker keeps a representative slice), `-trimpath -ldflags="-s -w"`, on darwin-arm64 (host platform).
3. Compare both to the 18 MB hard ceiling (D2) and 10 MB aspirational target.

## Findings
| Binary | Platform | Stripped size | vs 18 MB cap | vs 10 MB aspirational |
|---|---|---|---|---|
| `gum` v0.16.0 (real Charm app) | darwin-arm64 | **12.6 MB** | 5.4 MB headroom | exceeds by 2.6 MB |
| hostie stub (7 deps, no business logic) | darwin-arm64 | **2.7 MB** | 15.3 MB headroom | well under |

Stub deps imported: cobra v1.10.1 (used), bubbletea v1.3.6, bubbles v0.21.0, lipgloss v1.1.0, yaml.v3 v3.0.1, oklog/ulid/v2 v2.1.1, sahilm/fuzzy v0.1.1 (blank-used via `var _ = pkg.Symbol`). `go mod tidy` pulled bubbletea's transitive deps (`go-colorful`, `colorprofile`, `golang.org/x/exp`).

Toolchain: Go 1.25.5 (CI will use 1.22; minor size variance expected, no material impact).

## Interpretation
- **Stub at 2.7 MB ≠ realistic.** The Go linker strips unused symbols aggressively when deps are only blank-imported. Real port code (TUI components, modals, search, store, render, atomic write, sudo re-exec, validators) will pull in substantially more of each dep.
- **gum at 12.6 MB ≈ realistic upper bound** for a Charm-stack app of hostie's size. hostie has fewer components than gum but adds cobra + yaml + ulid + fuzzy, roughly netting out.
- **18 MB hard ceiling: CONFIRMED reachable** with comfortable 5+ MB headroom over the realistic upper bound. Pivot signal will not fire on the stub measurement.
- **10 MB aspirational: at risk.** gum already exceeds it. hostie's realistic landing zone is likely 10–14 MB, similar band to glow (~14–16 MB). Document as expected miss, not as escalation.

## Verdict
**CONFIRMED.** D2 hard ceiling is achievable with the current 7-dep set. Phase 1 may proceed. The stub `go-size-check` job will report well under 18 MB on first run — pivot signal will not fire prematurely.

Aspirational ≤10 MB is unlikely to hold once Phase 4 lands the TUI; that is OK per D2 ("hard ceiling 18 MB; aspirational ≤10 MB") — only the hard ceiling gates Phase 2.

## Implications for Phase 1 beads
- No bead changes required.
- `hosts-cli-go-mig-p1-budget-doc-bpj` should document this reference measurement (gum 12.6 MB, stub 2.7 MB) alongside the first real CI measurements so future readers understand why a 9 MB Phase-3 binary is normal even though the bootstrap stub was 2.7 MB.

## Reproduction
```bash
# Reference
gh release download v0.16.0 -R charmbracelet/gum -p '*Darwin_arm64*'
tar -xzf gum_0.16.0_Darwin_arm64.tar.gz
ls -lh gum_0.16.0_Darwin_arm64/gum

# Stub (see .spikes/go-migration/p1-size-reference/ for go.mod + main.go)
cd /tmp/hostie-spike
go mod tidy
go build -trimpath -ldflags="-s -w" -o hostie-spike ./...
ls -lh hostie-spike
```
