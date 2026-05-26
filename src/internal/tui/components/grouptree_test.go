package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// forceColor pins the lipgloss color profile to TrueColor for the duration
// of a test. Without this, `go test` runs without a TTY and lipgloss drops
// all ANSI styling, making selection-highlight assertions trivially pass
// (or, here, fail because plain == styled).
func forceColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

// mockGroups mirrors the fixture used by v1 src/tui/components/__tests__/GroupTree.test.tsx
// so failing assertions cite the same data the v1 baseline asserted against.
func mockGroups() []domain.Group {
	return []domain.Group{
		{
			Name: "work",
			Entries: []domain.Entry{
				{ID: "1", IP: "10.0.0.1", Hostname: "api.work", Enabled: true},
			},
			Groups: []domain.Group{
				{
					Name: "prod",
					Entries: []domain.Entry{
						{ID: "2", IP: "10.0.0.2", Hostname: "db.prod", Enabled: true},
					},
				},
				{Name: "staging"},
			},
		},
		{
			Name: "personal",
			Entries: []domain.Entry{
				{ID: "3", IP: "192.168.1.1", Hostname: "home.local", Enabled: true},
				{ID: "4", IP: "192.168.1.2", Hostname: "nas.local", Enabled: false},
			},
		},
	}
}

// TestGroupTree_RendersHierarchically ports v1 "renders groups hierarchically"
// — the top-level group names must appear in the rendered output.
func TestGroupTree_RendersHierarchically(t *testing.T) {
	out := RenderGroupTree(mockGroups(), nil, nil)
	require.Contains(t, out, "work")
	require.Contains(t, out, "personal")
}

// TestGroupTree_DisplaysEntryCount ports v1 "displays entry count per group"
// — every group row carries `(<directEntryCount>)`. `work` has 1 direct entry
// (nested entries do not bubble up); `personal` has 2.
func TestGroupTree_DisplaysEntryCount(t *testing.T) {
	out := RenderGroupTree(mockGroups(), nil, nil)
	require.Contains(t, out, "work (1)")
	require.Contains(t, out, "personal (2)")
}

// TestGroupTree_ArrowIndicatorForCollapsibleGroups ports v1 arrow-indicator
// case. `work` has subgroups → must render the expanded triangle.
func TestGroupTree_ArrowIndicatorForCollapsibleGroups(t *testing.T) {
	out := RenderGroupTree(mockGroups(), nil, nil)
	require.Contains(t, out, glyphExpanded+"work")
	// Leaf group "personal" uses the leaf glyph (two spaces), never an arrow.
	require.NotContains(t, out, glyphExpanded+"personal")
	require.NotContains(t, out, glyphCollapsed+"personal")
}

// TestGroupTree_HidesNestedGroupsWhenCollapsed ports v1
// "hides nested groups when collapsed" — collapsing `work` must hide
// `prod` and `staging` while keeping `work` itself visible with a collapsed
// glyph.
func TestGroupTree_HidesNestedGroupsWhenCollapsed(t *testing.T) {
	collapsed := map[string]struct{}{"work": {}}
	out := RenderGroupTree(mockGroups(), nil, collapsed)
	require.Contains(t, out, "work")
	require.Contains(t, out, glyphCollapsed+"work")
	require.NotContains(t, out, "prod")
	require.NotContains(t, out, "staging")
}

// TestGroupTree_HighlightsSelectedGroup ports v1 "highlights selected group"
// — selecting `["work"]` must wrap the work row in selection ANSI codes
// (bold/reverse) that are absent from the unselected variant.
func TestGroupTree_HighlightsSelectedGroup(t *testing.T) {
	forceColor(t)
	plain := RenderGroupTree(mockGroups(), nil, nil)
	selected := RenderGroupTree(mockGroups(), []string{"work"}, nil)
	require.Contains(t, selected, "work")
	// Selection MUST add ANSI styling. Equality would mean the highlight is
	// a no-op (regression: v1 used bold+inverse+cyan).
	require.NotEqual(t, plain, selected)
	// Sibling rows remain unstyled — only the selected row gets the
	// styled segment. We verify by checking that `personal (2)` still
	// appears verbatim (no ANSI codes spliced in by accident).
	require.Contains(t, selected, "personal (2)")
}

