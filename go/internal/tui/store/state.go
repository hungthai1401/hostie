// Package store provides the TUI application state for Hostie.
//
// It is a port of v1 src/tui/store.ts (zustand) to a plain Go struct
// guarded by sync.RWMutex per design decision D8.
//
// All mutation methods take the write lock; all accessor methods take the
// read lock. Returned values are deep-copied or value-typed where it matters
// for the caller (Bubble Tea's Update loop) so that no shared pointer can be
// observed mutating concurrently.
package store

import (
	"sync"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// StoreMode mirrors the v1 `Mode` union: "normal" | "search" | "edit" | "modal".
type StoreMode int

const (
	ModeNormal StoreMode = iota
	ModeSearch
	ModeEdit
	ModeModal
)

// String returns the v1-equivalent lowercase mode name for status/display.
func (m StoreMode) String() string {
	switch m {
	case ModeNormal:
		return "normal"
	case ModeSearch:
		return "search"
	case ModeEdit:
		return "edit"
	case ModeModal:
		return "modal"
	default:
		return "normal"
	}
}

// ModalType mirrors the v1 modal-type union.
type ModalType int

const (
	ModalGroupCreator ModalType = iota
	ModalEntryCreator
	ModalEntryEditor
	ModalConfirmation
	ModalMoveToGroup
	ModalHelp
)

// ModalState describes the currently open modal (nil when none).
//
// Data is intentionally `any` to preserve v1 flexibility — concrete shape is
// per-modal-type and validated by the consuming component, not the store.
type ModalState struct {
	Type ModalType
	Data any
}

// StatusLevel mirrors v1 status-message levels.
type StatusLevel int

const (
	StatusInfo StatusLevel = iota
	StatusSuccess
	StatusError
)

// StatusMessage is a transient banner shown in the status bar.
type StatusMessage struct {
	Text  string
	Level StatusLevel
}

// Store is the TUI's centralized state container.
//
// Concurrency: every exported method that mutates state takes mu in write
// mode; every read accessor takes mu in read mode. Callers must not retain
// pointers into the store after the call returns — use the value-returning
// accessors for snapshots.
type Store struct {
	mu sync.RWMutex

	hostsFile         *domain.HostsFile
	selectedEntryID   string
	selectedGroupPath []string
	searchQuery       string
	mode              StoreMode
	dirty             bool
	modal             *ModalState
	statusMessage     *StatusMessage
}

// New returns a Store initialized with v1-equivalent defaults:
// empty hosts file (version=1), normal mode, no selection, no modal.
func New() *Store {
	return &Store{
		hostsFile:         &domain.HostsFile{Version: 1, Groups: []domain.Group{}},
		selectedGroupPath: []string{},
		mode:              ModeNormal,
	}
}

// -----------------------------------------------------------------------------
// Read accessors (RLock)
// -----------------------------------------------------------------------------

// HostsFile returns a pointer to the current hosts file.
//
// Callers MUST NOT mutate the returned value. Use the store's mutating methods
// instead. (Pointer is returned to avoid a full deep-copy on every read; the
// store's invariant is that mutating methods always replace the underlying
// slice/struct rather than editing in place.)
func (s *Store) HostsFile() *domain.HostsFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hostsFile
}

// SelectedEntryID returns the currently selected entry ID, or "" if none.
func (s *Store) SelectedEntryID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedEntryID
}

// SelectedGroupPath returns a copy of the currently selected group path.
func (s *Store) SelectedGroupPath() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.selectedGroupPath))
	copy(out, s.selectedGroupPath)
	return out
}

// SearchQuery returns the current search query.
func (s *Store) SearchQuery() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.searchQuery
}

// Mode returns the current TUI mode.
func (s *Store) Mode() StoreMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// Dirty reports whether the state has unsaved changes.
func (s *Store) Dirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

// Modal returns the current modal state (nil when no modal is open).
func (s *Store) Modal() *ModalState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.modal == nil {
		return nil
	}
	cp := *s.modal
	return &cp
}

// StatusMessage returns the transient status message (nil when none).
func (s *Store) StatusMessage() *StatusMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.statusMessage == nil {
		return nil
	}
	cp := *s.statusMessage
	return &cp
}

