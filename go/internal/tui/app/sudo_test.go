// sudo_test.go — covers the TUI sudo handoff branch wired by
// hosts-cli-go-mig-p4-sudo-wire-jpr.
//
// Test matrix:
//
//   - applyCmdDispatch CanWrite==true  → direct path (applyCmd, no ExecProcess)
//   - applyCmdDispatch CanWrite==false → sudoPendingMsg, then handler returns
//                                        the ExecProcess Cmd from the prep
//                                        result
//   - sudoPendingMsg success → cleanup stashed, exeCmd returned
//   - sudoPendingMsg error   → ApplyResultMsg{Err} via handleApplyResult,
//                              cleanup NOT stashed (nothing to clean)
//   - SudoFinishedMsg success → cleanup fires, status banner green, dirty cleared
//   - SudoFinishedMsg failure → cleanup fires, status banner red with exit code
//   - End-to-end: ApplyTriggerMsg with CanWrite==false → sudoPendingMsg pipeline
//
// All tests stub SudoApplyCmd / apply.WritePayloadToTempfile /
// apply.ResolveSelfExe / apply.CanWriteEtcHosts so no real /etc/hosts I/O
// happens and no TTY is touched.

package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// stubSudoDeps installs in-memory replacements for the package-level
// indirection points used by the sudo branch. Returns a sudoObservation
// populated as the stubs run. Note that the payload-tempfile and ~/.hosts
// writes now happen inside the applyRunner (via PrepareSudoHandoff) — the
// fakeApplyRunner set on the Model is the stub for those concerns. This
// helper only stubs the two TUI-layer indirections that remain:
// canWriteEtcHostsFn, resolveSelfExeFn, and sudoApplyCmdFn.
type sudoObservation struct {
	cleanupCalls int
	sudoCmdCalls int
	lastExe      string
	lastPayload  string
	lastUID      int
	sudoMsg      tea.Msg // message returned by the fake sudo Cmd
}

func stubSudoDeps(t *testing.T, canWrite bool, sudoMsg tea.Msg) *sudoObservation {
	t.Helper()
	obs := &sudoObservation{sudoMsg: sudoMsg}

	prevCan := canWriteEtcHostsFn
	prevResolve := resolveSelfExeFn
	prevSudo := sudoApplyCmdFn

	canWriteEtcHostsFn = func() bool { return canWrite }
	resolveSelfExeFn = func() (string, error) {
		return "/usr/local/bin/hostie-stub", nil
	}
	sudoApplyCmdFn = func(exe, payload string, uid int) tea.Cmd {
		obs.sudoCmdCalls++
		obs.lastExe = exe
		obs.lastPayload = payload
		obs.lastUID = uid
		return func() tea.Msg { return obs.sudoMsg }
	}

	t.Cleanup(func() {
		canWriteEtcHostsFn = prevCan
		resolveSelfExeFn = prevResolve
		sudoApplyCmdFn = prevSudo
	})
	return obs
}

// stubbedSudoRunner returns a fakeApplyRunner pre-wired for the sudo path:
// PrepareSudoHandoff returns a stub payload path and a cleanup that
// increments the supplied observation. Use this with stubSudoDeps when a
// test needs to drive sudoApplyDispatch end-to-end.
func stubbedSudoRunner(t *testing.T, obs *sudoObservation) *fakeApplyRunner {
	t.Helper()
	path := filepath.Join(os.TempDir(), "hostie-stub-payload-test")
	return &fakeApplyRunner{
		result:         &apply.ApplyResult{Changed: true, Message: "ok"},
		preparePath:    path,
		prepareCleanup: func() { obs.cleanupCalls++ },
	}
}