// TestGroupTree_NestedIndentationParity ports v1 "shows nested groups with
// proper indentation" — children render with one indentUnit per depth level.
func TestGroupTree_NestedIndentationParity(t *testing.T) {
	out := RenderGroupTree(mockGroups(), nil, nil)
	require.Contains(t, out, "work")
	// Depth-1 children prefixed with one indentUnit + leaf glyph.
	require.Contains(t, out, indentUnit+glyphLeaf+"prod")
	require.Contains(t, out, indentUnit+glyphLeaf+"staging")
}

// TestGroupTree_EmptyGroupsArray ports v1 "handles empty groups array"
// — renders the dimmed "No groups" placeholder so the Sidebar slot is
// never blank.
func TestGroupTree_EmptyGroupsArray(t *testing.T) {
	out := RenderGroupTree(nil, nil, nil)
	require.Contains(t, out, emptyMessage)
	// Also accepts an explicitly-empty slice (treated the same as nil).
	require.Contains(t, RenderGroupTree([]domain.Group{}, nil, nil), emptyMessage)
}

// TestGroupTree_DeeplyNestedGroups ports v1 "handles deeply nested groups"
// — three levels deep must all render with progressively larger indentation.
func TestGroupTree_DeeplyNestedGroups(t *testing.T) {
	deep := []domain.Group{{
		Name: "level1",
		Groups: []domain.Group{{
			Name: "level2",
			Groups: []domain.Group{{
				Name: "level3",
				Entries: []domain.Entry{
					{ID: "5", IP: "10.0.0.5", Hostname: "deep.host", Enabled: true},
				},
			}},
		}},
	}}
	out := RenderGroupTree(deep, nil, nil)
	require.Contains(t, out, "level1")
	require.Contains(t, out, indentUnit+glyphExpanded+"level2")
	require.Contains(t, out, indentUnit+indentUnit+glyphLeaf+"level3 (1)")
}

// TestGroupTree_SelectionOnNestedPath verifies that selectedPath uses full
// path semantics (not just the leaf name) — `["work", "prod"]` highlights
// the nested `prod` row, not a top-level `prod` (which doesn't exist).
func TestGroupTree_SelectionOnNestedPath(t *testing.T) {
	forceColor(t)
	plain := RenderGroupTree(mockGroups(), nil, nil)
	selectedNested := RenderGroupTree(mockGroups(), []string{"work", "prod"}, nil)
	require.NotEqual(t, plain, selectedNested)

	// Selecting just ["prod"] (which does not match any top-level path)
	// must produce the same output as no selection — partial-leaf-name
	// matches would be a regression of v1's arraysEqual semantics.
	selectedLeafOnly := RenderGroupTree(mockGroups(), []string{"prod"}, nil)
	require.Equal(t, plain, selectedLeafOnly)
}

// TestGroupTree_RenderShapeForLayoutSlot is a structural contract test: the
// returned string must contain one line per visible row (no trailing blank
// line) so Layout's lipgloss.JoinHorizontal pads sidebar+main correctly.
func TestGroupTree_RenderShapeForLayoutSlot(t *testing.T) {
	out := RenderGroupTree(mockGroups(), nil, nil)
	require.NotEmpty(t, out)
	require.False(t, strings.HasSuffix(out, "\n"), "trailing newline breaks Layout column join")

	// Visible rows when nothing is collapsed: work, prod, staging, personal = 4.
	lines := strings.Split(out, "\n")
	require.Equal(t, 4, len(lines), "expected 4 visible rows, got %d: %q", len(lines), out)
}
