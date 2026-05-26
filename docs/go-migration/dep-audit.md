# Transitive Dependency Audit — `github.com/atotto/clipboard`

**Bead:** `hosts-cli-go-mig-p2-clipboard-audit-fn0`
**Phase:** 2 / Story S6 (Fileio)
**Date:** 2026-05-25
**Auditor:** CopperFox
**Status:** RESOLVED — TUI-gated, acceptable for Phase 2; flagged for Phase 4 TUI port.

---

## 1. Background

Phase-1 review (D8 allowed-deps audit) surfaced a surprise: the binary links
`github.com/atotto/clipboard` v0.1.4, which is not in design.md's D8 allow-list
and not modeled in D12's threat model. `atotto/clipboard` shells out to
platform binaries (`pbcopy` / `xclip` / `wl-copy` / `clip.exe`) to read/write
the system clipboard. If reachable from `go/internal/core/` or
`go/internal/domain/` it would represent an unmodeled exec sink and would
contradict D8.

This audit resolves **Phase-2 Pivot Signal #5**: determine whether the dep is
reachable from non-TUI code (escalate → design.md amendment) or strictly
gated behind the Bubble Tea TUI (record + close).

## 2. Dependency Chain (verbatim `go mod why`)

```
$ cd go && go mod why -m github.com/atotto/clipboard
# github.com/atotto/clipboard
github.com/hungthai1401/hostie/go/cmd/hostie
github.com/charmbracelet/bubbles/textinput
github.com/atotto/clipboard
```

```
$ cd go && go mod graph | rg atotto/clipboard
github.com/hungthai1401/hostie/go github.com/atotto/clipboard@v0.1.4
github.com/charmbracelet/bubbles@v0.21.0 github.com/atotto/clipboard@v0.1.4
```

```
$ cd go && rg -n 'atotto/clipboard' go/
go/go.mod:16:	github.com/atotto/clipboard v0.1.4 // indirect
go/go.sum:1:github.com/atotto/clipboard v0.1.4 h1:...
go/go.sum:2:github.com/atotto/clipboard v0.1.4/go.mod h1:...
```

Zero direct imports under `go/cmd/` or `go/internal/`. The dep is pulled in
**only** because `cmd/hostie/main.go` keeps a dep-usage proof
(`var _ = textinput.New`) so the Phase-1 size measurement reflects the full
TUI surface. `bubbles/textinput` in turn imports `atotto/clipboard` for its
paste handler.

## 3. Reachability Analysis

```
$ cd go && go list -deps ./internal/... 2>/dev/null | rg clipboard
(no output)

$ cd go && go list -deps ./internal/core/... 2>/dev/null | rg clipboard
(no output)

$ cd go && go list -deps ./internal/domain/... 2>/dev/null | rg clipboard
(no output)

$ cd go && for pkg in $(go list ./...); do
    if go list -deps "$pkg" 2>/dev/null | rg -q '^github.com/atotto/clipboard$'; then
      echo "$pkg"
    fi
  done
github.com/hungthai1401/hostie/go/cmd/hostie
```

| Package                                  | Links `atotto/clipboard`? | Path                                                  |
| ---------------------------------------- | ------------------------- | ----------------------------------------------------- |
| `go/cmd/hostie` (main binary)            | **YES**                   | `main.go` → `bubbles/textinput` → `atotto/clipboard`  |
| `go/internal/...` (all internal pkgs)    | NO                        | —                                                     |
| `go/internal/core/...` (when introduced) | NO                        | —                                                     |
| `go/internal/domain/...` (when introduced)| NO                       | —                                                     |

Only **one** package — the `main` package — transitively links the clipboard
dep, and only through the TUI textinput component. No `internal/` package
links it, and `internal/core/` / `internal/domain/` directories are currently
empty (Phase 2 has not yet populated them); future code there will not pull
clipboard in unless someone imports `bubbles/textinput` from core, which D11
(layering) already forbids.

## 4. Verdict

**TUI-gated. Acceptable for Phase 2.** The dep is reachable only from
`cmd/hostie` through `bubbles/textinput`, which is a Phase-4 TUI dependency
intentionally referenced now via dep-usage proofs so Phase-1 binary-size
measurements are realistic. No core/domain/cli logic touches clipboard, no
exec sink is reachable from Phase-2 CLI commands, and the D12 threat model
remains intact for Phase 2 scope.

No design.md amendment required at this time. Pivot Signal #5 is **NOT
triggered**.

## 5. Mitigation & Phase-4 Follow-up

When Phase 4 lands the real TUI port, the clipboard sink becomes user-facing.
Three mitigations are pre-recorded here so the Phase-4 planner inherits the
context:

1. **Document upstream behavior.** `atotto/clipboard` is a no-op (returns
   `ErrUnsupported`) when no platform helper is available (e.g. headless
   Linux without `xclip`/`wl-copy`/`DISPLAY`, sandboxed CI). It does **not**
   raise a security exception by default; it silently fails. Phase-4 TUI
   tests must not assume clipboard works in CI.

2. **Consider a clipboard-free textinput.** If D12 threat model expands to
   exclude exec sinks entirely, evaluate replacing `bubbles/textinput` with
   a minimal text-input model that does not import `atotto/clipboard`
   (either a fork with paste disabled, or a hand-rolled input model on top
   of Bubble Tea key events). This keeps the TUI surface but drops the exec
   path.

3. **Add D8/D12 amendment to the Phase-4 entry contract.** Phase 4 should
   explicitly add `github.com/atotto/clipboard` to D8 (allowed deps,
   TUI-only) and update D12 to model paste/copy as a user-initiated exec to
   `pbcopy`/`xclip`/`wl-copy`/`clip.exe` with no untrusted-data flow.

## 6. Reference

This audit applies the critical pattern recorded in
`docs/learnings/critical-patterns.md`:

> **Three-Level Artifact Verification Extends to CI Jobs and Dependency
> Imports** — verify (a) the dep is declared, (b) the dep is substantively
> used (not just declared), (c) the dep is wired through a code path that is
> actually reachable from the intended entry points. Phase-1's review found
> `atotto/clipboard` at level (a) but did not check (c); this audit closes
> the gap by mapping reachability with `go list -deps` per-package, not just
> `go mod why`.

## 7. Verification Performed

- `cd go && go vet ./...` → "Go vet: No issues found" (no production code
  changed).
- No files under `go/` modified by this bead; only this audit doc was
  written.
