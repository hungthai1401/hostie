// Package app: modal host — overlay routing for the TUI modal pattern.
//
// This file is the second half of the spike artifact for bead
// hosts-cli-go-mig-p4-modal-spike-xh5 (the first half is
// components/modal.go + components/confirm_modal.go). See
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md for the design
// rationale and the v1-flake autopsy that motivated it.
//
// Invariant the four subsequent modal beads MUST preserve:
//
//   - When a modal is active, the root Update MUST delegate to
//     ModalHost.Update BEFORE its own key router runs. The modal consumes
//     every key (deterministically — see TestConfirmModal_EscDeterminism).
//   - Non-KeyMsg events (WindowSizeMsg in particular) are passed through
//     unchanged so the base layout still reflows behind the overlay.
//   - When ModalHost returns a (nil, cmd) pair, the caller closes the modal
//     and runs cmd to deliver the result to the per-result handler.
//
// The three-line root-Update integration (deferred to bead
// hosts-cli-go-mig-p4-app-mutations-9fk because update.go is reserved by
// the search-mode worker) looks like this in Model.Update at the very top
// of the switch:
//
//	if m.modalHost != nil && m.modalHost.Active() {
//	    next, cmd := m.modalHost.Update(msg)
//	    m.modalHost = next
//	    return m, cmd
//	}
//
// Plus one new case for components.ModalResultMsg to route results into
// per-modal-id handlers. See FINDINGS.md §5 "Integration patch".

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/src/internal/tui/components"
	"github.com/hungthai1401/hostie/src/internal/tui/store"
)

// ModalHost owns the active modal (if any) and routes tea.Msgs to it.
//
// The host is a single-slot stack — modals do not nest in the hostie TUI
// (confirmed against v1: nested modals were never reachable). If a future
// requirement demands nesting, replace the field with a []components.Modal
// and adjust Active/View accordingly.
//
// Concurrency: ModalHost is not safe for concurrent use; Bubble Tea's Update
// loop is single-threaded by contract, which is the only writer.
type ModalHost struct {
	store  *store.Store
	active components.Modal
}

// NewModalHost constructs a ModalHost bound to the given store. The store is
// needed so Open/CloseModal can keep StoreMode in sync (ModeModal vs
// ModeNormal) — the status bar reads from the store and would otherwise lag
// the actual modal state.
func NewModalHost(s *store.Store) *ModalHost {
	return &ModalHost{store: s}
}

// Active reports whether a modal is currently displayed. Use this in the root
// Update to decide whether to intercept keys.
func (h *ModalHost) Active() bool {
	return h != nil && h.active != nil
}

// Current returns the currently-active modal (nil if none). Exposed for
// tests and for app/view.go to render the overlay.
func (h *ModalHost) Current() components.Modal {
	if h == nil {
		return nil
	}
	return h.active
}

// Open pushes a modal onto the host and switches the store into ModeModal.
// Returns the modal's Init cmd (may be nil) so the caller can include it in
// the tea.Cmd batch.
//
// Calling Open while a modal is already active replaces the existing modal —
// this matches v1's "last-open-wins" semantics. The replaced modal does not
// emit a result.
func (h *ModalHost) Open(m components.Modal) tea.Cmd {
	if h == nil {
		return nil
	}
	h.active = m
	if h.store != nil {
		h.store.OpenModal(store.ModalConfirmation, nil)
	}
	return m.Init()
}

// Close clears the active modal and restores the store to ModeNormal. Safe
// to call when no modal is active (idempotent no-op).
func (h *ModalHost) Close() {
	if h == nil {
		return
	}
	h.active = nil
	if h.store != nil {
		h.store.CloseModal()
	}
}

// Update routes a tea.Msg through the active modal.
//
// Returns:
//
//   - If no modal is active: (h, nil). Caller continues with normal routing.
//   - If the modal produced a ModalResultMsg-yielding Cmd: (h, cmd). The
//     caller dispatches cmd in the tea.Cmd batch; ModalHost.HandleResult
//     (called when the resulting ModalResultMsg reaches the root Update)
//     then performs the Close.
//   - Otherwise: (h, cmd) where cmd may be nil. The modal stays open.
//
// Note: ModalHost does NOT auto-close on cmd return — close happens when the
// ModalResultMsg is dispatched, so the per-modal-id handler in app/update.go
// runs in the same Update tick that closes the modal. This is the
// deterministic ordering the spike's flake-canary test (≥100 iterations)
// verifies.
func (h *ModalHost) Update(msg tea.Msg) (*ModalHost, tea.Cmd) {
	if h == nil || h.active == nil {
		return h, nil
	}

	// ModalResultMsg from this modal's own emit() — close and pass through
	// so the root Update's case branch can route by ID. This lets a single
	// ModalResultMsg fan out to per-modal-id handlers in app/update.go.
	if result, ok := msg.(components.ModalResultMsg); ok {
		if result.ID == h.active.ID() {
			h.Close()
		}
		return h, nil
	}

	next, cmd := h.active.Update(msg)
	h.active = next
	return h, cmd
}

// View renders the active modal, or "" if none. The root view composes this
// over the base layout (typically by replacing the main pane while the
// modal is open — see app/view.go for the composition).
func (h *ModalHost) View() string {
	if h == nil || h.active == nil {
		return ""
	}
	return h.active.View()
}
