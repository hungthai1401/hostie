// GroupCreatorModal — form for creating a new group.
//
// Ports v1 src/tui/components/GroupCreatorModal.tsx. v1 shipped a single Name
// input; the Go port (bead hosts-cli-go-mig-p4-modal-groupcreator-zed)
// extends it with a Description field per the Phase 4 dispatch contract:
// Tab cycles focus between Name and Description, Enter submits, Esc cancels.
//
// Validation (preserved from v1):
//
//   - Name: required (non-empty), kebab-case only ([a-z0-9-]+), no leading/
//     trailing/consecutive hyphens, no slashes.
//   - Description: optional, any text.
//
// Uniqueness against existing groups is NOT checked here — the modal has no
// access to the hosts file. The app-layer dispatcher (bead
// hosts-cli-go-mig-p4-app-mutations-9fk) is the correct place to reject a
// duplicate name and re-open the modal with an error, since only it has the
// full hosts tree in scope.
//
// Implements components.Modal. Result Data is a domain.Group with Name and
// Description populated and empty Entries/Groups slices (the dispatcher
// merges into the parent group).
//
// Determinism: every KeyMsg branches to exactly one outcome (mutate input,
// switch focus, submit, cancel). The determinism canary in
// group_creator_modal_test.go runs the Esc and Enter paths ≥100 iterations
// each — together with `-count=100` this exercises 10,000 cycles to foreclose
// the v1 MoveToGroupModal Esc-routing flake class on this modal.

package components

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// groupCreatorFocus enumerates the focusable inputs (in Tab order).
type groupCreatorFocus int

const (
	groupCreatorFocusName groupCreatorFocus = iota
	groupCreatorFocusDescription
)

// kebabCaseRE is v1's group-name shape: lowercase letters, digits, hyphens.
// Leading/trailing/consecutive hyphens are rejected by separate guards so the
// error messages can be specific (matches v1 validateGroupName).
var kebabCaseRE = regexp.MustCompile(`^[a-z0-9-]+$`)

// GroupCreatorModal is a two-field form for creating a new group.
//
// Construct via NewGroupCreatorModal. id is echoed on the ModalResultMsg so
// the app-layer dispatcher can route the result to the correct handler.
type GroupCreatorModal struct {
	id       string
	parent   []string // optional parent path, shown above the form
	name     textinput.Model
	desc     textinput.Model
	focus    groupCreatorFocus
	errorMsg string
}

// NewGroupCreatorModal constructs a GroupCreatorModal with the given id and
// optional parent path. The Name input is focused on open.
//
// parentPath is purely display — the dispatcher determines actual placement
// in the hosts tree.
func NewGroupCreatorModal(id string, parentPath []string) GroupCreatorModal {
	name := textinput.New()
	name.Placeholder = "group-name"
	name.CharLimit = 64
	name.Width = 40
	name.Focus()

	desc := textinput.New()
	desc.Placeholder = "Optional description"
	desc.CharLimit = 128
	desc.Width = 40

	return GroupCreatorModal{
		id:     id,
		parent: parentPath,
		name:   name,
		desc:   desc,
		focus:  groupCreatorFocusName,
	}
}

// ID implements Modal.
func (g GroupCreatorModal) ID() string { return g.id }

// Name returns the current name input value. Exported for tests.
func (g GroupCreatorModal) Name() string { return g.name.Value() }

// Description returns the current description input value. Exported for tests.
func (g GroupCreatorModal) Description() string { return g.desc.Value() }

// Focus reports which input is currently focused. Exported for tests.
func (g GroupCreatorModal) Focus() groupCreatorFocus { return g.focus }

// Error returns the current validation error string (empty when valid).
// Exported for tests.
func (g GroupCreatorModal) Error() string { return g.errorMsg }

// Init implements Modal. Kicks off the textinput blink cursor.
func (g GroupCreatorModal) Init() tea.Cmd { return textinput.Blink }

// Update implements Modal.
//
// Routing table (every KeyMsg branches to exactly one outcome):
//
//   - Esc → emit Confirmed=false, Data=nil. Cancel.
//   - Enter → validate; if valid emit Confirmed=true, Data=domain.Group.
//     If invalid set errorMsg and stay open (no result emitted).
//   - Tab / Shift+Tab → cycle focus between Name and Description.
//   - Any other key → delegated to the focused textinput.Model.
//
// Non-KeyMsg events (WindowSizeMsg, textinput.blinkMsg) are delegated to the
// focused input so cursor blink continues to animate.
func (g GroupCreatorModal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		// Delegate non-key messages (blink, etc.) to the focused input so
		// the cursor animation keeps running.
		return g.updateFocused(msg)
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		return g, g.emit(false, nil)

	case tea.KeyEnter:
		name := strings.TrimSpace(g.name.Value())
		if err := validateGroupName(name); err != "" {
			g.errorMsg = err
			return g, nil
		}
		group := domain.Group{
			Name:        name,
			Description: strings.TrimSpace(g.desc.Value()),
			Entries:     []domain.Entry{},
			Groups:      []domain.Group{},
		}
		return g, g.emit(true, group)

	case tea.KeyTab:
		g = g.cycleFocus(1)
		return g, nil

	case tea.KeyShiftTab:
		g = g.cycleFocus(-1)
		return g, nil
	}

	// Any other key (including runes, backspace, arrows) → delegate.
	return g.updateFocused(msg)
}

