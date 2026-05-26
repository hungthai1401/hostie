# Size Budget — Phase 2 Core Port

> **Status:** Phase 2 complete. Binary size grew from 2.5–2.7 MB (Phase 1) to ~4.5 MB (Phase 2) due to real use of yaml.v3, oklog/ulid, validators, render, and etchosts packages. Still well under both the 10 MB aspirational target and 18 MB hard ceiling (per **D2**). No pivot signal fired.

## 1. Phase 2 measured sizes

Measured after implementing all Phase 2 core packages (domain, yaml, render, etchosts, fileio).

| Platform | Bytes | MB | Status |
|---|---:|---:|---|
| darwin-arm64 | 4,698,112 | 4.48 | ✅ under 10 MB |

Phase 2 adds real usage of yaml.v3, oklog/ulid/v2, validators, render pipeline, atomic etchosts writes, and fileio — resulting in ~1.9 MB growth from Phase 1 baseline.

## 2. Phase 1 baseline (for comparison)

| Platform | Bytes | MB | Status |
|---|---:|---:|---|
| darwin-arm64 | 2,691,490 | 2.57 | ✅ under 10 MB |
| darwin-amd64 | 2,698,912 | 2.57 | ✅ under 10 MB |
| linux-amd64  | 2,621,592 | 2.50 | ✅ under 10 MB |
| linux-arm64  | 2,621,592 | 2.50 | ✅ under 10 MB |

## 3. Ceiling status (vs D2)

`design.md` **D2**: Build with `go build -trimpath -ldflags="-s -w"`. **Hard ceiling: 18 MB** per platform; **aspirational: ≤10 MB**.

| Platform | Phase 2 Size | vs 18 MB hard | vs 10 MB aspirational |
|---|---:|---:|---:|
| darwin-arm64 | 4.48 MB | 13.52 MB headroom | 5.52 MB under |

Phase 2 still clears both ceilings with comfortable margin.

## 3. Headroom analysis

The Phase 1 stub imports all 7 primary deps (cobra used; bubbletea, bubbles, lipgloss, yaml.v3, oklog/ulid/v2, sahilm/fuzzy blank-used) but exercises none of them — so the Go linker strips most symbols. Realistic landing size grows substantially as each phase wires its dep into actual code:

| Phase | Adds | Realistic delta | Cumulative best-guess |
|---|---|---:|---:|
| **Phase 1** (bootstrap, stub) | dep imports only | baseline | **~2.5–2.7 MB** (measured) |
| **Phase 2** (core port) | yaml.v3 real-use, oklog/ulid real-use, atomic writes, fileio, validators | +0.5–1.0 MB | ~3–4 MB |
| **Phase 3** (CLI port) | cobra subcommands real-use, completion generator, exit-codes, apply.Runner + dryrun | +0.5–1.5 MB | ~4–5 MB |
| **Phase 4** (TUI port) | bubbletea program loop, bubbles widgets (list/textinput/spinner/viewport), lipgloss styles in anger, sahilm/fuzzy weighted scorer, store + components + modals | **+5–10 MB** | **~10–14 MB** (likely band) |

Phase 4 is the dominant size jump — bubbles + lipgloss + bubbletea altogether weigh several MB once their symbols are actually referenced from the TUI components.

**Bottom line:** the projected landing zone after Phase 4 is roughly **10–14 MB**, leaving ~4–8 MB of slack against the 18 MB hard ceiling and likely missing the 10 MB aspirational target — which D2 explicitly permits.

## 4. Escalation path (approach.md §3 Option C)

Per `approach.md` §3 Option C and §8 ("If first measurement > 15 MB, escalate dep choice"):

- **Trigger:** any platform exceeds **15 MB** during Phase 2–4 measurements.
- **Action:** stop forward porting. Open an orchestrator escalation against approach.md §3 Option C — **drop `bubbles`** (use raw Bubble Tea + Lipgloss only; hand-roll list/textinput/spinner primitives, ~500–800 LOC) **before Phase 4 ships**.
- **Why 15, not 18:** the 3 MB cushion below the hard ceiling absorbs late polish (additional CLI subcommands, transient build-time imports) without forcing an emergency rewrite after we've already pushed past the cap.

Currently no escalation needed — we are 12.43 MB below the 15 MB warn line.

## 5. Reference reality check

From `.spikes/go-migration/p1-size-reference/FINDINGS.md` (executed pre-Phase-1):

| Reference | Platform | Stripped size | Comment |
|---|---|---:|---|
| `charmbracelet/gum` v0.16.0 | darwin-arm64 | **12.6 MB** | Real Charm-stack app, heavy bubbles+lipgloss+bubbletea use. Closest analog. |
| `charmbracelet/glow` (per `docs/go-migration/go-port-feasibility.md`) | typical | **~14–16 MB** | Markdown TUI; richer rendering than hostie. Upper bound of realistic envelope. |
| Phase 1 spike stub (7 deps blank-imported) | darwin-arm64 | 2.7 MB | Linker strips ~everything; same shape as today's CI numbers. |
| **hostie Phase 1 bootstrap (this measurement)** | darwin-arm64 | **2.57 MB** | Stub baseline; expected to grow toward gum-band by Phase 4. |

hostie will likely land **near gum (~12 MB), below glow (~14–16 MB)** because gum has more TUI components than hostie but hostie adds yaml + cobra completions + ulid + fuzzy weighting.

## 6. Methodology

Build invocation (from `.github/workflows/release.yml` `go-build-release` job):

```bash
CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
  go build -trimpath \
  -ldflags="-s -w -X main.version=${{ github.ref_name }}" \
  -o ../dist/hostie-go-$os-$arch ./cmd/hostie
```

Matrix: darwin-arm64 (macos-latest), darwin-amd64 (macos-latest), linux-amd64 (ubuntu-latest), linux-arm64 (ubuntu-latest). Cross-compile only — no QEMU run.

Verification (from `go-size-check` job): downloads all four artifacts, runs `stat -c%s` on each binary, fails the build if any platform exceeds `18 * 1024 * 1024` bytes, warns if any exceeds `10 * 1024 * 1024`, writes the result table to `$GITHUB_STEP_SUMMARY`.

## 7. Provenance

- **CI run:** [#26390304200](https://github.com/hungthai1401/hostie/actions/runs/26390304200) — `Release` workflow on `feature/go-migration`
- **Commit:** `04d14217aedf02438ff0ac2bc9cc93d5f0632688`
- **Branch:** `feature/go-migration`
- **Toolchain:** Go version pinned in `go/go.mod` (`actions/setup-go@v5` with `go-version-file: go/go.mod`)
- **size-check job:** `77678281901` (✅ passed, 6 s)

## 8. Re-measurement policy

Phase exit-state for Phases 2, 3, and 4 must re-run this measurement and append a new row (or replace this table) so the trend is visible at every gate. If any platform crosses **10 MB**, the warning is informational only. If any platform crosses **15 MB**, see §4 (escalation).

---

_Last updated: Phase 1 bootstrap, CI run #26390304200, commit `04d1421`._
