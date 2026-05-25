# Phase 1 Contract: Go Migration — Bootstrap

## Entry State

- `feature/go-migration` branch active.
- `docs/go-migration/{design.md,discovery.md,approach.md,phase-plan.md,go-port-feasibility.md}` committed.
- v1 source (`src/`, `tests/`, `build.ts`, `package.json`, `bun.lock`, `tsconfig.json`) untouched.
- v1 CI lanes (`.github/workflows/ci.yml` typecheck/test/build) green on `feature/go-migration`.
- No `go/` directory exists.
- `go` toolchain ≥ 1.22 installed on the working machine (verify with `go version`).

---

## Exit State

- `go/go.mod` declares `module github.com/khunglong/hostie/go` (or repo-matching module path) at Go ≥ 1.22 with **pinned** versions of: `spf13/cobra`, `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss`, `gopkg.in/yaml.v3`, `github.com/oklog/ulid/v2`, `github.com/sahilm/fuzzy`.
- `go/cmd/hostie/main.go` compiles to a stub binary that prints `hostie v<version>` (via `-ldflags="-X main.version=…"`) and exits 0. It imports every Charm dep at least once (blank-import or trivial use) so the size measurement reflects realistic dependency weight.
- `go build -trimpath -ldflags="-s -w"` produces a binary on every CI matrix entry (`ubuntu-latest`, `macos-latest`).
- `.github/workflows/ci.yml` runs four parallel Go jobs in addition to the existing Bun jobs: `go-build`, `go-test`, `go-vet`, `go-staticcheck` (advisory — non-blocking). All four green.
- A `go-size-check` CI job runs `ls -l` on the stripped binary; **fails** if any artifact > 18 MB; **prints a warning** if > 10 MB. Green on first run.
- `docs/go-migration/size-budget.md` records measured stripped-binary sizes per platform (darwin-arm64, darwin-x64, linux-x64, linux-arm64) and compares to the 18 MB ceiling and 10 MB aspirational target.
- v1 CI lanes still green (no regressions in `src/`).
- All Phase 1 beads closed via `br close`.

---

## Demo Story

Open a terminal on a fresh `feature/go-migration` checkout. Run `cd go && go build -trimpath -ldflags="-s -w" -o /tmp/hostie-stub ./cmd/hostie && ls -lh /tmp/hostie-stub`. The binary builds in under a minute and weighs in well under 18 MB (number recorded in `docs/go-migration/size-budget.md`). Run `/tmp/hostie-stub --version`; it prints `hostie v0.0.0-bootstrap` and exits 0. Open `.github/workflows/ci.yml` — there are now Bun jobs *and* Go jobs running side-by-side. Push a no-op commit; both lanes go green within ~3 minutes. The size-budget doc shows we have headroom for the real port.

---

## Unlocks

- Phase 2 (Core port) can begin writing Go code with confidence the toolchain, CI lane, and size budget all work.
- Validating skill can attach spike beads to the existing Go module (no module-bootstrap churn during spikes).
- Size measurement provides empirical signal on whether the ≤10 MB aspirational target is reachable without dropping a Charm dep — decision feeds Phase 4 risk assessment.
- Pinned dep versions let downstream phases reproduce builds bit-for-bit during golden-harness validation.

---

## Pivot Signals

- **First stripped-binary size > 18 MB.** Cannot meet D2 hard ceiling with current dep set. Escalate: revisit Option C from approach.md §3 (drop `bubbles`) before continuing. Do not start Phase 2 with a known-over-budget binary.
- **`charmbracelet/bubbletea` v2.x ships and breaks `tea.ExecProcess` API.** Pin to v1.x in Phase 1; if v1.x cannot be installed for any reason, escalate before Phase 2.
- **Go ≥ 1.22 not available in `actions/setup-go@v5` on macos-latest** (extremely unlikely; sanity check). Block on toolchain availability before continuing.
- **CI cannot run Go + Bun jobs concurrently** (matrix conflict, runner quota, etc.). Resolve before any Phase 2 bead lands; the parallel-lane invariant from D6 is non-negotiable.
