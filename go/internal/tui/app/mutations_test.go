// mutations_test.go — port of v1 useKeyboard.ts mutation behavior tests.
//
// Covers each new keybind path added by hosts-cli-go-mig-p4-app-mutations-9fk:
// Space toggle, d delete (confirm + cancel), a add, e edit, g group create,
// m move-to-group, dirty-aware q. Each case drives the Bubble Tea Update
// loop directly (no teatest dependency) and asserts both the store-side
// mutation and the ApplyTriggerMsg emission.

package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/components"
)

// seedAndSelect builds the standard fixture model and selects entry e1 so
// mutation handlers have something to operate on. Returns the Model and the
// initial entries (handy for tests that compare before/after).
func seedAndSelect(t *testing.T, entryID string) Model {
	t.Helper()
	m := seedModel(t)
	m.store.SelectEntry(entryID)
	return m
}

// drainResultMsg invokes the Cmd, asserts it produced a ModalResultMsg with
// the expected ID, and returns the unwrapped result for further assertions.
func drainResultMsg(t *testing.T, cmd tea.Cmd, wantID string) components.ModalResultMsg {
	t.Helper()
	require.NotNil(t, cmd, "expected a Cmd, got nil")
	msg := cmd()
	result, ok := msg.(components.ModalResultMsg)
	require.True(t, ok, "expected ModalResultMsg, got %T", msg)
	require.Equal(t, wantID, result.ID, "result ID mismatch")
	return result
}

// triggeredApply reports whether a Cmd batch contains an ApplyTriggerMsg.
// Bubble Tea's tea.Batch collapses cmds into a tea.BatchMsg; in our handlers
// we always return either a single ApplyTriggerMsg-producing Cmd or a
// tea.Batch with one. We just invoke the cmd and walk the result tree.
func triggeredApply(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	msg := cmd()
	if _, ok := msg.(ApplyTriggerMsg); ok {
		return true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if _, ok := sub().(ApplyTriggerMsg); ok {
				return true
			}
		}
	}
	return false
}

// TestMutations_Space_TogglesAndApplies covers v1 useKeyboard.ts 'space'
// branch: toggleEntry(selectedId) → writeHostsFile. We assert both store
// mutation and ApplyTriggerMsg emission.
func TestMutations_Space_TogglesAndApplies(t *testing.T) {
	m := seedAndSelect(t, "e1")
	require.True(t, m.Store().HostsFile().Groups[0].Entries[0].Enabled)

	m2, cmd := m.Update(key(" "))
	mm := m2.(Model)
	require.False(t, mm.Store().HostsFile().Groups[0].Entries[0].Enabled,
		"Space must flip Enabled on selected entry")
	require.True(t, triggeredApply(t, cmd), "Space mutation must trigger apply")
}

// TestMutations_Space_NoSelection_NoOp guards the v1 `selectedEntryId`
// truthy-check — Space with no selection is a no-op (no mutation, no Cmd).
func TestMutations_Space_NoSelection_NoOp(t *testing.T) {
	m := seedModel(t)
	require.Equal(t, "", m.Store().SelectedEntryID())

	m2, cmd := m.Update(key(" "))
	require.Nil(t, cmd)
	require.True(t, m2.(Model).Store().HostsFile().Groups[0].Entries[0].Enabled,
		"Space with no selection must not toggle anything")
}

