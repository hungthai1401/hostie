// apply_cmd_test.go — covers the applyCmd / ApplyResultMsg plumbing wired by
// hosts-cli-go-mig-p4-app-applycmd-91r.
//
// What's covered:
//
//   - applyCmd success path (Changed=true)              → status "Applied (changed)"
//   - applyCmd success path (Changed=false, idempotent) → status "Applied (no changes)"
//   - applyCmd failure path                              → status "Apply failed: ... — YAML kept"
//     (D13: store stays mutated, dirty flag stays set)
//   - Ctrl+S explicit re-apply                          → dispatches applyCmd again
//   - mutation → ApplyTriggerMsg → applyCmd chain       → end-to-end via Update
//   - applyCmd nil-runner guard                         → ApplyResultMsg.Err set, no panic
//   - applyCmd nil-result guard                         → ApplyResultMsg.Err set, no panic
//   - ApplyResultMsg success clears Dirty               → D11 contract
//   - ApplyResultMsg failure preserves Dirty            → D13 contract
//
// All tests use fakeApplyRunner — never the real apply.Runner — so /etc/hosts
// is never touched.

package app

import (
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/src/internal/apply"
	"github.com/hungthai1401/hostie/src/internal/domain"
	"github.com/hungthai1401/hostie/src/internal/tui/store"
)

// fakeApplyRunner is a test double for apply.Runner that records every call
// and returns canned (result, err). Goroutine-safe so the Bubble Tea runtime
// invoking the Cmd off the UI goroutine doesn't race with assertions.
type fakeApplyRunner struct {
	mu     sync.Mutex
	calls  int
	last   domain.HostsFile
	result *apply.ApplyResult
	err    error

	// PrepareSudoHandoff fake state.
	prepareCalls   int
	prepareLast    domain.HostsFile
	preparePath    string // path to return
	prepareErr     error
	prepareCleanup func() // cleanup closure to return; if nil a no-op is supplied
}

func (f *fakeApplyRunner) Apply(hf domain.HostsFile) (*apply.ApplyResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last = hf
	return f.result, f.err
}

func (f *fakeApplyRunner) PrepareSudoHandoff(hf domain.HostsFile) (string, func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prepareCalls++
	f.prepareLast = hf
	if f.prepareErr != nil {
		return "", nil, f.prepareErr
	}
	cleanup := f.prepareCleanup
	if cleanup == nil {
		cleanup = func() {}
	}
	return f.preparePath, cleanup, nil
}

// drainApplyResult invokes a Cmd and asserts it produced an ApplyResultMsg.
// Returns the message for further assertions.
func drainApplyResult(t *testing.T, cmd tea.Cmd) ApplyResultMsg {
	t.Helper()
	require.NotNil(t, cmd, "expected an apply Cmd, got nil")
	msg := cmd()
	result, ok := msg.(ApplyResultMsg)
	require.True(t, ok, "expected ApplyResultMsg, got %T", msg)
	return result
}

// -----------------------------------------------------------------------------
// applyCmd direct behavior
// -----------------------------------------------------------------------------

// TestApplyCmd_Success_Changed exercises the happy path where the runner
// reports the /etc/hosts content actually changed. The returned Cmd must
// produce an ApplyResultMsg with Changed=true, Err=nil, and the runner must
// have been handed the supplied HostsFile snapshot.
func TestApplyCmd_Success_Changed(t *testing.T) {
	hf := *fixture()
	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: true, Message: "/etc/hosts updated successfully"}}

	cmd := applyCmd(fake, hf)
	result := drainApplyResult(t, cmd)

	require.NoError(t, result.Err)
	require.True(t, result.Changed)
	require.Equal(t, "/etc/hosts updated successfully", result.Message)
	require.Equal(t, 1, fake.calls)
	require.Len(t, fake.last.Groups, len(hf.Groups), "runner must see the snapshot")
}

// TestApplyCmd_Success_NoChange covers the idempotent re-apply path where the
// runner reports the rendered block already matched /etc/hosts.
func TestApplyCmd_Success_NoChange(t *testing.T) {
	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: false, Message: "already up to date"}}
	result := drainApplyResult(t, applyCmd(fake, *fixture()))
	require.NoError(t, result.Err)
	require.False(t, result.Changed)
}

// TestApplyCmd_Failure covers the apply-fails path. The Cmd must surface the
// runner's error verbatim in ApplyResultMsg.Err so Update can render the
// "Apply failed: <err> — YAML kept" banner.
func TestApplyCmd_Failure(t *testing.T) {
	want := errors.New("permission denied writing /etc/hosts")
	fake := &fakeApplyRunner{err: want}
	result := drainApplyResult(t, applyCmd(fake, *fixture()))
	require.ErrorIs(t, result.Err, want)
}

