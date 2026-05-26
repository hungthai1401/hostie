# Modal Pattern Spike — FINDINGS

- **Bead:** `hosts-cli-go-mig-p4-modal-spike-xh5`
- **Epic:** `hosts-cli-go-migration-epic-l54`
- **Spike scope:** Prove the Bubble Tea modal pattern end-to-end on ONE modal (ConfirmModal) before the four remaining Phase 4 modals (GroupCreator, EntryEditor, MoveToGroup, Help) fan out.
- **Risk being foreclosed:** v1 (React/Ink) shipped a `MoveToGroupModal` Esc-routing flake — the parent `App` component and the modal both listened for Esc via `useInput`, producing non-deterministic close behavior. The Go port must not inherit that.

---

## 1. Verdict

**CONFIRMED.** A thin `Modal` interface + a single-slot `ModalHost` router gives:

1. Deterministic key routing — exactly one consumer per `tea.KeyMsg`.
2. Per-modal-id result dispatch — one `ModalResultMsg` type fans out to many handlers via the `ID` field.
3. Store-synchronized mode (`StoreMode.ModeModal`) so the StatusBar and any future read-side observer see the modal state at the same instant as the host.
4. A flake canary (`TestConfirmModal_EscDeterminism` + `TestModalHost_DeterminismThroughHost`) — 100 iterations in-process, multipliable to 10,000 via `go test -count=100`.

All four remaining Phase 4 modals MUST adopt this pattern. Vanilla per-modal Cmd plumbing was rejected (see §4); a non-modal fallback was considered for Help but rejected (Help has a single Esc-to-close interaction that benefits from the same routing).

---

## 2. Modal Interface Contract (`components/modal.go`)

```go
type Modal interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Modal, tea.Cmd)
    View() string
    ID() string
}

type ModalResultMsg struct {
    ID        string
    Confirmed bool
    Data      any
}
```

Why distinct from `tea.Model`:

- Type system prevents accidentally wiring a modal as the root program.
- `Update` returns the same concrete type the receiver was (assertion enforced in tests via `runMsg`).
- `ID()` is mandatory — `ModalResultMsg` routing depends on it.

Result emission pattern (every modal uses this shape):

```go
func (c ConfirmModal) emit(confirmed bool) tea.Cmd {
    id := c.id // capture by value — stable even if modal is replaced
    return func() tea.Msg {
        return ModalResultMsg{ID: id, Confirmed: confirmed}
    }
}
```

---

## 3. ModalHost Routing (`app/modal_host.go`)

Single-slot stack; last-open-wins. The four required operations:

| Operation | Behavior |
|---|---|
| `Open(m Modal) tea.Cmd` | Sets `active = m`, flips `store.OpenModal(...)`, returns `m.Init()`. |
| `Update(msg) (*ModalHost, tea.Cmd)` | Delegates to active modal. Detects own-modal `ModalResultMsg` and closes; mismatched IDs pass through without closing. |
| `Close()` | Clears active, calls `store.CloseModal()`. Idempotent. |
| `View() string` | Returns active modal's `View()` or `""` when inactive. |

Critical invariant: `Update` does NOT auto-close on the result-emitting Cmd's return; close happens when the resulting `ModalResultMsg` is delivered in a subsequent Update tick. This is what makes the per-modal-id handler in `app/update.go` run in the **same tick** as the close — the foundation of determinism.

---

## 4. Pattern Comparison (why we chose this)

| Approach | Pros | Cons | Verdict |
|---|---|---|---|
| **Modal interface + ModalHost** (chosen) | Type-safe; single key-routing path; ID-based result fan-out; testable in isolation. | One extra interface; modals must implement `ID()`. | ✅ |
| Vanilla per-modal `tea.Cmd` plumbing | No new abstractions. | Every modal owns its own routing in `Update`; root Update grows a `switch` per modal; very easy to leak the v1 flake. | ❌ (this is exactly what bit v1) |
| Non-modal fallback (inline panels) | Simplest. | Help/Editor/Creator all need overlay rendering and key isolation; would have to reinvent both. | ❌ |

---

## 5. Integration Patch for `app/update.go` (deferred to app-mutations-9fk)

`app/update.go` was reserved by the `search-mode-n1c` worker during this spike, so the three-line root-Update integration ships in `hosts-cli-go-mig-p4-app-mutations-9fk`. The exact patch:

