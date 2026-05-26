package store

import (
	"sync"
	"testing"

	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/stretchr/testify/require"
)

// helper builders -------------------------------------------------------------

func mkEntry(id, host string, enabled bool) domain.Entry {
	return domain.Entry{
		ID:       id,
		IP:       "127.0.0.1",
		Hostname: host,
		Aliases:  []string{},
		Enabled:  enabled,
	}
}

// 1. initializes with default state
func TestStore_InitialDefaults(t *testing.T) {
	s := New()
	require.Equal(t, &domain.HostsFile{Version: 1, Groups: []domain.Group{}}, s.HostsFile())
	require.Equal(t, "", s.SelectedEntryID())
	require.Equal(t, []string{}, s.SelectedGroupPath())
	require.Equal(t, "", s.SearchQuery())
	require.Equal(t, ModeNormal, s.Mode())
	require.False(t, s.Dirty())
	require.Nil(t, s.Modal())
	require.Nil(t, s.StatusMessage())
}

// 2. loadHostsFile updates hostsFile and clears dirty
func TestStore_LoadHostsFile(t *testing.T) {
	s := New()
	s.MarkDirty()
	f := &domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}}
	s.LoadHostsFile(f)
	require.Equal(t, f, s.HostsFile())
	require.False(t, s.Dirty(), "load resets dirty")
}

// 3. selectEntry updates selectedEntryID
func TestStore_SelectEntry(t *testing.T) {
	s := New()
	s.SelectEntry("test-id-123")
	require.Equal(t, "test-id-123", s.SelectedEntryID())
}

// 4. selectEntry with "" clears
func TestStore_SelectEntry_Clear(t *testing.T) {
	s := New()
	s.SelectEntry("x")
	s.SelectEntry("")
	require.Equal(t, "", s.SelectedEntryID())
}

// 5. setSearchQuery
func TestStore_SetSearchQuery(t *testing.T) {
	s := New()
	s.SetSearchQuery("localhost")
	require.Equal(t, "localhost", s.SearchQuery())
}

// 6. setMode — table-driven
func TestStore_SetMode(t *testing.T) {
	cases := []struct {
		name string
		mode StoreMode
		str  string
	}{
		{"normal", ModeNormal, "normal"},
		{"search", ModeSearch, "search"},
		{"edit", ModeEdit, "edit"},
		{"modal", ModeModal, "modal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.SetMode(tc.mode)
			require.Equal(t, tc.mode, s.Mode())
			require.Equal(t, tc.str, s.Mode().String())
		})
	}
}

// 7. selectGroup updates selectedGroupPath and returns a copy
func TestStore_SelectGroup(t *testing.T) {
	s := New()
	s.SelectGroup([]string{"work", "prod"})
	got := s.SelectedGroupPath()
	require.Equal(t, []string{"work", "prod"}, got)
	got[0] = "MUTATED"
	require.Equal(t, []string{"work", "prod"}, s.SelectedGroupPath(), "returned slice must be a copy")
}

// 8. markDirty / clearDirty
func TestStore_DirtyFlag(t *testing.T) {
	s := New()
	s.MarkDirty()
	require.True(t, s.Dirty())
	s.ClearDirty()
	require.False(t, s.Dirty())
}

// 9. addEntry appends to selected group and marks dirty
func TestStore_AddEntry(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}})
	s.SelectGroup([]string{"work"})
	e := mkEntry("e1", "test.local", true)
	s.AddEntry(e)

	hf := s.HostsFile()
	require.Len(t, hf.Groups[0].Entries, 1)
	require.Equal(t, e, hf.Groups[0].Entries[0])
	require.True(t, s.Dirty())
}

// 10. addEntry with empty selection is a no-op
func TestStore_AddEntry_EmptyPath_NoOp(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}})
	// no SelectGroup
	s.AddEntry(mkEntry("e1", "x", true))
	require.Len(t, s.HostsFile().Groups[0].Entries, 0)
}

// 11. updateEntry replaces existing
func TestStore_UpdateEntry(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "old.local", true)}, Groups: []domain.Group{}},
	}})
	updated := domain.Entry{ID: "e1", IP: "192.168.1.1", Hostname: "new.local", Aliases: []string{"a1"}, Enabled: false, Comment: "u"}
	s.UpdateEntry("e1", updated)
	require.Equal(t, updated, s.HostsFile().Groups[0].Entries[0])
	require.True(t, s.Dirty())
}

// 12. deleteEntry removes & clears selection when selected
func TestStore_DeleteEntry_ClearsSelection(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "x", true)}, Groups: []domain.Group{}},
	}})
	s.SelectEntry("e1")
	s.DeleteEntry("e1")
	require.Len(t, s.HostsFile().Groups[0].Entries, 0)
	require.Equal(t, "", s.SelectedEntryID())
	require.True(t, s.Dirty())
}

// 13. deleteEntry preserves selection if different
func TestStore_DeleteEntry_PreservesOtherSelection(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "x", true), mkEntry("e2", "y", true)}, Groups: []domain.Group{}},
	}})
	s.SelectEntry("e2")
	s.DeleteEntry("e1")
	require.Equal(t, "e2", s.SelectedEntryID())
}

