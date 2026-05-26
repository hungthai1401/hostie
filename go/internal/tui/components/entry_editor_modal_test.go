// EntryEditorModal tests — ports v1
// src/tui/components/__tests__/EntryEditorModal.test.tsx and adds the
// determinism canary required by .spikes/go-migration/p4-modal-pattern
// FINDINGS.md §7.
//
// Run determinism stress with: go test ./internal/tui/components/... \
//   -race -count=100 -run TestEntryEditorModal_Esc

package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// runEditorMsg pushes a message through the modal and returns the next modal
// plus any ModalResultMsg the cmd produced.
func runEditorMsg(t *testing.T, m Modal, msg tea.Msg) (EntryEditorModal, *ModalResultMsg) {
	t.Helper()
	next, cmd := m.Update(msg)
	em, ok := next.(EntryEditorModal)
	require.True(t, ok, "Update must preserve EntryEditorModal type, got %T", next)
	if cmd == nil {
		return em, nil
	}
	// The cmd may be a textinput.Blink or other non-result cmd; we only
	// treat it as a result if it produces a ModalResultMsg synchronously.
	out := cmd()
	if res, ok := out.(ModalResultMsg); ok {
		return em, &res
	}
	return em, nil
}

func typeString(t *testing.T, m EntryEditorModal, s string) EntryEditorModal {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		em, ok := next.(EntryEditorModal)
		require.True(t, ok)
		m = em
	}
	return m
}

// --- Render tests -----------------------------------------------------------

func TestEntryEditorModal_AddModeRendersBlankForm(t *testing.T) {
	m := NewEntryEditorModal("add-entry")
	out := m.View()
	require.Contains(t, out, "Add Entry")
	require.Contains(t, out, "IP Address:")
	require.Contains(t, out, "Hostname:")
	require.Contains(t, out, "Aliases:")
	require.Contains(t, out, "Enabled:")
	require.Contains(t, out, "Comment:")
	require.Contains(t, out, "comma-separated")
	require.Contains(t, out, "Save")
	require.Contains(t, out, "Cancel")
}

func TestEntryEditorModal_EditModePrefillsValuesAndTitle(t *testing.T) {
	entry := domain.Entry{
		ID:       "test-id-123",
		IP:       "192.168.1.10",
		Hostname: "test.local",
		Aliases:  []string{"alias1", "alias2"},
		Enabled:  true,
		Comment:  "Test comment",
	}
	m := NewEntryEditorModalForEdit("edit-entry", entry)
	out := m.View()
	require.Contains(t, out, "Edit Entry")
	require.Contains(t, out, "192.168.1.10")
	require.Contains(t, out, "test.local")
	require.Contains(t, out, "alias1, alias2")
	require.Contains(t, out, "Test comment")
	require.Contains(t, out, "[✓]") // enabled = true
}

func TestEntryEditorModal_EnabledFalseShowsUnchecked(t *testing.T) {
	entry := domain.Entry{
		ID: "x", IP: "1.1.1.1", Hostname: "h.local", Enabled: false,
	}
	m := NewEntryEditorModalForEdit("e", entry)
	require.Contains(t, m.View(), "[ ]")
}

// --- Routing tests ---------------------------------------------------------

func TestEntryEditorModal_TabCyclesFocusForward(t *testing.T) {
	m := NewEntryEditorModal("add")
	require.Equal(t, fieldIP, m.Focused())

	expected := []editorField{fieldHostname, fieldAliases, fieldEnabled, fieldComment, fieldSubmit, fieldIP}
	cur := Modal(m)
	for _, want := range expected {
		next, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		em, ok := next.(EntryEditorModal)
		require.True(t, ok)
		require.Equal(t, want, em.Focused(), "Tab must advance focus")
		cur = em
	}
}

func TestEntryEditorModal_ShiftTabCyclesFocusBackward(t *testing.T) {
	m := NewEntryEditorModal("add")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	em := next.(EntryEditorModal)
	require.Equal(t, fieldSubmit, em.Focused(), "Shift+Tab from IP must wrap to Submit")
}

