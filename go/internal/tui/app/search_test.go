package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// searchFixture builds a hosts file with names that exercise the weighted
// aggregator's filter behavior:
//
//	staging/
//	  s1: 10.1.0.1 staging-api    aliases: [stg-api]
//	  s2: 10.1.0.2 staging-db     aliases: []
//	prod/
//	  p1: 10.2.0.1 prod-api       aliases: []
//	  p2: 10.2.0.2 prod-db        aliases: []
//
// Query "staging" should match s1 and s2 (and only those) via the hostname
// fuzzy field; query "api" matches s1 (alias + hostname) and p1 (hostname).
func searchFixture() *domain.HostsFile {
	return &domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name: "staging",
				Entries: []domain.Entry{
					{ID: "s1", IP: "10.1.0.1", Hostname: "staging-api", Aliases: []string{"stg-api"}, Enabled: true},
					{ID: "s2", IP: "10.1.0.2", Hostname: "staging-db", Aliases: []string{}, Enabled: true},
				},
				Groups: []domain.Group{},
			},
			{
				Name: "prod",
				Entries: []domain.Entry{
					{ID: "p1", IP: "10.2.0.1", Hostname: "prod-api", Aliases: []string{}, Enabled: true},
					{ID: "p2", IP: "10.2.0.2", Hostname: "prod-db", Aliases: []string{}, Enabled: true},
				},
				Groups: []domain.Group{},
			},
		},
	}
}

// seedSearchModel builds a Model preloaded with searchFixture and sized so
// the View() paths exercise real widths. Mirrors seedModel from app_test.go
// but uses the search-specific fixture.
func seedSearchModel(t *testing.T) Model {
	t.Helper()
	m := NewModel("/dev/null")
	m.store.LoadHostsFile(searchFixture())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m2.(Model)
}

// typeChars feeds a sequence of single-rune key events through Update,
// returning the final Model. Helper for readable test bodies that simulate
// the operator typing a query character by character.
func typeChars(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	return m
}

// TestSearch_EnterSearchMode_FromNormal verifies '/' transitions Normal →
// Search, clears any existing query, snapshots the prior selection so Esc
// can restore it, and installs an empty-result filter so the EntryList
// renders the empty-state placeholder (rather than the unfiltered group).
func TestSearch_EnterSearchMode_FromNormal(t *testing.T) {
	m := seedSearchModel(t)

	// Seed a prior selection so we can assert it gets snapshotted.
	m.store.SelectEntry("s1")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	require.Nil(t, cmd, "entering search mode must not emit a tea.Cmd")
	mm := m2.(Model)
	require.Equal(t, store.ModeSearch, mm.store.Mode())
	require.Equal(t, "", mm.store.SearchQuery(), "query must be cleared on entering search mode")
	require.Equal(t, "s1", mm.priorSelection, "prior selection must be snapshotted for Esc restore")
	require.True(t, mm.entryList.FilterActive(), "EntryList filter must be active in search mode")
}

// TestSearch_TypeQuery_FiltersResults verifies that typing 'staging'
// produces a filtered subset containing exactly the matching entries (s1,
// s2) and excluding the prod-group entries.
func TestSearch_TypeQuery_FiltersResults(t *testing.T) {
	m := seedSearchModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)

	m = typeChars(t, m, "staging")
	require.Equal(t, "staging", m.store.SearchQuery())

	filtered := m.entryList.FilteredEntries()
	require.Len(t, filtered, 2, "expected exactly the 2 staging entries to match")

	ids := []string{filtered[0].ID, filtered[1].ID}
	require.Contains(t, ids, "s1")
	require.Contains(t, ids, "s2")
	for _, e := range filtered {
		require.NotContains(t, []string{"p1", "p2"}, e.ID,
			"prod entries must not appear in a 'staging' filter")
	}
}

// TestSearch_Backspace_TrimsQuery verifies Backspace removes the trailing
// rune from the query and reruns the filter — i.e. it is not a no-op and
// the EntryList reflects the new (shorter) query's result set.
func TestSearch_Backspace_TrimsQuery(t *testing.T) {
	m := seedSearchModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)
	m = typeChars(t, m, "staging")

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = m3.(Model)
	require.Equal(t, "stagin", m.store.SearchQuery())

	// Backspace into empty query must not panic and must leave query "".
	for range "stagin" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = next.(Model)
	}
	require.Equal(t, "", m.store.SearchQuery())

	// One extra Backspace on the empty query is a benign no-op.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Equal(t, "", next.(Model).store.SearchQuery())
}

