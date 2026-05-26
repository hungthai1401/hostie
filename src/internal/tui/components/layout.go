// Package components contains render-only Bubble Tea / Lipgloss components
// that compose the hostie TUI. Components in this package do not own
// application state and do not produce tea.Cmd values; they take rendered
// strings (or domain snapshots, in higher-level components) and return a
// styled view. The root app/Update loop is responsible for driving them.
//
// Layout is the 3-pane frame: Sidebar (left, ~30%), Main (right, ~70%),
// and StatusBar (full-width, bottom). Widths are derived from the terminal
// dimensions communicated via tea.WindowSizeMsg and passed through the
// constructor (NewLayout) or the WithSize accessor.
package components

import (
	"github.com/charmbracelet/lipgloss"
)

// Minimum dimensions below which the layout collapses to bare content.
// These mirror v1's behavior where Ink renders content even when the
// terminal is narrower than the nominal 30/70 split.
const (
	minSidebarWidth = 10
	minMainWidth    = 1
	minContentRows  = 1
	// statusBarRows is the rendered height of the StatusBar pane,
	// including its top-border row. Kept constant so callers can
	// reason about main-content height as height - statusBarRows.
	statusBarRows = 2
)

// Layout is a render-only 3-pane frame. It holds the current terminal
// dimensions and exposes a View(sidebar, main, statusBar) method that
// composes the three rendered child views into a single string.
//
// Layout owns no domain state and dispatches no tea.Cmd. The root Model
// updates Layout's dimensions on tea.WindowSizeMsg and calls View during
// its own View() pass.
type Layout struct {
	width  int
	height int
}

// NewLayout constructs a Layout sized for a width x height terminal.
// Both dimensions may be zero before the first tea.WindowSizeMsg has been
// received; View tolerates this and renders an empty frame.
func NewLayout(width, height int) Layout {
	return Layout{width: width, height: height}
}

// WithSize returns a copy of l with the given dimensions applied. This is
// the accessor the root Model uses when handling tea.WindowSizeMsg.
func (l Layout) WithSize(width, height int) Layout {
	l.width = width
	l.height = height
	return l
}

// Width returns the total terminal width Layout was constructed with.
func (l Layout) Width() int { return l.width }

// Height returns the total terminal height Layout was constructed with.
func (l Layout) Height() int { return l.height }

// SidebarWidth returns the nominal Sidebar pane width (~30% of total),
// clamped to a minimum of minSidebarWidth so the GroupTree always has
// room to render group names.
func (l Layout) SidebarWidth() int {
	w := l.width * 30 / 100
	if w < minSidebarWidth {
		w = minSidebarWidth
	}
	if w > l.width-minMainWidth && l.width > minMainWidth {
		w = l.width - minMainWidth
	}
	return w
}

// MainWidth returns the Main pane width: total width minus Sidebar.
func (l Layout) MainWidth() int {
	w := l.width - l.SidebarWidth()
	if w < minMainWidth {
		w = minMainWidth
	}
	return w
}

// ContentHeight returns the height available for Sidebar + Main, i.e.
// total height minus the StatusBar's rendered rows.
func (l Layout) ContentHeight() int {
	h := l.height - statusBarRows
	if h < minContentRows {
		h = minContentRows
	}
	return h
}

// FocusPane identifies which pane the parent considers focused, so Layout
// can render a visually distinct border. The zero value (FocusPaneNone)
// renders the default neutral border on both panes.
type FocusPane int

const (
	FocusPaneNone FocusPane = iota
	FocusPaneSidebar
	FocusPaneMain
)

// View composes the three pre-rendered pane contents into the final
// frame string. Callers pass the already-rendered Sidebar (GroupTree),
// Main (EntryList), and StatusBar views; Layout is responsible only for
// sizing and joining.
func (l Layout) View(sidebar, main, statusBar string) string {
	return l.ViewFocused(sidebar, main, statusBar, FocusPaneNone)
}

// ViewFocused is View with an explicit focus indicator. The focused pane
// gets a bright cyan border so the operator can see at a glance which pane
// receives j/k/Tab. Callers without a focus model can keep using View.
func (l Layout) ViewFocused(sidebar, main, statusBar string, focus FocusPane) string {
	if l.width <= 0 || l.height <= 0 {
		// No dimensions yet (pre-WindowSizeMsg). Fall back to a vertical
		// stack so the operator still sees content during the first frame.
		return lipgloss.JoinVertical(lipgloss.Left, sidebar, main, statusBar)
	}

	contentHeight := l.ContentHeight()
	sidebarWidth := l.SidebarWidth()
	mainWidth := l.MainWidth()

	// Sidebar: fixed width minus its right border column; vertical
	// separator on the right edge matches v1's borderRight Box. When
	// focused, the border color brightens so the operator sees which
	// pane receives keystrokes.
	sidebarBorder := lipgloss.NormalBorder()
	sidebarStyle := lipgloss.NewStyle().
		Width(maxInt(sidebarWidth-1, 1)).
		Height(contentHeight).
		BorderStyle(sidebarBorder).
		BorderRight(true).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false)
	if focus == FocusPaneSidebar {
		sidebarStyle = sidebarStyle.BorderForeground(lipgloss.Color("39")) // cyan
	}

	// Main: remaining width minus a single column of left padding so
	// the GroupTree and EntryList aren't visually glued together. When
	// focused, render a left border in the focus color (the sidebar's
	// right border becomes the visual separator otherwise).
	mainStyle := lipgloss.NewStyle().
		Width(maxInt(mainWidth-1, 1)).
		Height(contentHeight).
		PaddingLeft(1)
	if focus == FocusPaneMain {
		mainStyle = mainStyle.
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderTop(false).
			BorderBottom(false).
			BorderRight(false).
			BorderForeground(lipgloss.Color("39")). // cyan
			PaddingLeft(0)                          // border takes the column the padding occupied
	}

	// StatusBar: full width with a top border separating it from Main.
	statusStyle := lipgloss.NewStyle().
		Width(maxInt(l.width-2, 1)).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderRight(false).
		BorderBottom(false).
		BorderLeft(false).
		PaddingLeft(1).
		PaddingRight(1)

	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Render(sidebar),
		mainStyle.Render(main),
	)
	bottom := statusStyle.Render(statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
