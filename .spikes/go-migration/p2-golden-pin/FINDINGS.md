# Spike: S7A — Golden-pin (v1 release URL + SHA-256 + cache shape)

**Date:** 2026-05-25
**Bead:** `hosts-cli-go-mig-p2-golden-pin-spike-dbo`
**Reproducer:** `repro.sh [platform]` (default: linux-x64)
**Verdict:** **CONFIRMED** — v1.0.0 is pinnable; cache shape is `.cache/<tag>/<platform>/hostie-<platform>`; the harness must refuse on SHA mismatch (proven by accident during this spike).

## Risk (from approach.md HIGH item)

The golden harness is the parity contract. If we can't:
1. Reliably download a SHA-pinned v1 binary
2. Reliably verify it bit-for-bit
3. Refuse loudly when verification fails

…then Phase 3+4 ports have no parity safety net, and the cutover at Phase 5 has nothing to gate on.

Pivot Signal #3: golden tarball unverifiable.

## Findings

### 1. v1.0.0 release exists with per-platform binaries + sidecar SHA-256 files

```
$ gh release view v1.0.0 --repo hungthai1401/hostie
title: v1.0.0
asset: hostie-darwin-arm64       (61.2 MB)
asset: hostie-darwin-arm64.sha256
asset: hostie-darwin-x64
asset: hostie-darwin-x64.sha256
asset: hostie-linux-arm64
asset: hostie-linux-arm64.sha256
asset: hostie-linux-x64          (24.7 MB compressed download size on macOS net; full asset 90.9 MB linux-elf)
asset: hostie-linux-x64.sha256
```

All 4 platform builds we need are present. Sidecar `.sha256` files use standard `shasum -a 256` format (`<hex>  <filename>`).

### 2. Pinned hashes (locked into the harness)

| Platform | SHA-256 |
|---|---|
| darwin-arm64 | `e1ff4b47d02cc8a7872ed0fc4da0616301c92b50377e88ffb96f3eb07ca68119` |
| linux-x64    | `f97fa80cc2a3bb6a2e689009837bcb80fdeb8a0853554988c3e34b51f1dd9eef` |
| darwin-x64   | *(fetch on demand from sidecar at harness-run time)* |
| linux-arm64  | *(fetch on demand from sidecar at harness-run time)* |

For S7B, the harness should hard-code the two CI-relevant SHAs (darwin-arm64 + linux-x64) as Go constants AND assert the published sidecar matches the constant — belt + suspenders. If GitHub Releases ever silently re-publishes an asset, the assertion fails loudly.

### 3. URL pattern (stable)

```
https://github.com/hungthai1401/hostie/releases/download/<TAG>/<ASSET>
https://github.com/hungthai1401/hostie/releases/download/<TAG>/<ASSET>.sha256
```

This is the GitHub Releases canonical URL — stable across release format changes since 2015.

### 4. Cache layout

```
go/test/golden/.cache/v1.0.0/<platform>/hostie-<platform>
go/test/golden/.cache/v1.0.0/<platform>/hostie-<platform>.sha256
```

- Gitignored entry: `go/test/golden/.cache/`
- Re-downloads only if file missing or SHA mismatched
- Platform is detected at test time via `runtime.GOOS` + `runtime.GOARCH`
- CI uses `actions/cache@<sha>` keyed on `golden-pin-<tag>-<platform>` for cross-run reuse

### 5. SHA mismatch refusal — proven accidentally

During this spike, a flaky network truncated the linux-x64 download at 35.9 MB (of 90.9 MB expected). Running `shasum -a 256 -c hostie-linux-x64.sha256` produced:

```
hostie-linux-x64: FAILED
shasum: WARNING: 1 computed checksum did NOT match
```

→ exactly the loud-failure behaviour the harness must produce. The S7B harness implements this as:

```go
if computedSHA != expectedSHA {
    _ = os.Remove(cachePath)
    t.Fatalf("golden binary SHA mismatch: expected %s, got %s — refusing to proceed", expectedSHA, computedSHA)
}
```

`os.Remove` on mismatch ensures the next test run re-fetches rather than getting stuck with a corrupted cache.

### 6. Download size + CI time budget

linux-x64 is 90.9 MB. Download to GitHub Actions Linux runner is typically ~50 MB/s → ~2s. With `actions/cache@<sha>` keyed on `golden-pin-v1.0.0-linux-x64`, subsequent runs hit cache in ~100ms.

darwin-arm64 is 61.2 MB. Same cache strategy.

CI time budget impact: **< 5s amortized** (first run on a cache miss costs ~3s; all subsequent runs ~100ms). Acceptable.

## Pinned Decision for S7B Implementation

```go
package golden

const (
    PinnedV1Tag = "v1.0.0"
    PinnedV1URLBase = "https://github.com/hungthai1401/hostie/releases/download/" + PinnedV1Tag
)

var PinnedV1SHA = map[string]string{
    "darwin-arm64": "e1ff4b47d02cc8a7872ed0fc4da0616301c92b50377e88ffb96f3eb07ca68119",
    "linux-x64":    "f97fa80cc2a3bb6a2e689009837bcb80fdeb8a0853554988c3e34b51f1dd9eef",
    // darwin-x64, linux-arm64 — fetch sidecar at run time + assert against constant once captured
}
```

`docs/go-migration/golden-pin.md` (created in S7B) holds:
- Pinned tag + commit SHA of the tagged release
- Per-platform SHA-256 table
- Rotation policy (when to bump pin: only when v1 publishes a new tagged release we want to parity-test against)

## Verdict

**CONFIRMED.** Pivot Signal #3 dormant.

S7B implementation contract:
1. Build tag `//go:build golden` — does NOT run on plain `go test ./...`
2. Cache layout `go/test/golden/.cache/<tag>/<platform>/`
3. Hard-coded SHA constants for darwin-arm64 + linux-x64 (the CI matrix platforms)
4. Loud `t.Fatalf` on SHA mismatch + `os.Remove(cachePath)` to force re-fetch next run
5. First parity surface: YAML round-trip on ≥3 fixtures

## Files Kept

- `repro.sh` — runnable reproducer
- `hostie-darwin-arm64.sha256` — captured sidecar (86B)
- `hostie-linux-x64.sha256` — captured sidecar (83B)
- `FINDINGS.md` — this file

Binaries themselves are gitignored per existing `.spikes/**/*.tar.gz` pattern (which doesn't cover bare binaries — see Adjustments below).

## Adjustments needed elsewhere

`.gitignore` currently has `.spikes/**/*.tar.gz` but the golden binaries are bare ELF/Mach-O. Add `.spikes/**/hostie-*` (matching the asset names, excluding the .sha256 sidecars) so a future re-run doesn't accidentally commit a 90 MB binary.
