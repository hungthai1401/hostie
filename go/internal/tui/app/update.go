package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/components"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// Update implements tea.Model. It dispatches:
//
//   - hostsLoadedMsg → seed the store via store.LoadHostsFile; surface a
//     red status banner on load error (skeleton-only; later beads will
//     replace this with a richer error path).
//   - tea.WindowSizeMsg → propagate dimensions into Layout / EntryList.
//   - tea.KeyMsg → navigation only: j/k (wrap), Tab focus swap, q quit.
//
// Out-of-scope per bead hosts-cli-go-mig-p4-app-skeleton-kgg (deferred to
// later beads): Space toggle, d delete, a/e/g/m modal openings, Enter
// apply, / search, ? help, dirty-aware q confirm. The intentional
// no-op-on-unknown-key behavior here is exercised in the test suite.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Modal interception: per the spike (FINDINGS.md §5), when a modal is
	// active it consumes EVERY message — keys, results, and pass-through
	// non-key events — before the root key router runs. This is the
	// deterministic ordering that forecloses the v1 Esc-routing flake.
	if m.modalHost != nil && m.modalHost.Active() {
		next, cmd := m.modalHost.Update(msg)
		m.modalHost = next
		// Special-case ModalResultMsg so per-id handlers fire in the same
		// tick that the modal closes.
		if result, ok := msg.(components.ModalResultMsg); ok {
			m2, dispatchCmd := m.dispatchModalResult(result)
			return m2, tea.Batch(cmd, dispatchCmd)
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case hostsLoadedMsg:
		return m.handleHostsLoaded(msg), nil

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg), nil

	case components.ModalResultMsg:
		// Modal closed itself between the open Cmd firing and the result
		// landing. Route to the dispatcher anyway so the mutation runs.
		return m.dispatchModalResult(msg)

	case ApplyTriggerMsg:
		// Auto-apply on mutation (D11). The store mutation has already happened
		// upstream; applyCmdDispatch chooses between the direct apply path
		// (apply.Runner.Apply on a goroutine) and the sudo handoff path
		// (tea.ExecProcess → __apply-privileged) based on whether the current
		// process can write /etc/hosts directly. The result lands back here as
		// an ApplyResultMsg (direct path) or sudoPendingMsg → SudoFinishedMsg
		// → ApplyResultMsg (sudo path).
		hf := m.store.HostsFile()
		if hf == nil {
			return m, nil
		}
		return m, applyCmdDispatch(m.applyRunner, *hf, m.hostsPath)

	case sudoPendingMsg:
		return m.handleSudoPending(msg)

	case apply.SudoFinishedMsg:
		return m.handleSudoFinished(msg)

	case ApplyResultMsg:
		return m.handleApplyResult(msg), nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleHostsLoaded seeds the store on successful load. On error, the
// store stays at its empty default and a red status banner reports the
// failure so the operator sees something on the first frame instead of
// an empty TUI with no explanation.
func (m Model) handleHostsLoaded(msg hostsLoadedMsg) Model {
	if msg.err != nil {
		m.store.SetStatusMessage("Failed to load hosts file: "+msg.err.Error(), store.StatusError)
		return m
	}
	// LoadHostsFile takes a pointer; copy the value into a heap-allocated
	// HostsFile so the store's internal pointer survives msg leaving scope.
	hf := msg.file
	m.store.LoadHostsFile(&hf)
	return m
}

// handleWindowSize propagates terminal dimensions into the layout and the
// EntryList (EntryList renders with an explicit pane width so its columns
// align to MainWidth). Layout's WithSize is a value-returning copy, so we
// reassign the field rather than mutating in place.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.layout = m.layout.WithSize(msg.Width, msg.Height)
	m.entryList = m.entryList.WithWidth(m.layout.MainWidth())
	return m
}

// handleKey is the navigation-only key router. Every branch returns the
// updated Model; unknown keys fall through to a (m, nil) no-op.
//
// Mode-aware dispatch (added by bead hosts-cli-go-mig-p4-search-mode-n1c):
// when the store reports Mode==Search, all key events are routed to
// handleSearchKey (see search_mode.go) which owns the search input
// capture loop. The navigation keys below only fire in Mode==Normal.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.store.Mode() == store.ModeSearch {
		m2, cmd := m.handleSearchKey(msg)
		return m2, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		// Dirty-aware quit (port of v1 useKeyboard.ts q branch). See
		// mutations.go handleQuitKey for the D11/D13 rationale.
		return m.handleQuitKey()

	case "tab":
		return m.handleTab(), nil

	case "j", "down":
		return m.handleNavigate(+1), nil

	case "k", "up":
		return m.handleNavigate(-1), nil

	case "/":
		// Enter search mode (port of v1 useKeyboard.ts '/' branch).
		return m.enterSearchMode(), nil

	case " ", "space":
		// Space → toggle selected entry enabled (v1 'space' branch).
		return m.handleSpaceToggle()

	case "d":
		// Delete-with-confirm (v1 'd' branch + ConfirmModal).
		return m.handleDeleteKey()

	case "a":
		// Add entry via EntryEditorModal (v1 'a' branch).
		return m.handleAddKey()

	case "e":
		// Edit selected entry via EntryEditorModal (v1 'e' branch).
		return m.handleEditKey()

	case "g":
		// Create group via GroupCreatorModal (v1 'g' branch).
		return m.handleGroupKey()

	case "m":
		// Move entry via MoveToGroupModal (v1 'm' branch).
		return m.handleMoveKey()

	case "enter", "ctrl+s":
		// Explicit re-apply. Ctrl+S is the post-D11/D13 retry hook (re-run
		// the apply pipeline against the current store snapshot after an
		// auto-apply failure without performing another mutation). Enter
		// is the v1 keybind for the same action — both share handleReapplyKey.
		return m.handleReapplyKey()

	case "?":
		// Open the HelpModal (v1 '?' keybind). HelpModal is close-only —
		// see mutations.go handleHelpKey and modal_routing.go modalIDHelp.
		return m.handleHelpKey()
	}
	return m, nil
}

