// Package app wires the Bubble Tea root program for the hostie TUI.
//
// This is the navigation-only skeleton (Phase 4 Story 3 / bead
// hosts-cli-go-mig-p4-app-skeleton-kgg). It implements the boot →
// render → quit cycle with j/k navigation (with wrap), Tab focus swap
// between sidebar and main, and q quit. It does NOT handle modals,
// mutations, search, or /etc/hosts apply — those land in later beads
// (hosts-cli-go-mig-p4-app-mutations-9fk, search-mode-n1c, etc.).
//
// Design references:
//   - design.md D3 (TUI stack: Bubble Tea + Bubbles + Lipgloss)
//   - design.md D8 (store is plain struct + sync.RWMutex, no zustand)
//   - phase-4-contract.md Exit-State clause 4 (app/{model,update,view}.go)
//   - phase-4-story-map.md Story 3 (navigation only — no mutations)
//
// The root Model composes:
//   - *store.Store — the centralized application state
//   - components.Layout — the 3-pane Lipgloss frame
//   - components.EntryList — the main-pane table
//   - components.StatusBar — the bottom status row
//   - GroupTree is rendered via components.RenderGroupTree (function-only)
//   - a Focus enum tracking which pane receives j/k
//   - a hostsPath string passed in via NewModel (typically "~/.hosts")
//
// All Bubble Tea-facing methods (Init, Update, View) live in this package
// per the Elm-loop convention. update.go owns the message dispatch; view.go
// owns the render composition; model.go owns state plumbing and the
// constructor.
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/components"
	"github.com/hungthai1401/hostie/go/internal/tui/search"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// Focus identifies which pane currently receives keyboard input for
// navigation. Tab toggles between FocusSidebar and FocusMain.
type Focus int

const (
	// FocusMain is the default — j/k navigates the EntryList.
	FocusMain Focus = iota
	// FocusSidebar — j/k navigates the GroupTree.
	FocusSidebar
)

// String returns the lowercase pane name for status/diagnostic surfaces.
func (f Focus) String() string {
	switch f {
	case FocusSidebar:
		return "sidebar"
	case FocusMain:
		return "main"
	default:
		return "main"
	}
}

// Model is the Bubble Tea root model for the hostie TUI skeleton.
//
// It is a value-type Bubble Tea model (Update returns a tea.Model by value,
// matching the bubbletea v1 convention). The Store is held by pointer
// because it owns the sync.RWMutex; copying it would copy the mutex.
type Model struct {
	// store is the centralized TUI state container.
	store *store.Store

	// layout, entryList, statusBar are the render-only components.
	// grouptree is function-only (components.RenderGroupTree).
	layout    components.Layout
	entryList components.EntryList
	statusBar components.StatusBar

	// focus tracks which pane receives navigation keys.
	focus Focus

	// width / height are mirrored from the last tea.WindowSizeMsg.
	// Both are zero until the first WindowSizeMsg arrives.
	width  int
	height int

	// hostsPath is the path to the YAML hosts file to load on Init.
	// Typically "~/.hosts"; tests pass an explicit fixture path.
	hostsPath string

	// quitting is set when q is pressed so View() can render a final
	// frame before the program exits.
	quitting bool

	// searchEngine is non-nil only while Mode==Search. It is rebuilt on
	// entering search mode (and lazily on the first applyQuery after) so
	// the engine always reflects the current HostsFile snapshot. See
	// search_mode.go for the lifecycle.
	searchEngine *search.Engine

	// priorSelection captures the SelectedEntryID at the moment '/' was
	// pressed so Esc can restore it (parity with v1 where leaving search
	// mode without committing returned the cursor to where it was).
	priorSelection string

	// modalHost owns the active modal (if any). Constructed in NewModel
	// and bound to the same *store.Store so the host can keep StoreMode in
	// sync. See app/modal_host.go and
	// .spikes/go-migration/p4-modal-pattern/FINDINGS.md §5 for the
	// integration contract.
	modalHost *ModalHost

	// applyRunner runs the apply pipeline (~/.hosts write → /etc/hosts
	// render+write) on every successful mutation per design.md D11. It is
	// constructed in NewModel against hostsPath with dryRun=false (D14: no
	// dry-run path in the TUI). Held as an applyRunner interface so tests
	// can inject a fake that records invocations and synthesizes
	// success/failure outcomes without touching /etc/hosts.
	applyRunner applyRunner
}

// NewModel constructs a Model rooted at the given hosts file path.
// Pass "~/.hosts" for production; pass an explicit path in tests so
// fileio's tilde expansion is not relied on.
func NewModel(hostsPath string) Model {
	s := store.New()
	return Model{
		store:        s,
		layout:       components.NewLayout(0, 0),
		entryList:    components.NewEntryList(0),
		statusBar:    components.NewStatusBar(),
		focus:        FocusMain,
		hostsPath:    hostsPath,
		modalHost:    NewModalHost(s),
		applyRunner:  apply.NewRunner(hostsPath, false),
	}
}

// WithApplyRunner returns a copy of the Model with the supplied apply runner
// installed. Used by tests to inject a fake runner; production code uses the
// runner constructed by NewModel.
func (m Model) WithApplyRunner(r applyRunner) Model {
	m.applyRunner = r
	return m
}

// Store returns the underlying *store.Store. Exported for tests and for
// future beads that need to drive store mutations from outside the model
// (e.g., the apply.Runner integration in app-mutations-9fk).
func (m Model) Store() *store.Store { return m.store }

// Focus returns the current focus pane. Exported for tests.
func (m Model) Focus() Focus { return m.focus }

// HostsPath returns the path passed to NewModel. Exported for tests.
func (m Model) HostsPath() string { return m.hostsPath }

// Init implements tea.Model. Returns a tea.Cmd that loads the hosts file
// off the UI goroutine and posts a hostsLoadedMsg back into the Update loop.
//
// If the file does not exist or fails to parse, hostsLoadedMsg.err is set;
// Update keeps the store at its empty default and surfaces the failure via
// the status bar. The TUI never aborts on a missing ~/.hosts (matches v1).
func (m Model) Init() tea.Cmd {
	return loadHostsCmd(m.hostsPath)
}

// hostsLoadedMsg is delivered to Update after the initial fileio.ReadHostsFile
// completes. On success, file points at the parsed HostsFile; on failure,
// err is set and file is the zero value.
type hostsLoadedMsg struct {
	file domain.HostsFile
	err  error
}

// loadHostsCmd reads the hosts file asynchronously and wraps the result in
// a hostsLoadedMsg. Exposed at package scope (lowercase) so update.go and
// tests can refer to the message type.
func loadHostsCmd(path string) tea.Cmd {
	return func() tea.Msg {
		hf, err := fileio.ReadHostsFile(path)
		return hostsLoadedMsg{file: hf, err: err}
	}
}