func TestEntryEditorModal_SpaceTogglesEnabledOnlyWhenFocused(t *testing.T) {
	m := NewEntryEditorModal("add")
	require.True(t, m.Enabled(), "default enabled = true")

	// Space while on IP must NOT toggle enabled — it must go to the input.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	em := next.(EntryEditorModal)
	require.True(t, em.Enabled(), "Space on IP must not toggle enabled")

	// Tab to enabled (IP → Hostname → Aliases → Enabled).
	for i := 0; i < 3; i++ {
		nx, _ := Modal(em).Update(tea.KeyMsg{Type: tea.KeyTab})
		em = nx.(EntryEditorModal)
	}
	require.Equal(t, fieldEnabled, em.Focused())

	nx, _ := Modal(em).Update(tea.KeyMsg{Type: tea.KeySpace})
	em = nx.(EntryEditorModal)
	require.False(t, em.Enabled(), "Space on Enabled must toggle off")

	nx, _ = Modal(em).Update(tea.KeyMsg{Type: tea.KeySpace})
	em = nx.(EntryEditorModal)
	require.True(t, em.Enabled(), "Space on Enabled must toggle back on")
}

func TestEntryEditorModal_EscEmitsCancel(t *testing.T) {
	m := NewEntryEditorModal("add-entry")
	_, res := runEditorMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, res)
	require.False(t, res.Confirmed)
	require.Equal(t, "add-entry", res.ID)
}

// --- Validation tests ------------------------------------------------------

func TestEntryEditorModal_SubmitWithEmptyFieldsShowsInlineErrors(t *testing.T) {
	m := NewEntryEditorModal("add")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	em := next.(EntryEditorModal)
	require.Nil(t, cmd, "invalid submit must not emit a result")

	ipErr, hostErr, _, _ := em.Errors()
	require.NotEmpty(t, ipErr, "empty IP must produce an inline error")
	require.NotEmpty(t, hostErr, "empty hostname must produce an inline error")

	// Validation error text must reflect domain.* errors.
	require.True(t, strings.Contains(ipErr, "IPv4") || strings.Contains(ipErr, "IP"),
		"ip error must come from domain.ValidateIP, got %q", ipErr)
	require.True(t, strings.Contains(hostErr, "hostname") || strings.Contains(hostErr, "empty"),
		"hostname error must come from domain.ValidateHostname, got %q", hostErr)

	// Rendered view must surface the errors.
	out := em.View()
	require.Contains(t, out, ipErr)
	require.Contains(t, out, hostErr)
}

func TestEntryEditorModal_SubmitWithInvalidHostnameShowsError(t *testing.T) {
	m := NewEntryEditorModal("add")
	// Fill valid IP.
	m = typeString(t, m, "10.0.0.1")
	// Tab to hostname.
	next, _ := Modal(m).Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(EntryEditorModal)
	m = typeString(t, m, "-bad-host")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	em := next.(EntryEditorModal)
	require.Nil(t, cmd, "invalid hostname must not submit")
	_, hostErr, _, _ := em.Errors()
	require.Contains(t, hostErr, "letter or digit", "expected domain.ValidateHostname error text")
}

