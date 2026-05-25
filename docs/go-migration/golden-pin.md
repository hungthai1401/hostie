# Golden Harness — v1 Binary Pin

**Purpose:** The golden harness (`go/test/golden/harness_test.go`) runs cross-binary parity tests between the v1 TypeScript binary and the v2 Go implementation. This document records the pinned v1 release used as the parity reference.

---

## Pinned Release

- **Tag:** `v1.0.0`
- **Commit:** (GitHub release tag — see https://github.com/hungthai1401/hostie/releases/tag/v1.0.0)
- **Release Date:** 2026-05-25 (approximate)

---

## Pinned SHA-256 Checksums

These checksums are hard-coded in `go/test/golden/harness_test.go` as `PinnedV1SHA`. The harness **refuses to proceed** if the downloaded binary does not match the expected SHA-256.

| Platform       | SHA-256                                                          |
|----------------|------------------------------------------------------------------|
| darwin-arm64   | `e1ff4b47d02cc8a7872ed0fc4da0616301c92b50377e88ffb96f3eb07ca68119` |
| linux-x64      | `f97fa80cc2a3bb6a2e689009837bcb80fdeb8a0853554988c3e34b51f1dd9eef` |
| darwin-x64     | *(fetch on demand when CI matrix expands)*                       |
| linux-arm64    | *(fetch on demand when CI matrix expands)*                       |

---

## Cache Layout

```
go/test/golden/.cache/
  v1.0.0/
    darwin-arm64/
      hostie-darwin-arm64
    linux-x64/
      hostie-linux-x64
```

- The `.cache/` directory is gitignored.
- Binaries are downloaded once per platform and reused across test runs.
- If the SHA-256 verification fails, the cached binary is removed and re-downloaded.

---

## Download URL Pattern

```
https://github.com/hungthai1401/hostie/releases/download/<TAG>/<ASSET>
```

Example:
```
https://github.com/hungthai1401/hostie/releases/download/v1.0.0/hostie-darwin-arm64
```

---

## Running the Golden Harness

The golden harness is gated behind a build tag and does **not** run on plain `go test ./...`.

### Local (single platform)

```bash
cd go
go test -tags=golden ./test/golden/... -v
```

This downloads the v1 binary for your host platform (darwin-arm64 or linux-x64), verifies the SHA-256, and runs the YAML round-trip parity tests against ≥3 fixtures.

### CI (matrix)

CI runs the golden harness on both `darwin-arm64` and `linux-x64` runners. The cache is keyed by `golden-pin-v1.0.0-<platform>` for cross-run reuse.

---

## Rotation Policy

**When to update the pin:**

1. A new v1 release is tagged (e.g., `v1.0.1`) that we want to test parity against.
2. A critical bug is discovered in the pinned v1 binary that invalidates the parity contract.

**How to update the pin:**

1. Download the new release binaries for all platforms.
2. Compute SHA-256 checksums: `shasum -a 256 hostie-<platform>`
3. Update `PinnedV1SHA` in `go/test/golden/harness_test.go` with the new checksums.
4. Update `PinnedV1Tag` in `go/test/golden/harness_test.go` to the new tag.
5. Update this document (`docs/go-migration/golden-pin.md`) with the new tag, commit SHA, and checksums.
6. Delete the old cache: `rm -rf go/test/golden/.cache/`
7. Run the golden harness locally to verify: `cd go && go test -tags=golden ./test/golden/... -v`
8. Commit the changes with message: `chore(golden): update pin to v<new-version>`

**Do NOT rotate the pin** unless there is a specific reason. The pin is a stability anchor — changing it invalidates the parity baseline.

---

## Parity Surfaces (Current)

- **YAML round-trip:** For each fixture in `go/test/fixtures/hosts/`, the Go v2 `yaml.Unmarshal` → `render.RenderManagedBlock` pipeline must produce byte-identical output to the v1 binary's `hostie list` command.

## Parity Surfaces (Planned — Phase 3)

- **`list --json`:** JSON output comparison.
- **`apply --dry-run`:** Diff output comparison.

---

## Failure Modes

### SHA-256 mismatch

If the downloaded binary does not match the expected SHA-256, the test **fails loudly** with:

```
SHA-256 mismatch for downloaded v1 binary: expected <expected>, got <actual> — refusing to proceed
```

The corrupted cache file is removed automatically. Re-run the test to re-download.

### Download failure

If the download fails (network error, 404, timeout), the test fails with:

```
Failed to download v1 binary from <URL>: <error>
```

Check your network connection and verify the release URL is still valid.

### v1 binary execution failure

If the v1 binary exits non-zero or produces unexpected output, the test fails with:

```
v1 binary failed: <error>
Output:
<stdout/stderr>
```

This indicates a fixture incompatibility or a v1 binary issue. Investigate the fixture or consider rotating the pin.

---

## References

- Spike findings: `.spikes/go-migration/p2-golden-pin/FINDINGS.md`
- Design decision: `docs/go-migration/design.md` (D9, D15)
- Harness implementation: `go/test/golden/harness_test.go`
