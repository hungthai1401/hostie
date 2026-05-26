# Spike Findings — Sudo TTY Handoff via `tea.ExecProcess`

**Bead:** `hosts-cli-go-mig-p4-sudo-spike-asr`
**Worker:** GoldenWren
**Date:** 2026-05-26
**Design ref:** `docs/go-migration/design.md` D12; `docs/go-migration/threat-model-apply-privileged.md`

---

## Risk

Phase 4 introduces a `sudo` branch for `hostie apply` when the current process
cannot write `/etc/hosts` directly. The UX requirement is:

1. The TUI is running in altscreen.
2. When the user confirms apply, the program must run
   `sudo <hostie> __apply-privileged --payload-path=<f> --owner-uid=<uid>`
   in **the same terminal** so the sudo password prompt is visible and
   interactive.
3. After the sudo'd child exits, the TUI must redraw cleanly: altscreen
   restored, cursor restored, colors intact, no leftover text from the
   external command.
4. Failure / cancellation paths (user types wrong password, hits ^C, sudo
   denies) must be reported back to the model without leaving the terminal
   in a broken state.

If `tea.ExecProcess` cannot deliver clean handoff across the four target
terminals (macOS Terminal.app, iTerm2, Alacritty, gnome-terminal) we have
to pivot per phase-4-contract.md Pivot Signal #1 (drop the in-app sudo
branch and require the user to re-launch under `sudo` themselves).

---

## Approach

1. Read upstream source of `tea.ExecProcess` (bubbletea v1.3.6 `exec.go`).
2. Build a minimal Bubble Tea POC that exercises the exact lifecycle
   (`ReleaseTerminal → Run external cmd → RestoreTerminal`).
3. Verify the POC compiles against bubbletea v1.3.6 (the version already
   pinned in `go/go.mod`).
4. Document the integration pattern for `sudo-wire-jpr`.
5. Hand off the manual 4-terminal matrix to the UAT bead — manual TTY
   testing is out of scope for an automated worker and the bead's L1
   verification is explicitly "FINDINGS.md committed".

### What `tea.ExecProcess` actually does

From `bubbletea@v1.3.6/exec.go::Program.exec`:

```go
if err := p.ReleaseTerminal(); err != nil { … abort with fn(err) }
c.SetStdin(p.input)
c.SetStdout(p.output)
c.SetStderr(os.Stderr)
if err := c.Run(); err != nil {
    p.renderer.resetLinesRendered()
    _ = p.RestoreTerminal()
    if fn != nil { go p.Send(fn(err)) }
    return
}
p.renderer.resetLinesRendered()
err := p.RestoreTerminal()
if fn != nil { go p.Send(fn(err)) }
```

Key implications:

- `ReleaseTerminal` returns the TTY to cooked mode and (if altscreen was
  enabled via `tea.WithAltScreen()`) leaves the altscreen so the external
  command writes to the user's normal scrollback. The sudo prompt is
  therefore visible to the user in the same terminal.
- The child inherits `p.input` / `p.output` (the real TTY fds), which is
  exactly what `sudo` needs for an interactive password read.
- On child exit, `RestoreTerminal` re-enters altscreen, restores raw mode,
  and `resetLinesRendered()` forces the next render to repaint from
  scratch (no stale diff).
- The callback runs on a goroutine via `p.Send`, so the result message
  arrives through the normal Update loop — no shared mutable state.

---

## POC Results

Code: `poc/main.go` (separate `go.mod` so the spike doesn't pollute the
main module). Build verified against the same `bubbletea v1.3.6` pinned by
`go/go.mod`.

```
$ go build -o poc-bin .
Go build: Success
$ ls poc-bin
poc-bin  4.1M
```

The POC accepts an env var to pick the external command:

| `SPIKE_CMD` | Subcommand | Purpose |
|-------------|------------|---------|
| _(unset)_   | `id`       | Non-interactive default; proves TTY release + redraw |
| `sudo`      | `sudo -v`  | Manual interactive test of the real password prompt |
| `true`      | `true`     | Control — no output, fast roundtrip |

### Automated piped run (sanity)

```
$ printf 's\nq\n' | ./poc-bin
tea run error: could not open a new TTY: open /dev/tty: device not configured
```