func TestEntryEditorModal_SubmitWithValidFormEmitsEntry(t *testing.T) {
	m := NewEntryEditorModal("add-entry")
	m = typeString(t, m, "192.168.1.50")
	next, _ := Modal(m).Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(EntryEditorModal)
	m = typeString(t, m, "host.local")
	next, _ = Modal(m).Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(EntryEditorModal)
	m = typeString(t, m, "alias1, alias2")
	// Skip enabled (default true) and go to comment.
	next, _ = Modal(m).Update(tea.KeyMsg{Type: tea.KeyTab}) // → enabled
	m = next.(EntryEditorModal)
	next, _ = Modal(m).Update(tea.KeyMsg{Type: tea.KeyTab}) // → comment
	m = next.(EntryEditorModal)
	m = typeString(t, m, "hello")

	_, res := runEditorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res, "valid submit must emit ModalResultMsg")
	require.True(t, res.Confirmed)
	require.Equal(t, "add-entry", res.ID)

	entry, ok := res.Data.(domain.Entry)
	require.True(t, ok, "Data must be domain.Entry, got %T", res.Data)
	require.Equal(t, "192.168.1.50", entry.IP)
	require.Equal(t, "host.local", entry.Hostname)
	require.Equal(t, []string{"alias1", "alias2"}, entry.Aliases)
	require.True(t, entry.Enabled)
	require.Equal(t, "hello", entry.Comment)
	require.Empty(t, entry.ID, "add mode leaves ID empty for caller to assign")
}

func TestEntryEditorModal_EditModeSubmitPreservesID(t *testing.T) {
	entry := domain.Entry{
		ID: "ulid-abc", IP: "10.0.0.1", Hostname: "host.local",
		Aliases: []string{"a1"}, Enabled: true,
	}
	m := NewEntryEditorModalForEdit("edit", entry)
	_, res := runEditorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
	got := res.Data.(domain.Entry)
	require.Equal(t, "ulid-abc", got.ID, "edit mode must preserve original ID")
}

func TestEntryEditorModal_NonKeyMsgIsForwardedNotConsumed(t *testing.T) {
	m := NewEntryEditorModal("add")
	// WindowSizeMsg must not panic and must not produce a ModalResultMsg.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, ok := next.(EntryEditorModal)
	require.True(t, ok)
}

// --- Determinism canary -----------------------------------------------------

// TestEntryEditorModal_EscDeterminism mirrors the spike's canary:
// Open → Esc → assert Confirmed=false, ID stable, for ≥100 iterations.
// Run with -count=100 to multiply.
func TestEntryEditorModal_EscDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := NewEntryEditorModal("add-entry")
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iteration %d: Esc must produce a Cmd", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d: expected ModalResultMsg, got %T", i, cmd())
		require.False(t, res.Confirmed, "iteration %d returned Confirmed=true (FLAKE)", i)
		require.Equal(t, "add-entry", res.ID, "iteration %d: ID mismatch (FLAKE)", i)
	}
}

// TestEntryEditorModal_ValidSubmitDeterminism stresses the submit path: a
// pre-filled editor (edit mode) must produce the same payload every time.
func TestEntryEditorModal_ValidSubmitDeterminism(t *testing.T) {
	const iterations = 100
	entry := domain.Entry{
		ID: "ulid-xyz", IP: "10.0.0.1", Hostname: "host.local",
		Aliases: []string{"a1", "a2"}, Enabled: true, Comment: "c",
	}
	for i := 0; i < iterations; i++ {
		m := NewEntryEditorModalForEdit("edit", entry)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "iteration %d: Enter on valid form must emit", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.True(t, res.Confirmed, "iteration %d", i)
		require.Equal(t, "edit", res.ID, "iteration %d", i)
		got := res.Data.(domain.Entry)
		require.Equal(t, entry.ID, got.ID, "iteration %d", i)
		require.Equal(t, entry.IP, got.IP, "iteration %d", i)
		require.Equal(t, entry.Hostname, got.Hostname, "iteration %d", i)
		require.Equal(t, entry.Aliases, got.Aliases, "iteration %d", i)
	}
}

// --- parseAliases unit ------------------------------------------------------

func TestParseAliases(t *testing.T) {
	require.Nil(t, parseAliases(""))
	require.Nil(t, parseAliases("   "))
	require.Equal(t, []string{"a"}, parseAliases("a"))
	require.Equal(t, []string{"a", "b", "c"}, parseAliases("a, b ,c"))
	require.Equal(t, []string{"a", "b"}, parseAliases("a,,b"))
}
