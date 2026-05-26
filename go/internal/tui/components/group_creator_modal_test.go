// GroupCreatorModal tests — ports v1 src/tui/components/GroupCreatorModal.tsx
// behavior (validateGroupName cases + Esc cancel + Enter submit) plus
// determinism canaries matching the spike methodology in
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md §6.
//
// Why no teatest? FINDINGS.md §6 explicitly rejected teatest for this
// pattern: in-process Modal.Update iteration is strictly stronger at catching
// the v1 routing-flake class because it removes Program-loop noise (timer
// fires, stdin pacing) that would otherwise mask the bug. The canaries here
// follow the same shape — a tight loop of fresh-modal → key → assert result
// — and produce 10,000 cycles when run with `go test -count=100 -race`.

package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// runGroupCreatorMsg pushes a single message through the modal and decodes
// any ModalResultMsg the modal's Cmd produced. Mirrors runMsg from
// confirm_modal_test.go so the determinism test shape is consistent across
// modals.
func runGroupCreatorMsg(t *testing.T, m Modal, msg tea.Msg) (GroupCreatorModal, *ModalResultMsg) {
	t.Helper()
	next, cmd := m.Update(msg)
	gm, ok := next.(GroupCreatorModal)
	require.True(t, ok, "Update must preserve concrete type, got %T", next)

	if cmd == nil {
		return gm, nil
	}
	out := cmd()
	res, ok := out.(ModalResultMsg)
	if !ok {
		// Non-result Cmds (e.g., textinput cursor blink) — not an error.
		return gm, nil
	}
	return gm, &res
}

// typeRunes feeds each rune of s into the modal as a KeyRunes message,
// returning the resulting modal. Used to populate the Name/Description
// inputs in tests.
func typeRunes(t *testing.T, m Modal, s string) Modal {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next
	}
	return m
}

// --- Render tests -----------------------------------------------------------

func TestGroupCreatorModal_RendersTitleAndFields(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	out := m.View()
	require.Contains(t, out, "Create Group")
	require.Contains(t, out, "Name")
	require.Contains(t, out, "Description")
	require.Contains(t, out, "Esc")
}

func TestGroupCreatorModal_RendersParentPathWhenProvided(t *testing.T) {
	m := NewGroupCreatorModal("create-group", []string{"work", "prod"})
	out := m.View()
	require.Contains(t, out, "work/prod")
}

func TestGroupCreatorModal_DefaultsToNameFocus(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	require.Equal(t, groupCreatorFocusName, m.Focus())
}

// --- Routing tests ----------------------------------------------------------

func TestGroupCreatorModal_EscEmitsCancel(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	_, res := runGroupCreatorMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, res, "Esc must emit a result")
	require.False(t, res.Confirmed)
	require.Equal(t, "create-group", res.ID)
	require.Nil(t, res.Data, "Cancel must not carry a payload")
}

func TestGroupCreatorModal_EnterWithEmptyNameSetsErrorAndDoesNotEmit(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	next, res := runGroupCreatorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, res, "Enter with empty name must NOT emit a result")
	require.NotEmpty(t, next.Error(), "validation error must be visible to operator")
	require.Contains(t, next.Error(), "empty")
}

func TestGroupCreatorModal_EnterWithInvalidNameSetsError(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"slash", "foo/bar", "slashes"},
		{"leading hyphen", "-foo", "start"},
		{"trailing hyphen", "foo-", "end"},
		{"consecutive hyphens", "foo--bar", "consecutive"},
		{"uppercase", "FooBar", "kebab-case"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Modal(NewGroupCreatorModal("create-group", nil))
			m = typeRunes(t, m, tc.input)
			next, res := runGroupCreatorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
			require.Nil(t, res, "invalid name must NOT emit")
			require.Contains(t, next.Error(), tc.wantErr)
		})
	}
}

func TestGroupCreatorModal_EnterWithValidNameEmitsGroup(t *testing.T) {
	m := Modal(NewGroupCreatorModal("create-group", nil))
	m = typeRunes(t, m, "staging-api")

	_, res := runGroupCreatorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
	require.Equal(t, "create-group", res.ID)

	grp, ok := res.Data.(domain.Group)
	require.True(t, ok, "Data must be domain.Group, got %T", res.Data)
	require.Equal(t, "staging-api", grp.Name)
	require.Empty(t, grp.Description)
	require.NotNil(t, grp.Entries)
	require.NotNil(t, grp.Groups)
}

