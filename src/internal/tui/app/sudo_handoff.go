// sudo_handoff.go — TUI-side sudo branch for the auto-apply pipeline.
//
// Bead: hosts-cli-go-mig-p4-sudo-wire-jpr
//
// Why this exists separate from apply_cmd.go:
//
// applyCmd's happy path (CanWriteEtcHosts == true) runs apply.Runner.Apply
// on a goroutine and is done. The sudo path is structurally different:
//
//   1. Pre-render the managed block in-process (we can't let the runner
//      attempt the write and re-exec via apply.ReexecWithSudo — that calls
//      os.Exit and would tear down Bubble Tea's altscreen mid-render).
//   2. Write the payload to a 0600 tempfile under $TMPDIR (D12).
//   3. Use tea.ExecProcess to release the TTY, run `sudo <self>
//      __apply-privileged --payload-path=… --owner-uid=…`, then re-acquire
//      the TTY (FINDINGS.md §"What tea.ExecProcess actually does").
//   4. Clean up the tempfile on every return path (D13).
//
// The privileged subcommand re-validates the payload, writes /etc/hosts, and
// unlinks the tempfile on its side as well — the two cleanup paths are
// idempotent via os.IsNotExist (see cmd/apply_privileged.go).
//
// Step 1 is also where we write ~/.hosts. Per D13 the YAML write is
// independent of the /etc/hosts write — even if sudo fails the YAML is on
// disk reflecting the mutated state. We therefore do that write BEFORE
// kicking off tea.ExecProcess, mirroring apply.Runner.Apply's ordering.

package app

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/src/internal/apply"
	"github.com/hungthai1401/hostie/src/internal/domain"
	"github.com/hungthai1401/hostie/src/internal/tui/store"
)

// canWriteEtcHostsFn is the indirection point that lets tests force either
// branch (direct vs sudo) without touching /etc/hosts. Production wires
// apply.CanWriteEtcHosts; tests override it via withCanWriteEtcHosts.
var canWriteEtcHostsFn = apply.CanWriteEtcHosts

// sudoApplyCmdFn is the indirection point for the tea.Cmd that drives
// tea.ExecProcess. Production wires app.SudoApplyCmd (sudo_exec.go); tests
// override it to
// observe the call shape (exe, payload, uid) without releasing the TTY.
var sudoApplyCmdFn = SudoApplyCmd

// resolveSelfExeFn is the indirection point for os.Executable + EvalSymlinks.
// Production wires apply.ResolveSelfExe; tests override it to return a stable
// path so the test binary's location does not leak into assertions.
var resolveSelfExeFn = apply.ResolveSelfExe

// withCanWriteEtcHosts is a test-only helper that swaps canWriteEtcHostsFn
// for the duration of a test and returns a restore function suitable for
// t.Cleanup.
func withCanWriteEtcHosts(canWrite bool) (restore func()) {
	prev := canWriteEtcHostsFn
	canWriteEtcHostsFn = func() bool { return canWrite }
	return func() { canWriteEtcHostsFn = prev }
}

// applyCmdDispatch chooses between the direct apply path (applyCmd) and the
// sudo handoff path (sudoApplyDispatch) based on canWriteEtcHostsFn.
//
// This is the single entry point Update should use for ApplyTriggerMsg
// dispatch. apply_cmd.go's applyCmd is now an implementation detail of the
// direct branch.
//
// Returned Cmd is never nil: even the failure-to-render path yields a Cmd
// that posts an ApplyResultMsg{Err: ...} so the status bar can report it.
//
// hostsPath is retained on the API for symmetry with prior versions but the
// sudo path no longer uses it directly — the runner owns the ~/.hosts path
// it was constructed with (see apply.NewRunner). The parameter remains for
// future direct-path callers that may need it.
func applyCmdDispatch(runner applyRunner, hostsFile domain.HostsFile, hostsPath string) tea.Cmd {
	_ = hostsPath
	if canWriteEtcHostsFn() {
		return applyCmd(runner, hostsFile)
	}
	return sudoApplyDispatch(runner, hostsFile)
}

