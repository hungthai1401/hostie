package app

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/domain"
	"github.com/hungthai1401/hostie/src/internal/tui/components"
)

// View implements tea.Model. It composes the three rendered panes into
// a single Layout frame. No business logic lives here — every mutation
// has already been applied via Update before View is invoked.
//
// The composition order matches v1 src/tui/App.tsx:
//
//	Layout
//	  Sidebar  ← GroupTree
//	  Main     ← EntryList (entries of currently-selected group)
//	  StatusBar
//
// When the program is quitting (q pressed), we render a one-line farewell
// so the terminal isn't left with the altscreen contents flashing past.
func (m Model) View() string {
	if m.quitting {
		return "bye\n"
	}

	hf := m.store.HostsFile()

	// Sidebar: full GroupTree rendered from the store snapshot. Collapse
	// state is not tracked yet (no expand/collapse keybinds in the
	// navigation-only skeleton); pass nil so every group renders expanded.
	sidebar := components.RenderGroupTree(
		hf.Groups,
		m.store.SelectedGroupPath(),
		nil,
	)

	// Main: EntryList shows the entries of the currently selected group.
	// When no group is selected (boot state on a populated file), fall
	// back to the entries of the first top-level group so the operator
	// sees content immediately — matches v1's default-selection behavior.
	main := m.entryList.View(
		visibleEntries(hf.Groups, m.store.SelectedGroupPath()),
		m.store.SelectedEntryID(),
	)

	// StatusBar: store-driven (dirty marker, mode, query, status message).
	status := m.statusBar.View(m.store)

	focusPane := components.FocusPaneMain
	if m.focus == FocusSidebar {
		focusPane = components.FocusPaneSidebar
	}
	base := m.layout.ViewFocused(sidebar, main, status, focusPane)
	if m.modalHost != nil && m.modalHost.Active() {
		return OverlayModal(base, m.modalHost.View(), m.width, m.height)
	}
	return base
}

// visibleEntries returns the entries to render in the Main pane given the
// current selection state.
//
//   - selectedPath non-empty → entries of the addressed group (or empty
//     slice if the path doesn't resolve, which can happen mid-edit).
//   - selectedPath empty → entries of the first top-level group if any
//     groups exist; nil otherwise (EntryList renders the empty-state
//     placeholder).
//
// Subgroup entries are not flattened into the parent's listing — that
// mirrors v1, where the Main pane shows only the directly-selected
// group's entries.
func visibleEntries(groups []domain.Group, selectedPath []string) []domain.Entry {
	if len(selectedPath) > 0 {
		if g := findGroup(groups, selectedPath); g != nil {
			return g.Entries
		}
		return nil
	}
	if len(groups) > 0 {
		return groups[0].Entries
	}
	return nil
}

// findGroup walks the recursive Group tree to the node at path. Returns
// nil if any path segment is missing. Empty path returns nil (matches the
// v1 findGroupByPath in store.ts).
func findGroup(groups []domain.Group, path []string) *domain.Group {
	if len(path) == 0 {
		return nil
	}
	for i := range groups {
		if groups[i].Name != path[0] {
			continue
		}
		if len(path) == 1 {
			return &groups[i]
		}
		return findGroup(groups[i].Groups, path[1:])
	}
	return nil
}

// OverlayModal composes a modal body string over a base view by centering it
// within the given width/height. This is the rendering primitive every Phase 4
// modal bead calls from View() when ModalHost.Active() is true; see
// .spikes/go-migration/p4-modal-pattern/FINDINGS.md for the full integration
// sequence (model.go modalHost field, update.go intercept, view.go overlay).
//
// The base parameter is preserved in the returned string for terminals that
// don't support overlay positioning (Place falls back gracefully); for normal
// terminals, the centered modal visually replaces the base contents while the
// underlying buffer stays intact for the next render.
//
// An empty modal string returns base unchanged — callers can pass
// ModalHost.View() directly without guarding on Active().
func OverlayModal(base, modal string, width, height int) string {
	if modal == "" {
		return base
	}
	if width <= 0 || height <= 0 {
		return modal
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}

