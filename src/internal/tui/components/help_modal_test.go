// HelpModal tests — render + routing + determinism canaries.
//
// Mirrors the structure of confirm_modal_test.go (which v1 had no
// dedicated HelpModal test for; we port from the canonical behavior in
// src/tui/components/HelpModal.tsx and the root key router in
// src/tui/hooks/useKeyboard.ts). The determinism canaries follow the
// in-process loop pattern from the spike (FINDINGS.md §6); teatest was
// considered and rejected at the spike (FINDINGS.md §6 "Why not teatest")
// because the Program event loop adds non-determinism noise that masks
// the routing race we actually want to catch.

package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// runHelpMsg pushes a single message through a HelpModal and returns the
// next modal plus any decoded ModalResultMsg (nil if none was emitted).
// Centralizes the boilerplate every test case shares.
func runHelpMsg(t *testing.T, m Modal, msg tea.Msg) (HelpModal, *ModalResultMsg) {
	t.Helper()
	next, cmd := m.Update(msg)
	hm, ok := next.(HelpModal)
	require.True(t, ok, "Update must preserve concrete type, got %T", next)

	if cmd == nil {
		return hm, nil
	}
	out := cmd()
	res, ok := out.(ModalResultMsg)
	require.True(t, ok, "expected ModalResultMsg from emit Cmd, got %T", out)
	return hm, &res
}

// --- Render tests -----------------------------------------------------------

func TestHelpModal_RendersTitleAndCategories(t *testing.T) {
	m := NewHelpModal("help")
	out := m.View()
	require.Contains(t, out, "Help - Keyboard Shortcuts")
	// Each category heading from v1 HelpModal.tsx must be present.
	for _, title := range []string{"Navigation", "Actions", "Modals", "Entry Editor", "General"} {
		require.Contains(t, out, title, "category %q must render", title)
	}
}

func TestHelpModal_RendersCanonicalKeybindings(t *testing.T) {
	m := NewHelpModal("help")
	out := m.View()
	// Spot-check keys from each category — the full set is verified
	// indirectly via the category headings above and the routing in
	// useKeyboard.ts; here we pin the most commonly-used keys.
	for _, key := range []string{"j", "k", "Tab", "Space", "d", "a", "e", "g", "m", "?", "Esc", "Ctrl+S", "Enter", "/", "q", "Ctrl+C"} {
		require.Contains(t, out, key, "key %q must render", key)
	}
}

func TestHelpModal_RendersFooterHint(t *testing.T) {
	m := NewHelpModal("help")
	out := m.View()
	require.Contains(t, out, "to close", "footer hint must instruct how to close")
}

// --- Routing tests ----------------------------------------------------------

func TestHelpModal_EscEmitsClose(t *testing.T) {
	m := NewHelpModal("help")
	_, res := runHelpMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, res)
	require.False(t, res.Confirmed, "HelpModal close is not a confirmation")
	require.Equal(t, "help", res.ID)
	require.Nil(t, res.Data, "HelpModal carries no payload")
}

func TestHelpModal_QuestionMarkEmitsClose(t *testing.T) {
	m := NewHelpModal("help")
	_, res := runHelpMsg(t, m, runeKey('?'))
	require.NotNil(t, res)
	require.False(t, res.Confirmed)
	require.Equal(t, "help", res.ID)
}

func TestHelpModal_UnknownKeyIsNoOp(t *testing.T) {
	m := NewHelpModal("help")
	// HelpModal must consume keys silently (no leak to root Update) but
	// produce no result. Test the most-likely-to-leak candidates: keys
	// the root key router would otherwise act on (j/k/q/d/a/e/g/m/Space).
	for _, r := range []rune{'j', 'k', 'q', 'd', 'a', 'e', 'g', 'm', ' ', 'y', 'n', 'x', '/'} {
		_, res := runHelpMsg(t, m, runeKey(r))
		require.Nil(t, res, "rune %q must not emit a result", r)
	}
	// Enter and arrows likewise must not close.
	for _, kt := range []tea.KeyType{tea.KeyEnter, tea.KeyLeft, tea.KeyRight, tea.KeyTab, tea.KeyUp, tea.KeyDown} {
		_, res := runHelpMsg(t, m, tea.KeyMsg{Type: kt})
		require.Nil(t, res, "key type %v must not emit a result", kt)
	}
}

func TestHelpModal_NonKeyMsgIsPassThrough(t *testing.T) {
	m := NewHelpModal("help")
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	require.Nil(t, cmd)
	require.Equal(t, m, next, "WindowSizeMsg must be a no-op so base layout reflows")
}

// --- Determinism canaries ---------------------------------------------------

// TestHelpModal_EscDeterminism is the close-path canary, matching the
// pattern of TestConfirmModal_EscDeterminism. Per FINDINGS.md §6, we use
// an in-process iteration loop (multipliable to 10,000 via
// `go test -count=100`) rather than teatest, because the Program event
// loop adds non-determinism noise that masks the routing race.
func TestHelpModal_EscDeterminism(t *testing.T) {
	const iterations = 100

	results := make([]bool, 0, iterations)
	ids := make([]string, 0, iterations)

	for i := 0; i < iterations; i++ {
		m := NewHelpModal("help")
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iteration %d: Esc must produce a Cmd", i)
		msg := cmd()
		res, ok := msg.(ModalResultMsg)
		require.True(t, ok, "iteration %d: expected ModalResultMsg, got %T", i, msg)
		results = append(results, res.Confirmed)
		ids = append(ids, res.ID)
	}

	for i, c := range results {
		require.False(t, c, "iteration %d returned Confirmed=true (FLAKE)", i)
	}
	for i, id := range ids {
		require.Equal(t, "help", id, "iteration %d returned wrong ID (FLAKE)", i)
	}
}

// TestHelpModal_QuestionMarkDeterminism is the companion canary for the
// '?' close path. Same methodology as the Esc canary; together they pin
// both close paths against routing-layer regressions.
func TestHelpModal_QuestionMarkDeterminism(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		m := NewHelpModal("help")
		_, cmd := m.Update(runeKey('?'))
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.False(t, res.Confirmed, "iteration %d: '?' must close without confirming", i)
		require.Equal(t, "help", res.ID, "iteration %d: ID mismatch (FLAKE)", i)
	}
}

// TestHelpModal_ViewMultiLineSanity guards the bordered card from
// silently collapsing if styles are accidentally dropped.
func TestHelpModal_ViewMultiLineSanity(t *testing.T) {
	m := NewHelpModal("help")
	out := m.View()
	require.GreaterOrEqual(t, strings.Count(out, "\n"), 10,
		"View must produce a multi-line bordered card with categorized bindings")
}
