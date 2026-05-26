// MoveToGroupModal tests — ports v1's React Testing Library suite (the source
// .test.tsx file appears never to have been committed; the cases below are
// reconstructed from MoveToGroupModal.tsx behavior plus the contract laid out
// in components/modal.go and confirm_modal_test.go).
//
// CRITICAL: this is the bead's flake canary. v1 had a known Esc race; the
// TestMoveToGroupModal_EscDeterminism test below MUST stay green under
// `go test -count=100 -race`. If it ever flakes, escalate per
// docs/go-migration/phase-4-contract.md Pivot Signals — that signals the
// ModalHost routing contract is broken, not a local modal bug.

package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// sampleGroups returns a small but non-trivial tree (3 top-level, one with
// nested children) so navigation and indentation are exercised by every
// rendering test.
func sampleGroups() []domain.Group {
	return []domain.Group{
		{Name: "work", Groups: []domain.Group{
			{Name: "prod"},
			{Name: "staging"},
		}},
		{Name: "personal"},
		{Name: "infra"},
	}
}

// runMTG pushes a single message through the modal and decodes any emitted
// ModalResultMsg (nil if the modal did not emit a result). Mirrors runMsg in
// confirm_modal_test.go so test patterns stay consistent across the modal
// family.
func runMTG(t *testing.T, m Modal, msg tea.Msg) (MoveToGroupModal, *ModalResultMsg) {
	t.Helper()
	next, cmd := m.Update(msg)
	mm, ok := next.(MoveToGroupModal)
	require.True(t, ok, "Update must preserve concrete type, got %T", next)

	if cmd == nil {
		return mm, nil
	}
	out := cmd()
	res, ok := out.(ModalResultMsg)
	require.True(t, ok, "expected ModalResultMsg from emit Cmd, got %T", out)
	return mm, &res
}

// --- Construction & flattening --------------------------------------------

func TestMoveToGroupModal_FlattensTreeDepthFirst(t *testing.T) {
	m := NewMoveToGroupModal(sampleGroups())
	require.Equal(t, 5, m.FlatCount(), "expected: work, work/prod, work/staging, personal, infra")
	require.Equal(t, []string{"work"}, m.groups[0].path)
	require.Equal(t, []string{"work", "prod"}, m.groups[1].path)
	require.Equal(t, 1, m.groups[1].level, "nested rows must record their depth")
	require.Equal(t, "work/staging", m.groups[2].displayName)
	require.Equal(t, []string{"personal"}, m.groups[3].path)
}

func TestMoveToGroupModal_EmptyTreeIsValid(t *testing.T) {
	m := NewMoveToGroupModal(nil)
	require.Equal(t, 0, m.FlatCount())
	require.Equal(t, 0, m.Selected())
	require.Nil(t, m.SelectedPath())
	require.Contains(t, m.View(), "No groups available")
}

// --- Navigation -----------------------------------------------------------

func TestMoveToGroupModal_JKVimNavigation(t *testing.T) {
	m := Modal(NewMoveToGroupModal(sampleGroups()))
	m, _ = runMTG(t, m, runeKey('j'))
	mm, _ := runMTG(t, m, runeKey('j'))
	require.Equal(t, 2, mm.Selected())

	mm2, _ := runMTG(t, mm, runeKey('k'))
	require.Equal(t, 1, mm2.Selected())
}

func TestMoveToGroupModal_ArrowNavigationMatchesVim(t *testing.T) {
	m := Modal(NewMoveToGroupModal(sampleGroups()))
	m, _ = runMTG(t, m, tea.KeyMsg{Type: tea.KeyDown})
	mm, _ := runMTG(t, m, tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 2, mm.Selected())

	mm2, _ := runMTG(t, mm, tea.KeyMsg{Type: tea.KeyUp})
	require.Equal(t, 1, mm2.Selected())
}

func TestMoveToGroupModal_NavigationClampsAtEdges(t *testing.T) {
	m := Modal(NewMoveToGroupModal(sampleGroups()))
	// Spam k at top — must stay at 0.
	for i := 0; i < 5; i++ {
		m, _ = runMTG(t, m, runeKey('k'))
	}
	mm := m.(MoveToGroupModal)
	require.Equal(t, 0, mm.Selected(), "k at top must clamp to 0")

	// Spam j past the bottom — must stay at last.
	for i := 0; i < 20; i++ {
		m, _ = runMTG(t, m, runeKey('j'))
	}
	mm = m.(MoveToGroupModal)
	require.Equal(t, mm.FlatCount()-1, mm.Selected(), "j past end must clamp to last")
}

// --- Enter & Esc routing --------------------------------------------------