// seedSudoModel returns a Model with a hosts file loaded and a fake apply
// runner installed. Uses a tempdir hosts path so the sudo prep step's
// fileio.WriteHostsFile call against ~/.hosts succeeds without polluting the
// user's real file.
func seedSudoModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")

	m := NewModel(hostsPath)
	m.store.LoadHostsFile(fixture())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mm := m2.(Model)
	mm = mm.WithApplyRunner(&fakeApplyRunner{
		result: &apply.ApplyResult{Changed: true, Message: "ok"},
	})
	return mm
}

// -----------------------------------------------------------------------------
// applyCmdDispatch routing
// -----------------------------------------------------------------------------

// TestApplyCmdDispatch_CanWriteTrue_TakesDirectPath verifies that when the
// process can write /etc/hosts directly the dispatcher returns the direct
// applyCmd (which yields ApplyResultMsg from the fake runner) — no
// sudoPendingMsg should ever fire.
func TestApplyCmdDispatch_CanWriteTrue_TakesDirectPath(t *testing.T) {
	restore := withCanWriteEtcHosts(true)
	defer restore()

	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: true, Message: "ok"}}
	cmd := applyCmdDispatch(fake, *fixture(), "/dev/null")
	require.NotNil(t, cmd)

	msg := cmd()
	result, ok := msg.(ApplyResultMsg)
	require.True(t, ok, "direct path must yield ApplyResultMsg, got %T", msg)
	require.NoError(t, result.Err)
	require.True(t, result.Changed)
	require.Equal(t, 1, fake.calls, "direct path must invoke runner.Apply")
}

// TestApplyCmdDispatch_CanWriteFalse_TakesSudoPath verifies that when
// CanWriteEtcHosts returns false, the dispatcher yields sudoPendingMsg
// instead of ApplyResultMsg, and the fake runner is NOT invoked (the sudo
// path bypasses apply.Runner entirely — the privileged subcommand performs
// the actual /etc/hosts write).
func TestApplyCmdDispatch_CanWriteFalse_TakesSudoPath(t *testing.T) {
	obs := stubSudoDeps(t, false, SudoFinishedMsg{})

	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.yaml")

	stubPath := filepath.Join(os.TempDir(), "hostie-stub-payload-test")
	fake := &fakeApplyRunner{
		result:         &apply.ApplyResult{Changed: true, Message: "ok"},
		preparePath:    stubPath,
		prepareCleanup: func() { obs.cleanupCalls++ },
	}
	cmd := applyCmdDispatch(fake, *fixture(), hostsPath)
	require.NotNil(t, cmd)

	msg := cmd()
	pending, ok := msg.(sudoPendingMsg)
	require.True(t, ok, "sudo path must yield sudoPendingMsg, got %T", msg)
	require.NoError(t, pending.err)
	require.NotNil(t, pending.exeCmd)
	require.NotNil(t, pending.cleanup)

	require.Equal(t, 0, fake.calls, "sudo path must NOT invoke runner.Apply")
	require.Equal(t, 1, fake.prepareCalls, "sudo path must invoke runner.PrepareSudoHandoff")
	require.Equal(t, stubPath, obs.lastPayload, "SudoApplyCmd must receive the runner-supplied payload path")
	require.Equal(t, "/usr/local/bin/hostie-stub", obs.lastExe)
	require.Equal(t, os.Getuid(), obs.lastUID)

	// ~/.hosts write is now the runner's responsibility (verified
	// in TestRunner_PrepareSudoHandoff_WritesHostsAndPayload). Here we only
	// assert the TUI-layer wiring: the runner was invoked. The fake does
	// not touch the filesystem, so hostsPath remains absent.
	_, err := os.Stat(hostsPath)
	require.True(t, os.IsNotExist(err), "fake runner does not touch ~/.hosts; real runner is covered by apply tests")
}

// -----------------------------------------------------------------------------
// sudoPendingMsg handler
// -----------------------------------------------------------------------------

