// modal_routing.go — ModalResultMsg dispatch keyed by Modal.ID().
//
// Every modal in components/ emits a ModalResultMsg whose ID echoes the
// modal's ID(). The root Update intercepts ModalResultMsg (via the modal
// host's pass-through in update.go) and forwards it here. This file owns
// the per-modal-id switch — adding a new modal means adding one case here
// plus the corresponding key handler in mutations.go.
//
// Why a separate file: keeps update.go small (just the key router) and
// makes the "what does this modal id do?" lookup trivial. Mirrors the
// recipe documented in .spikes/go-migration/p4-modal-pattern/FINDINGS.md
// §7 step 7 ("Wiring in the corresponding app-mutations follow-up").

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/components"
)

// dispatchModalResult routes a ModalResultMsg to the per-id handler.
//
// Cancel paths (Confirmed=false) are a no-op for everything except the
// quit-dirty modal (which exits when confirmed). The modal has already been
// closed by ModalHost.Update before this runs (see modal_host.go: result
// detection closes the host first), so handlers only need to perform the
// post-modal mutation.
func (m Model) dispatchModalResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	switch result.ID {
	case modalIDDeleteEntry:
		return m.handleDeleteResult(result)
	case modalIDQuitDirty:
		return m.handleQuitDirtyResult(result)
	case modalIDEntryAdd:
		return m.handleEntryAddResult(result)
	case modalIDEntryEdit:
		return m.handleEntryEditResult(result)
	case modalIDGroupCreate:
		return m.handleGroupCreateResult(result)
	case modalIDMoveToGroup:
		return m.handleMoveToGroupResult(result)
	case modalIDHelp:
		// HelpModal is close-only: the modal has already closed itself,
		// no mutation to perform. Acknowledge and return.
		return m, nil
	}
	// Unknown modal id — defensive no-op. A future modal that forgets to
	// add a case here will simply be a silent close; tests on the
	// modal-opening keybind catch this.
	return m, nil
}

// handleDeleteResult applies the deletion if confirmed and triggers apply.
// Cancel is a no-op (modal already closed). Mirrors v1 useKeyboard.ts 'd'
// branch's onConfirm: deleteEntry → re-select next/prev → writeHostsFile.
func (m Model) handleDeleteResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	// Compute next selection BEFORE deletion (mirrors v1).
	entries := flattenEntries(m.store.HostsFile().Groups)
	idx := indexOfEntry(entries, id)
	m.store.DeleteEntry(id)
	if len(entries) > 1 {
		switch {
		case idx >= 0 && idx < len(entries)-1:
			m.store.SelectEntry(entries[idx+1].ID)
		case idx > 0:
			m.store.SelectEntry(entries[idx-1].ID)
		}
	}
	return m, triggerApply()
}

// handleQuitDirtyResult honors the v1 "Unsaved changes — quit anyway?"
// confirmation. Confirmed=true → tea.Quit; cancel → stay in the TUI.
func (m Model) handleQuitDirtyResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	m.quitting = true
	return m, tea.Quit
}

// handleEntryAddResult adds the new entry to the currently-selected group
// (matches v1 store.addEntry: writes to SelectedGroupPath). Then triggers
// apply.
//
// Per D11 the apply trigger fires unconditionally; per D13 the YAML write
// inside store.AddEntry is independent from the apply outcome.
func (m Model) handleEntryAddResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	entry, ok := result.Data.(domain.Entry)
	if !ok {
		return m, nil
	}
	if entry.ID == "" {
		entry.ID = domain.NewID()
	}
	m.store.AddEntry(entry)
	m.store.SelectEntry(entry.ID)
	return m, triggerApply()
}

// handleEntryEditResult updates the in-place entry (ID preserved by the
// EntryEditorModal in edit mode).
func (m Model) handleEntryEditResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	entry, ok := result.Data.(domain.Entry)
	if !ok || entry.ID == "" {
		return m, nil
	}
	m.store.UpdateEntry(entry.ID, entry)
	return m, triggerApply()
}

// handleGroupCreateResult adds a new group beneath the currently-selected
// group path (matches v1 store.addGroup: parentPath = selectedGroupPath).
func (m Model) handleGroupCreateResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	group, ok := result.Data.(domain.Group)
	if !ok || group.Name == "" {
		return m, nil
	}
	m.store.AddGroup(group.Name, m.store.SelectedGroupPath())
	return m, triggerApply()
}

// handleMoveToGroupResult moves the currently-selected entry to the chosen
// group path. Cancel and empty-list (Data == nil) both fall through as
// no-ops, matching the modal's documented divergence from v1 (see
// move_to_group_modal.go emitSelect comment).
func (m Model) handleMoveToGroupResult(result components.ModalResultMsg) (Model, tea.Cmd) {
	if !result.Confirmed {
		return m, nil
	}
	path, ok := result.Data.([]string)
	if !ok || len(path) == 0 {
		return m, nil
	}
	id := m.store.SelectedEntryID()
	if id == "" {
		return m, nil
	}
	m.store.MoveEntry(id, path)
	return m, triggerApply()
}