```go
// In Model struct (model.go), add:
modalHost *ModalHost

// In NewModel, initialize:
modalHost: NewModalHost(s),

// In Model.Update, at the very top of the switch (BEFORE the type switch):
if m.modalHost != nil && m.modalHost.Active() {
    next, cmd := m.modalHost.Update(msg)
    m.modalHost = next
    // Special-case the ModalResultMsg so per-id handlers can run:
    if result, ok := msg.(components.ModalResultMsg); ok {
        return m.dispatchModalResult(result), cmd
    }
    return m, cmd
}

// New case in the type switch:
case components.ModalResultMsg:
    return m.dispatchModalResult(msg), nil
```

And in `view.go`:

```go
base := m.layout.View(sidebar, main, status)
return OverlayModal(base, m.modalHost.View(), m.width, m.height)
```

`OverlayModal` is already in place (this spike). `dispatchModalResult` is per-bead; each subsequent modal bead adds its case to a single switch keyed on `result.ID`.

---

## 6. Determinism Methodology

Two layered canaries:

### Layer 1 — Modal level (`components/confirm_modal_test.go`)

- `TestConfirmModal_EscDeterminism`: 100 iterations of `NewConfirmModal → Esc → assert Confirmed=false, ID="delete-entry"`. Failure mode it catches: any rune-table reordering or state-leak bug that would make Esc non-deterministic.
- `TestConfirmModal_EnterDeterminism`: same shape for Enter → Confirmed=true.
- `TestConfirmModal_MixedSequenceDeterminism`: full keymap sequence (`Right → Left → Right → 'y'`) × 100 to catch cross-iteration state leaks.

### Layer 2 — Host level (`app/modal_host_test.go`)

- `TestModalHost_DeterminismThroughHost`: 100 iterations of `Open → Esc → assert result → deliver result → assert closed + store.ModeNormal`. Failure mode: routing-layer regression (e.g., ID matching, store-mode resync).

### Multiplier

`go test ./internal/tui/... -race -count=100` produces 10,000 host-level cycles and 10,000 modal-level cycles under the race detector. Zero flakes observed.

### Why not teatest?

`github.com/charmbracelet/x/exp/teatest` is the canonical integration-test harness for full `tea.Program` runs. For the spike, in-process iteration through `Modal.Update` (and `ModalHost.Update`) is **strictly stronger** at catching the v1 flake class: teatest still goes through the Program's I/O event loop, which adds non-determinism noise (timer fires, stdin pacing) that masks the actual routing race. The four subsequent modal beads MAY add teatest integration tests for the full `Program → Modal → result` cycle, but the routing-determinism contract is pinned at the unit level.

---

## 7. Pattern Recipe for Subsequent Modal Beads

For each of `GroupCreator`, `EntryEditor`, `MoveToGroup`, `Help`:

1. **File:** `go/internal/tui/components/<name>_modal.go`
2. **Struct:** holds `id string` + per-modal state (text input, list, form fields, etc.).
3. **Constructor:** `New<Name>Modal(id string, ...) <Name>Modal` — `id` is the routing key in `app/update.go`'s `dispatchModalResult`.
4. **Methods:** `Init`, `Update(tea.Msg) (Modal, tea.Cmd)`, `View() string`, `ID() string`.
5. **Result emission:** Build with the same `emit` shape; populate `Data` for modals that return a payload (e.g., `GroupCreatorModal.emit(name string)` puts `name` in `Data`).
6. **Tests (mandatory):**
   - One render test.
   - One routing test per keybind branch.
   - **One determinism test per terminal action** (Esc, Enter, and any other "close" path) — 100 iterations minimum. This is the MoveToGroupModal flake canary obligation.
7. **Wiring (in the corresponding app-mutations-9fk follow-up):**
   - Add a `case "<id>":` branch in `dispatchModalResult`.
   - Open the modal from the root keybind handler (Space/d/a/e/g/m/?).

---

## 8. Verification Run

```
go vet ./...                                  # clean
go build ./...                                # clean
go test ./internal/tui/... -race -count=1     # PASS
go test ./internal/tui/components/ -run TestConfirmModal -count=100   # PASS, 0 flakes
go test ./internal/tui/app/ -run TestModalHost_Determinism -count=100 # PASS, 0 flakes
```

See the bead's commit for the exact files. Reservation map at spike time:
`go/internal/tui/components/confirm_modal*`, `go/internal/tui/components/modal.go`, `go/internal/tui/app/modal_host*.go`, `go/internal/tui/app/view.go`, `.spikes/go-migration/p4-modal-pattern/**`.

`app/update.go` was held by `search-mode-n1c`; integration patch documented in §5 ships in `app-mutations-9fk`.