// TestHandleSudoPending_Success stashes the cleanup on the Model and
// returns the ExecProcess Cmd unchanged so Bubble Tea can release the TTY.
func TestHandleSudoPending_Success(t *testing.T) {
	m := seedSudoModel(t)
	cleanupCalled := 0
	exeCmd := func() tea.Msg { return SudoFinishedMsg{} }

	m2, cmd := m.handleSudoPending(sudoPendingMsg{
		exeCmd:  exeCmd,
		cleanup: func() { cleanupCalled++ },
	})
	require.NotNil(t, cmd, "handleSudoPending must return the exec Cmd")
	require.NotNil(t, m2.pendingSudoCleanup, "cleanup must be stashed on Model")
	require.Equal(t, 0, cleanupCalled, "cleanup must not fire until SudoFinishedMsg")
}

// TestHandleSudoPending_Error surfaces a red banner via handleApplyResult and
// does NOT stash a cleanup (the prep failed before the tempfile was created,
// so there is nothing to clean).
func TestHandleSudoPending_Error(t *testing.T) {
	m := seedSudoModel(t)
	m.store.MarkDirty()

	prepErr := errors.New("payload tempfile failed")
	m2, cmd := m.handleSudoPending(sudoPendingMsg{err: prepErr})
	require.Nil(t, cmd)
	require.Nil(t, m2.pendingSudoCleanup)

	status := m2.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.Contains(t, status.Text, "payload tempfile failed")
	require.Contains(t, status.Text, "YAML kept")
	require.True(t, m2.Store().Dirty(), "prep failure must NOT clear dirty (D13)")
}

// -----------------------------------------------------------------------------
// SudoFinishedMsg handler
// -----------------------------------------------------------------------------

// TestHandleSudoFinished_Success_CleansAndGreenBanner verifies that on a
// zero-exit return the cleanup fires, the status bar shows green, and the
// dirty flag is cleared (D11 contract).
func TestHandleSudoFinished_Success_CleansAndGreenBanner(t *testing.T) {
	m := seedSudoModel(t)
	m.store.MarkDirty()
	cleanupCalls := 0
	m.pendingSudoCleanup = func() { cleanupCalls++ }

	m2, cmd := m.handleSudoFinished(SudoFinishedMsg{ExitCode: 0})
	require.Nil(t, cmd)
	require.Equal(t, 1, cleanupCalls, "success must run the tempfile cleanup")
	require.Nil(t, m2.pendingSudoCleanup, "cleanup pointer must be nil'd after firing")

	status := m2.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
	require.Equal(t, "Applied (changed)", status.Text)
	require.False(t, m2.Store().Dirty(), "successful sudo apply must clear dirty")
}

// TestHandleSudoFinished_Failure_CleansAndRedBanner verifies that a non-zero
// exit fires the cleanup, surfaces a red banner including the exit code,
// preserves the dirty flag, and never panics on a nil Err with a positive
// ExitCode (defensive).
func TestHandleSudoFinished_Failure_CleansAndRedBanner(t *testing.T) {
	m := seedSudoModel(t)
	m.store.MarkDirty()
	cleanupCalls := 0
	m.pendingSudoCleanup = func() { cleanupCalls++ }

	wrongPw := errors.New("exit status 1")
	m2, _ := m.handleSudoFinished(SudoFinishedMsg{Err: wrongPw, ExitCode: 1})
	require.Equal(t, 1, cleanupCalls, "failure must STILL run the tempfile cleanup (D13)")
	require.Nil(t, m2.pendingSudoCleanup)

	status := m2.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.Contains(t, status.Text, "exit 1")
	require.Contains(t, status.Text, "YAML kept")
	require.True(t, m2.Store().Dirty(), "failed sudo apply must NOT clear dirty (D13)")
}