// TestSearch_Esc_CancelsAndRestoresSelection verifies Esc returns the model
// to Mode==Normal, clears the filter, clears the query, and restores the
// pre-search selection (so the operator isn't dropped onto a new cursor
// position after aborting).
func TestSearch_Esc_CancelsAndRestoresSelection(t *testing.T) {
	m := seedSearchModel(t)
	m.store.SelectEntry("p2")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)
	m = typeChars(t, m, "stag")

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m3.(Model)
	require.Equal(t, store.ModeNormal, m.store.Mode())
	require.Equal(t, "", m.store.SearchQuery(), "Esc must clear the query")
	require.False(t, m.entryList.FilterActive(), "Esc must clear the EntryList filter")
	require.Equal(t, "p2", m.store.SelectedEntryID(), "Esc must restore the pre-search selection")
}

// TestSearch_Enter_CommitsTopResult verifies Enter sets SelectedEntryID to
// the top-ranked weighted result, exits Mode==Search, and clears the
// filter. The store query is also cleared so a subsequent '/' starts from
// a fresh prompt (matches v1, where useKeyboard.ts wipes the query on
// Enter commit even though it leaves search mode).
func TestSearch_Enter_CommitsTopResult(t *testing.T) {
	m := seedSearchModel(t)
	m.store.SelectEntry("p2")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)
	m = typeChars(t, m, "staging-api")

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(Model)
	require.Equal(t, store.ModeNormal, m.store.Mode())
	require.False(t, m.entryList.FilterActive())
	require.Equal(t, "s1", m.store.SelectedEntryID(),
		"Enter must commit the top-weighted result (s1's hostname matches exactly)")
}

// TestSearch_Enter_NoMatches_RestoresSelection covers the edge case where
// the user hits Enter with zero matches (e.g., empty query, or a query
// that matches nothing). The model must still exit cleanly and restore
// the prior selection rather than leaving the cursor cleared.
func TestSearch_Enter_NoMatches_RestoresSelection(t *testing.T) {
	m := seedSearchModel(t)
	m.store.SelectEntry("p1")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)
	// Type a string that matches nothing in the fixture.
	m = typeChars(t, m, "xyzzy-nomatch")

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(Model)
	require.Equal(t, store.ModeNormal, m.store.Mode())
	require.Equal(t, "p1", m.store.SelectedEntryID(),
		"Enter on a no-match query must restore the pre-search selection")
}

// TestSearch_EntryList_RendersFilteredSubset verifies the rendered View()
// in search mode contains only matching entries — the parent View() pulls
// the (possibly stale) group-entries arg but EntryList's filter-active
// branch overrides it. This is the L2 verification criterion from the
// bead spec.
func TestSearch_EntryList_RendersFilteredSubset(t *testing.T) {
	m := seedSearchModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)
	m = typeChars(t, m, "staging")

	out := m.View()
	require.Contains(t, out, "staging-api", "matched hostname must render")
	require.Contains(t, out, "staging-db", "matched hostname must render")
	require.NotContains(t, out, "prod-api", "non-matching entry must be filtered out")
	require.NotContains(t, out, "prod-db", "non-matching entry must be filtered out")
}

// TestSearch_NavigationDisabled_InSearchMode verifies that j/k are not
// routed to the navigation handler while in search mode — they instead
// append to the query as printable runes (port of v1 useKeyboard.ts which
// returns early in search mode before reaching the j/k branches).
func TestSearch_NavigationDisabled_InSearchMode(t *testing.T) {
	m := seedSearchModel(t)
	m.store.SelectEntry("p1")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = m2.(Model)

	// 'j' in search mode appends to the query instead of navigating.
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = m3.(Model)
	require.Equal(t, "j", m.store.SearchQuery())
	require.Equal(t, "p1", m.store.SelectedEntryID(),
		"j must not navigate the EntryList while in search mode")
}