// updateFocused passes the message to the currently focused input and stores
// the updated model back. Clears errorMsg on any user input mutation so the
// error doesn't linger after the operator corrects the field.
func (g GroupCreatorModal) updateFocused(msg tea.Msg) (Modal, tea.Cmd) {
	var cmd tea.Cmd
	switch g.focus {
	case groupCreatorFocusName:
		g.name, cmd = g.name.Update(msg)
	case groupCreatorFocusDescription:
		g.desc, cmd = g.desc.Update(msg)
	}
	// Clear stale error on any keystroke so the operator gets immediate
	// visual feedback that their correction is being typed.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		g.errorMsg = ""
	}
	return g, cmd
}

// cycleFocus moves focus by delta (+1 or -1), wrapping. Blurs the old input
// and focuses the new one so the cursor renders in the right place.
func (g GroupCreatorModal) cycleFocus(delta int) GroupCreatorModal {
	const n = 2 // name + description
	next := groupCreatorFocus((int(g.focus) + delta + n) % n)
	g.focus = next
	switch next {
	case groupCreatorFocusName:
		g.desc.Blur()
		g.name.Focus()
	case groupCreatorFocusDescription:
		g.name.Blur()
		g.desc.Focus()
	}
	return g
}

// emit returns a tea.Cmd that yields a ModalResultMsg for ModalHost to
// intercept. Capturing id by value mirrors ConfirmModal.emit and keeps the
// message stable even if the modal is replaced before the Cmd runs.
func (g GroupCreatorModal) emit(confirmed bool, data any) tea.Cmd {
	id := g.id
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: confirmed, Data: data}
	}
}

// validateGroupName returns the empty string when valid, or a human-readable
// error message. Mirrors v1 src/tui/components/GroupCreatorModal.tsx
// validateGroupName.
func validateGroupName(name string) string {
	if name == "" {
		return "Group name cannot be empty"
	}
	if strings.Contains(name, "/") {
		return "Group name cannot contain slashes (use parent group for nesting)"
	}
	if strings.HasPrefix(name, "-") {
		return "Group name cannot start with a hyphen"
	}
	if strings.HasSuffix(name, "-") {
		return "Group name cannot end with a hyphen"
	}
	if strings.Contains(name, "--") {
		return "Group name cannot contain consecutive hyphens"
	}
	if !kebabCaseRE.MatchString(name) {
		return "Group name must be kebab-case (lowercase letters, numbers, and hyphens only)"
	}
	return ""
}

// View implements Modal. Renders a bordered card with the parent path (if
// any), the two inputs, an optional error, and the keybind hint.
func (g GroupCreatorModal) View() string {
	rows := []string{
		groupCreatorTitleStyle.Render("Create Group"),
		"",
	}

	if len(g.parent) > 0 {
		rows = append(rows,
			groupCreatorParentStyle.Render("Parent: ")+
				groupCreatorParentValueStyle.Render(strings.Join(g.parent, "/")),
			"",
		)
	}

	rows = append(rows,
		groupCreatorLabel("Name", g.focus == groupCreatorFocusName)+g.name.View(),
		groupCreatorLabel("Description", g.focus == groupCreatorFocusDescription)+g.desc.View(),
	)

	if g.errorMsg != "" {
		rows = append(rows, "", groupCreatorErrorStyle.Render("✗ "+g.errorMsg))
	}

	rows = append(rows, "", groupCreatorHintStyle.Render("[Tab] Switch field  [Enter] Save  [Esc] Cancel"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return groupCreatorFrameStyle.Render(body)
}

// groupCreatorLabel renders a field label with focus indicator. Focused
// labels are bold + colored to mirror v1's focus styling.
func groupCreatorLabel(text string, focused bool) string {
	label := text + ": "
	if focused {
		return groupCreatorFocusedLabelStyle.Render(label)
	}
	return groupCreatorLabelStyle.Render(label)
}

// --- Styles -----------------------------------------------------------------

var (
	groupCreatorFrameStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("51")). // cyan (matches v1)
				Padding(1, 2)

	groupCreatorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("51"))

	groupCreatorParentStyle = lipgloss.NewStyle().Faint(true)

	groupCreatorParentValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))

	groupCreatorLabelStyle = lipgloss.NewStyle().Faint(true)

	groupCreatorFocusedLabelStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("51"))

	groupCreatorErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	groupCreatorHintStyle = lipgloss.NewStyle().Faint(true)
)