// TestMutations_Delete_Confirmed exercises the full d → ConfirmModal → y path.
func TestMutations_Delete_Confirmed(t *testing.T) {
	m := seedAndSelect(t, "e1")

	// Step 1: press 'd' → modal opens, no mutation yet.
	m2, _ := m.Update(key("d"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active(), "d must open the ConfirmModal")
	require.Equal(t, modalIDDeleteEntry, mm.modalHost.Current().ID())
	require.Len(t, mm.Store().HostsFile().Groups[0].Entries, 2, "no deletion until confirm")

	// Step 2: press 'y' inside the modal → emits ModalResultMsg(Confirmed=true).
	m3, cmd := mm.Update(key("y"))
	mm = m3.(Model)
	result := drainResultMsg(t, cmd, modalIDDeleteEntry)
	require.True(t, result.Confirmed)

	// Step 3: deliver the result → dispatchModalResult runs the deletion.
	m4, cmd := mm.Update(result)
	mm = m4.(Model)
	require.Len(t, mm.Store().HostsFile().Groups[0].Entries, 1, "entry must be gone")
	require.Equal(t, "e2", mm.Store().HostsFile().Groups[0].Entries[0].ID)
	require.True(t, triggeredApply(t, cmd), "delete must trigger apply")
	require.False(t, mm.modalHost.Active(), "modal must be closed after result delivery")
}

// TestMutations_Delete_Cancelled exercises the cancel branch: d → Esc.
// Cancel path is the v1 onCancel: closeModal with no deletion.
func TestMutations_Delete_Cancelled(t *testing.T) {
	m := seedAndSelect(t, "e1")

	m2, _ := m.Update(key("d"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())

	// Esc → cancel result.
	m3, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = m3.(Model)
	result := drainResultMsg(t, cmd, modalIDDeleteEntry)
	require.False(t, result.Confirmed)

	// Deliver the cancel → no mutation, modal closes.
	m4, cmd := mm.Update(result)
	mm = m4.(Model)
	require.Len(t, mm.Store().HostsFile().Groups[0].Entries, 2, "cancel must not delete")
	require.False(t, triggeredApply(t, cmd), "cancel must not trigger apply")
	require.False(t, mm.modalHost.Active())
}

// TestMutations_Add_OpensEditor verifies 'a' opens the EntryEditorModal in
// add mode (NewEntryEditorModal, not the *ForEdit variant).
func TestMutations_Add_OpensEditor(t *testing.T) {
	m := seedModel(t)

	m2, _ := m.Update(key("a"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())
	require.Equal(t, modalIDEntryAdd, mm.modalHost.Current().ID())
	editor, ok := mm.modalHost.Current().(components.EntryEditorModal)
	require.True(t, ok, "expected EntryEditorModal, got %T", mm.modalHost.Current())
	require.Equal(t, components.EntryEditorModeAdd, editor.Mode())
}

// TestMutations_Add_Submission_AddsEntry simulates the operator filling out
// the form and confirming — store gains a new entry under the selected
// group and ApplyTriggerMsg fires. ID assignment happens in dispatch.
func TestMutations_Add_Submission_AddsEntry(t *testing.T) {
	m := seedModel(t)
	m.store.SelectGroup([]string{"work"})

	// Synthesize the result directly (the EntryEditorModal submit path is
	// covered by its own component-level tests; here we test the dispatch).
	newEntry := domain.Entry{IP: "1.2.3.4", Hostname: "added.dev", Enabled: true}
	result := components.ModalResultMsg{ID: modalIDEntryAdd, Confirmed: true, Data: newEntry}
	m2, cmd := m.Update(result)
	mm := m2.(Model)

	entries := mm.Store().HostsFile().Groups[0].Entries
	require.Len(t, entries, 3, "work group must gain a new entry")
	added := entries[2]
	require.Equal(t, "1.2.3.4", added.IP)
	require.NotEmpty(t, added.ID, "dispatch must assign a ULID")
	require.Equal(t, added.ID, mm.Store().SelectedEntryID(), "new entry must be selected")
	require.True(t, triggeredApply(t, cmd))
}

// TestMutations_Edit_OpensEditorPrePopulated verifies 'e' opens the
// EntryEditorModal in edit mode with the selected entry's data.
func TestMutations_Edit_OpensEditorPrePopulated(t *testing.T) {
	m := seedAndSelect(t, "e1")

	m2, _ := m.Update(key("e"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())
	editor, ok := mm.modalHost.Current().(components.EntryEditorModal)
	require.True(t, ok)
	require.Equal(t, components.EntryEditorModeEdit, editor.Mode())
	ip, hostname, _, _ := editor.Values()
	require.Equal(t, "10.0.0.1", ip)
	require.Equal(t, "api.dev", hostname)
}

// TestMutations_Edit_Submission_UpdatesEntry simulates the result delivery
// and asserts UpdateEntry runs with the preserved ID.
func TestMutations_Edit_Submission_UpdatesEntry(t *testing.T) {
	m := seedAndSelect(t, "e1")

	updated := domain.Entry{ID: "e1", IP: "99.99.99.99", Hostname: "edited.dev", Enabled: false}
	result := components.ModalResultMsg{ID: modalIDEntryEdit, Confirmed: true, Data: updated}
	m2, cmd := m.Update(result)
	mm := m2.(Model)

	got := mm.Store().HostsFile().Groups[0].Entries[0]
	require.Equal(t, "e1", got.ID, "ID must be preserved on edit")
	require.Equal(t, "99.99.99.99", got.IP)
	require.False(t, got.Enabled)
	require.True(t, triggeredApply(t, cmd))
}

// TestMutations_GroupCreate_AddsGroup exercises the g → result → addGroup path.
func TestMutations_GroupCreate_AddsGroup(t *testing.T) {
	m := seedModel(t)
	// 'g' opens the modal.
	m2, _ := m.Update(key("g"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())
	require.Equal(t, modalIDGroupCreate, mm.modalHost.Current().ID())

	// Deliver a confirmed result with the new group payload.
	newGroup := domain.Group{Name: "qa", Description: "QA hosts"}
	result := components.ModalResultMsg{ID: modalIDGroupCreate, Confirmed: true, Data: newGroup}
	m3, cmd := mm.Update(result)
	mm = m3.(Model)

	groups := mm.Store().HostsFile().Groups
	require.Len(t, groups, 3, "fixture's 2 groups + 1 new")
	require.Equal(t, "qa", groups[2].Name)
	require.True(t, triggeredApply(t, cmd))
}

// TestMutations_MoveToGroup_MovesEntry exercises m → result → moveEntry.
func TestMutations_MoveToGroup_MovesEntry(t *testing.T) {
	m := seedAndSelect(t, "e1")

	m2, _ := m.Update(key("m"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())
	require.Equal(t, components.MoveToGroupModalID, mm.modalHost.Current().ID())

	// Deliver a confirmed result selecting "personal" as the target.
	result := components.ModalResultMsg{
		ID:        components.MoveToGroupModalID,
		Confirmed: true,
		Data:      []string{"personal"},
	}
	m3, cmd := mm.Update(result)
	mm = m3.(Model)

	work := mm.Store().HostsFile().Groups[0]
	personal := mm.Store().HostsFile().Groups[1]
	require.Len(t, work.Entries, 1, "e1 must leave work")
	require.Equal(t, "e2", work.Entries[0].ID)
	require.Len(t, personal.Entries, 2, "e1 must arrive in personal")
	require.True(t, triggeredApply(t, cmd))
}

// TestMutations_Quit_DirtyOpensConfirm verifies the D11/D13 dirty-aware quit
// path: when store.Dirty()==true, q opens a ConfirmModal instead of
// quitting immediately.
func TestMutations_Quit_DirtyOpensConfirm(t *testing.T) {
	m := seedAndSelect(t, "e1")
	m.store.MarkDirty()
	require.True(t, m.Store().Dirty())

	m2, cmd := m.Update(key("q"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active(), "dirty q must open ConfirmModal, not quit")
	require.Equal(t, modalIDQuitDirty, mm.modalHost.Current().ID())
	require.Nil(t, cmd, "dirty q must not return tea.Quit; that waits for confirm")
	require.False(t, mm.quitting, "quitting flag must not be set until confirm")
}

// TestMutations_Quit_DirtyConfirmed_Quits verifies that confirming the
// quit-dirty modal actually exits.
func TestMutations_Quit_DirtyConfirmed_Quits(t *testing.T) {
	m := seedAndSelect(t, "e1")
	m.store.MarkDirty()
	m2, _ := m.Update(key("q"))
	mm := m2.(Model)

	result := components.ModalResultMsg{ID: modalIDQuitDirty, Confirmed: true}
	m3, cmd := mm.Update(result)
	mm = m3.(Model)
	require.True(t, mm.quitting)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	require.True(t, ok, "confirmed quit-dirty must yield tea.QuitMsg")
}

// TestMutations_Quit_Clean_QuitsImmediately preserves the v1 fast-path:
// clean store + q → tea.Quit with no modal.
func TestMutations_Quit_Clean_QuitsImmediately(t *testing.T) {
	m := seedModel(t)
	require.False(t, m.Store().Dirty())

	m2, cmd := m.Update(key("q"))
	mm := m2.(Model)
	require.False(t, mm.modalHost.Active(), "clean q must not open a modal")
	require.True(t, mm.quitting)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	require.True(t, ok)
}

// TestMutations_ApplyTriggerMsg_IsHandled confirms the message is consumed
// silently by the root Update — applycmd-91r will replace this branch with
// the real apply pipeline. Today the contract is: no panic, no spurious
// state change, no Cmd.
func TestMutations_ApplyTriggerMsg_IsHandled(t *testing.T) {
	m := seedAndSelect(t, "e1")
	before := m.Store().SelectedEntryID()
	m2, cmd := m.Update(ApplyTriggerMsg{})
	require.Nil(t, cmd, "ApplyTriggerMsg in this bead must be a no-op (applycmd-91r wires the apply Cmd)")
	require.Equal(t, before, m2.(Model).Store().SelectedEntryID())
}

// TestMutations_ModalActive_InterceptsKeys verifies the spike contract: when
// a modal is open, the root key router never sees keys — they all reach the
// modal. We open the add modal then send 'j' (a Normal-mode navigation key)
// and assert the selection does NOT change.
func TestMutations_ModalActive_InterceptsKeys(t *testing.T) {
	m := seedAndSelect(t, "e1")
	m2, _ := m.Update(key("a"))
	mm := m2.(Model)
	require.True(t, mm.modalHost.Active())

	before := mm.Store().SelectedEntryID()
	m3, _ := mm.Update(key("j"))
	mm = m3.(Model)
	require.Equal(t, before, mm.Store().SelectedEntryID(),
		"navigation keys must not leak past an active modal (v1 flake foreclosed)")
	require.True(t, mm.modalHost.Active(), "modal must remain active")
}
