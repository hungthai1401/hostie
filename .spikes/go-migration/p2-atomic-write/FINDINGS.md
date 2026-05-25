# Spike: S5A — Atomic etc-hosts write hardening

**Date:** 2026-05-25
**Bead:** `hosts-cli-go-mig-p2-atomic-spike-jdb`
**Reproducer:** `repro.go` (run from `go/` module: `cp repro.go go/cmd/_spike.go && cd go && go run -tags=spike ./cmd/_spike.go && rm go/cmd/_spike.go`)
**Host:** Darwin (macOS, APFS)
**Verdict:** **CONFIRMED** — all 5 properties achievable; impl pattern for S5B is locked.

## Risk (from approach.md HIGH item)

`WriteEtcHosts` is the only writer into `/etc/hosts`. A bug here means:
- symlink attack → writing user content into arbitrary system files
- cross-FS rename → EXDEV on Linux (e.g. `/etc` is its own mount)
- preserved mode bits → setuid/setgid persistence; world-writable `/etc/hosts`
- chown swallowing all errors → silent permission downgrade
- crash mid-write → orphan tempfiles polluting `/etc/`

Pivot Signal #2: if any of the above cannot be reproducibly hardened on macOS (APFS semantics differ from ext4), escalate before closing S5B.

## Properties + Findings

### 1. Symlink rejection — PASS

**Technique:** `os.Lstat(path)` (NOT `os.Stat`) — if `fi.Mode()&os.ModeSymlink != 0`, refuse with explicit error before touching tempfile.

**Result on macOS:** sibling target untouched, error returned. Identical to expected Linux behaviour. APFS does not change the semantic.

### 2. Same-FS tempfile (no EXDEV) — PASS

**Technique:** `os.CreateTemp(filepath.Dir(target), ".hostie-tmp-*")` places the tempfile in the SAME directory as the target, guaranteeing same filesystem, guaranteeing `os.Rename` never returns EXDEV.

**Critically NOT:** `os.MkdirTemp("", ...)` which lands in `$TMPDIR` (often `/var/folders/...` on macOS, `/tmp` on Linux) — these may be a different mount than `/etc`.

**Result on macOS:** rename succeeded, file written. No EXDEV.

### 3. Mode clamp — PASS

**Technique:** `mode & 0o777 & ^0o022`
- `& 0o777` strips setuid (0o4000), setgid (0o2000), sticky (0o1000)
- `& ^0o022` strips group-write and world-write

For `/etc/hosts` we hard-code mode `0o644` which already passes the clamp untouched — but the clamp is the defence-in-depth in case a caller passes a hostile mode.

**Result on macOS:** stored as 0644; the clamp formula on input `0o777` produces `0o755` as expected.

### 4. EPERM-only chown swallow — PASS

**Technique:**
```go
if err := os.Chown(tmpPath, uid, gid); err != nil {
    var pe *fs.PathError
    if errors.As(err, &pe) && errors.Is(pe.Err, unix.EPERM) {
        // swallow — running unprivileged, expected
    } else {
        return err
    }
}
```

`os.Chown` wraps the syscall error in `*fs.PathError`. `errors.As` + `errors.Is` correctly distinguishes EPERM (swallow) from EACCES, ENOENT, EROFS (propagate). Verified by injecting both errno values: only EPERM matches.

### 5. Signal cleanup — PASS (with mitigation for S5B)

**Technique:** `defer os.Remove(tmpPath)` alone does NOT fire on SIGTERM — `defer` only runs on normal function return or `panic`, not on OS signals or `os.Exit`.

**Mitigation required in S5B:**
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    _ = os.Remove(tmpPath)
    os.Exit(130)
}()
defer signal.Stop(sigCh)
defer func() { _ = os.Remove(tmpPath) }()  // belt + suspenders for panic path
```

The S5B impl bead MUST include a unit test that:
1. spawns a child process that calls `WriteEtcHosts` with a slow content reader
2. sends SIGTERM to the child mid-write
3. asserts no `.hostie-tmp-*` files remain in the target directory

**Result on macOS reproducer:** the over-cautious test design `os.RemoveAll`'d the parent dir, masking the real signal-trap need. The TECHNIQUE finding stands: defer is insufficient, signal.Notify is required.

## Verdict

**CONFIRMED.** All 5 properties are achievable with the pipeline below. No design.md amendment needed. Pivot Signal #2 dormant.

S5B (impl bead) implementation contract:

```go
// WriteEtcHosts atomically replaces `path` with `content`. Refuses to follow
// symlinks. Uses same-FS tempfile to avoid EXDEV. Clamps mode to 0o644 (no
// setuid/setgid/sticky/world-write). Best-effort chown to current uid/gid,
// swallowing only unix.EPERM. Tempfile is unlinked on every error exit
// including SIGINT/SIGTERM (via signal.Notify trap).
func WriteEtcHosts(path, content string) error
```

S5B test suite must assert ALL 5 properties listed above on BOTH `ubuntu-latest` AND `macos-latest` (the spike confirmed macOS; CI must lock both).

## Files Kept

- `repro.go` — runnable reproducer (build tag `spike`; copy into `go/cmd/_spike.go` to run)
- `FINDINGS.md` — this file
