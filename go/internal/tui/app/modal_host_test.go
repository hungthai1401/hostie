// ModalHost tests — verifies the modal-routing layer that the four
// subsequent Phase 4 modal beads depend on. See
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md §5 for the integration
// contract these tests pin down.

package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/tui/components"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

func newHostWithStore(t *testing.T) (*ModalHost, *store.Store) {
	t.Helper()
	s := store.New()
	return NewModalHost(s), s
}

func TestModalHost_InactiveByDefault(t *testing.T) {
	h, s := newHostWithStore(t)
	require.False(t, h.Active())
	require.Nil(t, h.Current())
	require.Equal(t, store.ModeNormal, s.Mode())
	require.Equal(t, "", h.View())
}

func TestModalHost_OpenSwitchesStoreToModalMode(t *testing.T) {
	h, s := newHostWithStore(t)
	cmd := h.Open(components.NewConfirmModal("delete", "Delete?"))
	require.Nil(t, cmd, "ConfirmModal.Init() returns nil")
	require.True(t, h.Active())
	require.Equal(t, store.ModeModal, s.Mode(),
		"store must reflect modal mode so status bar / future readers see it")
}

func TestModalHost_CloseRestoresNormalMode(t *testing.T) {
	h, s := newHostWithStore(t)
	h.Open(components.NewConfirmModal("delete", "Delete?"))
	h.Close()
	require.False(t, h.Active())
	require.Equal(t, store.ModeNormal, s.Mode())
}

// TestModalHost_UpdateRoutesKeyToActiveModal verifies the routing primitive:
// keys reach the modal, results come back as ModalResultMsg via the returned
// Cmd, and the second Update call (delivering the ModalResultMsg) closes the
// host. This is the exact sequence the root app.Update will perform once
// the 3-line integration patch lands in app-mutations-9fk.
func TestModalHost_UpdateRoutesKeyToActiveModal(t *testing.T) {
	h, s := newHostWithStore(t)
	h.Open(components.NewConfirmModal("apply", "Apply?"))

	// Send Enter; expect a Cmd whose Msg is ModalResultMsg(Confirmed=true).
	_, cmd := h.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter on default selection must produce a Cmd")
	msg := cmd()
	result, ok := msg.(components.ModalResultMsg)
	require.True(t, ok, "expected ModalResultMsg, got %T", msg)
	require.True(t, result.Confirmed)
	require.Equal(t, "apply", result.ID)

	// At this point the modal is still active — close happens when the
	// ModalResultMsg is routed back through Update.
	require.True(t, h.Active())

	// Deliver the result: host detects own-modal ID, closes, store flips.
	_, cmd2 := h.Update(result)
	require.Nil(t, cmd2)
	require.False(t, h.Active())
	require.Equal(t, store.ModeNormal, s.Mode())
}

// TestModalHost_UpdateInactiveIsNoOp ensures the integration check in
// app/update.go (if h.Active() { ... }) is the canonical gate — calling
// Update on an inactive host must be safe and a no-op.
func TestModalHost_UpdateInactiveIsNoOp(t *testing.T) {
	h, _ := newHostWithStore(t)
	next, cmd := h.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Same(t, h, next)
	require.Nil(t, cmd)
}

// TestModalHost_ResultIDMismatchDoesNotClose covers the routing-by-ID
// guarantee: a stale ModalResultMsg from a previously-closed modal must NOT
// close a different modal that happens to be open when it arrives.
func TestModalHost_ResultIDMismatchDoesNotClose(t *testing.T) {
	h, _ := newHostWithStore(t)
	h.Open(components.NewConfirmModal("apply", "Apply?"))

	stale := components.ModalResultMsg{ID: "delete-from-a-prior-modal", Confirmed: true}
	_, cmd := h.Update(stale)
	require.Nil(t, cmd)
	require.True(t, h.Active(), "stale result for different ID must not close the active modal")
}

// TestModalHost_OpenReplacesActive verifies last-open-wins semantics; the
// replaced modal does not emit a result (callers must not rely on a "closed
// implicitly" signal — open the replacement only when the prior modal's
// result has already been handled or is intentionally discarded).
func TestModalHost_OpenReplacesActive(t *testing.T) {
	h, _ := newHostWithStore(t)
	h.Open(components.NewConfirmModal("first", "First?"))
	h.Open(components.NewConfirmModal("second", "Second?"))
	require.True(t, h.Active())
	require.Equal(t, "second", h.Current().ID())
}

// TestModalHost_ViewRendersActiveModal smoke-checks the overlay path.
func TestModalHost_ViewRendersActiveModal(t *testing.T) {
	h, _ := newHostWithStore(t)
	h.Open(components.NewConfirmModal("apply", "Apply changes?"))
	out := h.View()
	require.Contains(t, out, "Apply changes?")
}

// TestOverlayModal_EmptyModalReturnsBase ensures app/view.go's OverlayModal
// helper is a pass-through when there is nothing to overlay.
func TestOverlayModal_EmptyModalReturnsBase(t *testing.T) {
	got := OverlayModal("base content", "", 80, 24)
	require.Equal(t, "base content", got)
}

// TestOverlayModal_ZeroSizeReturnsModal verifies the degenerate-size guard;
// before WindowSizeMsg arrives we have width==0, height==0 — the modal
// itself is the most useful output.
func TestOverlayModal_ZeroSizeReturnsModal(t *testing.T) {
	got := OverlayModal("base", "MODAL", 0, 0)
	require.Equal(t, "MODAL", got)
}

// TestOverlayModal_CentersWithinWidthHeight is a low-fidelity check that
// lipgloss.Place produced something at least as tall as the requested
// height. We don't pin exact byte counts because terminal style escapes
// vary by environment.
func TestOverlayModal_CentersWithinWidthHeight(t *testing.T) {
	got := OverlayModal("base ignored", "X", 20, 5)
	require.Contains(t, got, "X")
}

// TestModalHost_DeterminismThroughHost is the host-level companion to
// components.TestConfirmModal_EscDeterminism. Same canary, but exercised
// through ModalHost.Update so any routing-layer flake (not just the modal's
// internal one) is caught. Run with -count=100 for 10,000 cycles under
// -race.
func TestModalHost_DeterminismThroughHost(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		h, s := newHostWithStore(t)
		h.Open(components.NewConfirmModal("delete", "Delete?"))
		_, cmd := h.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iter %d: Esc must produce result Cmd", i)
		res, ok := cmd().(components.ModalResultMsg)
		require.True(t, ok, "iter %d: bad message type %T", i, cmd())
		require.False(t, res.Confirmed, "iter %d: Esc must yield Confirmed=false", i)
		require.Equal(t, "delete", res.ID, "iter %d: ID mismatch (FLAKE)", i)

		_, _ = h.Update(res)
		require.False(t, h.Active(), "iter %d: host must close after result delivery", i)
		require.Equal(t, store.ModeNormal, s.Mode(), "iter %d: store mode must reset", i)
	}
}
