// apply_cmd.go — wires the TUI auto-apply pipeline to apply.Runner.
//
// Bead: hosts-cli-go-mig-p4-app-applycmd-91r
//
// Responsibility:
//
//   - Expose ApplyResultMsg as the result envelope returned by the apply
//     goroutine into the Bubble Tea Update loop.
//   - Expose applyCmd(runner, hostsFile) as the tea.Cmd that runs
//     apply.Runner.Apply on a goroutine and yields ApplyResultMsg.
//
// The actual store mutation has already happened by the time applyCmd runs
// (mutations.go emits ApplyTriggerMsg after every successful store change).
// applyCmd therefore takes a snapshot of the in-memory HostsFile and hands it
// to apply.Runner.Apply directly — Runner.Apply is the single funnel that
// writes ~/.hosts then renders the managed block into /etc/hosts.
//
// Design references:
//
//   - design.md D11: every TUI mutation auto-applies after the mutation
//     completes; there is no explicit "save" key in the TUI. (Ctrl+S exists as
//     an explicit re-apply for the failure-recovery case below.)
//   - design.md D13: ~/.hosts write is independent from /etc/hosts apply.
//     apply.Runner.Apply writes ~/.hosts first (step 1) and only then attempts
//     the /etc/hosts side; if step 1 succeeded but the /etc/hosts side fails,
//     ApplyResultMsg.Err carries the error and ~/.hosts remains on disk
//     reflecting the mutated state. The status bar surfaces the error and the
//     operator can retry via Ctrl+S or by performing another mutation.
//   - design.md D14: no --dry-run path in the TUI. applyCmd always runs the
//     real apply; the runner held by Model is constructed with dryRun=false.
//
// Why an interface (applyRunner) rather than a *apply.Runner pointer:
// keeping the dependency abstract lets tests inject a fake that records
// invocations and synthesizes both success and failure outcomes without
// touching the real /etc/hosts. Production code wires apply.NewRunner — the
// concrete type satisfies applyRunner because its method set already exposes
// Apply with the right signature.

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/hungthai1401/hostie/go/internal/tui/store"
)

// applyRunner is the minimal contract applyCmd needs from apply.Runner.
// Defined here (the consumer) so tests can inject fakes without depending on
// internals of the apply package.
type applyRunner interface {
	Apply(hostsFile domain.HostsFile) (*apply.ApplyResult, error)
}

// ApplyResultMsg is the message yielded by applyCmd once apply.Runner.Apply
// returns. It is dispatched in Update.
//
// Field semantics:
//
//   - Err != nil      → apply pipeline failed. Per D13, ~/.hosts is still on
//                       disk with the mutated content (the failure is on the
//                       /etc/hosts side); the store is also still mutated.
//                       Update surfaces a red status banner with the error
//                       text suffixed by "YAML kept" to remind the operator
//                       the mutation is durable and only the /etc/hosts side
//                       needs recovery.
//   - Err == nil      → ApplyResult is valid.
//   - Changed=true    → /etc/hosts was rewritten. Success banner reads
//                       "Applied (changed)".
//   - Changed=false   → /etc/hosts already matched the render (idempotent
//                       re-apply, or no-op mutation). Success banner reads
//                       "Applied (no changes)".
//
// Message is the human-readable detail returned by apply.Runner (e.g.,
// "/etc/hosts updated successfully" or the dry-run/permission text). It is
// not surfaced verbatim today; we keep it on the message so future status-bar
// detail rows can render it without re-running the apply pipeline.
type ApplyResultMsg struct {
	Changed bool
	Message string
	Err     error
}

// applyCmd returns a tea.Cmd that runs runner.Apply against the supplied
// in-memory HostsFile snapshot on a background goroutine. The Cmd is invoked
// by the Bubble Tea runtime off the UI goroutine, so the runner's blocking
// /etc/hosts work does not stall key handling.
//
// Pass the current store snapshot (m.store.HostsFile() dereferenced) — do not
// pass a pointer that the store may concurrently mutate. The runner takes the
// HostsFile by value so the goroutine operates on its own copy.
//
// runner is nil-safe: a nil runner produces a Cmd that yields an
// ApplyResultMsg carrying a synthetic error. This keeps the failure path
// visible if a code path forgets to construct a runner (instead of panicking
// inside the goroutine).
func applyCmd(runner applyRunner, hostsFile domain.HostsFile) tea.Cmd {
	return func() tea.Msg {
		if runner == nil {
			return ApplyResultMsg{Err: errNilApplyRunner}
		}
		result, err := runner.Apply(hostsFile)
		if err != nil {
			return ApplyResultMsg{Err: err}
		}
		if result == nil {
			// Defensive: a runner that returns (nil, nil) is a contract
			// violation, but surfacing it via the status bar is friendlier
			// than panicking inside the Bubble Tea goroutine.
			return ApplyResultMsg{Err: errNilApplyResult}
		}
		return ApplyResultMsg{
			Changed: result.Changed,
			Message: result.Message,
		}
	}
}

// errNilApplyRunner / errNilApplyResult are returned through ApplyResultMsg
// rather than panicking so the failure is visible in the StatusBar.
var (
	errNilApplyRunner = &applyError{msg: "apply runner not configured"}
	errNilApplyResult = &applyError{msg: "apply runner returned no result"}
)

type applyError struct{ msg string }

func (e *applyError) Error() string { return e.msg }

// handleApplyResult routes an ApplyResultMsg to the StatusBar via the store.
//
// D11/D13 semantics:
//
//   - Success + Changed=true   → green "Applied (changed)".
//   - Success + Changed=false  → green "Applied (no changes)" (idempotent).
//   - Failure                  → red   "Apply failed: <err> — YAML kept".
//
// The trailing "YAML kept" reminds the operator that ~/.hosts has been
// written with the mutation — only the /etc/hosts side needs recovery. Per
// D13 we never roll back the store or the YAML on apply failure.
//
// Apply success also clears the store's dirty flag: with auto-apply, every
// successful apply means the in-memory state is durable on disk and matches
// /etc/hosts, so the dirty marker should not show. On failure the dirty flag
// stays set so the operator sees the unsaved-vs-/etc/hosts mismatch.
func (m Model) handleApplyResult(msg ApplyResultMsg) Model {
	if msg.Err != nil {
		m.store.SetStatusMessage("Apply failed: "+msg.Err.Error()+" — YAML kept", store.StatusError)
		return m
	}
	if msg.Changed {
		m.store.SetStatusMessage("Applied (changed)", store.StatusSuccess)
	} else {
		m.store.SetStatusMessage("Applied (no changes)", store.StatusSuccess)
	}
	m.store.ClearDirty()
	return m
}

// handleReapplyKey is the Ctrl+S handler: re-run the apply pipeline against
// the current store snapshot. Used to recover from a prior auto-apply failure
// (D13) without forcing the operator to make a dummy mutation first.
//
// No-op when the store has no hosts file loaded (Init failed). No-op when an
// apply runner has not been wired (defensive; production always wires one in
// NewModel).
func (m Model) handleReapplyKey() (Model, tea.Cmd) {
	hf := m.store.HostsFile()
	if hf == nil {
		return m, nil
	}
	return m, applyCmdDispatch(m.applyRunner, *hf, m.hostsPath)
}