// -----------------------------------------------------------------------------
// Mutating actions (Lock) — match v1 zustand actions 1:1
// -----------------------------------------------------------------------------

// LoadHostsFile replaces the hosts file and clears the dirty flag.
//
// Mirrors v1 `loadHostsFile`: a freshly-loaded file is by definition clean.
func (s *Store) LoadHostsFile(file *domain.HostsFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = file
	s.dirty = false
}

// SelectEntry sets the selected entry ID (pass "" to clear selection).
func (s *Store) SelectEntry(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectedEntryID = id
}

// SelectGroup sets the selected group path (pass nil/empty to clear).
func (s *Store) SelectGroup(path []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == nil {
		s.selectedGroupPath = []string{}
		return
	}
	cp := make([]string, len(path))
	copy(cp, path)
	s.selectedGroupPath = cp
}

// SetSearchQuery updates the search query string.
func (s *Store) SetSearchQuery(q string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searchQuery = q
}

// SetMode changes the current TUI mode.
func (s *Store) SetMode(m StoreMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = m
}

// MarkDirty sets the dirty flag.
func (s *Store) MarkDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true
}

// ClearDirty clears the dirty flag.
func (s *Store) ClearDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = false
}

// AddEntry appends an entry to the currently selected group and marks dirty.
//
// If no group is selected (path empty), the entry is dropped — matching v1
// `addEntryToGroup` which returns groups unchanged for an empty path.
func (s *Store) AddEntry(entry domain.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups:  addEntryToGroup(s.hostsFile.Groups, s.selectedGroupPath, entry),
	}
	s.dirty = true
}

// UpdateEntry replaces the entry with the given ID anywhere in the tree.
func (s *Store) UpdateEntry(id string, entry domain.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups: findAndUpdateEntry(s.hostsFile.Groups, id, func(_ domain.Entry) (domain.Entry, bool) {
			return entry, true
		}),
	}
	s.dirty = true
}

// DeleteEntry removes the entry with the given ID and clears selection if it
// pointed at the deleted entry.
func (s *Store) DeleteEntry(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups: findAndUpdateEntry(s.hostsFile.Groups, id, func(_ domain.Entry) (domain.Entry, bool) {
			return domain.Entry{}, false
		}),
	}
	if s.selectedEntryID == id {
		s.selectedEntryID = ""
	}
	s.dirty = true
}

// ToggleEntry flips the Enabled flag on the entry with the given ID.
func (s *Store) ToggleEntry(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups: findAndUpdateEntry(s.hostsFile.Groups, id, func(e domain.Entry) (domain.Entry, bool) {
			e.Enabled = !e.Enabled
			return e, true
		}),
	}
	s.dirty = true
}

// MoveEntry relocates an entry to the target group path. If the entry does
// not exist or the target path is empty, the call is a no-op (matches v1).
func (s *Store) MoveEntry(id string, targetGroupPath []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed *domain.Entry
	groupsWithout := findAndUpdateEntry(s.hostsFile.Groups, id, func(e domain.Entry) (domain.Entry, bool) {
		cp := e
		removed = &cp
		return domain.Entry{}, false
	})

	if removed == nil || len(targetGroupPath) == 0 {
		return
	}

	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups:  addEntryToGroup(groupsWithout, targetGroupPath, *removed),
	}
	s.dirty = true
}

// AddGroup creates a new (empty) group at parentPath. Empty parentPath adds
// at the root level (matches v1 `addGroupToPath`).
func (s *Store) AddGroup(name string, parentPath []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostsFile = &domain.HostsFile{
		Version: s.hostsFile.Version,
		Groups:  addGroupToPath(s.hostsFile.Groups, name, parentPath),
	}
	s.dirty = true
}

// OpenModal opens a modal of the given type with optional payload data, and
// switches the mode to `modal` (matches v1).
func (s *Store) OpenModal(t ModalType, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modal = &ModalState{Type: t, Data: data}
	s.mode = ModeModal
}

// CloseModal clears any open modal and returns to normal mode.
func (s *Store) CloseModal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modal = nil
	s.mode = ModeNormal
}

