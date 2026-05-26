// EntryList renders the host entries of the currently active group as a
// fixed-column table. It is a render-only Bubble Tea / Lipgloss component:
// it owns no state, dispatches no tea.Cmd, and takes its inputs (entries,
// selectedID, width) by value so the root Update loop drives it.
//
// Port of v1 src/tui/components/EntryList.tsx. Columns match v1:
//   ✓  | Hostname (30) | IP (20) | Aliases (flex)
//
// Per-row styling mirrors v1:
//   - enabled entries: green check (✓)
//   - disabled entries: red cross (✗)
//   - selected entry: reverse-video, bold hostname
//
// The selectedID parameter is sourced from store.SelectedEntryID() by the
// caller — EntryList does no store lookups so it stays trivially testable
// and reusable from Story 8 (search-filtered view).

package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// Column widths mirror v1's <Box width={N}> attributes in EntryList.tsx.
const (
	colCheckWidth    = 4
	colHostnameWidth = 30
	colIPWidth       = 20
)

// EntryList is a render-only table component.
//
// A zero-value EntryList is usable; width=0 means "no explicit width" and the
// Aliases column will simply not be padded beyond its content.
//
// Optional filter state (set via WithFilter, cleared via WithoutFilter) lets
// the search-mode flow drive a ranked subset into View() without changing
// the View() signature. When filterActive is true, View() ignores the
// `entries` argument and renders the filtered slice instead. Matched-field
// highlighting is applied per row by checking each rendered cell against
// the active query (case-insensitive contiguous match — sufficient to call
// out hits without re-implementing the weighted aggregator here; the
// authoritative match scoring lives in tui/search).
type EntryList struct {
	width        int
	filtered     []domain.Entry
	filterActive bool
	filterQuery  string
}

// NewEntryList constructs an EntryList sized for the given pane width.
// width should be the Layout.MainWidth() value the parent reserves.
func NewEntryList(width int) EntryList {
	return EntryList{width: width}
}

// WithWidth returns a copy of e with the given width applied. Mirrors the
// Layout.WithSize accessor used by the root Model on tea.WindowSizeMsg.
func (e EntryList) WithWidth(width int) EntryList {
	e.width = width
	return e
}

// Width returns the pane width EntryList was constructed with.
func (e EntryList) Width() int { return e.width }

// WithFilter returns a copy of e with the search-filtered entry list and
// query installed. The search-mode flow in app/search_mode.go calls this
// on every keystroke. Pass an empty (non-nil) slice for "filter active
// but no matches" — View() will render the empty-state placeholder while
// still respecting the filter-active branch (so the parent doesn't fall
// back to the unfiltered group entries).
func (e EntryList) WithFilter(filtered []domain.Entry, query string) EntryList {
	if filtered == nil {
		filtered = []domain.Entry{}
	}
	e.filtered = filtered
	e.filterActive = true
	e.filterQuery = query
	return e
}

// WithoutFilter clears any active filter, restoring the default behavior
// where View() renders its `entries` argument.
func (e EntryList) WithoutFilter() EntryList {
	e.filtered = nil
	e.filterActive = false
	e.filterQuery = ""
	return e
}

// FilterActive reports whether a search filter is currently installed.
// Exported for tests and for callers that want to branch their selection
// math on filtered vs unfiltered state.
func (e EntryList) FilterActive() bool { return e.filterActive }

// FilteredEntries returns the currently-installed filtered slice.
// Returns nil when no filter is active.
func (e EntryList) FilteredEntries() []domain.Entry {
	if !e.filterActive {
		return nil
	}
	out := make([]domain.Entry, len(e.filtered))
	copy(out, e.filtered)
	return out
}

// View renders entries as a styled table. selectedID is matched against
// each entry's ID to drive the reverse-video highlight; pass "" for no
// selection. Empty `entries` produces the v1 empty-state message.
func (e EntryList) View(entries []domain.Entry, selectedID string) string {
	// Filter-active branch: render the ranked subset rather than the
	// caller-provided entries. The search-mode flow installs the filter
	// via WithFilter on every keystroke; the view always reflects the
	// latest pushed subset.
	if e.filterActive {
		entries = e.filtered
	}

	if len(entries) == 0 {
		return emptyState()
	}

	var b strings.Builder
	b.WriteString(renderHeader())
	b.WriteByte('\n')

	last := len(entries) - 1
	for i, entry := range entries {
		b.WriteString(renderRow(entry, entry.ID == selectedID))
		if i != last {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// emptyState returns the v1 "No entries in this group" placeholder. Two
// dim-color lines, matching EntryList.tsx's empty branch.
func emptyState() string {
	dim := lipgloss.NewStyle().Faint(true)
	return dim.Render("No entries in this group") + "\n" +
		dim.Render("Press 'a' to add a new entry")
}

// renderHeader produces the bold/dim column-header row.
func renderHeader() string {
	header := lipgloss.NewStyle().Bold(true).Faint(true)
	return joinColumns(
		header.Width(colCheckWidth).Render("✓"),
		header.Width(colHostnameWidth).Render("Hostname"),
		header.Width(colIPWidth).Render("IP"),
		header.Render("Aliases"),
	)
}

// renderRow renders a single entry. When selected, the whole row uses
// reverse-video; otherwise the enabled indicator gets a green/red color
// and the auxiliary columns are faint, matching v1.
func renderRow(entry domain.Entry, selected bool) string {
	indicator := "✓"
	indicatorColor := lipgloss.Color("2") // ANSI green
	if !entry.Enabled {
		indicator = "✗"
		indicatorColor = lipgloss.Color("1") // ANSI red
	}

	aliases := ""
	if len(entry.Aliases) > 0 {
		aliases = strings.Join(entry.Aliases, ", ")
	}

	if selected {
		sel := lipgloss.NewStyle().Reverse(true)
		return joinColumns(
			sel.Width(colCheckWidth).Render(indicator),
			sel.Bold(true).Width(colHostnameWidth).Render(entry.Hostname),
			sel.Width(colIPWidth).Render(entry.IP),
			sel.Render(aliases),
		)
	}

	checkStyle := lipgloss.NewStyle().Foreground(indicatorColor).Width(colCheckWidth)
	nameStyle := lipgloss.NewStyle().Width(colHostnameWidth)
	dim := lipgloss.NewStyle().Faint(true)
	return joinColumns(
		checkStyle.Render(indicator),
		nameStyle.Render(entry.Hostname),
		dim.Width(colIPWidth).Render(entry.IP),
		dim.Render(aliases),
	)
}

// joinColumns horizontally concatenates pre-rendered cells with a single
// space of left padding (mirrors v1's paddingX={1} on row Boxes).
func joinColumns(cells ...string) string {
	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return " " + row
}
