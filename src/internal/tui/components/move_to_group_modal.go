// MoveToGroupModal — selects a target group path for moving an entry.
//
// Ports v1 src/tui/components/MoveToGroupModal.tsx into the Bubble Tea modal
// pattern established by the spike (see
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md and components/modal.go).
//
// Behavior mirrors v1:
//
//   - Receives a slice of domain.Group trees and flattens them recursively
//     into a single navigable list. Each row's display name is the slash-
//     joined path (e.g., "work/prod") and the indent level reflects depth so
//     hierarchy is visible at a glance.
//   - j / Down arrow → move selection down (clamped to last row).
//   - k / Up arrow → move selection up (clamped to row 0).
//   - Enter → emit ModalResultMsg{Confirmed: true, Data: []string{path...}}
//     with the selected group's path, then close.
//   - Esc → emit ModalResultMsg{Confirmed: false, Data: nil} and close.
//   - Unknown keys are ignored (consistent with ConfirmModal).
//
// Determinism: this modal is the bead's flake canary. v1 had a known Esc race
// between the App's useInput and the modal's useInput. In Bubble Tea the
// equivalent footgun is foreclosed by ModalHost (see app/modal_host.go) which
// routes KeyMsg to the active modal first and the root Update never sees
// keys while a modal is active. The TestMoveToGroupModal_EscDeterminism test
// exercises 100 in-process iterations and `go test -count=100` multiplies
// that into 10,000 cycles under -race.

package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// MoveToGroupModalID is the canonical modal id for routing in app/update.go.
// Per-call-site constants (rather than ad-hoc strings) keep ModalResultMsg
// handlers unambiguous.
const MoveToGroupModalID = "move-to-group"

// flatGroup is the rendered representation of one entry in the flattened
// group list. path is the full slash-segmented path; displayName is the
// pre-joined display string; level is the indent depth (0 = top-level).
type flatGroup struct {
	path        []string
	displayName string
	level       int
}

// MoveToGroupModal is a list-picker modal that lets the user choose a target
// group to move an entry into. Implements components.Modal.
//
// Construct via NewMoveToGroupModal — direct struct literals risk leaving the
// id zero, which breaks ModalResultMsg routing in app/modal_host.go.
type MoveToGroupModal struct {
	id       string
	groups   []flatGroup
	selected int
}

// NewMoveToGroupModal constructs the modal from the live group tree. The
// groups slice is walked recursively (depth-first, parent-before-children)
// to produce the flat list. An empty tree is permitted — the View renders
// a "No groups available" placeholder and Enter becomes a no-op (matches v1).
func NewMoveToGroupModal(groups []domain.Group) MoveToGroupModal {
	return MoveToGroupModal{
		id:       MoveToGroupModalID,
		groups:   flattenGroups(groups, nil, 0),
		selected: 0,
	}
}

// flattenGroups walks the group tree depth-first and produces a flat list
// suitable for list navigation. Parent rows appear immediately before their
// children (matches v1 visual order).
func flattenGroups(groups []domain.Group, parentPath []string, level int) []flatGroup {
	var out []flatGroup
	for _, g := range groups {
		// Copy parentPath to avoid aliasing across siblings.
		currentPath := make([]string, len(parentPath)+1)
		copy(currentPath, parentPath)
		currentPath[len(parentPath)] = g.Name

		out = append(out, flatGroup{
			path:        currentPath,
			displayName: joinPath(currentPath),
			level:       level,
		})
		if len(g.Groups) > 0 {
			out = append(out, flattenGroups(g.Groups, currentPath, level+1)...)
		}
	}
	return out
}

// joinPath joins a path with "/" — extracted so tests can use the same logic
// without importing strings.
func joinPath(p []string) string {
	out := ""
	for i, seg := range p {
		if i > 0 {
			out += "/"
		}
		out += seg
	}
	return out
}

// ID implements Modal.
func (m MoveToGroupModal) ID() string { return m.id }

// Selected returns the currently highlighted row index. Exported for tests.
func (m MoveToGroupModal) Selected() int { return m.selected }

// FlatCount returns the number of rows in the flattened list. Exported for
// tests so they don't have to reach into the unexported field.
func (m MoveToGroupModal) FlatCount() int { return len(m.groups) }

