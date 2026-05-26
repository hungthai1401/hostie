// ConfirmModal tests — ports v1 src/tui/components/__tests__/ConfirmModal.test.tsx
// plus a determinism harness for the v1 MoveToGroup Esc flake canary.
//
// See .spikes/go-migration/p4-modal-pattern/FINDINGS.md for the methodology
// behind TestConfirmModal_EscDeterminism (run with go test -count=100 or
// higher; the in-process loop here also performs ≥100 iterations per run).

package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// runMsg pushes a single message through the modal and returns the next
// modal plus any ModalResultMsg the modal's Cmd produced (nil if the modal
// did not emit a result). Centralizes the boilerplate every test case
// shares: type-assert the Modal back to ConfirmModal, run the Cmd, decode
// the result.
func runMsg(t *testing.T, m Modal, msg tea.Msg) (ConfirmModal, *ModalResultMsg) {
	t.Helper()
	next, cmd := m.Update(msg)
	cm, ok := next.(ConfirmModal)
	require.True(t, ok, "Update must preserve concrete type, got %T", next)

	if cmd == nil {
		return cm, nil
	}
	out := cmd()
	res, ok := out.(ModalResultMsg)
	require.True(t, ok, "expected ModalResultMsg from emit Cmd, got %T", out)
	return cm, &res
}

// runeKey constructs a tea.KeyMsg for a single rune. Mirrors the helper in
// app/app_test.go so test patterns stay consistent across the modal/host
// boundary.
func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// --- Render tests -----------------------------------------------------------

func TestConfirmModal_RendersMessageAndButtons(t *testing.T) {
	m := NewConfirmModal("delete-entry", "Delete this entry?")
	out := m.View()
	require.Contains(t, out, "Delete this entry?")
	require.Contains(t, out, "Yes")
	require.Contains(t, out, "No")
	// Hint line is present so first-time users see the controls.
	require.Contains(t, out, "Esc to cancel")
}

func TestConfirmModal_DefaultsToYes(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	require.Equal(t, confirmYes, m.Selected())
}

// --- Routing tests (one per v1 case + Enter+No) -----------------------------

func TestConfirmModal_EnterConfirmsYesByDefault(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	_, res := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
	require.Equal(t, "apply", res.ID)
}

func TestConfirmModal_EscEmitsCancel(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	_, res := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, res)
	require.False(t, res.Confirmed)
	require.Equal(t, "apply", res.ID)
}

func TestConfirmModal_RightArrowSelectsNoThenEnterCancels(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	m2, res := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, res, "arrow key alone must not emit result")
	require.Equal(t, confirmNo, m2.Selected())

	_, res2 := runMsg(t, m2, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res2)
	require.False(t, res2.Confirmed, "Enter on No must emit Confirmed=false")
}

func TestConfirmModal_LeftArrowReturnsToYes(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRight})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, confirmYes, m.Selected())

	_, res := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
}

func TestConfirmModal_YKeyConfirms(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	for _, r := range []rune{'y', 'Y'} {
		_, res := runMsg(t, m, runeKey(r))
		require.NotNil(t, res, "rune %q must emit", r)
		require.True(t, res.Confirmed, "rune %q must confirm", r)
	}
}

func TestConfirmModal_NKeyCancels(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	for _, r := range []rune{'n', 'N'} {
		_, res := runMsg(t, m, runeKey(r))
		require.NotNil(t, res, "rune %q must emit", r)
		require.False(t, res.Confirmed, "rune %q must cancel", r)
	}
}

func TestConfirmModal_UnknownKeyIsNoOp(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	// 'q', 'j', 'k' must not produce a result (those are root-Update keys;
	// the spike contract says only y/n/Enter/Esc/arrows are consumed).
	for _, r := range []rune{'q', 'j', 'k', 'x', ' '} {
		m2, res := runMsg(t, m, runeKey(r))
		require.Nil(t, res, "rune %q must not emit", r)
		require.Equal(t, m.Selected(), m2.Selected(), "rune %q must not change selection", r)
	}
}

