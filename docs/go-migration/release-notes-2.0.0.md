# Hostie 2.0.0 — Release Notes (DRAFT)

This document tracks user-visible changes introduced by the Go rewrite
(`feature/go-migration`). It is a living draft; sections land as their owning
beads close.

## Breaking: /etc/hosts managed-block formatting

- **v1** emitted a blank line immediately after `# BEGIN HOSTIE` and immediately
  before `# END HOSTIE` whenever the `apply --dry-run` preview path was used
  (`src/core/render.ts:wrapManagedBlock`). The actual on-disk write path
  (`src/core/apply.ts:renderManagedBlock`) emitted **no** padding. The two
  shapes disagreed.
- **v2** emits the marker block without surrounding blank lines on **all**
  code paths (dry-run preview and on-disk write produce identical bytes).
  This collapses the v1 divergence between `wrapManagedBlock`
  (`src/core/render.ts`) and `renderManagedBlock` (`src/core/apply.ts`) into
  one canonical shape, enforcing the "One Renderer, One Parser — Share or Pin"
  critical pattern structurally.
- **Impact:** the first `hostie apply` under v2 against a v1-managed
  `/etc/hosts` will produce a diff equal to two removed blank lines (one
  immediately after `# BEGIN HOSTIE`, one immediately before `# END HOSTIE`).
  Operationally harmless — `/etc/hosts` is whitespace-tolerant per the resolver
  contract. The diff is shown by `hostie apply --dry-run` before any write,
  so users see it coming.
- **Migration path:** none required. Run `hostie apply` once after upgrading
  to reconcile the marker-block shape; subsequent applies are idempotent.
