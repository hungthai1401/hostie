// EntryEditorModal — form modal for adding or editing a host entry.
//
// Ports v1 src/tui/components/EntryEditorModal.tsx. Implements the Modal
// contract documented in components/modal.go (see also
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md §7).
//
// One component covers both create and edit modes:
//
//   - NewEntryEditorModal(id) → blank form ("Add Entry").
//   - NewEntryEditorModalForEdit(id, entry) → pre-filled form ("Edit Entry").
//
// Fields: IP, Hostname, Aliases (comma-separated), Comment. Enabled is a
// togglable checkbox (Space). Each text field uses bubbles/textinput so the
// cursor, editing keys, and width handling come for free.
//
// Navigation: Tab cycles focus forward, Shift+Tab cycles backward. Enter
// submits if the form is valid; Esc cancels (emits ModalResultMsg with
// Confirmed=false). Invalid submit re-runs validation and shows the first
// error inline next to each offending field — exactly mirroring v1.
//
// Result payload (ModalResultMsg.Data) on submit: domain.Entry — IP, Hostname,
// Aliases, Enabled, Comment are populated; ID is preserved from the input
// entry in edit mode and left empty in add mode (the caller assigns a ULID).

package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// editorField enumerates focusable form fields.
type editorField int

const (
	fieldIP editorField = iota
	fieldHostname
	fieldAliases
	fieldEnabled
	fieldComment
	fieldSubmit

	editorFieldCount = 6
)

// EntryEditorMode distinguishes "Add Entry" from "Edit Entry" rendering and
// payload semantics. Exported so call-sites can inspect the modal post-hoc.
type EntryEditorMode int

const (
	EntryEditorModeAdd EntryEditorMode = iota
	EntryEditorModeEdit
)

// EntryEditorModal is the form modal. Implements components.Modal.
//
// Construct via NewEntryEditorModal (add) or NewEntryEditorModalForEdit
// (edit). Direct struct literals are unsupported — internal textinput models
// require initialization.
type EntryEditorModal struct {
	id   string
	mode EntryEditorMode
	// originalID is preserved in edit mode so the result payload echoes
	// the same ULID. Empty in add mode.
	originalID string

	ipInput       textinput.Model
	hostnameInput textinput.Model
	aliasesInput  textinput.Model
	commentInput  textinput.Model

	enabled bool
	focused editorField

	// Inline validation errors. Cleared on field change; populated by
	// submit-time validation.
	ipErr       string
	hostnameErr string
	aliasesErr  string
	commentErr  string
}

// NewEntryEditorModal constructs a blank editor in add mode.
func NewEntryEditorModal(id string) EntryEditorModal {
	m := newEntryEditorModal(id, EntryEditorModeAdd, "")
	m.enabled = true
	return m
}

// NewEntryEditorModalForEdit constructs an editor pre-filled with an existing
// entry. The entry's ID is preserved on the result payload.
func NewEntryEditorModalForEdit(id string, entry domain.Entry) EntryEditorModal {
	m := newEntryEditorModal(id, EntryEditorModeEdit, entry.ID)
	m.ipInput.SetValue(entry.IP)
	m.hostnameInput.SetValue(entry.Hostname)
	m.aliasesInput.SetValue(strings.Join(entry.Aliases, ", "))
	m.commentInput.SetValue(entry.Comment)
	m.enabled = entry.Enabled
	return m
}

func newEntryEditorModal(id string, mode EntryEditorMode, originalID string) EntryEditorModal {
	mk := func(placeholder string, width int) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.Width = width
		ti.Prompt = ""
		return ti
	}
	m := EntryEditorModal{
		id:            id,
		mode:          mode,
		originalID:    originalID,
		ipInput:       mk("192.168.1.1", 40),
		hostnameInput: mk("example.local", 40),
		aliasesInput:  mk("alias1, alias2", 40),
		commentInput:  mk("(optional)", 40),
		enabled:       true,
		focused:       fieldIP,
	}
	m.ipInput.Focus()
	return m
}

// ID implements Modal.
func (m EntryEditorModal) ID() string { return m.id }

// Init implements Modal. Returns the textinput blink cmd for the initially
// focused field.
func (m EntryEditorModal) Init() tea.Cmd { return textinput.Blink }