func TestConfirmModal_NonKeyMsgIsPassThrough(t *testing.T) {
	m := NewConfirmModal("apply", "Apply changes?")
	// WindowSizeMsg must NOT be consumed by the modal — the base layout
	// needs to receive it. Modal returns (self, nil) for non-key events.
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	require.Nil(t, cmd)
	require.Equal(t, m, next)
}

// --- Determinism canary -----------------------------------------------------

// TestConfirmModal_EscDeterminism is the spike's primary deliverable: the
// canary that forecloses the v1 MoveToGroupModal Esc-routing flake.
//
// Methodology: open a fresh ConfirmModal, send Esc, decode the emitted
// ModalResultMsg, assert (Confirmed=false, ID matches) — repeated for N
// iterations. ANY divergence (missing result, wrong Confirmed value,
// missing ID, or a panic) fails the test. We track per-iteration outcomes
// in a slice rather than failing fast so that a flaky test shows the
// failure pattern, not just the first occurrence.
//
// Combined with `go test -count=100 -race`, this exercises 10,000 cycles
// of the modal contract under the race detector. The pattern is the
// template every subsequent modal bead's test must implement (MoveToGroup
// in particular — that's where the v1 flake lived).
func TestConfirmModal_EscDeterminism(t *testing.T) {
	const iterations = 100

	results := make([]bool, 0, iterations)
	ids := make([]string, 0, iterations)

	for i := 0; i < iterations; i++ {
		m := NewConfirmModal("delete-entry", "Delete this entry?")
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iteration %d: Esc must produce a Cmd", i)
		msg := cmd()
		res, ok := msg.(ModalResultMsg)
		require.True(t, ok, "iteration %d: expected ModalResultMsg, got %T", i, msg)
		results = append(results, res.Confirmed)
		ids = append(ids, res.ID)
	}

	// Every iteration must report Confirmed=false and the same ID.
	for i, c := range results {
		require.False(t, c, "iteration %d returned Confirmed=true (FLAKE)", i)
	}
	for i, id := range ids {
		require.Equal(t, "delete-entry", id, "iteration %d returned wrong ID (FLAKE)", i)
	}
}

// TestConfirmModal_EnterDeterminism is the companion canary for Enter →
// Confirmed=true. Same methodology as the Esc canary; together they assert
// the routing table has zero ambiguity.
func TestConfirmModal_EnterDeterminism(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		m := NewConfirmModal("apply", "Apply changes?")
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.True(t, res.Confirmed, "iteration %d: Enter on default selection must confirm", i)
	}
}

// TestConfirmModal_MixedSequenceDeterminism stresses the full keymap by
// running a sequence (Right → Left → Right → 'y') 100 times and asserting
// the final state is identical every cycle. This catches state-leak bugs
// where one iteration's selection carries into the next (a class of bug
// hidden by single-shot tests).
func TestConfirmModal_MixedSequenceDeterminism(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		m := Modal(NewConfirmModal("mixed", "Continue?"))
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		_, cmd := m.Update(runeKey('y'))
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok)
		// 'y' is unconditional confirm regardless of current selection.
		require.True(t, res.Confirmed, "iteration %d: 'y' must always confirm", i)
		require.Equal(t, "mixed", res.ID)
	}
}

// TestConfirmModal_ViewContainsExpectedDecorations sanity-checks the View
// output so future style changes don't silently drop affordances.
func TestConfirmModal_ViewContainsExpectedDecorations(t *testing.T) {
	m := NewConfirmModal("test", "Are you sure?")
	out := m.View()
	require.Contains(t, out, "Are you sure?")
	// Border rendering produces non-ASCII glyphs from lipgloss; we just
	// check the body is multi-line (border + content + buttons + hint).
	require.GreaterOrEqual(t, strings.Count(out, "\n"), 3,
		"View must produce a multi-line bordered card")
}
