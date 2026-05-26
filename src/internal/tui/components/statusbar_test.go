package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/src/internal/tui/store"
)

// stripANSI removes ANSI escape sequences from rendered Lipgloss output so
// assertions can inspect the plain-text content. Lipgloss colorizes per the
// active terminfo profile (including in test runs), so substring assertions
// on raw output are flaky across environments.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestStatusBar_NormalMode_NoDirty_NoQuery_NoMessage verifies the minimal
// render path: only the mode pill is shown.
func TestStatusBar_NormalMode_NoDirty_NoQuery_NoMessage(t *testing.T) {
	s := store.New()
	out := stripANSI(NewStatusBar().View(s))

	require.Contains(t, out, "NORMAL")
	require.NotContains(t, out, dirtyMarker, "dirty marker must be absent when clean")
	require.NotContains(t, out, "/", "search query zone must be absent when no query")
}

// TestStatusBar_Dirty_ShowsMarker verifies the dirty marker appears once the
// store reports unsaved changes.
func TestStatusBar_Dirty_ShowsMarker(t *testing.T) {
	s := store.New()
	s.MarkDirty()

	out := stripANSI(NewStatusBar().View(s))
	require.Contains(t, out, dirtyMarker)
	require.Contains(t, out, "NORMAL")
}

// TestStatusBar_AllModes verifies every StoreMode renders as its uppercase
// label in the mode pill.
func TestStatusBar_AllModes(t *testing.T) {
	cases := []struct {
		mode store.StoreMode
		want string
	}{
		{store.ModeNormal, "NORMAL"},
		{store.ModeSearch, "SEARCH"},
		{store.ModeEdit, "EDIT"},
		{store.ModeModal, "MODAL"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.want, func(t *testing.T) {
			s := store.New()
			s.SetMode(c.mode)
			out := stripANSI(NewStatusBar().View(s))
			require.Contains(t, out, c.want)
		})
	}
}

// TestStatusBar_SearchMode_RendersQueryZone verifies the search query zone
// appears in SEARCH mode even when the query is empty (so the operator sees
// the prompt as they type).
func TestStatusBar_SearchMode_RendersQueryZone(t *testing.T) {
	s := store.New()
	s.SetMode(store.ModeSearch)

	out := stripANSI(NewStatusBar().View(s))
	require.Contains(t, out, "SEARCH")
	require.Contains(t, out, "/", "search prompt must render in SEARCH mode")
}

// TestStatusBar_NonEmptyQuery_RendersOutsideSearchMode verifies a persistent
// query string is surfaced even when the operator has left SEARCH mode (a
// filter is still active).
func TestStatusBar_NonEmptyQuery_RendersOutsideSearchMode(t *testing.T) {
	s := store.New()
	s.SetSearchQuery("staging")

	out := stripANSI(NewStatusBar().View(s))
	require.Contains(t, out, "/staging")
	require.Contains(t, out, "NORMAL")
}

// TestStatusBar_StatusMessage_AllLevels verifies the status zone renders the
// transient message text for each StatusLevel.
func TestStatusBar_StatusMessage_AllLevels(t *testing.T) {
	cases := []struct {
		name  string
		level store.StatusLevel
		text  string
	}{
		{"info", store.StatusInfo, "loaded ~/.hosts"},
		{"success", store.StatusSuccess, "applied · 1 entry"},
		{"error", store.StatusError, "apply failed: permission denied"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s := store.New()
			s.SetStatusMessage(c.text, c.level)

			out := stripANSI(NewStatusBar().View(s))
			require.Contains(t, out, c.text)
		})
	}
}

// TestStatusBar_StatusMessage_TTL_ClearsZone is the parity test for the TTL
// behavior. The store is set with a status message, then the TTL "expires"
// (modeled by the root Update loop calling ClearStatusMessage). The status
// zone must disappear from the next View() pass.
//
// This validates the contract called out in statusbar.go's package comment:
// the StatusBar is render-only; TTL expiration is driven by the app layer
// via store.ClearStatusMessage; the StatusBar simply mirrors the store.
func TestStatusBar_StatusMessage_TTL_ClearsZone(t *testing.T) {
	s := store.New()
	s.SetStatusMessage("saved", store.StatusSuccess)

	bar := NewStatusBar()

	before := stripANSI(bar.View(s))
	require.Contains(t, before, "saved")

	// Simulate TTL elapsing (root Update would dispatch this on tea.Tick).
	s.ClearStatusMessage()

	after := stripANSI(bar.View(s))
	require.NotContains(t, after, "saved", "status zone must clear after TTL")
}

// TestStatusBar_AllZonesTogether is the "all four zones present" snapshot
// — the bead's L2 verification criterion. We assert structural invariants
// (each zone's marker appears in the output) rather than golden-file the
// rendered string, mirroring layout_test.go's approach (rendered widths
// vary across lipgloss versions).
func TestStatusBar_AllZonesTogether(t *testing.T) {
	s := store.New()
	s.MarkDirty()
	s.SetMode(store.ModeSearch)
	s.SetSearchQuery("api")
	s.SetStatusMessage("3 results", store.StatusInfo)

	out := stripANSI(NewStatusBar().View(s))

	require.Contains(t, out, dirtyMarker, "zone 1: dirty marker")
	require.Contains(t, out, "SEARCH", "zone 2: mode pill")
	require.Contains(t, out, "/api", "zone 3: search query")
	require.Contains(t, out, "3 results", "zone 4: status message")
}

// TestStatusBar_NilStore_ReturnsEmpty verifies the defensive nil-store path
// (mirrors layout.go's tolerance of pre-WindowSizeMsg state). A nil store
// must not panic and must return an empty string so Layout's join produces
// a sensible empty frame on the first paint.
func TestStatusBar_NilStore_ReturnsEmpty(t *testing.T) {
	out := NewStatusBar().View(nil)
	require.Equal(t, "", out)
}

// TestStatusBar_View_NoCmdProduced is the render-only contract test (matches
// TestLayout_View_NoCmdProduced in layout_test.go). The signature must
// remain a pure (store) → string function — adding tea.Cmd would break the
// "components don't drive the event loop" invariant from phase-4-story-map.md
// §Story 2.
func TestStatusBar_View_NoCmdProduced(t *testing.T) {
	var _ func(*store.Store) string = NewStatusBar().View
}
