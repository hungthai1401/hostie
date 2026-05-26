// mutations.go — keybind → store mutation glue for the TUI.
//
// This file owns the Normal-mode key handlers that mutate the centralized
// store via the actions ported in store/state.go (ToggleEntry, DeleteEntry,
// AddEntry, UpdateEntry, AddGroup, MoveEntry). Each handler either:
//
//   - Mutates the store directly and emits an ApplyTriggerMsg (Space toggle —
//     no confirmation), or
//   - Opens a modal via ModalHost.Open and defers the mutation to
//     dispatchModalResult (see modal_routing.go) once the modal emits its
//     ModalResultMsg.
//
// Design references:
//
//   - design.md D11: TUI mutations auto-apply (every mutation runs the apply
//     pipeline immediately; no explicit "save" key).
//   - design.md D13: YAML write is independent from /etc/hosts apply — even
//     if apply fails, the store stays mutated and the ~/.hosts file remains
//     written. Apply-error surfacing lives in app-applycmd-91r.
//   - design.md D14: no dry-run path in the TUI. All mutations are real.
//
// ApplyTriggerMsg is the minimal hook this bead defines for applycmd-91r to
// wire into apply.Runner.Apply with StatusBar plumbing. applycmd-91r adds the
// case ApplyTriggerMsg branch in update.go that returns the actual
// applyCmd(hostsPath) tea.Cmd. Until that bead lands, ApplyTriggerMsg is a
// no-op (consumed but not acted on) — store mutations still happen and
// ~/.hosts is still written via store actions; only the /etc/hosts apply
// half is deferred. This is the smallest possible seam between this bead and
// applycmd-91r.

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/src/internal/domain"
	"github.com/hungthai1401/hostie/src/internal/tui/components"
)

// Modal IDs for ModalResultMsg routing. Per-call-site constants keep the
// dispatch switch in modal_routing.go unambiguous (one ID per logical action).
const (
	modalIDDeleteEntry  = "delete-entry"
	modalIDQuitDirty    = "quit-dirty"
	modalIDEntryAdd     = "entry-add"
	modalIDEntryEdit    = "entry-edit"
	modalIDGroupCreate  = "group-create"
	modalIDMoveToGroup  = components.MoveToGroupModalID // canonical, declared in the modal package
	modalIDHelp         = "help"
)

// ApplyTriggerMsg is emitted after every successful store mutation to signal
// that the apply pipeline (apply.Runner.Apply) should run. applycmd-91r adds
// the Update branch that maps ApplyTriggerMsg → applyCmd(hostsPath) tea.Cmd
// and surfaces the result through the StatusBar.
//
// Per D13: this message fires regardless of apply outcome. The store mutation
// has already happened; the apply pipeline runs on top of the now-mutated
// state. A failed apply leaves ~/.hosts written and shows an error banner.
type ApplyTriggerMsg struct{}

// triggerApply returns a tea.Cmd that yields ApplyTriggerMsg.
//
// Wired by app-applycmd-91r to apply.Runner.Apply with StatusBar plumbing.
// In this bead the Cmd is fired after every mutation so the Update loop sees
// the message and (post-applycmd) runs the apply pipeline.
func triggerApply() tea.Cmd {
	return func() tea.Msg { return ApplyTriggerMsg{} }
}

// -----------------------------------------------------------------------------
// Normal-mode mutation key handlers
// -----------------------------------------------------------------------------

// handleSpaceToggle implements the Space keybind from v1
// src/tui/hooks/useKeyboard.ts: toggle the currently-selected entry's
// Enabled flag and trigger auto-apply. No confirmation modal — Space is the
// "fast path" mutation in v1.
//
// No-op if there is no selected entry (matches v1 `selectedEntryId` guard).
func (m Model) handleSpaceToggle() (Model, tea.Cmd) {
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	m.store.ToggleEntry(id)
	return m, triggerApply()
}

// handleDeleteKey opens the ConfirmModal for entry deletion. The actual
// DeleteEntry call is deferred to dispatchModalResult so the operator has a
// chance to cancel via Esc/n.
func (m Model) handleDeleteKey() (Model, tea.Cmd) {
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	modal := components.NewConfirmModal(modalIDDeleteEntry, "Delete this entry?")
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleAddKey opens the EntryEditorModal in add mode.
func (m Model) handleAddKey() (Model, tea.Cmd) {
	modal := components.NewEntryEditorModal(modalIDEntryAdd)
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleEditKey opens the EntryEditorModal pre-populated with the currently
// selected entry. No-op if nothing is selected (matches v1).
func (m Model) handleEditKey() (Model, tea.Cmd) {
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	entry, ok := findEntryByID(m.store.HostsFile().Groups, id)
	if !ok {
		return m, nil
	}
	modal := components.NewEntryEditorModalForEdit(modalIDEntryEdit, entry)
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleGroupKey opens the GroupCreatorModal with the currently-selected
// group path as the parent (mirrors v1 which passes selectedGroupPath as
// parentPath).
func (m Model) handleGroupKey() (Model, tea.Cmd) {
	parent := m.store.SelectedGroupPath()
	modal := components.NewGroupCreatorModal(modalIDGroupCreate, parent)
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleMoveKey opens the MoveToGroupModal. No-op if no entry is selected
// (matches v1 — `selectedEntryId` guard).
func (m Model) handleMoveKey() (Model, tea.Cmd) {
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	modal := components.NewMoveToGroupModal(m.store.HostsFile().Groups)
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleHelpKey opens the HelpModal. The modal is close-only ('?' or Esc
// inside the modal emits a ModalResultMsg with Confirmed=false and no
// payload); dispatchModalResult treats modalIDHelp as a no-op acknowledgment.
func (m Model) handleHelpKey() (Model, tea.Cmd) {
	modal := components.NewHelpModal(modalIDHelp)
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// handleQuitKey ports the v1 useKeyboard.ts q/ctrl+c branch. In v1 a dirty
// store opens a ConfirmModal "Unsaved changes — quit anyway?"; clean stores
// quit immediately.
//
// Note on D11 semantics: with auto-apply on every mutation, store.Dirty() is
// (in practice) always false at q-time because each mutation auto-applies and
// the store's clear-dirty path runs via apply success. However, applycmd-91r
// is the bead that wires clear-dirty; in this bead we still mirror v1's
// dirty-check so that:
//   - apply-failure scenarios (D13) where the store has been mutated but
//     /etc/hosts apply errored still get the confirmation.
//   - the implementation matches v1 line-for-line until applycmd-91r decides
//     whether to keep or simplify the guard.
func (m Model) handleQuitKey() (Model, tea.Cmd) {
	if !m.store.Dirty() {
		m.quitting = true
		return m, tea.Quit
	}
	modal := components.NewConfirmModal(modalIDQuitDirty, "Unsaved changes — quit anyway?")
	cmd := m.modalHost.Open(modal)
	return m, cmd
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// findEntryByID walks the group tree and returns the first entry matching
// id. Used by handleEditKey to seed the EntryEditorModal.
func findEntryByID(groups []domain.Group, id string) (domain.Entry, bool) {
	for _, g := range groups {
		for _, e := range g.Entries {
			if e.ID == id {
				return e, true
			}
		}
		if e, ok := findEntryByID(g.Groups, id); ok {
			return e, true
		}
	}
	return domain.Entry{}, false
}