// TestApplyCmd_NilRunner verifies the defensive nil-runner guard — the Cmd
// must yield an ApplyResultMsg.Err rather than panicking inside the Bubble Tea
// goroutine.
func TestApplyCmd_NilRunner(t *testing.T) {
	result := drainApplyResult(t, applyCmd(nil, *fixture()))
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "not configured")
}

// TestApplyCmd_NilResult verifies the defensive nil-result guard for a
// misbehaving runner that returns (nil, nil).
func TestApplyCmd_NilResult(t *testing.T) {
	fake := &fakeApplyRunner{result: nil, err: nil}
	result := drainApplyResult(t, applyCmd(fake, *fixture()))
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "no result")
}

// -----------------------------------------------------------------------------
// handleApplyResult → StatusBar plumbing
// -----------------------------------------------------------------------------

// TestApplyResult_Success_Changed_SetsGreenStatus verifies the success banner
// text and color, and that Dirty is cleared per D11.
func TestApplyResult_Success_Changed_SetsGreenStatus(t *testing.T) {
	m := seedModel(t)
	m.store.MarkDirty()
	require.True(t, m.Store().Dirty())

	m2, cmd := m.Update(ApplyResultMsg{Changed: true, Message: "ok"})
	require.Nil(t, cmd, "ApplyResultMsg dispatch must not return a follow-up Cmd")
	mm := m2.(Model)
	status := mm.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
	require.Equal(t, "Applied (changed)", status.Text)
	require.False(t, mm.Store().Dirty(), "successful apply must clear dirty flag")
}

// TestApplyResult_Success_NoChange_SetsGreenStatus covers the idempotent
// re-apply banner.
func TestApplyResult_Success_NoChange_SetsGreenStatus(t *testing.T) {
	m := seedModel(t)
	m2, _ := m.Update(ApplyResultMsg{Changed: false})
	status := m2.(Model).Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
	require.Equal(t, "Applied (no changes)", status.Text)
}

// TestApplyResult_Failure_SetsRedStatus_PreservesDirty verifies D13: apply
// failure surfaces a red banner whose text includes the error and the "YAML
// kept" reminder, and the dirty flag is NOT cleared (because the in-memory
// state is now ahead of /etc/hosts).
func TestApplyResult_Failure_SetsRedStatus_PreservesDirty(t *testing.T) {
	m := seedModel(t)
	m.store.MarkDirty()
	require.True(t, m.Store().Dirty())

	hostsBefore := *m.Store().HostsFile()

	failure := errors.New("permission denied")
	m2, _ := m.Update(ApplyResultMsg{Err: failure})
	mm := m2.(Model)

	status := mm.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.True(t, strings.HasPrefix(status.Text, "Apply failed: "), "got %q", status.Text)
	require.Contains(t, status.Text, "permission denied")
	require.Contains(t, status.Text, "YAML kept")

	require.True(t, mm.Store().Dirty(), "failed apply must NOT clear dirty (D13)")
	require.Equal(t, len(hostsBefore.Groups), len(mm.Store().HostsFile().Groups),
		"failed apply must NOT roll back store state (D13)")
}

// -----------------------------------------------------------------------------
// Ctrl+S — explicit re-apply
// -----------------------------------------------------------------------------

// TestCtrlS_Reapply_DispatchesApply verifies the Ctrl+S keybind re-runs the
// apply pipeline against the current store snapshot without performing a
// mutation first. This is the documented D13 recovery path: after an
// auto-apply failure, the operator can retry without forcing a dummy edit.
func TestCtrlS_Reapply_DispatchesApply(t *testing.T) {
	defer withCanWriteEtcHosts(true)()
	m := seedModel(t)
	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: true, Message: "ok"}}
	m = m.WithApplyRunner(fake)

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd, "Ctrl+S must dispatch the apply Cmd")

	result := drainApplyResult(t, cmd)
	require.NoError(t, result.Err)
	require.True(t, result.Changed)
	require.Equal(t, 1, fake.calls)

	// Selection / focus must not change as a side effect of Ctrl+S.
	require.Equal(t, m.Focus(), m2.(Model).Focus())
}

