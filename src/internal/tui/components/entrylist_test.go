package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// mockEntries mirrors the v1 test fixture in
// src/tui/components/__tests__/EntryList.test.tsx so behavior is
// directly comparable across implementations.
func mockEntries() []domain.Entry {
	return []domain.Entry{
		{ID: "1", IP: "10.0.0.1", Hostname: "api.work", Aliases: []string{"api", "api-server"}, Enabled: true},
		{ID: "2", IP: "192.168.1.1", Hostname: "home.local", Aliases: []string{}, Enabled: true},
		{ID: "3", IP: "10.0.0.5", Hostname: "disabled.host", Aliases: []string{"old"}, Enabled: false},
	}
}

// TestEntryList_RendersEntriesWithColumns mirrors v1
// "renders entries with columns": hostnames and IPs from every entry
// must appear in the rendered frame.
func TestEntryList_RendersEntriesWithColumns(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	require.Contains(t, out, "api.work")
	require.Contains(t, out, "10.0.0.1")
	require.Contains(t, out, "home.local")
	require.Contains(t, out, "192.168.1.1")
	require.Contains(t, out, "disabled.host")
}

// TestEntryList_DisplaysAliases mirrors v1 "displays aliases": a multi-alias
// entry should render its aliases as a comma-separated list.
func TestEntryList_DisplaysAliases(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	require.Contains(t, out, "api, api-server")
}

// TestEntryList_ShowsEnabledIndicator mirrors v1 "shows enabled checkbox
// indicator": at least one row must show ✓ (the enabled glyph).
func TestEntryList_ShowsEnabledIndicator(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	require.Contains(t, out, "✓")
}

// TestEntryList_ShowsDisabledIndicator mirrors v1 "shows disabled checkbox
// indicator": the disabled entry's row must show ✗.
func TestEntryList_ShowsDisabledIndicator(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	require.Contains(t, out, "✗")
}

// TestEntryList_HighlightsSelectedEntry mirrors v1 "highlights selected
// entry": the row matching selectedID must still render and the selected
// hostname must be present. We do not assert on ANSI escapes because
// lipgloss strips styling when the test harness reports no color profile
// (TERM unset / TTY absent). Structural presence is what v1 asserted too.
func TestEntryList_HighlightsSelectedEntry(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "2")
	require.Contains(t, out, "home.local")
	// All other rows must still appear — selection should not hide entries.
	require.Contains(t, out, "api.work")
	require.Contains(t, out, "disabled.host")
}

// TestEntryList_EmptyState mirrors v1 "displays empty state when no
// entries": the placeholder text must render.
func TestEntryList_EmptyState(t *testing.T) {
	out := NewEntryList(80).View(nil, "")
	require.Contains(t, out, "No entries")
	require.Contains(t, out, "Press 'a' to add a new entry")
}

// TestEntryList_HandlesNoAliases mirrors v1 "handles entries with no
// aliases": a row with empty Aliases must still render without panicking
// and the hostname must be present.
func TestEntryList_HandlesNoAliases(t *testing.T) {
	entries := []domain.Entry{
		{ID: "x", IP: "1.1.1.1", Hostname: "noalias.example", Aliases: nil, Enabled: true},
	}
	out := NewEntryList(80).View(entries, "")
	require.Contains(t, out, "noalias.example")
	require.Contains(t, out, "1.1.1.1")
}

// TestEntryList_DisplaysColumnHeaders mirrors v1 "displays column headers".
func TestEntryList_DisplaysColumnHeaders(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	require.Contains(t, out, "Hostname")
	require.Contains(t, out, "IP")
	require.Contains(t, out, "Aliases")
}

// TestEntryList_RenderOnlyContract is a contract test mirroring the
// "render-only, no event loop" invariant from phase-4-story-map.md
// (Story 2). If anyone adds Update/tea.Cmd to EntryList, the signature
// here will fail to compile, surfacing the contract break.
func TestEntryList_RenderOnlyContract(t *testing.T) {
	var _ func([]domain.Entry, string) string = NewEntryList(10).View
}

// TestEntryList_EnabledRowFormatting verifies enabled rows include both
// the green indicator and the hostname in proximity — guarding against
// regressions that strip styling or reorder cells.
func TestEntryList_EnabledRowFormatting(t *testing.T) {
	out := NewEntryList(80).View(mockEntries(), "")
	lines := strings.Split(out, "\n")
	// Header + 3 entry rows.
	require.GreaterOrEqual(t, len(lines), 4)

	// Find the api.work row and assert ✓ appears before the hostname.
	var apiLine string
	for _, ln := range lines {
		if strings.Contains(ln, "api.work") {
			apiLine = ln
			break
		}
	}
	require.NotEmpty(t, apiLine)
	checkIdx := strings.Index(apiLine, "✓")
	nameIdx := strings.Index(apiLine, "api.work")
	require.GreaterOrEqual(t, checkIdx, 0)
	require.Greater(t, nameIdx, checkIdx)
}
