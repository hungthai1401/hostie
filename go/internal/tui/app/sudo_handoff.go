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

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/core/render"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// canWriteEtcHostsFn is the indirection point that lets tests force either
// branch (direct vs sudo) without touching /etc/hosts. Production wires
// apply.CanWriteEtcHosts; tests override it via withCanWriteEtcHosts.
var canWriteEtcHostsFn = apply.CanWriteEtcHosts

// sudoApplyCmdFn is the indirection point for the tea.Cmd that drives
// tea.ExecProcess. Production wires apply.SudoApplyCmd; tests override it to
// observe the call shape (exe, payload, uid) without releasing the TTY.
var sudoApplyCmdFn = apply.SudoApplyCmd

// resolveSelfExeFn is the indirection point for os.Executable + EvalSymlinks.
// Production wires apply.ResolveSelfExe; tests override it to return a stable
// path so the test binary's location does not leak into assertions.
var resolveSelfExeFn = apply.ResolveSelfExe

// writePayloadTempfileFn is the indirection point for the 0600 tempfile
// creation. Production wires apply.WritePayloadToTempfile; tests override it
// to observe the payload bytes and avoid disk I/O.
var writePayloadTempfileFn = apply.WritePayloadToTempfile

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
func applyCmdDispatch(runner applyRunner, hostsFile domain.HostsFile, hostsPath string) tea.Cmd {
	if canWriteEtcHostsFn() {
		return applyCmd(runner, hostsFile)
	}
	return sudoApplyDispatch(hostsFile, hostsPath)
}

// sudoPendingMsg is delivered after sudoApplyDispatch finishes its
// in-process prep work (~/.hosts write + payload tempfile creation) and
// before tea.ExecProcess runs. It carries the cleanup closure so Update can
// stash it on the Model — when apply.SudoFinishedMsg arrives, Update fires
// the cleanup and emits an ApplyResultMsg.
//
// We do NOT chain ExecProcess directly behind the goroutine that does the
// rendering work because tea.ExecProcess must be returned synchronously from
// Update. Splitting prep and exec via this intermediate message keeps the
// blocking I/O off the UI goroutine while still letting Update see the
// ExecProcess Cmd before returning.
type sudoPendingMsg struct {
	// exeCmd is the tea.Cmd returned by apply.SudoApplyCmd. Update returns
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
//   - Rendering the managed block, reading /etc/hosts, and writing ~/.hosts
//     all block on disk I/O; we want them off the UI goroutine.
//   - tea.ExecProcess must be a synchronous return value from Update so the
//     bubbletea runtime can call ReleaseTerminal in the same tick. Update
//     does that synchronous return when it receives sudoPendingMsg.
//
// The hostsPath is needed to write ~/.hosts (D13 step 1). Pass the same path
// the Model was constructed with.
func sudoApplyDispatch(hostsFile domain.HostsFile, hostsPath string) tea.Cmd {
	return func() tea.Msg {
		// Step 1 (D13): write ~/.hosts first; YAML stays on disk even if
		// the /etc/hosts side fails.
		if err := fileio.WriteHostsFile(hostsPath, hostsFile); err != nil {
			return sudoPendingMsg{err: fmt.Errorf("write %s: %w", hostsPath, err)}
		}

		// Step 2: render the new managed block in-process.
		newBlock := render.RenderManagedBlock(&hostsFile)

		// Step 3: build the full /etc/hosts contents (preamble + new block
		// + suffix). The privileged subcommand will overwrite /etc/hosts
		// with these bytes — we cannot defer the merge to the child
		// because the child only sees the payload tempfile.
		currentContent, err := os.ReadFile(apply.ETC_HOSTS_PATH)
		if err != nil && !os.IsNotExist(err) {
			return sudoPendingMsg{err: fmt.Errorf("read /etc/hosts: %w", err)}
		}
		newContent, err := etchosts.ReplaceManagedBlock(currentContent, []byte(newBlock))
		if err != nil {
			return sudoPendingMsg{err: fmt.Errorf("merge managed block: %w", err)}
		}

		// Step 4: write payload to 0600 tempfile under $TMPDIR.
		payloadPath, cleanup, err := writePayloadTempfileFn(newContent)
		if err != nil {
			return sudoPendingMsg{err: fmt.Errorf("create payload tempfile: %w", err)}
		}

		// Step 5: resolve self exe.
		exePath, err := resolveSelfExeFn()
		if err != nil {
			cleanup()
			return sudoPendingMsg{err: fmt.Errorf("resolve self exe: %w", err)}
		}

		// Step 6: build the tea.Cmd that wraps tea.ExecProcess.
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

// handleSudoFinished maps apply.SudoFinishedMsg into ApplyResultMsg, after
// running the deferred tempfile cleanup. The Changed flag is true on
// success: by the time SudoFinishedMsg arrives, the privileged subcommand
// has already rewritten /etc/hosts (or surfaced an error via non-zero exit).
func (m Model) handleSudoFinished(msg apply.SudoFinishedMsg) (Model, tea.Cmd) {
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