// TestCtrlS_Reapply_EmptyStore_StillDispatches verifies Ctrl+S works against
// a freshly-constructed store (empty HostsFile but non-nil). The runner is
// invoked with the empty file — apply.Runner handles the empty render
// correctly (returns "no changes").
func TestCtrlS_Reapply_EmptyStore_StillDispatches(t *testing.T) {
	defer withCanWriteEtcHosts(true)()
	m := NewModel("/dev/null")
	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: false}}
	m = m.WithApplyRunner(fake)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd, "Ctrl+S with empty store still dispatches a re-apply")
	result := drainApplyResult(t, cmd)
	require.NoError(t, result.Err)
	require.Equal(t, 1, fake.calls)
}

// -----------------------------------------------------------------------------
// End-to-end: mutation → ApplyTriggerMsg → applyCmd → ApplyResultMsg → status
// -----------------------------------------------------------------------------

// TestMutationToApplyChain_Success drives the full chain for a Space toggle:
// store mutates, ApplyTriggerMsg fires, applyCmd runs against the fake, the
// resulting ApplyResultMsg lands back in Update and the StatusBar reflects
// success.
func TestMutationToApplyChain_Success(t *testing.T) {
	defer withCanWriteEtcHosts(true)()
	m := seedAndSelect(t, "e1")
	fake := &fakeApplyRunner{result: &apply.ApplyResult{Changed: true, Message: "ok"}}
	m = m.WithApplyRunner(fake)

	// Step 1: Space toggles the entry and emits triggerApply().
	m2, cmd := m.Update(key(" "))
	mm := m2.(Model)
	require.False(t, mm.Store().HostsFile().Groups[0].Entries[0].Enabled, "Space must toggle")
	require.NotNil(t, cmd, "Space must produce an ApplyTriggerMsg Cmd")

	// Step 2: that Cmd yields ApplyTriggerMsg.
	trig := cmd()
	_, ok := trig.(ApplyTriggerMsg)
	require.True(t, ok, "expected ApplyTriggerMsg, got %T", trig)

	// Step 3: feed ApplyTriggerMsg through Update → it dispatches applyCmd.
	m3, applyCmdResult := mm.Update(trig)
	mm = m3.(Model)
	require.NotNil(t, applyCmdResult, "ApplyTriggerMsg must dispatch the apply Cmd")

	// Step 4: drain → ApplyResultMsg.
	result := drainApplyResult(t, applyCmdResult)
	require.NoError(t, result.Err)
	require.True(t, result.Changed)
	require.Equal(t, 1, fake.calls)

	// Step 5: feed ApplyResultMsg through Update → status set to success.
	m4, _ := mm.Update(result)
	status := m4.(Model).Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusSuccess, status.Level)
	require.Equal(t, "Applied (changed)", status.Text)
}

// TestMutationToApplyChain_FailureKeepsYAML drives the full chain for a Space
// toggle where the apply step fails. Per D13: the store mutation stands, the
// YAML "would have been" written (the runner attempted it before returning
// the error), and the StatusBar shows the failure banner.
//
// We verify the store-side invariants directly: mutation visible in store,
// dirty flag preserved, status banner red with "YAML kept" suffix.
func TestMutationToApplyChain_FailureKeepsYAML(t *testing.T) {
	defer withCanWriteEtcHosts(true)()
	m := seedAndSelect(t, "e1")
	failure := errors.New("etc/hosts permission denied")
	fake := &fakeApplyRunner{err: failure}
	m = m.WithApplyRunner(fake)

	// Space toggles AND captures the resulting Cmd chain.
	m2, cmd := m.Update(key(" "))
	mm := m2.(Model)
	require.False(t, mm.Store().HostsFile().Groups[0].Entries[0].Enabled, "store mutation must stick (D13)")

	// Drive the chain.
	trig := cmd()
	_, isTrigger := trig.(ApplyTriggerMsg)
	require.True(t, isTrigger)

	m3, applyCmdResult := mm.Update(trig)
	mm = m3.(Model)
	require.NotNil(t, applyCmdResult)

	result := drainApplyResult(t, applyCmdResult)
	require.ErrorIs(t, result.Err, failure)

	// Final state: store still mutated, YAML kept, dirty preserved, red banner.
	m4, _ := mm.Update(result)
	mm = m4.(Model)
	require.False(t, mm.Store().HostsFile().Groups[0].Entries[0].Enabled,
		"D13: failed apply must NOT roll back the store mutation")
	require.True(t, mm.Store().Dirty(), "D13: failed apply must NOT clear dirty")

	status := mm.Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.Contains(t, status.Text, "etc/hosts permission denied")
	require.Contains(t, status.Text, "YAML kept")
}