func TestGroupCreatorModal_TabCyclesFocusToDescription(t *testing.T) {
	m := Modal(NewGroupCreatorModal("create-group", nil))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	gm := next.(GroupCreatorModal)
	require.Equal(t, groupCreatorFocusDescription, gm.Focus())

	// Tab again wraps back to Name.
	next2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyTab})
	gm2 := next2.(GroupCreatorModal)
	require.Equal(t, groupCreatorFocusName, gm2.Focus())
}

func TestGroupCreatorModal_ShiftTabCyclesBackward(t *testing.T) {
	m := Modal(NewGroupCreatorModal("create-group", nil))
	// From Name, Shift+Tab wraps to Description.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	gm := next.(GroupCreatorModal)
	require.Equal(t, groupCreatorFocusDescription, gm.Focus())
}

func TestGroupCreatorModal_TypingInDescriptionAfterTab(t *testing.T) {
	m := Modal(NewGroupCreatorModal("create-group", nil))
	m = typeRunes(t, m, "ok-name")
	// Tab to description.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next
	m = typeRunes(t, m, "my description")

	_, res := runGroupCreatorMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, res)
	require.True(t, res.Confirmed)
	grp := res.Data.(domain.Group)
	require.Equal(t, "ok-name", grp.Name)
	require.Equal(t, "my description", grp.Description)
}

func TestGroupCreatorModal_ErrorClearsOnNextKeystroke(t *testing.T) {
	m := Modal(NewGroupCreatorModal("create-group", nil))
	// Trigger error with empty Enter.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotEmpty(t, next.(GroupCreatorModal).Error())
	// Typing a rune clears the error so the operator gets immediate
	// feedback that the correction is being applied.
	next2, _ := next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Empty(t, next2.(GroupCreatorModal).Error())
}

func TestGroupCreatorModal_NonKeyMsgIsPassThrough(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	// WindowSizeMsg must not emit a result. The modal may return a cmd
	// (e.g., from textinput) but no ModalResultMsg.
	_, res := runGroupCreatorMsg(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	require.Nil(t, res)
}

// --- Determinism canaries ---------------------------------------------------

// TestGroupCreatorModal_EscDeterminism is the v1-flake canary for the cancel
// path. See FINDINGS.md §6. Run with `go test -count=100` for 10,000 cycles.
func TestGroupCreatorModal_EscDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := NewGroupCreatorModal("create-group", nil)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd, "iteration %d: Esc must produce a Cmd", i)
		msg := cmd()
		res, ok := msg.(ModalResultMsg)
		require.True(t, ok, "iteration %d: expected ModalResultMsg, got %T", i, msg)
		require.False(t, res.Confirmed, "iteration %d: Esc must always cancel (FLAKE)", i)
		require.Equal(t, "create-group", res.ID, "iteration %d: wrong ID (FLAKE)", i)
		require.Nil(t, res.Data, "iteration %d: cancel must not carry payload", i)
	}
}

// TestGroupCreatorModal_EnterSubmitDeterminism drives the full happy path
// (type → Enter → assert Group payload) 100 iterations. Catches any state
// leak between iterations (e.g., textinput.Model carrying stale runes).
func TestGroupCreatorModal_EnterSubmitDeterminism(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		m := Modal(NewGroupCreatorModal("create-group", nil))
		m = typeRunes(t, m, "valid-name")
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "iteration %d", i)
		res, ok := cmd().(ModalResultMsg)
		require.True(t, ok, "iteration %d", i)
		require.True(t, res.Confirmed, "iteration %d: valid name must submit", i)
		grp, ok := res.Data.(domain.Group)
		require.True(t, ok, "iteration %d: Data must be domain.Group, got %T", i, res.Data)
		require.Equal(t, "valid-name", grp.Name, "iteration %d (FLAKE)", i)
	}
}

// TestGroupCreatorModal_ViewContainsHints sanity-checks the View output so
// future style changes don't silently drop affordances.
func TestGroupCreatorModal_ViewContainsHints(t *testing.T) {
	m := NewGroupCreatorModal("create-group", nil)
	out := m.View()
	require.GreaterOrEqual(t, strings.Count(out, "\n"), 3, "View must be multi-line")
	require.Contains(t, out, "Tab")
	require.Contains(t, out, "Enter")
	require.Contains(t, out, "Esc")
}