// 14. toggleEntry flips Enabled
func TestStore_ToggleEntry(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "x", true)}, Groups: []domain.Group{}},
	}})
	s.ToggleEntry("e1")
	require.False(t, s.HostsFile().Groups[0].Entries[0].Enabled)
	require.True(t, s.Dirty())
	s.ToggleEntry("e1")
	require.True(t, s.HostsFile().Groups[0].Entries[0].Enabled)
}

// 15. addEntry into a NESTED group via selectedGroupPath
func TestStore_AddEntry_Nested(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{
			{Name: "prod", Entries: []domain.Entry{}, Groups: []domain.Group{}},
		}},
	}})
	s.SelectGroup([]string{"work", "prod"})
	s.AddEntry(mkEntry("e1", "api.prod", true))

	root := s.HostsFile().Groups[0]
	require.Len(t, root.Entries, 0, "root group untouched")
	require.Len(t, root.Groups[0].Entries, 1, "entry landed in nested group")
}

// 16. moveEntry between nested groups
func TestStore_MoveEntry(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "api", true)}, Groups: []domain.Group{}},
		{Name: "home", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}})
	s.MoveEntry("e1", []string{"home"})
	hf := s.HostsFile()
	require.Len(t, hf.Groups[0].Entries, 0)
	require.Len(t, hf.Groups[1].Entries, 1)
	require.Equal(t, "e1", hf.Groups[1].Entries[0].ID)
	require.True(t, s.Dirty())
}

// 17. moveEntry no-op when target empty
func TestStore_MoveEntry_NoOpOnEmptyTarget(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "api", true)}, Groups: []domain.Group{}},
	}})
	s.MoveEntry("e1", []string{})
	require.Len(t, s.HostsFile().Groups[0].Entries, 1, "entry remains")
	require.False(t, s.Dirty())
}

// 18. moveEntry no-op when entry missing
func TestStore_MoveEntry_MissingEntry(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
		{Name: "home", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}})
	s.MoveEntry("nonexistent", []string{"home"})
	require.Len(t, s.HostsFile().Groups[1].Entries, 0)
	require.False(t, s.Dirty())
}

// 19. addGroup at root
func TestStore_AddGroup_Root(t *testing.T) {
	s := New()
	s.AddGroup("work", nil)
	hf := s.HostsFile()
	require.Len(t, hf.Groups, 1)
	require.Equal(t, "work", hf.Groups[0].Name)
	require.True(t, s.Dirty())
}

// 20. addGroup nested
func TestStore_AddGroup_Nested(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
	}})
	s.AddGroup("prod", []string{"work"})
	require.Equal(t, "prod", s.HostsFile().Groups[0].Groups[0].Name)
}

// 21. modal open/close lifecycle
func TestStore_ModalOpenClose(t *testing.T) {
	s := New()
	s.OpenModal(ModalHelp, map[string]string{"k": "v"})
	require.NotNil(t, s.Modal())
	require.Equal(t, ModalHelp, s.Modal().Type)
	require.Equal(t, ModeModal, s.Mode())

	s.CloseModal()
	require.Nil(t, s.Modal())
	require.Equal(t, ModeNormal, s.Mode())
}

// 22. status message set/clear
func TestStore_StatusMessage(t *testing.T) {
	s := New()
	s.SetStatusMessage("applied", StatusSuccess)
	got := s.StatusMessage()
	require.NotNil(t, got)
	require.Equal(t, "applied", got.Text)
	require.Equal(t, StatusSuccess, got.Level)

	s.ClearStatusMessage()
	require.Nil(t, s.StatusMessage())
}

// 23. findGroupByPath — table-driven
func TestFindGroupByPath(t *testing.T) {
	tree := []domain.Group{
		{Name: "work", Groups: []domain.Group{
			{Name: "prod"},
			{Name: "dev"},
		}},
		{Name: "home"},
	}
	cases := []struct {
		name string
		path []string
		want string // group name or "" for nil
	}{
		{"empty path returns nil", nil, ""},
		{"top-level match", []string{"work"}, "work"},
		{"nested match", []string{"work", "prod"}, "prod"},
		{"missing top-level", []string{"missing"}, ""},
		{"missing nested", []string{"work", "missing"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := findGroupByPath(tree, tc.path)
			if tc.want == "" {
				require.Nil(t, g)
				return
			}
			require.NotNil(t, g)
			require.Equal(t, tc.want, g.Name)
		})
	}
}

// 24. UpdateEntry deep in a nested group
func TestStore_UpdateEntry_Nested(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Groups: []domain.Group{
			{Name: "prod", Entries: []domain.Entry{mkEntry("e1", "old", true)}},
		}},
	}})
	updated := mkEntry("e1", "new", false)
	s.UpdateEntry("e1", updated)
	got := s.HostsFile().Groups[0].Groups[0].Entries[0]
	require.Equal(t, "new", got.Hostname)
	require.False(t, got.Enabled)
}

// 25. Concurrent readers + writer race test (run with -race)
func TestStore_ConcurrentAccess(t *testing.T) {
	s := New()
	s.LoadHostsFile(&domain.HostsFile{Version: 1, Groups: []domain.Group{
		{Name: "work", Entries: []domain.Entry{mkEntry("e1", "x", true)}, Groups: []domain.Group{}},
	}})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.ToggleEntry("e1")
		}()
		go func() {
			defer wg.Done()
			_ = s.HostsFile()
			_ = s.Dirty()
			_ = s.SelectedGroupPath()
		}()
	}
	wg.Wait()
	require.True(t, s.Dirty())
}