// sudoPendingMsg is delivered after sudoApplyDispatch finishes its
// in-process prep work (~/.hosts write + payload tempfile creation) and
// before tea.ExecProcess runs. It carries the cleanup closure so Update can
// stash it on the Model — when SudoFinishedMsg arrives, Update fires
// the cleanup and emits an ApplyResultMsg.
//
// We do NOT chain ExecProcess directly behind the goroutine that does the
// rendering work because tea.ExecProcess must be returned synchronously from
// Update. Splitting prep and exec via this intermediate message keeps the
// blocking I/O off the UI goroutine while still letting Update see the
// ExecProcess Cmd before returning.
type sudoPendingMsg struct {
	// exeCmd is the tea.Cmd returned by SudoApplyCmd. Update returns
	// it from the sudoPendingMsg branch; tea.ExecProcess runs synchronously
	// from that return.
	exeCmd tea.Cmd

	// cleanup removes the payload tempfile. Update stashes it on the Model
	// and invokes it when SudoFinishedMsg arrives.
	cleanup func()

	// err is non-nil if prep failed (render, fileio, tempfile, exe
	// resolution). Update converts it to ApplyResultMsg{Err: ...} directly
	// without ever invoking ExecProcess.
	err error
}

// sudoApplyDispatch runs the pre-exec preparation on a goroutine and yields
// a sudoPendingMsg. The actual tea.ExecProcess Cmd is delivered to Update via
// the message rather than returned from this function because:
//
//   - Rendering the managed block and writing ~/.hosts both block on disk
//     I/O; we want them off the UI goroutine.
//   - tea.ExecProcess must be a synchronous return value from Update so the
//     bubbletea runtime can call ReleaseTerminal in the same tick. Update
//     does that synchronous return when it receives sudoPendingMsg.
//
// All managed-block rendering, ~/.hosts writes, and payload-tempfile
// creation are delegated to runner.PrepareSudoHandoff (apply.Runner). The
// payload contains ONLY the rendered managed block with markers — the
// privileged subcommand re-reads /etc/hosts under root and re-derives the
// merge, restoring threat-model §3.3.
//
// This package no longer imports core/etchosts, core/fileio, or core/render;
// the merge happens entirely in the privileged child.
func sudoApplyDispatch(runner applyRunner, hostsFile domain.HostsFile) tea.Cmd {
	return func() tea.Msg {
		if runner == nil {
			return sudoPendingMsg{err: fmt.Errorf("apply runner not configured")}
		}

		// Delegate ~/.hosts write + managed-block render + payload tempfile
		// creation to the runner. The runner owns the hostsPath it was
		// constructed with (D14: TUI runner is dryRun=false, hostsPath
		// fixed at NewModel time).
		payloadPath, cleanup, err := runner.PrepareSudoHandoff(hostsFile)
		if err != nil {
			return sudoPendingMsg{err: err}
		}

		// Resolve self exe for the sudo argv.
		exePath, err := resolveSelfExeFn()
		if err != nil {
			cleanup()
			return sudoPendingMsg{err: fmt.Errorf("resolve self exe: %w", err)}
		}

		// Build the tea.Cmd that wraps tea.ExecProcess.
		exeCmd := sudoApplyCmdFn(exePath, payloadPath, os.Getuid())
		return sudoPendingMsg{exeCmd: exeCmd, cleanup: cleanup}
	}
}

// handleSudoPending wires the sudoPendingMsg through Update:
//
//   - On prep failure: surface an ApplyResultMsg directly so the status bar
//     reports the error and no ExecProcess runs.
//   - On prep success: stash the cleanup on the Model and return the
//     tea.ExecProcess Cmd. Bubble Tea releases the TTY synchronously on this
//     return.
func (m Model) handleSudoPending(msg sudoPendingMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		// Render via the existing handleApplyResult path so the banner text
		// matches the direct-path failure format ("Apply failed: ... — YAML
		// kept").
		return m.handleApplyResult(ApplyResultMsg{Err: msg.err}), nil
	}
	m.pendingSudoCleanup = msg.cleanup
	return m, msg.exeCmd
}

// handleSudoFinished maps SudoFinishedMsg into ApplyResultMsg, after
// running the deferred tempfile cleanup. The Changed flag is true on
// success: by the time SudoFinishedMsg arrives, the privileged subcommand
// has already rewritten /etc/hosts (or surfaced an error via non-zero exit).
func (m Model) handleSudoFinished(msg SudoFinishedMsg) (Model, tea.Cmd) {
	if m.pendingSudoCleanup != nil {
		m.pendingSudoCleanup()
		m.pendingSudoCleanup = nil
	}
	if msg.Err != nil {
		applyErr := msg.Err
		if msg.ExitCode > 0 {
			applyErr = fmt.Errorf("sudo apply exit %d: %w", msg.ExitCode, msg.Err)
		}
		return m.handleApplyResult(ApplyResultMsg{Err: applyErr}), nil
	}
	// Success: render the same banner the direct-path emits and clear dirty.
	m.store.SetStatusMessage("Applied (changed)", store.StatusSuccess)
	m.store.ClearDirty()
	return m, nil
}
