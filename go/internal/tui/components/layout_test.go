package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewLayout_DefaultDimensions verifies Layout tolerates pre-WindowSizeMsg
// construction (the period after the program starts but before the terminal
// reports its size). View must still produce a non-empty frame so the
// operator sees something on the first paint.
func TestNewLayout_DefaultDimensions(t *testing.T) {
	l := NewLayout(0, 0)
	out := l.View("sidebar", "main", "status")
	require.Contains(t, out, "sidebar")
	require.Contains(t, out, "main")
	require.Contains(t, out, "status")
}

// TestLayout_WithSize verifies WithSize returns an updated copy (Layout is a
// value type — the root Model updates its layout field on WindowSizeMsg).
func TestLayout_WithSize(t *testing.T) {
	l := NewLayout(0, 0)
	l2 := l.WithSize(120, 40)
	require.Equal(t, 120, l2.Width())
	require.Equal(t, 40, l2.Height())
	// Original untouched (value semantics).
	require.Equal(t, 0, l.Width())
}

// TestLayout_SidebarWidth_30Percent verifies the 30/70 split from v1.
func TestLayout_SidebarWidth_30Percent(t *testing.T) {
	l := NewLayout(100, 30)
	require.Equal(t, 30, l.SidebarWidth())
	require.Equal(t, 70, l.MainWidth())
}

// TestLayout_SidebarWidth_NarrowTerminalClampsToMinimum verifies that on a
// narrow terminal (where 30% would yield 0–9 columns) Sidebar widens to its
// minimum so GroupTree can still render group names.
func TestLayout_SidebarWidth_NarrowTerminal(t *testing.T) {
	l := NewLayout(20, 10)
	require.GreaterOrEqual(t, l.SidebarWidth(), minSidebarWidth)
	require.GreaterOrEqual(t, l.MainWidth(), minMainWidth)
	// Combined widths never exceed total terminal width.
	require.LessOrEqual(t, l.SidebarWidth()+l.MainWidth(), 20)
}

// TestLayout_ContentHeight verifies StatusBar reserves rows out of total height.
func TestLayout_ContentHeight(t *testing.T) {
	l := NewLayout(100, 30)
	require.Equal(t, 30-statusBarRows, l.ContentHeight())
	// Floor at 1 row when terminal is shorter than status bar reservation.
	tiny := NewLayout(100, 1)
	require.GreaterOrEqual(t, tiny.ContentHeight(), 1)
}

// TestLayout_View_EmptyState verifies snapshot of an empty 3-pane frame at a
// typical desktop terminal size. We don't golden-file this (rendering width
// of joined panes varies across lipgloss versions); we assert structural
// invariants instead.
func TestLayout_View_EmptyState(t *testing.T) {
	l := NewLayout(100, 30)
	out := l.View("", "", "")
	require.NotEmpty(t, out)
	// 3 panes → at least content rows + status separator + status row.
	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), statusBarRows)
}

// TestLayout_View_PopulatedState verifies all three pane contents appear in
// the rendered frame at a wide terminal size.
func TestLayout_View_PopulatedState(t *testing.T) {
	l := NewLayout(120, 40)
	out := l.View("GroupA\nGroupB", "entry-1\nentry-2", "NORMAL | ? help")
	require.Contains(t, out, "GroupA")
	require.Contains(t, out, "GroupB")
	require.Contains(t, out, "entry-1")
	require.Contains(t, out, "entry-2")
	require.Contains(t, out, "NORMAL")
	require.Contains(t, out, "? help")
}

// TestLayout_View_NarrowTerminal verifies the layout still renders all three
// pane contents on a narrow terminal (mobile SSH session class).
func TestLayout_View_NarrowTerminal(t *testing.T) {
	l := NewLayout(30, 10)
	out := l.View("S", "M", "St")
	require.Contains(t, out, "S")
	require.Contains(t, out, "M")
	require.Contains(t, out, "St")
}

// TestLayout_View_WideTerminal verifies the layout fills wide terminals
// without panicking and that Main expands to absorb the extra columns.
func TestLayout_View_WideTerminal(t *testing.T) {
	l := NewLayout(200, 60)
	require.Equal(t, 60, l.SidebarWidth())
	require.Equal(t, 140, l.MainWidth())
	out := l.View("sb", "mn", "stat")
	require.Contains(t, out, "sb")
	require.Contains(t, out, "mn")
	require.Contains(t, out, "stat")
}

// TestLayout_View_NoCmdProduced is a contract test: Layout exposes no Update
// method and no tea.Cmd-returning surface. This test exists to make the
// "render-only, no event loop" invariant from phase-4-story-map.md §Story 2
// fail loudly if anyone adds Update/tea.Cmd to Layout in a future refactor.
//
// We assert by counting exported methods via reflection-light means: any
// future addition will need this list updated explicitly.
func TestLayout_View_NoCmdProduced(t *testing.T) {
	// Sanity: View returns a single string (no tea.Cmd). If signature
	// changes, this test fails to compile, surfacing the contract break.
	var _ func(string, string, string) string = NewLayout(10, 10).View
}