// Mode reports add vs edit. Exported for tests.
func (m EntryEditorModal) Mode() EntryEditorMode { return m.mode }

// Focused reports the currently focused field. Exported for tests.
func (m EntryEditorModal) Focused() editorField { return m.focused }

// Enabled reports the enabled-flag value (toggled with Space). Exported for tests.
func (m EntryEditorModal) Enabled() bool { return m.enabled }

// Values returns current form values for tests.
func (m EntryEditorModal) Values() (ip, hostname, aliases, comment string) {
	return m.ipInput.Value(), m.hostnameInput.Value(), m.aliasesInput.Value(), m.commentInput.Value()
}

// Errors returns current inline validation errors for tests.
func (m EntryEditorModal) Errors() (ip, hostname, aliases, comment string) {
	return m.ipErr, m.hostnameErr, m.aliasesErr, m.commentErr
}

// Update implements Modal.
//
// Determinism contract: every tea.KeyMsg is consumed by exactly one branch.
// Non-KeyMsg events are forwarded to the focused textinput so paste/blink
// messages reach it but no key-routing race can occur (mirrors the
// ConfirmModal spike pattern).
func (m EntryEditorModal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		// Forward non-key messages (textinput.blinkMsg, paste) to the
		// focused textinput only. This keeps cursor blinking working
		// without letting other inputs steal focus.
		return m.updateFocusedInput(msg)
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		return m, m.emit(false, nil)
	case tea.KeyTab:
		m.advanceFocus(+1)
		return m, m.refocusCmd()
	case tea.KeyShiftTab:
		m.advanceFocus(-1)
		return m, m.refocusCmd()
	case tea.KeyEnter:
		// Enter on any field attempts submit (matches v1 semantics).
		next, cmd := m.trySubmit()
		return next, cmd
	}

	// Space toggles the enabled checkbox when it has focus.
	if m.focused == fieldEnabled {
		if keyMsg.Type == tea.KeySpace ||
			(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == ' ') {
			m.enabled = !m.enabled
			return m, nil
		}
		// Non-toggle keys on the checkbox are no-ops (don't leak to textinputs).
		return m, nil
	}

	// All other keys flow to the focused textinput.
	return m.updateFocusedInput(msg)
}

// updateFocusedInput dispatches a message to the textinput that owns focus,
// clears any cached error for that field (since contents changed), and stores
// the updated input back.
func (m EntryEditorModal) updateFocusedInput(msg tea.Msg) (Modal, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focused {
	case fieldIP:
		m.ipInput, cmd = m.ipInput.Update(msg)
		m.ipErr = ""
	case fieldHostname:
		m.hostnameInput, cmd = m.hostnameInput.Update(msg)
		m.hostnameErr = ""
	case fieldAliases:
		m.aliasesInput, cmd = m.aliasesInput.Update(msg)
		m.aliasesErr = ""
	case fieldComment:
		m.commentInput, cmd = m.commentInput.Update(msg)
		m.commentErr = ""
	}
	return m, cmd
}

// advanceFocus moves focus by delta (+1 forward, -1 back) with wrap-around.
func (m *EntryEditorModal) advanceFocus(delta int) {
	next := (int(m.focused) + delta + editorFieldCount) % editorFieldCount
	m.focused = editorField(next)
}

// refocusCmd blurs every textinput and focuses the one matching m.focused
// (no-op for the Enabled checkbox and Submit button). Returns the focus cmd
// (typically textinput.Blink) so the cursor renders immediately.
func (m *EntryEditorModal) refocusCmd() tea.Cmd {
	m.ipInput.Blur()
	m.hostnameInput.Blur()
	m.aliasesInput.Blur()
	m.commentInput.Blur()
	switch m.focused {
	case fieldIP:
		return m.ipInput.Focus()
	case fieldHostname:
		return m.hostnameInput.Focus()
	case fieldAliases:
		return m.aliasesInput.Focus()
	case fieldComment:
		return m.commentInput.Focus()
	}
	return nil
}

