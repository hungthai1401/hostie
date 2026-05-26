// GroupTree is a render-only Bubble Tea / Lipgloss component that displays
// a hierarchical view of domain.Group values for the Sidebar pane.
//
// It is a port of v1 src/tui/components/GroupTree.tsx. Like every component
// in this package (see layout.go), GroupTree owns no application state and
// returns no tea.Cmd values. The root Model is responsible for:
//
//   - pulling the current groups from the *store.Store
//     (`store.HostsFile().Groups`)
//   - tracking the user's selected group path
//     (`store.SelectedGroupPath()` — D8)
//   - tracking which paths are collapsed (UI-local state, not persisted in
//     the store — matches v1 where `collapsedPaths` was a `useState` hook
//     local to the App, not a zustand slice)
//
// and then calling RenderGroupTree(...) to produce the styled string that Layout's
// `sidebar` slot consumes.
//
// Selection styling matches v1: the selected row is rendered with bold +
// inverse (background swap) + cyan foreground. Collapse glyphs are the same
// triangular markers (▼ expanded, ▸ collapsed); leaf groups use two spaces
// so column alignment is preserved across the tree.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// Collapse glyphs. Two trailing spaces on the leaf marker keep names
// vertically aligned with their expandable siblings (v1 parity).
const (
	glyphExpanded  = "▼ "
	glyphCollapsed = "▸ "
	glyphLeaf      = "  "
	indentUnit     = "  "
)

// emptyMessage mirrors v1's `<Text dimColor>No groups</Text>` placeholder
// shown when the hosts file contains no top-level groups.
const emptyMessage = "No groups"

// selectedStyle highlights the currently selected group row. Matches the
// v1 Ink `<Text color="cyan" bold inverse>` styling used in GroupTree.tsx.
var selectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("14")). // cyan
	Bold(true).
	Reverse(true)

// emptyStyle dims the "No groups" placeholder text, matching v1's
// `dimColor` Ink attribute.
var emptyStyle = lipgloss.NewStyle().Faint(true)

// RenderGroupTree produces the styled GroupTree view as a single multi-line string.
//
// Arguments:
//
//   - groups: top-level groups to render (typically store.HostsFile().Groups)
//   - selectedPath: currently selected group path (store.SelectedGroupPath()).
//     An empty slice means "no group selected" — no row is highlighted.
//   - collapsedPaths: set of slash-joined paths that should be collapsed.
//     Membership is exact: collapsing `"work"` does not auto-collapse
//     `"work/prod"`; the caller decides ancestor semantics. nil is treated
//     as an empty set.
//
// Returns the rendered tree. When groups is empty, returns the styled
// "No groups" placeholder so Layout's Sidebar slot is never blank.
func RenderGroupTree(groups []domain.Group, selectedPath []string, collapsedPaths map[string]struct{}) string {
	if len(groups) == 0 {
		return emptyStyle.Render(emptyMessage)
	}

	if collapsedPaths == nil {
		collapsedPaths = map[string]struct{}{}
	}

	var b strings.Builder
	for i := range groups {
		renderGroup(&b, groups[i], nil, 0, selectedPath, collapsedPaths)
	}

	// Trim the trailing newline produced by the final row so callers
	// (Layout) can concatenate without introducing a blank gap.
	return strings.TrimRight(b.String(), "\n")
}

// renderGroup writes a single group row (and recursively its expanded
// children) to b. depth controls indentation, path is the slash-joinable
// path to the *parent* of this group.
func renderGroup(
	b *strings.Builder,
	group domain.Group,
	path []string,
	depth int,
	selectedPath []string,
	collapsedPaths map[string]struct{},
) {
	currentPath := append(append([]string{}, path...), group.Name)
	pathKey := strings.Join(currentPath, "/")

	hasSubgroups := len(group.Groups) > 0
	_, isCollapsed := collapsedPaths[pathKey]
	isSelected := pathsEqual(currentPath, selectedPath)

	indent := strings.Repeat(indentUnit, depth)

	var glyph string
	switch {
	case !hasSubgroups:
		glyph = glyphLeaf
	case isCollapsed:
		glyph = glyphCollapsed
	default:
		glyph = glyphExpanded
	}

	// v1 format: "{indent}{glyph}{name} ({entryCount})"
	row := indent + glyph + group.Name + " (" + itoa(len(group.Entries)) + ")"

	if isSelected {
		b.WriteString(selectedStyle.Render(row))
	} else {
		b.WriteString(row)
	}
	b.WriteByte('\n')

	if !hasSubgroups || isCollapsed {
		return
	}
	for i := range group.Groups {
		renderGroup(b, group.Groups[i], currentPath, depth+1, selectedPath, collapsedPaths)
	}
}

// pathsEqual reports whether two path slices are element-wise equal. An
// empty selectedPath ([]string{} or nil) never matches a non-empty group
// path, matching v1's arraysEqual helper.
func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// itoa converts a non-negative int to its decimal string. Hand-rolled to
// keep this file dependency-free from strconv (the rest of the package
// avoids strconv too — see layout.go). Negative inputs are not expected
// (len() is non-negative); for safety, negatives fall back to "0".
func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
