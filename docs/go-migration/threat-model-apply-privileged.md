# Threat Model: `__apply-privileged` Subcommand

**Scope**: Privilege escalation flow used by `hostie apply` (and auto-apply from mutating commands) when the invoking user cannot write `/etc/hosts` directly.

**Last reviewed**: Phase 3 (Go migration)
**Related design decisions**: D11, D12, D13, D14
**Implementation**:
- `go/internal/apply/privilege.go`
- `go/internal/cmd/apply_privileged.go`
- `go/internal/apply/runner.go`

---

## 1. Architecture

When `apply.Runner.writeEtcHosts` detects that the current process cannot write `/etc/hosts`, it escalates via the following flow:

```
hostie apply (uid=user)
  └─> WritePayloadToTempfile (renders managed block to $TMPDIR/hostie-payload-<random>)
       perms 0600, owner=user
  └─> ReexecWithSudo: sudo <real-binary-path> <orig argv...>
       └─> sudo prompts for password, spawns child with uid=0
            └─> hostie __apply-privileged <payload-path>   (uid=0)
                 ├─> ValidatePayloadFile: ownership/perms/marker checks
                 ├─> WriteEtcHosts (atomic temp+rename)
                 └─> os.Remove(payloadPath)   (deferred, runs on all exit paths)
```

The hidden `__apply-privileged` subcommand is **never intended to be invoked manually**. It is:

- Hidden from `--help` (`Hidden: true` on the cobra command)
- Prefixed with `__` to signal "internal API, do not use"
- Documented as such in its `Short` description

---

## 2. Trust boundaries

| Boundary | From | To | Trust transition |
|----------|------|-----|------------------|
| User → unprivileged hostie | Shell argv, env | uid=user process | Standard CLI input validation |
| Unprivileged hostie → tempfile | Rendered managed block bytes | 0600 file in $TMPDIR | Owner=user, perms locked |
| Unprivileged hostie → sudo | argv, tempfile path | sudo (suid binary) | sudo policy gates auth |
| sudo → privileged hostie | argv inherited | uid=0 process | All re-validation happens here |
| Privileged hostie → /etc/hosts | Validated payload bytes | filesystem | Atomic write under root |

The **most important boundary** is **sudo → privileged hostie**: every input crossing it (argv, payload file contents, payload file metadata) is treated as adversarial.

---

## 3. Attack surface and mitigations

### 3.1. Direct invocation of `__apply-privileged` by an unprivileged attacker

**Attack**: Attacker who can run `hostie` directly invokes `hostie __apply-privileged /path/they/control` hoping to write `/etc/hosts`.

**Why it fails**: The subcommand itself does not escalate privilege. It must be invoked under uid=0 to write `/etc/hosts`. If an unprivileged user runs it, `etchosts.WriteEtcHosts` returns EACCES and nothing happens.

**Residual risk**: None for `/etc/hosts` itself. If the attacker has *some other* way to get the subcommand to run as root (e.g. via a misconfigured sudoers rule that allows `hostie __apply-privileged *` without password), they can substitute a malicious payload path. **Mitigation**: documentation must tell operators to scope sudo policies to `hostie apply` (or the unprefixed binary), never `hostie __apply-privileged *`.

### 3.2. Symlink attack on the payload tempfile

**Attack**: Attacker replaces `$TMPDIR/hostie-payload-<random>` with a symlink to a sensitive file (e.g. `/etc/shadow`) between tempfile creation and validation, hoping the privileged process reads/writes through the link.

**Mitigations**:
- `WritePayloadToTempfile` opens with `O_CREATE|O_EXCL|0600` — refuses to follow an existing path.
- Filename includes 8 random bytes (`crypto/rand`) → 64 bits of entropy, infeasible to predict.
- `ValidatePayloadFile` uses `os.Lstat` (does **not** follow symlinks) and asserts `info.Mode().IsRegular()` → rejects symlinks outright.
- Permissions must be exactly `0600`; ownership must match invoking user (or sudo user via `SUDO_UID`).

**Residual risk**: TOCTOU between `Lstat` and `ReadFile`. An attacker who can win that race could swap the file. Probability is low because the file is in `$TMPDIR` with 0600 and the attacker would need write access to the owning user's tempdir. Acceptable for now; could be hardened by `openat`/`O_NOFOLLOW` + reading from fd in a follow-up.

### 3.3. Malicious payload contents

**Attack**: An attacker who gained write access to the tempfile (despite the protections above) inserts shell metacharacters or extra hostie markers into the managed block to try to corrupt `/etc/hosts` outside the managed region.

**Mitigations**:
- `ValidatePayloadFile` requires the content to start with `#` (comment marker).
- `etchosts.ReplaceManagedBlock` operates on byte ranges between known BEGIN/END markers — it does not interpret payload content; it only substitutes the block.
- The payload bytes are written verbatim; there is no shell interpolation anywhere in the path.

**Residual risk**: Garbage inside the managed block is possible, but it cannot escape the managed region. Worst case: `/etc/hosts` parses oddly until the next `hostie apply` overwrites it.

### 3.4. Argument injection via `os.Args` forwarding

**Attack**: `ReexecWithSudo` forwards `os.Args[1:]` to `sudo`. A crafted argument could try to inject extra flags into the sudo command line.

**Mitigations**:
- `exec.Command("sudo", args...)` passes args as a separate slice; there is **no shell parsing**. Each argument is a single argv entry.
- The first arg is the resolved absolute path of the current binary (`filepath.EvalSymlinks(os.Executable())`), preventing PATH-based binary substitution.

