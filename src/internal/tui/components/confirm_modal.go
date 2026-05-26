// ConfirmModal — Yes/No dialog for destructive actions.
//
// This is the spike implementation that establishes the Modal pattern for the
// four remaining Phase 4 modals (GroupCreator, EntryEditor, MoveToGroup,
// Help). See .spikes/go-migration/p4-modal-pattern/FINDINGS.md for the
// pattern verdict and the determinism methodology (the v1 MoveToGroup Esc
// flake is the canary this spike forecloses).
//
// Behavior mirrors v1 src/tui/components/ConfirmModal.tsx:
//
//   - Default selection: Yes (Selected == 0).
//   - Left arrow → Yes, Right arrow → No (sets selection only, no return).
//   - 'y' / 'Y' → emit Confirmed=true and close.
//   - 'n' / 'N' → emit Confirmed=false and close.
//   - Enter → emit Confirmed=(Selected == 0) and close.
//   - Esc → emit Confirmed=false and close (canonical cancel).
//
// All routing is deterministic: every KeyMsg is consumed by exactly one
// branch in Update, and the return path is a single tea.Cmd that produces a
// ModalResultMsg. The determinism test (confirm_modal_test.go,
// TestConfirmModal_EscDeterminism) drives the Open → Esc flow ≥100 times in a
// tight loop and asserts the result is identical every iteration.

package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmSelection enumerates the focusable buttons.
type confirmSelection int

const (
	// confirmYes is the default selection on open (matches v1).
	confirmYes confirmSelection = 0
	// confirmNo is reached via right-arrow.
	confirmNo confirmSelection = 1
)

// ConfirmModal is a Yes/No confirmation dialog. Implements components.Modal.
//
// Construct via NewConfirmModal — direct struct literals risk leaving the
// id zero, which breaks ModalResultMsg routing in app/modal_host.go.
type ConfirmModal struct {
	id       string
	message  string
	selected confirmSelection
}

// NewConfirmModal constructs a ConfirmModal with the given id and message.
//
// id is the value echoed on the ModalResultMsg so the root Update can route
// the result to the correct handler. Pass a per-call-site constant (e.g.,
// "delete-entry", "quit-with-dirty") so handlers are unambiguous.
func NewConfirmModal(id, message string) ConfirmModal {
	return ConfirmModal{
		id:       id,
		message:  message,
		selected: confirmYes,
	}
}

// ID implements Modal.
func (c ConfirmModal) ID() string { return c.id }

// Message returns the prompt text. Exported for tests.
func (c ConfirmModal) Message() string { return c.message }

// Selected reports the currently highlighted button. Exported for tests.
func (c ConfirmModal) Selected() confirmSelection { return c.selected }

// Init implements Modal. ConfirmModal has no async initialization.
func (c ConfirmModal) Init() tea.Cmd { return nil }

// Update implements Modal.
//
// The fall-through (default branch) is a no-op return rather than a "consume
// everything" sink — this matches the contract: ModalHost.Update receives
// the unchanged tea.Msg and passes non-KeyMsg events (WindowSizeMsg) through
// to the underlying layout untouched.
func (c ConfirmModal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		return c, c.emit(false)
	case tea.KeyEnter:
		return c, c.emit(c.selected == confirmYes)
	case tea.KeyLeft:
		c.selected = confirmYes
		return c, nil
	case tea.KeyRight:
		c.selected = confirmNo
		return c, nil
	}

	// Rune keys (y/Y/n/N).
	if keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 {
		switch keyMsg.Runes[0] {
		case 'y', 'Y':
			return c, c.emit(true)
		case 'n', 'N':
			return c, c.emit(false)
		}
	}

	return c, nil
}

// emit returns a tea.Cmd that yields a ModalResultMsg for ModalHost to
// intercept. The capture of c.id (value) means the message is stable even
// if the modal is replaced before the Cmd runs.
func (c ConfirmModal) emit(confirmed bool) tea.Cmd {
	id := c.id
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: confirmed}
	}
}

// View implements Modal. Renders a bordered, centered card with the message
// and two buttons. The selected button gets reversed colors to match v1's
// inverse-video highlight.
func (c ConfirmModal) View() string {
	yes := buttonStyle(c.selected == confirmYes, confirmYesColor).Render(" Yes ")
	no := buttonStyle(c.selected == confirmNo, confirmNoColor).Render(" No ")

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yes, "  ", no)

	body := lipgloss.JoinVertical(
		lipgloss.Center,
		confirmMessageStyle.Render(c.message),
		"",
		buttons,
		"",
		confirmHintStyle.Render("← → to navigate • Enter to confirm • Esc to cancel"),
	)

	return confirmFrameStyle.Render(body)
}

// --- Styles -----------------------------------------------------------------

const (
	confirmYesColor = "42"  // green (matches v1)
	confirmNoColor  = "196" // red (matches v1)
)

var (
	confirmFrameStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")). // yellow accent (matches v1)
				Padding(1, 3).
				Align(lipgloss.Center)

	confirmMessageStyle = lipgloss.NewStyle().Bold(true)

	confirmHintStyle = lipgloss.NewStyle().Faint(true)
)

// buttonStyle returns the per-state Lipgloss style for a Yes/No button.
//
// Selected → reversed (background = accent, foreground = black). Unselected →
// plain accent foreground. This matches v1's `inverse` attribute toggling.
func buttonStyle(selected bool, color string) lipgloss.Style {
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color(color)).
			Bold(true)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}