// TestHandleSudoFinished_NoCleanup is the defensive path: if for any reason
// pendingSudoCleanup was never set (a programming bug, or a SudoFinishedMsg
// arriving with no prior sudoPendingMsg), the handler must still produce a
// valid status banner without panicking.
func TestHandleSudoFinished_NoCleanup(t *testing.T) {
	m := seedSudoModel(t)
	require.Nil(t, m.pendingSudoCleanup)
	m2, _ := m.handleSudoFinished(SudoFinishedMsg{ExitCode: 0})
	status := m2.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
}

// -----------------------------------------------------------------------------
// End-to-end: ApplyTriggerMsg under CanWrite=false routes through Update
// -----------------------------------------------------------------------------

// TestApplyTrigger_SudoPath_EndToEnd drives the full Update sequence:
// ApplyTriggerMsg → sudoPendingMsg → (stub exeCmd) → SudoFinishedMsg →
// success banner + dirty cleared. Verifies no fake runner Apply call and
// that the tempfile cleanup fires exactly once.
func TestApplyTrigger_SudoPath_EndToEnd(t *testing.T) {
	obs := stubSudoDeps(t, false, SudoFinishedMsg{ExitCode: 0})

	m := seedSudoModel(t)
	m = m.WithApplyRunner(stubbedSudoRunner(t, obs))
	m.store.MarkDirty()

	// Step 1: ApplyTriggerMsg → cmd that yields sudoPendingMsg.
	m2, cmd := m.Update(ApplyTriggerMsg{})
	require.NotNil(t, cmd)
	mm := m2.(Model)

	msg := cmd()
	pending, ok := msg.(sudoPendingMsg)
	require.True(t, ok, "expected sudoPendingMsg, got %T", msg)
	require.NoError(t, pending.err)

	// Step 2: feed sudoPendingMsg → Update returns exeCmd, stashes cleanup.
	m3, execCmd := mm.Update(pending)
	require.NotNil(t, execCmd, "expected exec Cmd to be returned")
	mm = m3.(Model)
	require.NotNil(t, mm.pendingSudoCleanup)

	// Step 3: invoke the (stubbed) exec Cmd → SudoFinishedMsg.
	finishedMsg := execCmd()
	finished, ok := finishedMsg.(SudoFinishedMsg)
	require.True(t, ok, "expected SudoFinishedMsg, got %T", finishedMsg)
	require.NoError(t, finished.Err)

	// Step 4: feed SudoFinishedMsg → cleanup runs, banner is green.
	m4, _ := mm.Update(finished)
	mm = m4.(Model)
	require.Equal(t, 1, obs.cleanupCalls, "cleanup must fire exactly once")
	require.Nil(t, mm.pendingSudoCleanup)

	status := mm.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
	require.Equal(t, "Applied (changed)", status.Text)
	require.False(t, mm.Store().Dirty())
	require.Equal(t, 1, obs.sudoCmdCalls, "exactly one tea.ExecProcess invocation")
}

// TestApplyTrigger_SudoPath_FailurePreservesDirty drives the same end-to-end
// chain but with a non-zero exit; verifies the failure banner and that
// dirty/YAML invariants from D13 hold.
func TestApplyTrigger_SudoPath_FailurePreservesDirty(t *testing.T) {
	obs := stubSudoDeps(t, false, SudoFinishedMsg{
		Err:      errors.New("exit status 1"),
		ExitCode: 1,
	})

	m := seedSudoModel(t)
	m = m.WithApplyRunner(stubbedSudoRunner(t, obs))
	m.store.MarkDirty()

	m2, cmd := m.Update(ApplyTriggerMsg{})
	pending := cmd().(sudoPendingMsg)
	require.NoError(t, pending.err)

	m3, execCmd := m2.(Model).Update(pending)
	finished := execCmd().(SudoFinishedMsg)
	m4, _ := m3.(Model).Update(finished)
	mm := m4.(Model)

	require.Equal(t, 1, obs.cleanupCalls)
	status := mm.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.Contains(t, status.Text, "exit 1")
	require.True(t, mm.Store().Dirty(), "D13: failed sudo apply preserves dirty")
}
