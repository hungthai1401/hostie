package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// fixture builds a deterministic 2-group hosts file for navigation tests.
//
//	work/                  (group)
//	  e1: 10.0.0.1 api.dev
//	  e2: 10.0.0.2 db.dev
//	personal/              (group)
//	  e3: 10.0.0.3 blog.dev
func fixture() *domain.HostsFile {
	return &domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name: "work",
				Entries: []domain.Entry{
					{ID: "e1", IP: "10.0.0.1", Hostname: "api.dev", Enabled: true},
					{ID: "e2", IP: "10.0.0.2", Hostname: "db.dev", Enabled: true},
				},
				Groups: []domain.Group{},
			},
			{
				Name: "personal",
				Entries: []domain.Entry{
					{ID: "e3", IP: "10.0.0.3", Hostname: "blog.dev", Enabled: true},
				},
				Groups: []domain.Group{},
			},
		},
	}
}

// seedModel constructs a Model with the fixture preloaded into its store,
// skipping the Init() fileio path. This is the harness every navigation
// test uses so the tests stay deterministic and disk-free.
func seedModel(t *testing.T) Model {
	t.Helper()
	m := NewModel("/dev/null")
	m.store.LoadHostsFile(fixture())
	// Give the Layout/EntryList realistic sizes so View() exercises the
	// width-aware code paths instead of the "no dimensions" early return.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m2.(Model)
}

// key builds a tea.KeyMsg for a single-rune key like "j". For special
// keys (tab, q, ctrl+c), the bubbletea KeyMsg.String() form is what
// Update switches on, so we construct the matching internal shape.
func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// TestNewModel_Defaults verifies the constructor produces an empty,
// usable model — no Init() call required for the navigation tests below
// that seed the store directly.
func TestNewModel_Defaults(t *testing.T) {
	m := NewModel("~/.hosts")
	require.NotNil(t, m.Store())
	require.Equal(t, FocusMain, m.Focus())
	require.Equal(t, "~/.hosts", m.HostsPath())
	require.Equal(t, store.ModeNormal, m.Store().Mode())
	// Pre-WindowSizeMsg View() must not panic.
	require.NotEmpty(t, m.View())
}

// TestInit_LoadsHostsFile verifies Init() returns a Cmd that reads the
// given path and posts a hostsLoadedMsg the Update loop can consume.
// This is the only test that touches disk — fileio is exercised indirectly
// to confirm wiring, not to retest fileio itself.
func TestInit_LoadsHostsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
groups:
  - name: work
    entries:
      - id: e1
        ip: 10.0.0.1
        hostname: api.dev
        aliases: []
        enabled: true
    groups: []