// SetStatusMessage sets the transient status banner.
func (s *Store) SetStatusMessage(text string, level StatusLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusMessage = &StatusMessage{Text: text, Level: level}
}

// ClearStatusMessage clears the transient status banner.
func (s *Store) ClearStatusMessage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusMessage = nil
}

// -----------------------------------------------------------------------------
// Recursive helpers — ported verbatim from v1 store.ts
// -----------------------------------------------------------------------------

// findAndUpdateEntry walks every group recursively. For each entry matching
// `id`, it calls `updater` and either replaces the entry (keep=true) or
// removes it (keep=false). Other entries pass through unchanged.
func findAndUpdateEntry(groups []domain.Group, id string, updater func(domain.Entry) (domain.Entry, bool)) []domain.Group {
	out := make([]domain.Group, len(groups))
	for i, g := range groups {
		newEntries := make([]domain.Entry, 0, len(g.Entries))
		for _, e := range g.Entries {
			if e.ID == id {
				if updated, keep := updater(e); keep {
					newEntries = append(newEntries, updated)
				}
				continue
			}
			newEntries = append(newEntries, e)
		}
		out[i] = domain.Group{
			Name:        g.Name,
			Description: g.Description,
			Entries:     newEntries,
			Groups:      findAndUpdateEntry(g.Groups, id, updater),
		}
	}
	return out
}

// findGroupByPath returns the group at the given path, or nil if not found.
// Empty path returns nil (matches v1).
func findGroupByPath(groups []domain.Group, path []string) *domain.Group {
	if len(path) == 0 {
		return nil
	}
	first, rest := path[0], path[1:]
	for i := range groups {
		if groups[i].Name == first {
			if len(rest) == 0 {
				return &groups[i]
			}
			return findGroupByPath(groups[i].Groups, rest)
		}
	}
	return nil
}

// addEntryToGroup returns a new slice with `entry` appended to the group
// at `path`. Empty path is a no-op (matches v1: "root-level entries not
// supported in current design").
func addEntryToGroup(groups []domain.Group, path []string, entry domain.Entry) []domain.Group {
	if len(path) == 0 {
		return groups
	}
	first, rest := path[0], path[1:]
	out := make([]domain.Group, len(groups))
	for i, g := range groups {
		if g.Name != first {
			out[i] = g
			continue
		}
		if len(rest) == 0 {
			newEntries := make([]domain.Entry, len(g.Entries)+1)
			copy(newEntries, g.Entries)
			newEntries[len(g.Entries)] = entry
			out[i] = domain.Group{
				Name:        g.Name,
				Description: g.Description,
				Entries:     newEntries,
				Groups:      g.Groups,
			}
			continue
		}
		out[i] = domain.Group{
			Name:        g.Name,
			Description: g.Description,
			Entries:     g.Entries,
			Groups:      addEntryToGroup(g.Groups, rest, entry),
		}
	}
	return out
}

// addGroupToPath returns a new slice with a new (empty) group named `name`
// added at `parentPath`. Empty parentPath appends at the root (matches v1).
func addGroupToPath(groups []domain.Group, name string, parentPath []string) []domain.Group {
	if len(parentPath) == 0 {
		out := make([]domain.Group, len(groups)+1)
		copy(out, groups)
		out[len(groups)] = domain.Group{Name: name, Entries: []domain.Entry{}, Groups: []domain.Group{}}
		return out
	}
	first, rest := parentPath[0], parentPath[1:]
	out := make([]domain.Group, len(groups))
	for i, g := range groups {
		if g.Name != first {
			out[i] = g
			continue
		}
		if len(rest) == 0 {
			newGroups := make([]domain.Group, len(g.Groups)+1)
			copy(newGroups, g.Groups)
			newGroups[len(g.Groups)] = domain.Group{Name: name, Entries: []domain.Entry{}, Groups: []domain.Group{}}
			out[i] = domain.Group{
				Name:        g.Name,
				Description: g.Description,
				Entries:     g.Entries,
				Groups:      newGroups,
			}
			continue
		}
		out[i] = domain.Group{
			Name:        g.Name,
			Description: g.Description,
			Entries:     g.Entries,
			Groups:      addGroupToPath(g.Groups, name, rest),
		}
	}
	return out
}