**Residual risk**: An attacker who controls the invoking user's argv already has the user's privileges; they don't gain anything new through this path. Acceptable.

### 3.5. Tempfile leak / non-cleanup

**Attack**: Attacker SIGKILLs the privileged process between payload-write and remove, leaving sensitive content behind.

**Mitigations**:
- `WritePayloadToTempfile` registers `SIGINT`/`SIGTERM` handlers that call cleanup before exit.
- `__apply-privileged` uses `defer os.Remove(payloadPath)` so the file is removed on every code path (success, error, panic).

**Residual risk**: `SIGKILL` cannot be intercepted. Leftover files have perms 0600 and contain only `/etc/hosts`-shaped data (no secrets). Acceptable.

### 3.6. Resolved-binary substitution via symlink

**Attack**: User installs `hostie` as a symlink to a malicious binary; sudo escalates the wrong program.

**Mitigation**: `filepath.EvalSymlinks(execPath)` resolves to the real binary path before passing to `sudo`. The actual executable that runs under root is the resolved file, not the symlink target chosen at sudo time.

**Residual risk**: Same as 3.4 — if an attacker can place the malicious binary in the resolved path, they already had write access to the real installation. Acceptable.

### 3.7. Race between `CanWriteEtcHosts` check and write

**Attack**: `/etc/hosts` permissions change between the probe and the actual write, causing the unprivileged process to attempt an unprivileged write that then fails.

**Mitigation**: This is benign. If the direct write fails, the runner returns an error to the user; YAML state is preserved (D13). No partial /etc/hosts corruption because writes are atomic (temp + rename).

### 3.8. Information disclosure via process listing

**Attack**: An observer running `ps` sees `sudo /path/to/hostie __apply-privileged /tmp/hostie-payload-abc123` and learns the payload path, allowing targeted attacks on the tempfile.

**Mitigation (current)**: None — the tempfile path is visible in argv. The protections in 3.2 (entropy, 0600, owner check, IsRegular) are designed to defeat an attacker who knows the path.

**Residual risk**: Documented. The tempfile is only valid for the lifetime of the privileged process (milliseconds). An attacker would need to win a TOCTOU race against `Lstat` → `ReadFile`. Acceptable.

---

## 4. Out-of-scope threats

The following are **explicitly not** addressed by this design and remain the user's responsibility:

- **Compromised sudoers configuration**: If the operator grants `NOPASSWD: ALL` to a low-trust user, hostie cannot prevent abuse. Recommendation: scope sudo policy to the user-facing `hostie apply` invocation only.
- **Compromised `/etc/hosts`**: hostie only manages content between its BEGIN/END markers. Modifications outside that block are out of scope.
- **Compromised `~/.hosts` YAML source**: A user who can write `~/.hosts` controls their own DNS resolution. This is by design.
- **Kernel-level attacks** (LD_PRELOAD, ptrace of privileged process): out of scope; relies on OS hardening.
- **Side channels** (timing of sudo password entry, etc.): out of scope.

---

## 5. Why not alternative designs?

### Alternative A: File-descriptor passing instead of tempfile

Pass the payload via an inherited FD from parent to sudo child. Eliminates the tempfile entirely.

**Rejected because**: `sudo` does not reliably preserve arbitrary open FDs across the privilege transition on all supported platforms (especially older Linux + BSD variants). Tempfile with 0600 + entropy in the filename is portable and well-understood.

### Alternative B: Persistent helper daemon (e.g. polkit / launchd agent)

Run a long-lived privileged daemon and IPC requests to it.

**Rejected because**: Adds installation complexity (service files, plist), increases attack surface (always-on root process), and requires platform-specific integration. Out of scope for a CLI tool that runs occasionally.

### Alternative C: setuid binary

Install `hostie` as setuid root.

**Rejected because**: Setuid binaries are a significant attack surface (env scrubbing, argv handling, library injection all become security-critical). sudo already does this work correctly and gates on user authentication.

---

## 6. Operator guidance

For deployments that want passwordless `hostie apply`:

```
# In /etc/sudoers.d/hostie:
%hostie-users ALL=(root) NOPASSWD: /usr/local/bin/hostie apply *
%hostie-users ALL=(root) NOPASSWD: /usr/local/bin/hostie add *
# ... etc for other mutating commands

# DO NOT do this — it bypasses argv validation:
# %hostie-users ALL=(root) NOPASSWD: /usr/local/bin/hostie *
```

Never grant sudo access to `hostie __apply-privileged` directly. That command is only safe when invoked by hostie itself, with a tempfile produced by the same process in the same invocation.

---

## 7. Verification

The privilege flow is covered by:

- **Unit tests**: `go/internal/apply/privilege_test.go` (if present) covers `WritePayloadToTempfile`, `ValidatePayloadFile` permission and ownership checks.
- **CLI integration tests**: `go/internal/cmd/integration_test.go` overrides `apply.ETC_HOSTS_PATH` to a temp directory so the apply flow runs without sudo. The privileged code path is not exercised in CI (would require interactive sudo).
- **Manual smoke**: `hostie apply` against a writable /etc/hosts (as root, or with appropriate perms) is part of the release smoke checklist.

Future work: extend the test harness to fake the sudo reexec (e.g. by injecting a script in PATH that simulates sudo) to cover the round-trip in CI.
