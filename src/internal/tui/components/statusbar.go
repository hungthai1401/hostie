// StatusBar is the bottom-row render-only component for the hostie TUI.
//
// It mirrors v1 src/tui/components/StatusBar.tsx (visual position + role)
// extended per the Phase 4 bead description to surface four zones derived
// from the centralized store:
//
//  1. Dirty marker  — "*" when the store has unsaved changes
//  2. Mode          — current StoreMode in uppercase (NORMAL/SEARCH/EDIT/MODAL)
//  3. Search query  — leading-"/" prefix when in SEARCH mode or query non-empty
//  4. Status message — transient banner colored by StatusLevel (info/success/error)
//
// TTL semantics: the status message is transient. The component itself is
// render-only and produces no tea.Cmd values (matching layout.go's invariant
// that this package does not own state or drive the event loop). Expiration
// is the responsibility of the root app/Update layer, which schedules a
// tea.Tick after SetStatusMessage and calls store.ClearStatusMessage when
// it fires. The StatusBar simply re-renders whatever the store currently
// reports — so once the message is cleared, the zone disappears. The
// statusbar_test.go suite exercises this contract end-to-end by toggling
// the store between "message set" and "message cleared" states.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/tui/store"
)

// dirtyMarker is the single-glyph indicator shown when store.Dirty() is true.
// Matches the conventional editor convention (vim/emacs use "*" / "**").
const dirtyMarker = "*"

// StatusBar is a render-only component that reads from a store snapshot and
// returns a single-line styled view. It holds no state of its own.
type StatusBar struct{}

// NewStatusBar constructs a StatusBar. Kept as a constructor (vs. a package
// function) to leave room for future style/theme injection without breaking
// the call site in components/layout.go.
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// View renders the four-zone status bar from the current store state.
//
// Layout (left → right):
//
//	[*] [MODE] [/query]                                   [status message]
//
// The dirty marker is omitted when Dirty()=false. The query zone is omitted
// when there is no active search query. The status zone is omitted when no
// message is set (post-TTL state).
//
// View takes the store by pointer so each render observes the latest
// snapshot under the store's read lock. Callers must not mutate the store
// from inside View (the store enforces this via its own mutex, but the
// convention is: render-only).
func (b StatusBar) View(s *store.Store) string {
	if s == nil {
		return ""
	}

	mode := s.Mode()
	dirty := s.Dirty()
	query := s.SearchQuery()
	msg := s.StatusMessage()

	// --- Left cluster: dirty + mode + query ---------------------------------
	var leftParts []string

	if dirty {
		leftParts = append(leftParts, dirtyStyle.Render(dirtyMarker))
	}

	leftParts = append(leftParts, modeStyle.Render(" "+strings.ToUpper(mode.String())+" "))

	// Search query is shown when in SEARCH mode (so the user sees the empty
	// prompt as they start typing) or whenever a non-empty query persists
	// (e.g., filter applied while focus has moved elsewhere).
	if mode == store.ModeSearch || query != "" {
		leftParts = append(leftParts, queryStyle.Render(fmt.Sprintf("/%s", query)))
	}

	left := strings.Join(leftParts, " ")

	// --- Right cluster: transient status message ----------------------------
	right := ""
	if msg != nil {
		right = styleForLevel(msg.Level).Render(msg.Text)
	}

	if right == "" {
		return left
	}

	// Two-zone layout: left-justified cluster, right-justified status. We use
	// a single space separator rather than lipgloss.JoinHorizontal so the
	// caller (Layout) controls the actual pane width and padding. Width-aware
	// right-alignment is a no-op without a known width; the parent Layout
	// applies a Width()/PaddingRight wrap around the returned string.
	return left + "  " + right
}

// --- Styles -----------------------------------------------------------------

var (
	// dirtyStyle highlights the unsaved-changes marker. Bold + accent color
	// matches v1's intent (a glance-visible flag).
	dirtyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")) // orange

	// modeStyle renders the current mode with inverse-video framing, matching
	// v1 StatusBar.tsx which used Ink's `inverse` attribute on the mode pill.
	modeStyle = lipgloss.NewStyle().Reverse(true)

	// queryStyle dims the search query so it reads as secondary metadata to
	// the mode pill (parity with v1's `dimColor` on the help hint).
	queryStyle = lipgloss.NewStyle().Faint(true)

	// Status-message colors per StatusLevel (D-derived; matches typical
	// terminal status conventions).
	statusInfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // cyan-blue
	statusSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
)

func styleForLevel(l store.StatusLevel) lipgloss.Style {
	switch l {
	case store.StatusSuccess:
		return statusSuccessStyle
	case store.StatusError:
		return statusErrorStyle
	case store.StatusInfo:
		fallthrough
	default:
		return statusInfoStyle
	}
}
