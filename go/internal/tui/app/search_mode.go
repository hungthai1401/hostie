// search_mode.go owns the TUI Search mode key handling and the model-side
// glue that connects the centralized store to components.EntryList's
// optional filtered-render branch.
//
// Port of v1 src/tui/hooks/useSearch.ts + the search-mode branch of
// src/tui/hooks/useKeyboard.ts. Behavior contract (bead
// hosts-cli-go-mig-p4-search-mode-n1c):
//
//   - '/' in Mode==Normal → enter Mode==Search. The current selection is
//     remembered so Esc can restore it. The store's search query is cleared
//     so the operator types from an empty prompt (matches v1).
//   - In Mode==Search: printable runes append to the query, Backspace
//     removes the trailing rune, Esc cancels (restore prior selection,
//     clear query, return to Normal), Enter accepts the top weighted
//     result (set SelectedEntryID, clear query/filter, return to Normal).
//   - On every query mutation we rebuild the filtered subset via
//     search.Engine.Query and push it into the EntryList component so
//     View() renders the ranked filtered list rather than the
//     selected-group entries (mirrors v1 where useSearch.results drove
//     the Main pane while in search).
//
// The Search engine itself is rebuilt every keystroke for simplicity —
// Flatten is O(entries) and Query is O(entries) over the flattened slice;
// at the entry counts a hosts file realistically holds (<10k) this is
// imperceptible. A future bead may memoize the engine across keystrokes
// once HostsFile mutations are wired through the store with a version
// counter.

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/search"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// enterSearchMode transitions Model into Mode==Search, snapshotting the
// current entry selection so Esc can restore it. Always returns a Model;
// callers wrap it for tea.Cmd-returning sites.
func (m Model) enterSearchMode() Model {
	m.priorSelection = m.store.SelectedEntryID()
	m.store.SetSearchQuery("")
	m.store.SetMode(store.ModeSearch)
	// Rebuild the engine from the current hosts file. The hosts file is
	// the canonical source — even if the store mutates later, the next
	// keystroke rebuilds the engine again, so we don't need to invalidate.
	hf := m.store.HostsFile()
	m.searchEngine = search.NewEngine(hf.Groups)
	// Empty query → empty filtered list (search.Engine.Query returns nil
	// for whitespace). Push the empty slice as a sentinel so EntryList
	// renders the empty-state placeholder while the operator starts typing.
	m.entryList = m.entryList.WithFilter([]domain.Entry{}, "")
	return m
}

// exitSearchMode releases the search filter and returns the model to
// Mode==Normal. `keepSelection`=true preserves whatever the store
// currently has as the selected entry (used by Enter to commit the top
// result); false restores priorSelection (used by Esc to cancel).
func (m Model) exitSearchMode(keepSelection bool) Model {
	if !keepSelection {
		m.store.SelectEntry(m.priorSelection)
	}
	m.priorSelection = ""
	m.store.SetSearchQuery("")
	m.store.SetMode(store.ModeNormal)
	m.entryList = m.entryList.WithoutFilter()
	m.searchEngine = nil
	return m
}

// applyQuery sets the store query, reruns search.Engine, and pushes the
// filtered subset into EntryList. Empty/whitespace queries render the
// empty-state placeholder via an explicitly empty (non-nil) slice — this
// distinguishes "filter active but no matches" from "no filter, render
// passed-in entries".
func (m Model) applyQuery(q string) Model {
	m.store.SetSearchQuery(q)
	if m.searchEngine == nil {
		hf := m.store.HostsFile()
		m.searchEngine = search.NewEngine(hf.Groups)
	}
	results := m.searchEngine.Query(q)
	filtered := make([]domain.Entry, 0, len(results))
	for _, r := range results {
		filtered = append(filtered, r.Entry)
	}
	m.entryList = m.entryList.WithFilter(filtered, q)
	return m
}

// handleSearchKey is the Mode==Search key router. Mirrors the search-mode
// branch of v1 useKeyboard.ts. Returns (Model, tea.Cmd); the Cmd is always
// nil today but kept in the signature so a future bead can wire a
// debounced rerun without touching every call site.
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.exitSearchMode(false), nil

	case tea.KeyEnter:
		// Commit the top weighted result. If the filter has zero matches
		// (empty query or no hits) we still exit cleanly, restoring the
		// prior selection so the operator isn't dropped onto nothing.
		if m.searchEngine != nil {
			results := m.searchEngine.Query(m.store.SearchQuery())
			if len(results) > 0 {
				m.store.SelectEntry(results[0].Entry.ID)
				return m.exitSearchMode(true), nil
			}
		}
		return m.exitSearchMode(false), nil

	case tea.KeyBackspace:
		q := m.store.SearchQuery()
		if len(q) > 0 {
			// Drop the trailing rune (not byte) so a multi-byte glyph
			// is removed atomically. This matches v1's
			// searchQuery.slice(0, -1) when the input is BMP-only and
			// behaves more correctly than byte-trimming for non-ASCII.
			runes := []rune(q)
			q = string(runes[:len(runes)-1])
		}
		return m.applyQuery(q), nil

	case tea.KeyRunes, tea.KeySpace:
		// Append the typed rune(s) to the query. KeySpace is a separate
		// bubbletea key type but for our purposes it's just another
		// printable character.
		appended := string(msg.Runes)
		if msg.Type == tea.KeySpace && appended == "" {
			appended = " "
		}
		return m.applyQuery(m.store.SearchQuery() + appended), nil
	}

	// Unknown keys are a no-op in search mode (matches v1 — only the
	// branches above produce mutations).
	return m, nil
}