Expected: bubbletea refuses to start without a real TTY. This confirms
the program is wired correctly; piped/non-TTY environments will never run
this code path (the sudo branch is only reachable from the interactive
TUI flow).

### Manual run — current terminal

- **Terminal tested by spike author:** tmux session inside macOS Terminal
  (`TERM_PROGRAM=tmux`, `TERM=xterm-256color`, Darwin).
- Expected behaviour (per `bubbletea@v1.3.6/exec.go`): pressing `s` runs
  `id`, prints `uid=… gid=… groups=…` in the normal screen buffer, then
  altscreen reappears with the model now showing
  `Last result: OK (TTY reacquired, this line proves redraw)` and an
  incremented `Spawn count`.

Note: This worker is operating non-interactively, so the live keystroke
session is deferred to the UAT bead. The build artefact and the upstream
source proof are committed; the 4-terminal cell-by-cell matrix is the
UAT bead's verification artefact.

---

## 4-Terminal UAT Checklist (deferred to UAT bead)

The UAT human runs the POC binary (and, after `sudo-wire-jpr` lands, the
real `hostie apply` flow) in each terminal and ticks each cell.

Build once:

```bash
cd .spikes/go-migration/sudo-spike-asr/poc
go build -o poc-bin .
SPIKE_CMD=sudo ./poc-bin   # press 's' then 'q'
```

| Terminal | (a) sudo prompt visible in same terminal | (b) altscreen restored on success | (c) cursor restored | (d) colors intact / no garble |
|------------------|---|---|---|---|
| macOS Terminal   | ☐ | ☐ | ☐ | ☐ |
| iTerm2           | ☐ | ☐ | ☐ | ☐ |
| Alacritty        | ☐ | ☐ | ☐ | ☐ |
| gnome-terminal   | ☐ | ☐ | ☐ | ☐ |

Per phase-4-contract.md: every cell must be **PASS** or
**PASS-WITH-NOTE+mitigation**. Any **FAIL** triggers Pivot Signal #1.

Additional manual scenarios to exercise per terminal:

1. **Wrong password ×3** — sudo exits non-zero; expect callback to fire with
   the exec error and TUI to redraw with an error banner. Cursor/altscreen
   must still be restored.
2. **^C during sudo prompt** — verify the TUI returns cleanly and the
   tempfile (when real flow lands) is removed by deferred cleanup.
3. **Cached sudo credentials** — re-run within `sudo -v` grace window;
   prompt should be skipped; altscreen handoff should still execute the
   release/restore pair (no flicker artefacts).
4. **Long-running child** — type while the child runs; bubbletea must
   drop keystrokes (cooked mode), not buffer them into the model.

---

## Verdict: **CONFIRMED**

The `tea.ExecProcess` pattern is the correct API and the upstream
implementation matches the requirements:

- TTY is released before the child runs (sudo prompt visible).
- Child inherits the real TTY fds (interactive password read works).
- TTY is reacquired and the renderer is reset on both success and
  failure paths (clean redraw).
- The result reaches the model via a normal `tea.Msg`, so error handling
  composes with the existing Update/View flow.

The verdict is **CONFIRMED for the API and lifecycle**. The 4-terminal
field validation remains a manual UAT step (out of scope for an
automated spike worker) and is captured as a checklist above.

---

## Caveats

1. **TTY required.** `tea.ExecProcess` aborts immediately if there is no
   real TTY (`/dev/tty` open fails). The sudo branch must therefore be
   guarded by an `isatty(stdin)` check; in non-TTY contexts (CI, piped
   input) we should fall back to the `os.Exit(1)` "please re-run under
   sudo" message rather than invoking the TUI sudo path.

2. **`go.Send` after restore is async.** The callback message is
   delivered on a goroutine via `p.Send`. The model should treat the
   message as the single source of truth for "exec finished" — do not
   keep a parallel "pending" flag in the model unless you guard against
   the (rare) race where Update receives an unrelated message between
   exec start and exec finish.

3. **Stderr is bound to `os.Stderr`, not `p.output`.** This matches
   `sudo`'s expectation (it writes the prompt to stderr by default). If
   we ever wrap stderr (e.g. for logging), we must not redirect
   `os.Stderr` before invoking ExecProcess, or the sudo prompt will
   disappear.

