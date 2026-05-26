// HelpModal — full keybind reference overlay.
//
// Ports v1 src/tui/components/HelpModal.tsx. The canonical key list comes
// from src/tui/hooks/useKeyboard.ts (the root key router); this modal is
// pure presentation plus its own close-key handling.
//
// Behavior:
//
//   - Renders categorized keybindings (Navigation, Actions, Modals, Entry
//     Editor, General) matching v1's layout.
//   - Esc → close, emit ModalResultMsg{ID: <id>, Confirmed: false}.
//   - '?' → close, same result. (v1 toggles the modal; in Bubble Tea the
//     "toggle" semantics live in the root key router — this modal just
//     closes.)
//   - Every other key is a no-op (the spike contract: a modal consumes
//     every key while active, so unhandled keys must not fall through to
//     the root Update).
//
// All routing is deterministic: a single Update branch per key, no
// goroutines, no timers. The determinism canary in help_modal_test.go
// drives the Open → Esc and Open → '?' flows ≥100 times each.

package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpBinding is one row in the help table (key + human description).
type helpBinding struct {
	Key  string
	Desc string
}

// helpCategory groups bindings under a heading (mirrors v1's
// KeybindingCategory in HelpModal.tsx).
type helpCategory struct {
	Title    string
	Bindings []helpBinding
}

// helpKeybindings is the canonical list ported from
// src/tui/components/HelpModal.tsx. Source of truth for the actual
// routing lives in src/tui/hooks/useKeyboard.ts — keep these aligned when
// either changes.
var helpKeybindings = []helpCategory{
	{
		Title: "Navigation",
		Bindings: []helpBinding{
			{"j", "Move down"},
			{"k", "Move up"},
			{"h", "Focus sidebar"},
			{"l", "Focus main"},
			{"Tab", "Switch focus (sidebar ↔ main)"},
			{"Esc", "Return to sidebar"},
			{"/", "Enter search mode"},
		},
	},
	{
		Title: "Actions",
		Bindings: []helpBinding{
			{"Space", "Toggle entry enabled/disabled"},
			{"d", "Delete selected entry"},
			{"a", "Add a new entry"},
			{"e", "Edit selected entry"},
			{"g", "Create a new group"},
			{"m", "Move entry to another group"},
			{"Enter", "Apply changes to /etc/hosts"},
			{"Ctrl+S", "Apply changes to /etc/hosts"},
		},
	},
	{
		Title: "Modals",
		Bindings: []helpBinding{
			{"?", "Show/hide this help"},
			{"Esc", "Close modal or cancel"},
			{"y / n", "Quick Yes/No in confirmations"},
			{"← →", "Navigate buttons in modals"},
		},
	},
	{
		Title: "Entry Editor",
		Bindings: []helpBinding{
			{"Tab", "Move to next field"},
			{"Shift+Tab", "Move to previous field"},
			{"Space", "Toggle enabled checkbox"},
			{"Ctrl+U", "Clear current field"},
			{"Backspace", "Delete character"},
		},
	},
	{
		Title: "General",
		Bindings: []helpBinding{
			{"q", "Quit application"},
			{"Ctrl+C", "Exit application"},
		},
	},
}

// HelpModal is the keybind reference overlay. Implements components.Modal.
//
// Construct via NewHelpModal — the zero-value id would break
// ModalResultMsg routing in app/modal_host.go.
type HelpModal struct {
	id string
}

// NewHelpModal constructs a HelpModal with the given routing id. The id is
// echoed on ModalResultMsg so the root Update can route the close event
// (typical site uses "help").
func NewHelpModal(id string) HelpModal {
	return HelpModal{id: id}
}

// ID implements Modal.
func (h HelpModal) ID() string { return h.id }

// Init implements Modal. HelpModal has no async initialization.
func (h HelpModal) Init() tea.Cmd { return nil }

// Update implements Modal. Esc and '?' close the modal; every other key
// is silently consumed (the modal must not leak keys to the root Update
// while open — that was the v1 flake class). Non-key messages are passed
// through unchanged so WindowSizeMsg still reflows the base layout.
func (h HelpModal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}

	if keyMsg.Type == tea.KeyEsc {
		return h, h.emit()
	}

	if keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == '?' {
		return h, h.emit()
	}

	return h, nil
}

// emit returns a tea.Cmd that yields the close ModalResultMsg. HelpModal
// returns no payload — the result is purely a "modal closed" signal
// (Confirmed=false, Data=nil) consistent with the spike contract.
func (h HelpModal) emit() tea.Cmd {
	id := h.id
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: false}
	}
}

// View implements Modal. Renders a bordered card with categorized
// keybindings; matches the v1 visual layout (cyan border, yellow category
// headings, green keys, dim descriptions, footer hint).
func (h HelpModal) View() string {
	rows := []string{
		helpTitleStyle.Render("Help - Keyboard Shortcuts"),
		"",
	}

	for i, cat := range helpKeybindings {
		rows = append(rows, helpCategoryStyle.Render(cat.Title))
		for _, b := range cat.Bindings {
			key := helpKeyStyle.Render(padRight(b.Key, helpKeyColWidth))
			desc := helpDescStyle.Render(b.Desc)
			rows = append(rows, "  "+lipgloss.JoinHorizontal(lipgloss.Top, key, desc))
		}
		if i < len(helpKeybindings)-1 {
			rows = append(rows, "")
		}
	}

	rows = append(rows,
		"",
		helpDescStyle.Render("Press ")+helpAccentStyle.Render("?")+
			helpDescStyle.Render(" or ")+helpAccentStyle.Render("Esc")+
			helpDescStyle.Render(" to close"),
	)

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return helpFrameStyle.Render(body)
}

// padRight is a tiny helper to align the key column without pulling in a
// table library. The Go runewidth-aware path isn't needed here because the
// keys ported from v1 are ASCII + a small set of well-known glyphs whose
// display width matches their rune count under standard terminal fonts.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	pad := width - len(s)
	out := make([]byte, 0, len(s)+pad)
	out = append(out, s...)
	for i := 0; i < pad; i++ {
		out = append(out, ' ')
	}
	return string(out)
}

// --- Styles -----------------------------------------------------------------

const helpKeyColWidth = 12

var (
	helpFrameStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("51")). // cyan (matches v1)
			Padding(1, 3)

	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("51")) // cyan

	helpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("226")) // yellow

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // green

	helpDescStyle = lipgloss.NewStyle().Faint(true)

	helpAccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
)