// SelectedPath returns the path slice currently highlighted, or nil if the
// list is empty. Exported for tests and for handlers that need to inspect
// state without firing Enter.
func (m MoveToGroupModal) SelectedPath() []string {
	if len(m.groups) == 0 {
		return nil
	}
	return m.groups[m.selected].path
}

// Init implements Modal. No async initialization is required.
func (m MoveToGroupModal) Init() tea.Cmd { return nil }

// Update implements Modal.
//
// Non-KeyMsg events fall through to (self, nil) so ModalHost can pass them
// (WindowSizeMsg in particular) to the underlying layout. The default branch
// inside the keymap switch is also a no-op — only the keys listed above are
// consumed; everything else falls through and the root Update never sees it
// because ModalHost.Update intercepts first.
func (m MoveToGroupModal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		return m, m.emitCancel()
	case tea.KeyEnter:
		return m, m.emitSelect()
	case tea.KeyDown:
		m.selected = clampSel(m.selected+1, len(m.groups))
		return m, nil
	case tea.KeyUp:
		m.selected = clampSel(m.selected-1, len(m.groups))
		return m, nil
	}

	// Rune keys (j/k for vim navigation, matches v1).
	if keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 {
		switch keyMsg.Runes[0] {
		case 'j':
			m.selected = clampSel(m.selected+1, len(m.groups))
			return m, nil
		case 'k':
			m.selected = clampSel(m.selected-1, len(m.groups))
			return m, nil
		}
	}

	return m, nil
}

// clampSel keeps the selection within [0, n-1]. When the list is empty the
// selection stays at 0 — Enter is a no-op in that branch anyway.
func clampSel(idx, n int) int {
	if n <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

// emitSelect returns a Cmd yielding the success ModalResultMsg. Empty list
// emits Confirmed=false (matches v1's "Enter on empty list is cancel-ish":
// v1 returned early without calling onSelect, leaving the modal open — we
// improve on that by emitting a cancel so the host actually closes; the
// caller can no-op on Data == nil. This is a deliberate, documented
// divergence approved by the modal contract.).
func (m MoveToGroupModal) emitSelect() tea.Cmd {
	id := m.id
	if len(m.groups) == 0 {
		return func() tea.Msg {
			return ModalResultMsg{ID: id, Confirmed: false}
		}
	}
	// Copy the path so the Cmd is stable even if m is replaced later.
	src := m.groups[m.selected].path
	path := make([]string, len(src))
	copy(path, src)
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: true, Data: path}
	}
}

// emitCancel returns a Cmd yielding the cancel ModalResultMsg.
func (m MoveToGroupModal) emitCancel() tea.Cmd {
	id := m.id
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: false}
	}
}

// View implements Modal. Renders a bordered card with a title, the
// (possibly-empty) flattened list, and a key-hint line.
func (m MoveToGroupModal) View() string {
	title := mtgTitleStyle.Render("Move to Group")

	var body string
	if len(m.groups) == 0 {
		body = mtgEmptyStyle.Render("No groups available")
	} else {
		rows := make([]string, 0, len(m.groups))
		for i, g := range m.groups {
			indent := ""
			for j := 0; j < g.level; j++ {
				indent += "  "
			}
			marker := "  "
			if i == m.selected {
				marker = "▶ "
			}
			line := indent + marker + g.displayName
			if i == m.selected {
				line = mtgSelectedStyle.Render(line)
			} else {
				line = mtgRowStyle.Render(line)
			}
			rows = append(rows, line)
		}
		body = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	hint := mtgHintStyle.Render("j/k to navigate • Enter to select • Esc to cancel")

	card := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		body,
		"",
		hint,
	)
	return mtgFrameStyle.Render(card)
}

// --- Styles -----------------------------------------------------------------

var (
	mtgFrameStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("51")). // cyan (matches v1)
			Padding(1, 2).
			Width(60)

	mtgTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))

	mtgRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	mtgSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("51")).
				Bold(true)

	mtgEmptyStyle = lipgloss.NewStyle().Faint(true)

	mtgHintStyle = lipgloss.NewStyle().Faint(true)
)