// trySubmit validates every field. On success, emits a ModalResultMsg with
// Confirmed=true and Data=domain.Entry. On failure, populates inline error
// fields and stays open.
func (m EntryEditorModal) trySubmit() (Modal, tea.Cmd) {
	// Clear previous errors.
	m.ipErr, m.hostnameErr, m.aliasesErr, m.commentErr = "", "", "", ""

	ip := strings.TrimSpace(m.ipInput.Value())
	hostname := strings.TrimSpace(m.hostnameInput.Value())
	aliases := parseAliases(m.aliasesInput.Value())
	comment := m.commentInput.Value()

	if err := domain.ValidateIP(ip); err != nil {
		m.ipErr = err.Error()
	}
	if err := domain.ValidateHostname(hostname); err != nil {
		m.hostnameErr = err.Error()
	}
	if err := domain.ValidateAliases(aliases); err != nil {
		m.aliasesErr = err.Error()
	}
	if err := domain.ValidateComment(comment); err != nil {
		m.commentErr = err.Error()
	}

	if m.ipErr != "" || m.hostnameErr != "" || m.aliasesErr != "" || m.commentErr != "" {
		// Stay open. No Cmd — host keeps the modal up.
		return m, nil
	}

	entry := domain.Entry{
		ID:       m.originalID,
		IP:       ip,
		Hostname: hostname,
		Aliases:  aliases,
		Enabled:  m.enabled,
		Comment:  comment,
	}
	return m, m.emit(true, entry)
}

// parseAliases splits a comma-separated string into trimmed, non-empty
// hostnames. Mirrors v1 EntryEditorModal.parseAliases.
func parseAliases(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// emit returns a tea.Cmd that yields a ModalResultMsg. The id is captured by
// value so the message is stable even if the modal is replaced.
func (m EntryEditorModal) emit(confirmed bool, data any) tea.Cmd {
	id := m.id
	return func() tea.Msg {
		return ModalResultMsg{ID: id, Confirmed: confirmed, Data: data}
	}
}

// View implements Modal.
func (m EntryEditorModal) View() string {
	title := "Add Entry"
	if m.mode == EntryEditorModeEdit {
		title = "Edit Entry"
	}

	rows := []string{
		editorTitleStyle.Render(title),
		"",
		m.renderField("IP Address:", fieldIP, m.ipInput.View(), m.ipErr, ""),
		m.renderField("Hostname:", fieldHostname, m.hostnameInput.View(), m.hostnameErr, ""),
		m.renderField("Aliases:", fieldAliases, m.aliasesInput.View(), m.aliasesErr, "(comma-separated)"),
		m.renderCheckbox("Enabled:", fieldEnabled, m.enabled),
		m.renderField("Comment:", fieldComment, m.commentInput.View(), m.commentErr, ""),
		"",
		m.renderSubmit(),
	}

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return editorFrameStyle.Render(body)
}

func (m EntryEditorModal) renderField(label string, field editorField, inputView, errMsg, hint string) string {
	labelText := label
	if hint != "" {
		labelText = label + " " + editorHintStyle.Render(hint)
	}
	if m.focused == field {
		labelText = editorFocusStyle.Render(labelText)
	}
	lines := []string{labelText, inputView}
	if errMsg != "" {
		lines = append(lines, editorErrorStyle.Render(errMsg))
	}
	lines = append(lines, "")
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m EntryEditorModal) renderCheckbox(label string, field editorField, on bool) string {
	box := "[ ]"
	boxStyle := editorErrorStyle
	if on {
		box = "[✓]"
		boxStyle = editorOnStyle
	}
	labelText := label
	if m.focused == field {
		labelText = editorFocusStyle.Render(label)
	}
	return labelText + " " + boxStyle.Render(box) + "\n"
}

func (m EntryEditorModal) renderSubmit() string {
	save := "[Enter] Save"
	cancel := "[Esc] Cancel"
	if m.focused == fieldSubmit {
		save = editorFocusStyle.Render(save)
	} else {
		save = editorHintStyle.Render(save)
	}
	return save + "  " + editorHintStyle.Render(cancel)
}

// --- Styles -----------------------------------------------------------------

var (
	editorFrameStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("51")). // cyan accent (matches v1)
				Padding(1, 2)

	editorTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))

	editorFocusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))

	editorErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	editorOnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	editorHintStyle = lipgloss.NewStyle().Faint(true)
)