// handleTab swaps focus between sidebar and main. When swapping into a
// pane that has no current selection, we seed the selection to the first
// item in that pane (matches v1 useKeyboard.ts Tab branch).
func (m Model) handleTab() Model {
	hf := m.store.HostsFile()
	if m.focus == FocusSidebar {
		m.focus = FocusMain
		entries := flattenEntries(hf.Groups)
		if len(entries) > 0 && m.store.SelectedEntryID() == "" {
			m.store.SelectEntry(entries[0].ID)
		}
		return m
	}
	m.focus = FocusSidebar
	paths := flattenGroupPaths(hf.Groups, nil)
	if len(paths) > 0 && len(m.store.SelectedGroupPath()) == 0 {
		m.store.SelectGroup(paths[0])
	}
	return m
}

// handleNavigate moves selection by step (+1=down/j, -1=up/k) within the
// currently focused pane, wrapping at both ends. Empty target lists are
// a no-op (matches v1).
func (m Model) handleNavigate(step int) Model {
	hf := m.store.HostsFile()
	if m.focus == FocusSidebar {
		paths := flattenGroupPaths(hf.Groups, nil)
		if len(paths) == 0 {
			return m
		}
		cur := indexOfPath(paths, m.store.SelectedGroupPath())
		next := wrap(cur, step, len(paths))
		m.store.SelectGroup(paths[next])
		return m
	}
	entries := flattenEntries(hf.Groups)
	if len(entries) == 0 {
		return m
	}
	cur := indexOfEntry(entries, m.store.SelectedEntryID())
	next := wrap(cur, step, len(entries))
	m.store.SelectEntry(entries[next].ID)
	return m
}

// -----------------------------------------------------------------------------
// Navigation helpers — ported from v1 src/tui/hooks/useKeyboard.ts
// -----------------------------------------------------------------------------

// flattenEntries returns every entry across every group in pre-order
// traversal. Matches v1 flattenEntries — used by Tab seeding and j/k
// EntryList navigation.
func flattenEntries(groups []domain.Group) []domain.Entry {
	var out []domain.Entry
	var walk func(g domain.Group)
	walk = func(g domain.Group) {
		out = append(out, g.Entries...)
		for _, child := range g.Groups {
			walk(child)
		}
	}
	for _, g := range groups {
		walk(g)
	}
	return out
}

// flattenGroupPaths returns every group's path slice in pre-order
// traversal. Matches v1 flattenGroupPaths. parentPath is nil at the
// top-level call.
func flattenGroupPaths(groups []domain.Group, parentPath []string) [][]string {
	var out [][]string
	for _, g := range groups {
		cur := make([]string, 0, len(parentPath)+1)
		cur = append(cur, parentPath...)
		cur = append(cur, g.Name)
		out = append(out, cur)
		out = append(out, flattenGroupPaths(g.Groups, cur)...)
	}
	return out
}

// indexOfPath returns the index of needle in haystack, or -1 if missing.
// Empty needle matches no element (mirrors v1 JSON.stringify behavior where
// "[]" != any non-empty path's string).
func indexOfPath(haystack [][]string, needle []string) int {
	if len(needle) == 0 {
		return -1
	}
	for i, p := range haystack {
		if pathsEqual(p, needle) {
			return i
		}
	}
	return -1
}

// indexOfEntry returns the index of the entry with the given ID, or -1.
func indexOfEntry(entries []domain.Entry, id string) int {
	if id == "" {
		return -1
	}
	for i, e := range entries {
		if e.ID == id {
			return i
		}
	}
	return -1
}

// pathsEqual is element-wise equality on string slices.
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

// wrap returns the next index after applying step, wrapping at the bounds
// of [0, n). When cur is -1 (no current selection), step=+1 selects index
// 0 and step=-1 selects the last element — matches v1's behavior where
// an unset selection seeds to first/last on j/k.
func wrap(cur, step, n int) int {
	if n <= 0 {
		return 0
	}
	if cur < 0 {
		if step > 0 {
			return 0
		}
		return n - 1
	}
	next := (cur + step) % n
	if next < 0 {
		next += n
	}
	return next
}