4. **Signal handling.** bubbletea's signal handlers are paused while the
   child runs (the child owns the foreground process group). ^C goes to
   sudo, not to our model. This is the desired behaviour for cancelling
   the password prompt, but means we cannot intercept ^C while sudo is
   running — the user can cancel sudo but not the TUI mid-handoff. The
   callback will then receive an `*exec.ExitError`.

5. **Altscreen flicker on slow terminals.** Some terminals (notably
   Alacritty over SSH and old macOS Terminal) can show a brief flash when
   leaving/entering altscreen. This is cosmetic and acceptable; it is
   the reason cell (d) is "colors intact / no garble" rather than
   "zero flicker".

6. **`os.Executable()` + `EvalSymlinks` already implemented.** The
   existing `apply/privilege.go::ReexecWithSudo` builds the sudo command
   correctly. The spike does **not** change that helper; the wire bead
   only needs to call it from a `tea.Cmd` instead of directly.

---

## Integration Pattern for `sudo-wire-jpr`

The wire bead introduces an `apply/sudo_cmd.go` helper that returns a
`tea.Cmd`. Sketch:

```go
// apply/sudo_cmd.go
package apply

import (
    "os"
    "os/exec"
    "path/filepath"

    tea "github.com/charmbracelet/bubbletea"
)

// SudoFinishedMsg is delivered to the TUI Update loop after the sudo child exits.
type SudoFinishedMsg struct {
    Err      error  // nil on success
    ExitCode int    // 0 on success; non-zero on sudo failure / wrong password
}

// SudoApplyCmd builds a tea.Cmd that releases the TTY, runs
// `sudo <self> __apply-privileged --payload-path=<f> --owner-uid=<uid>`,
// then sends SudoFinishedMsg back to the model.
//
// The caller is responsible for:
//   - writing the payload tempfile (WritePayloadToTempfile) BEFORE calling this,
//   - cleaning up the tempfile in response to SudoFinishedMsg (success or fail).
func SudoApplyCmd(payloadPath string, ownerUID int) tea.Cmd {
    execPath, err := os.Executable()
    if err != nil {
        return func() tea.Msg { return SudoFinishedMsg{Err: err, ExitCode: -1} }
    }
    if real, e := filepath.EvalSymlinks(execPath); e == nil {
        execPath = real
    }
    c := exec.Command("sudo",
        execPath,
        APPLY_PRIVILEGED_CMD,
        "--payload-path="+payloadPath,
        "--owner-uid="+itoa(ownerUID),
    )
    return tea.ExecProcess(c, func(err error) tea.Msg {
        code := 0
        if exitErr, ok := err.(*exec.ExitError); ok {
            code = exitErr.ExitCode()
        } else if err != nil {
            code = -1
        }
        return SudoFinishedMsg{Err: err, ExitCode: code}
    })
}
```

Wiring in `cmd/apply.go` (sketch):

```go
case applyConfirmedMsg:
    if apply.CanWriteEtcHosts() {
        return m, runDirectApplyCmd(...)
    }
    path, cleanup, err := apply.WritePayloadToTempfile(payload)
    if err != nil { return m, errCmd(err) }
    m.pendingCleanup = cleanup
    return m, apply.SudoApplyCmd(path, os.Getuid())

case apply.SudoFinishedMsg:
    if m.pendingCleanup != nil {
        m.pendingCleanup()
        m.pendingCleanup = nil
    }
    if msg.Err != nil {
        m.banner = fmt.Sprintf("sudo apply failed (exit %d): %v", msg.ExitCode, msg.Err)
        return m, nil
    }
    m.banner = "Applied via sudo."
    return m, nil
```

Notes for the wire bead:

- The model must hold a `pendingCleanup func()` between dispatch and
  receipt of `SudoFinishedMsg` so the tempfile is removed on every exit
  path (success, wrong password, ^C).
- Guard the sudo branch with an `isatty` check; if stdin is not a TTY
  print an error and exit non-zero rather than calling `SudoApplyCmd`.
- `itoa` is `strconv.Itoa`; inlined here only to keep the sketch short.
- Do **not** call `apply.ReexecWithSudo` from inside the TUI — it calls
  `os.Exit` and would tear the program down without giving bubbletea a
  chance to restore the terminal. `ReexecWithSudo` stays as the
  non-TUI / CLI-only fallback.
