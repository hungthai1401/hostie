// Package components: modal pattern contract.
//
// This file establishes the Modal interface that every TUI modal (ConfirmModal,
// GroupCreatorModal, EntryEditorModal, MoveToGroupModal, HelpModal) must
// implement. It is the spike artifact for bead
// hosts-cli-go-mig-p4-modal-spike-xh5 — see
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md for the verdict and the
// methodology behind the contract.
//
// Why a thin interface, not vanilla per-modal Cmd plumbing?
//
//   - v1 (React/Ink) leaked an Esc-routing race in MoveToGroupModal: the
//     parent App component and the modal both listened for Esc via useInput,
//     producing a non-deterministic close (the bug behind this spike).
//   - In Bubble Tea, the equivalent footgun is two Update branches both
//     consuming the same tea.KeyMsg. The fix is structural: a ModalHost (see
//     app/modal_host.go) routes KeyMsg to the active modal FIRST and the
//     root Update never sees keys while a modal is active. This contract
//     enforces that exactly one Update receives any given key.
//
// Modal lifecycle:
//
//  1. Caller invokes ModalHost.Open(modal) — pushes the modal onto the
//     single-element stack and switches the store into ModeModal.
//  2. Every tea.Msg (KeyMsg in particular) reaches the modal's Update via
//     ModalHost.Update — the root app.Update intercepts BEFORE its own key
//     router runs (see FINDINGS.md for the 3-line integration patch).
//  3. The modal returns a tea.Cmd that yields a ModalResultMsg when the
//     user's interaction completes (Enter, Esc, y/n, etc.).
//  4. ModalHost.Update recognizes ModalResultMsg, closes the modal, calls
//     CloseModal on the store, and forwards the result to the caller via the
//     same tea.Cmd channel so app/update.go's per-result handler runs.
package components

import tea "github.com/charmbracelet/bubbletea"

// Modal is the contract every TUI modal implements.
//
// It mirrors tea.Model but is intentionally distinct so the type system can
// distinguish "the root program" from "a modal nested inside it". The
// Update method returns a Modal (not a tea.Model) so callers cannot
// accidentally wire a modal as the root.
type Modal interface {
	// Init returns an optional command to run when the modal is opened
	// (e.g., focusing a text input, kicking off a fetch). Modals with no
	// initialization return nil.
	Init() tea.Cmd

	// Update receives the raw tea.Msg. Modals consume KeyMsg, never
	// WindowSizeMsg (the ModalHost passes WindowSize through unchanged so
	// the underlying layout still reflows). The returned Modal MUST be the
	// same concrete type as the receiver — ModalHost stores it back.
	Update(msg tea.Msg) (Modal, tea.Cmd)

	// View renders the modal's body. The ModalHost overlays this string on
	// top of the base View in app/view.go.
	View() string

	// ID returns a stable identifier for this modal instance. ModalHost
	// includes it in ModalResultMsg so callers can route results to the
	// right handler (one ModalResultMsg type, many possible originators).
	ID() string
}

// ModalResultMsg is the universal result message emitted by every modal when
// its interaction completes. Modals construct it via tea.Cmd returned from
// their Update method; ModalHost detects it and closes the modal stack.
//
// Confirmed is the canonical "OK/Cancel" outcome; Data carries any
// per-modal payload (entry being created, target group path, etc.).
// Modals that don't need a payload leave Data nil — callers must type-assert
// based on the modal type they routed.
//
// ID echoes the originating Modal.ID() so the root Update can dispatch to
// the correct handler when multiple modals could be active in a session.
type ModalResultMsg struct {
	ID        string
	Confirmed bool
	Data      any
}