func TestMoveToGroupModal_EnterEmitsSelectedPath(t *testing.T) {
	m := Modal(NewMoveToGroupModal(sampleGroups()))
	// Move to "work/prod" (index 1).
	m, _ = runMTG(t, m, runeKey('j'))
	_, res := runMTG(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
	require.Equal(t, MoveToGroupModalID, res.ID)
	path, ok := res.Data.([]string)
	require.True(t, ok, "Data must be []string, got %T", res.Data)
	require.Equal(t, []string{"work", "prod"}, path)
}

func TestMoveToGroupModal_EscEmitsCancel(t *testing.T) {
	m := NewMoveToGroupModal(sampleGroups())
	_, res := runMTG(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, res)
	require.False(t, res.Confirmed)
	require.Equal(t, MoveToGroupModalID, res.ID)
	require.Nil(t, res.Data)
}

func TestMoveToGroupModal_EnterOnEmptyListEmitsCancel(t *testing.T) {
	// Documented divergence from v1: v1 silently dropped Enter when the list
	// was empty (leaving the modal open). We emit cancel so the host closes.
	m := NewMoveToGroupModal(nil)
	_, res := runMTG(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.False(t, res.Confirmed)
	require.Nil(t, res.Data)
}

// --- Pass-through behavior ------------------------------------------------

func TestMoveToGroupModal_UnknownKeysAreNoOp(t *testing.T) {
	m := NewMoveToGroupModal(sampleGroups())
	for _, r := range []rune{'q', 'x', 'h', 'l', ' ', 'y', 'n'} {
		m2, res := runMTG(t, m, runeKey(r))
		require.Nil(t, res, "rune %q must not emit", r)
		require.Equal(t, m.Selected(), m2.Selected(), "rune %q must not change selection", r)
	}
}

func TestMoveToGroupModal_NonKeyMsgIsPassThrough(t *testing.T) {
	m := NewMoveToGroupModal(sampleGroups())
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	require.Nil(t, cmd)
	require.Equal(t, m, next, "non-key msg must return modal unchanged")
}

// --- View rendering -------------------------------------------------------

func TestMoveToGroupModal_ViewShowsHierarchyAndHints(t *testing.T) {
	m := NewMoveToGroupModal(sampleGroups())
	out := m.View()
	require.Contains(t, out, "Move to Group")
	require.Contains(t, out, "work")
	require.Contains(t, out, "work/prod")
	require.Contains(t, out, "work/staging")
	require.Contains(t, out, "personal")
	require.Contains(t, out, "Esc to cancel")
	// Border + title + body + hint => multi-line.
	require.GreaterOrEqual(t, strings.Count(out, "\n"), 4)
}

// --- Determinism canary (REQUIRED — v1 Esc flake foreclosure) -------------

// TestMoveToGroupModal_EscDeterminism is the bead's flake-canary. v1's
// React/Ink port had a non-deterministic Esc due to dual useInput listeners
// in App + modal. The Bubble Tea pattern forecloses that by routing through
// ModalHost (see modal_host.go), and this test verifies the modal half of
// the contract over 100 iterations. Combined with `-count=100 -race` we get
// 10,000 cycles under the race detector — zero flakes required.
//
// Failure mode this catches: any state leak across iterations, missing
// emit Cmd, wrong Confirmed value, wrong ID, or missing/wrong Data.
func TestMoveToGroupModal_EscDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := NewMoveToGroupModal(sampleGroups())
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iteration %d: Esc must produce a Cmd", i)
		msg := cmd()
		res, ok := msg.(ModalResultMsg)
		require.True(t, ok, "iteration %d: expected ModalResultMsg, got %T", i, msg)
		require.False(t, res.Confirmed, "iteration %d: Esc must yield Confirmed=false (FLAKE)", i)
		require.Equal(t, MoveToGroupModalID, res.ID, "iteration %d: ID drift (FLAKE)", i)
		require.Nil(t, res.Data, "iteration %d: cancel must carry no payload", i)
	}
}

// TestMoveToGroupModal_EnterDeterminism is the companion canary for Enter →
// Confirmed=true with the selected path attached. Same methodology as the
// Esc canary; together they pin both halves of the result-routing table.
func TestMoveToGroupModal_EnterDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := Modal(NewMoveToGroupModal(sampleGroups()))
		// Always navigate to index 1 ("work/prod") so result is non-trivial.
		m, _ = m.Update(runeKey('j'))
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.True(t, res.Confirmed, "iteration %d: Enter must confirm", i)
		require.Equal(t, MoveToGroupModalID, res.ID, "iteration %d: ID drift", i)
		path, ok := res.Data.([]string)
		require.True(t, ok, "iteration %d: Data must be []string, got %T", i, res.Data)
		require.Equal(t, []string{"work", "prod"}, path, "iteration %d: path drift (FLAKE)", i)
	}
}

// TestMoveToGroupModal_MixedSequenceDeterminism stresses the full keymap by
// running a navigation sequence and then Enter, 100 times, asserting the
// final result is identical every cycle. Catches state-leak bugs where one
// iteration's selection bleeds into the next (a class hidden by single-shot
// tests).
func TestMoveToGroupModal_MixedSequenceDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := Modal(NewMoveToGroupModal(sampleGroups()))
		m, _ = m.Update(runeKey('j'))                          // -> 1
		m, _ = m.Update(runeKey('j'))                          // -> 2
		m, _ = m.Update(runeKey('j'))                          // -> 3
		m, _ = m.Update(runeKey('k'))                          // -> 2
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})         // -> 3
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.True(t, res.Confirmed, "iteration %d: Enter must always confirm", i)
		path, _ := res.Data.([]string)
		// Index 3 in sampleGroups flatten is "personal".
		require.Equal(t, []string{"personal"}, path, "iteration %d: sequence drift (FLAKE)", i)
	}
}