`), 0o644))

	m := NewModel(path)
	cmd := m.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(hostsLoadedMsg)
	require.True(t, ok, "expected hostsLoadedMsg, got %T", msg)
	require.NoError(t, loaded.err)
	require.Len(t, loaded.file.Groups, 1)
	require.Equal(t, "work", loaded.file.Groups[0].Name)

	// Feeding the message back into Update seeds the store.
	m2, _ := m.Update(loaded)
	require.Len(t, m2.(Model).Store().HostsFile().Groups, 1)
}

// TestInit_LoadFailure_SetsStatusMessage verifies a missing/corrupt hosts
// file surfaces as a red status banner without aborting the program — the
// store keeps its empty default so View() still renders.
func TestInit_LoadFailure_SetsStatusMessage(t *testing.T) {
	m := NewModel("/nonexistent/path/to/.hosts")
	cmd := m.Init()
	msg := cmd()
	loaded := msg.(hostsLoadedMsg)
	require.Error(t, loaded.err)

	m2, _ := m.Update(loaded)
	status := m2.(Model).Store().StatusMessage()
	require.NotNil(t, status)
	require.Equal(t, store.StatusError, status.Level)
	require.Contains(t, status.Text, "Failed to load")
	// Store is still the empty default — TUI did not crash on missing file.
	require.Empty(t, m2.(Model).Store().HostsFile().Groups)
}

// TestNavigation_JKWrap_Main exercises j/k wrap in the Main pane.
// The fixture's first group ("work") has 2 entries [e1, e2] and that's
// what the Main pane shows when no group is explicitly selected.
// Sequence: boot → j (e1) → j (e2) → j (wraps to e1) → k (wraps to e2).
//
// Navigation is now constrained to the *visible* group's entries (the
// Main pane only renders one group at a time, per visibleEntries), so
// e3 in the "personal" group is unreachable from here without first
// selecting that group via the sidebar. See TestNavigation_VisibleGroup.
func TestNavigation_JKWrap_Main(t *testing.T) {
	m := seedModel(t)
	// Initial state: no selection.
	require.Equal(t, "", m.Store().SelectedEntryID())

	m2, _ := m.Update(key("j"))
	require.Equal(t, "e1", m2.(Model).Store().SelectedEntryID())

	m3, _ := m2.Update(key("j"))
	require.Equal(t, "e2", m3.(Model).Store().SelectedEntryID())

	// Wrap forward: j at last (e2) → first (e1).
	m4, _ := m3.Update(key("j"))
	require.Equal(t, "e1", m4.(Model).Store().SelectedEntryID())

	// Wrap backward: k at first (e1) → last (e2).
	m5, _ := m4.Update(key("k"))
	require.Equal(t, "e2", m5.(Model).Store().SelectedEntryID())
}

// TestNavigation_ArrowKeys_NoLongerBound verifies the arrow keys are not
// wired to navigation — pure vim-style j/k/h/l only (per UAT request).
// Esc remains bound (it's the explicit "return to sidebar" escape hatch).
func TestNavigation_ArrowKeys_NoLongerBound(t *testing.T) {
	m := seedModel(t)
	before := m.Store().SelectedEntryID()
	for _, k := range []string{"up", "down", "left", "right"} {
		m2, cmd := m.Update(key(k))
		require.Nil(t, cmd, "arrow %q must not produce a Cmd", k)
		require.Equal(t, before, m2.(Model).Store().SelectedEntryID(),
			"arrow %q must not move selection", k)
	}
}

// TestNavigation_TabSwap verifies Tab toggles focus and, on entering an
// unselected pane, seeds the first item there (matches v1 useKeyboard.ts).
func TestNavigation_TabSwap(t *testing.T) {
	m := seedModel(t)
	require.Equal(t, FocusMain, m.Focus())

	m2, _ := m.Update(key("tab"))
	mm2 := m2.(Model)
	require.Equal(t, FocusSidebar, mm2.Focus())
	// Sidebar seeded to first group path.
	require.Equal(t, []string{"work"}, mm2.Store().SelectedGroupPath())

	m3, _ := m2.Update(key("tab"))
	mm3 := m3.(Model)
	require.Equal(t, FocusMain, mm3.Focus())
	// Main seeded to first entry.
	require.Equal(t, "e1", mm3.Store().SelectedEntryID())
}

// TestNavigation_TabBackToMain_ReseedsForVisibleGroup verifies the fix
// for the UAT regression where Tab→sidebar→select new group→Tab→main
// left SelectedEntryID pointing at an entry that isn't in the visible
// group, causing j/k to appear inert and `e`/`d` to operate on the
// wrong entry. After the fix, Tab→main re-seeds selection to the first
// entry of the currently visible group whenever the prior selection is
// no longer in view.
func TestNavigation_TabBackToMain_ReseedsForVisibleGroup(t *testing.T) {
	m := seedModel(t)

	// Step into "work" entries by pressing j (selects e1).
	m2, _ := m.Update(key("j"))
	m = m2.(Model)
	require.Equal(t, "e1", m.Store().SelectedEntryID())

	// Tab → sidebar (work selected by default seed).
	m3, _ := m.Update(key("tab"))
	m = m3.(Model)
	require.Equal(t, FocusSidebar, m.Focus())

	// j in sidebar → move to "personal" group.
	m4, _ := m.Update(key("j"))
	m = m4.(Model)
	require.Equal(t, []string{"personal"}, m.Store().SelectedGroupPath())

	// Tab → main. e1 is no longer visible (it lives in "work"), so the
	// model must re-seed to "personal"'s first entry (e3). Pre-fix this
	// stayed on e1, leaving Main with no highlight.
	m5, _ := m.Update(key("tab"))
	m = m5.(Model)
	require.Equal(t, FocusMain, m.Focus())
	require.Equal(t, "e3", m.Store().SelectedEntryID(),
		"Tab→main must re-seed to first entry of visible group when prior selection is offscreen")
}

// TestNavigation_JKWrap_Sidebar exercises j/k wrap with focus on the
// sidebar. The fixture has 2 top-level groups (work, personal); j wraps
// after personal, k wraps after work.
func TestNavigation_JKWrap_Sidebar(t *testing.T) {
	m := seedModel(t)
	m, _ = updateAs(m, key("tab")) // → sidebar, selects "work"
	require.Equal(t, FocusSidebar, m.Focus())
	require.Equal(t, []string{"work"}, m.Store().SelectedGroupPath())

	m, _ = updateAs(m, key("j"))
	require.Equal(t, []string{"personal"}, m.Store().SelectedGroupPath())

	// Wrap: j at last → first.
	m, _ = updateAs(m, key("j"))
	require.Equal(t, []string{"work"}, m.Store().SelectedGroupPath())

	// Wrap: k at first → last.
	m, _ = updateAs(m, key("k"))
	require.Equal(t, []string{"personal"}, m.Store().SelectedGroupPath())
}

// TestQuit_EmitsTeaQuit verifies q returns tea.Quit (the standard
// program-termination Cmd). Bubble Tea identifies Quit by message-equality
// from the Cmd's returned tea.Msg, so we invoke the Cmd and assert the
// message type.
func TestQuit_EmitsTeaQuit(t *testing.T) {
	m := seedModel(t)
	m2, cmd := m.Update(key("q"))
	require.NotNil(t, cmd)
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	require.True(t, isQuit, "expected tea.QuitMsg, got %T", msg)
	require.True(t, m2.(Model).quitting, "quitting flag must be set so View() renders the farewell")
}

// TestQuit_CtrlCEquivalent verifies ctrl+c is the same as q (standard TUI
// convention; the v1 binding existed via Ink's default).
func TestQuit_CtrlCEquivalent(t *testing.T) {
	m := seedModel(t)
	_, cmd := m.Update(key("ctrl+c"))
	require.NotNil(t, cmd)
	_, isQuit := cmd().(tea.QuitMsg)
	require.True(t, isQuit)
}

// TestUpdate_UnknownKey_IsNoOp verifies non-handled keys do not mutate
// state. With app-mutations-9fk landed, Space/d/a/e/g/m are now active;
// and with review-p1 wiring, '?' opens HelpModal and Enter re-applies, so
// only truly unbound keys (x, z, etc.) remain in this no-op set.
func TestUpdate_UnknownKey_IsNoOp(t *testing.T) {
	m := seedModel(t)
	before := m.Store().SelectedEntryID()
	for _, k := range []string{"x", "z"} {
		m2, cmd := m.Update(key(k))
		require.Nil(t, cmd, "unbound key %q must not produce a Cmd", k)
		require.Equal(t, before, m2.(Model).Store().SelectedEntryID(),
			"unbound key %q must not change selection", k)
		require.Equal(t, m.Focus(), m2.(Model).Focus(),
			"unbound key %q must not change focus", k)
	}
}

// TestWindowSize_PropagatesToLayout verifies tea.WindowSizeMsg flows into
// Layout and EntryList so subsequent View() calls render at the right
// width — this is the contract Story 4 modal spike relies on.
func TestWindowSize_PropagatesToLayout(t *testing.T) {
	m := NewModel("/dev/null")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	mm := m2.(Model)
	require.Equal(t, 200, mm.layout.Width())
	require.Equal(t, 60, mm.layout.Height())
	// EntryList width should match Layout's MainWidth (200 * 70%).
	require.Equal(t, mm.layout.MainWidth(), mm.entryList.Width())
}

// TestView_RendersAllPanes is the boot-scenario smoke test required by
// phase-4-story-map.md Story 3: render → assert all three panes are
// visible in the composed View() output.
func TestView_RendersAllPanes(t *testing.T) {
	m := seedModel(t)
	out := m.View()
	// Sidebar content (group names).
	require.Contains(t, out, "work")
	require.Contains(t, out, "personal")
	// Main content (entries of first group, since nothing selected yet).
	require.Contains(t, out, "api.dev")
	require.Contains(t, out, "db.dev")
	// StatusBar content (mode pill).
	require.True(t, strings.Contains(out, "NORMAL"),
		"status bar must show mode pill; output was:\n%s", out)
}

// TestView_QuittingRendersFarewell verifies View() short-circuits to the
// farewell line once q has flipped the quitting flag — Bubble Tea calls
// View one more time between Quit and process exit.
func TestView_QuittingRendersFarewell(t *testing.T) {
	m := seedModel(t)
	m2, _ := m.Update(key("q"))
	require.Equal(t, "bye\n", m2.(Model).View())
}

// TestBootScenario_J3_Tab_Q is the end-to-end navigation scenario from
// phase-4-story-map.md Story 3: boot, send j×3, send Tab, send q, assert
// the final state. This replaces teatest (which isn't in go.mod yet) with
// direct Update-call sequencing — the contract is identical: drive the
// Elm loop, observe the post-conditions.
func TestBootScenario_J3_Tab_Q(t *testing.T) {
	m := seedModel(t)

	// j×3 in Main pane: visible group "work" has [e1, e2]; the third j
	// wraps back to e1 (was the bug surface — pre-fix this stepped into
	// "personal"'s e3 even though e3 isn't visible in the Main pane).
	for i, want := range []string{"e1", "e2", "e1"} {
		var cmd tea.Cmd
		var mm tea.Model
		mm, cmd = m.Update(key("j"))
		m = mm.(Model)
		require.Nil(t, cmd, "j[%d] must not produce a Cmd in skeleton", i)
		require.Equal(t, want, m.Store().SelectedEntryID(), "after j[%d]", i)
	}

	// Tab swaps to sidebar; entry selection is preserved on the store.
	mm2, cmd := m.Update(key("tab"))
	m = mm2.(Model)
	require.Nil(t, cmd)
	require.Equal(t, FocusSidebar, m.Focus())
	require.Equal(t, "e1", m.Store().SelectedEntryID(), "Tab must not clear entry selection")

	// q quits cleanly.
	mm3, cmd := m.Update(key("q"))
	m = mm3.(Model)
	require.NotNil(t, cmd)
	_, isQuit := cmd().(tea.QuitMsg)
	require.True(t, isQuit)
	require.Equal(t, "bye\n", m.View())
}

// TestVisibleEntries_SelectedGroup verifies the Main pane resolves to the
// entries of the currently-selected group rather than the boot-default
// first group.
func TestVisibleEntries_SelectedGroup(t *testing.T) {
	hf := fixture()
	require.Equal(t, []domain.Entry{
		{ID: "e3", IP: "10.0.0.3", Hostname: "blog.dev", Enabled: true},
	}, visibleEntries(hf.Groups, []string{"personal"}))

	require.Equal(t, hf.Groups[0].Entries,
		visibleEntries(hf.Groups, nil),
		"empty selection falls back to first top-level group")

	require.Nil(t, visibleEntries(hf.Groups, []string{"nonexistent"}))
	require.Nil(t, visibleEntries(nil, nil))
}

// TestWrap_Behavior covers the wrap helper directly so regressions in
// edge cases (empty, single-element, negative-current) fail loudly here
// rather than via navigation-test fallout.
func TestWrap_Behavior(t *testing.T) {
	cases := []struct {
		name             string
		cur, step, n     int
		want             int
	}{
		{"empty list", -1, 1, 0, 0},
		{"unset + down → first", -1, 1, 3, 0},
		{"unset + up → last", -1, -1, 3, 2},
		{"middle down", 1, 1, 3, 2},
		{"last down wraps", 2, 1, 3, 0},
		{"first up wraps", 0, -1, 3, 2},
		{"single elem stays put", 0, 1, 1, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, wrap(c.cur, c.step, c.n))
		})
	}
}

// TestFocus_String is a tiny sanity test so the diagnostic surface (used
// by future status-bar integrations) doesn't silently regress.
func TestFocus_String(t *testing.T) {
	require.Equal(t, "main", FocusMain.String())
	require.Equal(t, "sidebar", FocusSidebar.String())
}

// updateAs is a small helper that calls Update and coerces the result
// back to a concrete Model — keeps test bodies readable when chaining
// multiple key events.
func updateAs(m Model, msg tea.Msg) (Model, tea.Cmd) {
	m2, cmd := m.Update(msg)
	return m2.(Model), cmd
}
