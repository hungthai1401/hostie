# Story Map: Go Migration — Phase 1 (Bootstrap)

## Plan

Migrate hostie from Bun/TypeScript to Go to drive the compiled binary from 61.20 MB down to ≤18 MB, while preserving every parity surface from v1 and adding auto-apply on mutation.

---

## Phase: Bootstrap

Stand up the Go module, CI lane, and size budget so Phase 2 can begin porting business logic with empirical confidence in the toolchain.

---

### Story 1: Go module exists and builds a stub binary

**Purpose:** A developer can run `go build ./cmd/hostie` on a fresh checkout and get a working stub binary.

**Why Now:** Nothing else in this phase (or any later phase) compiles without `go.mod`. This story has the only obvious reason to be first.

**Contributes To:** Exit-state clauses 1 (`go.mod` with pinned deps) and 2 (`cmd/hostie/main.go` stub compiles).

**Creates:** `go/go.mod`, `go/go.sum`, `go/cmd/hostie/main.go`, `go/internal/` empty package skeleton, dep import-proof file.

**Unlocks:** Stories 2 and 3 — they need a buildable module to wire CI against and to measure.

**Done Looks Like:** `cd go && go build -trimpath -ldflags="-s -w -X main.version=0.0.0-bootstrap" -o /tmp/hostie-stub ./cmd/hostie && /tmp/hostie-stub --version` prints `hostie v0.0.0-bootstrap` and exits 0.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p1-gomod-hs4` | Initialize go module with pinned Charm + utility deps | go.mod + go.sum; pinned versions per approach.md |
| `hosts-cli-go-mig-p1-main-oy5` | Implement cmd/hostie/main.go version stub with dep import proofs | main.go prints version via ldflags; uses every primary dep so size measurement reflects realistic weight |

---

### Story 2: Go CI lane runs alongside Bun lane

**Purpose:** Every push to `feature/go-migration` runs `go build`, `go test`, `go vet`, and `staticcheck` in parallel with the existing Bun jobs, on both Ubuntu and macOS runners.

**Why Now:** Story 1 produces a buildable module. Without CI, regressions in subsequent phases would not surface until manual builds. CI must exist before any business logic lands.

**Contributes To:** Exit-state clauses 3 (`ci.yml` runs Go jobs) and 5 (v1 lanes still green).

**Creates:** Updated `.github/workflows/ci.yml` with `go-build`, `go-test`, `go-vet`, `go-staticcheck` jobs (matrix `[ubuntu-latest, macos-latest]`); `staticcheck` advisory (`continue-on-error: true`).

**Unlocks:** Story 3 — size-check job can hook into the same CI workflow. Phase 2 — workers get fast feedback on every push.

**Done Looks Like:** A no-op commit to `feature/go-migration` triggers 4 Go jobs × 2 OS = 8 Go matrix entries, all green, in ≤3 minutes. Existing Bun typecheck/test/build jobs are untouched and also green.

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p1-ci-of1` | Add Go CI jobs (build, test, vet, staticcheck) to ci.yml | matrix [ubuntu-latest, macos-latest]; uses actions/setup-go@v5; staticcheck advisory only |

---

### Story 3: Size budget measured and documented

**Purpose:** The team has empirical, per-platform measurements of stripped-binary size against the 18 MB ceiling and 10 MB aspirational target, asserted by CI.

**Why Now:** Last story in Phase 1 — needs the module (Story 1) and the CI lane (Story 2). Also the **pivot signal** for the whole feature: if the stub binary is already > 18 MB, we abort and revisit Option C from approach.md before starting Phase 2.

**Contributes To:** Exit-state clauses 4 (`go-size-check` CI job) and 6 (`size-budget.md` recorded).

**Creates:** `go-size-check` job in `.github/workflows/ci.yml` (fails > 18 MB, warns > 10 MB); `docs/go-migration/size-budget.md` with measured sizes per platform.

**Unlocks:** Phase 2 can begin with empirical confidence. If pivot signal fires, Phase 2 does not begin.

**Done Looks Like:** CI passes on `feature/go-migration` with the `go-size-check` job green; `docs/go-migration/size-budget.md` shows the actual measured stripped-binary size for darwin-arm64, darwin-x64, linux-x64, linux-arm64, each ≤ 18 MB (with a warning if > 10 MB).

**Beads:**

| Bead ID | Title | Notes |
|---------|-------|-------|
| `hosts-cli-go-mig-p1-release-g4v` | Add release.yml Go cross-compile matrix (4 platforms) | GOOS/GOARCH matrix; coexists with Bun job; artifacts named hostie-go-$os-$arch |
| `hosts-cli-go-mig-p1-sizecheck-aqy` | Add go-size-check job asserting ≤18 MB (fail) and warning >10 MB | fails build on >18 MB per D2; warns >10 MB; step summary shows table |
| `hosts-cli-go-mig-p1-budget-doc-bpj` | Write docs/go-migration/size-budget.md with measured per-platform sizes and headroom analysis | populated from first CI run; back-references approach.md §"Open Questions" |

---

## Story Order Check

- **Story 1 first:** nothing else compiles without `go.mod`.
- **Story 2 second:** CI hangs nothing off business logic; it just needs a buildable target. Could in theory run in parallel with Story 1 in different worktrees, but the matrix needs the build target to exist.
- **Story 3 last:** size measurement reads CI artifacts produced by Story 2's jobs (or builds inline using setup-go from Story 2). Also gates the whole feature (pivot signal) — must be the final exit-state confirmation for Phase 1.

If Story 3's pivot signal fires (binary > 18 MB), Phase 2 does not begin. Approach.md Option C (drop `bubbles`) is the documented fallback.

---

## Coverage Check

| Exit-state clause | Covered by |
|-------------------|-----------|
| 1. `go.mod` with pinned deps | Story 1 |
| 2. `cmd/hostie/main.go` stub compiles | Story 1 |
| 3. CI runs Go jobs alongside Bun | Story 2 |
| 4. `go-size-check` CI job | Story 3 |
| 5. v1 CI lanes still green | Story 2 (preservation, no edits to Bun jobs) |
| 6. `size-budget.md` recorded | Story 3 |

Every clause covered. No orphan stories. No orphan exit-state clauses.
