# Spike: P4-Search-Parity — sahilm/fuzzy aggregator vs fuse.js top-5

**Date:** 2026-05-26
**Bead:** `hosts-cli-go-mig-p4-search-spike-khb`
**Phase:** 4 (TUI) · Story 1 (Store + search foundation)
**Risk retired:** HIGH-risk #2 in `docs/go-migration/approach.md` §4 — "Search weighting parity"
**Verdict:** **CONFIRMED** — top-5 byte-equal parity on every (fixture, query) pair, with a documented tie-break rule and a hybrid match strategy.

---

## Risk

`src/tui/hooks/useSearch.ts` (v1) uses fuse.js' Bitap-derived scorer with per-key weights `host:2 / alias:1.5 / ip:1 / group:0.5`, `threshold: 0.3`, `ignoreLocation: true`, `minMatchCharLength: 2`. D8 locks the Go port to `github.com/sahilm/fuzzy`, which is a different algorithm (subsequence-based, no weights, no threshold). If the Go aggregator cannot reproduce v1 top-5 ordering on a representative corpus, **D8 must be amended** before `search/weighted.go` lands (escalate per phase-4 Pivot Signal #2).

---

## Method

1. **Fixture corpus** (`fixtures/corpus.yaml`) — 12 entries across 4 groups (`work/prod`, `work/staging`, `personal`, `dev`), covering hostname dominance, alias-only hits, IP-only hits, group-only hits, and one fuzzy-typo case.
2. **Query set** (`fixtures/queries.json`) — 12 queries spanning exact (`api`, `prod`), prefix (`postgres`), substring across multiple matches (`local`, `dev`), structural prefix (`192.168`, `10.0`), single-alias targets (`kafka`, `minio`), alias-only hits (`redis`), and one typo (`gatway` → matches alias `gateway`).
3. **Baseline capture** (`scripts/build-baseline.ts`) — runs fuse.js in-process with the **exact** config from `src/tui/hooks/useSearch.ts` (keys, weights, threshold, `ignoreLocation`, `minMatchCharLength`, `includeScore: true`), takes `slice(0, 5)`, writes `fuse-baseline.json` keyed by query.
4. **Prototype aggregator** (`aggregator.go`) — sahilm/fuzzy per-field matches, weighted aggregate, sort + tie-break.
5. **Parity test** (`prototype_test.go`) — loads `corpus.yaml` + `fuse-baseline.json`, runs the Go aggregator on each query, asserts `len(want)==len(got)` and `want[i].ID==got[i].Entry.ID` for every position.

```bash
# Reproduce
bun .spikes/go-migration/p4-search-parity/scripts/build-baseline.ts
cd .spikes/go-migration/p4-search-parity && go test -v -count=1 ./...
```

---

## Findings

### 1. Per-field match strategy is hybrid (NOT pure fuzzy)

Naive aggregation — `fuzzy.Find(query, [hostname, alias, ip, group])` on every field — **diverges** from fuse.js on structural fields (IP and group path). Reason:

- fuse.js with `threshold: 0.3` treats `10.0` against `10.99.0.1` as **non-matching** (edit distance over the threshold).
- sahilm/fuzzy treats `10.0` against `10.99.0.1` as **matching** (the chars `1`, `0`, `.`, `0` appear in order).
- Result: pure fuzzy aggregator over-matches on IP/group queries, polluting top-5.

**Resolution:** Use **case-insensitive contiguous substring** for `IP` and `groupPathString` (structural fields), and **sahilm/fuzzy subsequence** for `hostname` and `aliases` (human-typed fields where typo tolerance is desired — verified by query `gatway` → alias `gateway` → `router.local`).

This split is justified by the v1 behavior observed in the baseline: fuse.js' `ignoreLocation: true` + `threshold: 0.3` on IP/group strings degenerates to substring-like behavior in practice on this corpus, while remaining tolerant on hostname/alias.

### 2. Weight table (D8, locked, unchanged)

```
host  × 2.0     fuzzy subsequence (sahilm)
alias × 1.5     fuzzy subsequence (sahilm), max one alias hit per entry
ip    × 1.0     case-insensitive substring
group × 0.5     case-insensitive substring on "/".join(groupPath)
```

No partial scaling, no fuzzy distance multiplier. A field either contributes its full weight or 0. This is the **minimum** aggregator that achieves top-5 parity on the test corpus.

### 3. Tie-break rule (THE deliverable for the impl bead)

> **Tie-break on equal aggregate score = original insertion order, ascending.**
> Insertion order = the order produced by depth-first walk of `HostsFile.Groups` (same walk as v1 `flattenEntriesWithPaths` in `src/tui/hooks/useSearch.ts`).

This is required because virtually every fuse.js result in the baseline has `score = 0.0` (or `0.001`, or `0.068`) — fuse.js returns ties on perfect matches and on `ignoreLocation` matches. fuse.js silently preserves input order on ties; sahilm/fuzzy and Go's `sort.Sort` do not. Using `sort.SliceStable` with `Order ascending` as the second key reproduces v1's observed ordering exactly.

The aggregator stamps `Order: len(out)` at flatten time so the tie-break is deterministic regardless of post-flatten manipulation.

### 4. Filter rule

Entries with `aggregate == 0` are dropped (matches fuse.js behavior of returning only matched items). `len(results)` therefore varies per query (1–5 in this corpus), which the parity test handles.

### 5. Top-5 byte-equal parity result

| Query     | want len | got len | top-5 IDs match |
|-----------|---------:|--------:|-----------------|
| api       | 3        | 3       | ✓ ok            |
| prod      | 3        | 3       | ✓ ok            |
| staging   | 2        | 2       | ✓ ok            |
| postgres  | 2        | 2       | ✓ ok            |
| dev       | 4        | 4       | ✓ ok            |
| local     | 4        | 4       | ✓ ok            |
| 192.168   | 3        | 3       | ✓ ok            |
| 10.0      | 5        | 5       | ✓ ok            |
| kafka     | 1        | 1       | ✓ ok            |
| minio     | 1        | 1       | ✓ ok            |
| redis     | 1        | 1       | ✓ ok            |
| gatway    | 1        | 1       | ✓ ok (typo→alias `gateway`) |

**12 / 12 queries pass.** No tie-break collision, no false positive, no false negative in the top-5 window.

---

## Verdict

**CONFIRMED.** D8 (sahilm/fuzzy as the Go fuzzy lib, host×2 / alias×1.5 / ip×1 / group×0.5 weights) is implementable with **byte-equal top-5 parity** against v1 fuse.js on the spike corpus. **No design.md amendment needed.** Phase 4 Pivot Signal #2 is **dormant**. Phase 4 Story 1 bead `…p4-search-weighted-r1b` is unblocked.

---

## Implementation contract for `go/internal/tui/search/weighted.go`

The impl bead MUST honor:

1. **Flatten** entries with depth-first group walk, stamping insertion order as the tie-break key. Match v1 `flattenEntriesWithPaths` byte-for-byte (sidebar order = search tie-break order).
2. **Per-field match:**
   - `hostname` and each `alias`: `fuzzy.Find(query, []string{field})` non-empty → contribute weight.
   - `ip` and `groupPathString`: `strings.Contains(strings.ToLower(field), strings.ToLower(query))` → contribute weight.
3. **Aggregate:** sum of contributed weights; cap aliases at one hit per entry (alias×1.5, not per-alias).
4. **Filter:** drop entries with aggregate == 0.
5. **Sort:** `sort.SliceStable` — primary key `Score` desc, secondary key `Order` asc.
6. **Result struct** mirrors `Result{Entry, GroupPath, Score, Order}` — `Order` exposed for the assertion-friendly path; UI consumer drops it.
7. **Top-N**: caller-side slice; v1 returns top 10 in `useSearch.ts` (`.slice(0, 10)`). Engine MUST NOT pre-truncate at 5; the spike used 5 because that is the contracted parity surface.
8. **Score-parity tests** in the impl bead MUST port the spike fixtures + baseline JSON verbatim (copy `fixtures/corpus.yaml`, `fixtures/queries.json`, `fuse-baseline.json` under `go/internal/tui/search/testdata/`). The baseline is the regression oracle going forward — regenerate it only if v1 fuse.js config changes (it should not).

---

## Files kept

- `fixtures/corpus.yaml` — 12-entry fixture (`~/.hosts`-schema-compatible)
- `fixtures/queries.json` — 12-query test set (exact, prefix, infix, structural, typo, alias-only)
- `scripts/build-baseline.ts` — Bun script that captures fuse.js top-5 with the exact `useSearch.ts` config; re-runnable for baseline regen
- `fuse-baseline.json` — committed baseline (12 queries × ≤5 hits with id/hostname/score) for the Go test to diff against
- `aggregator.go` — prototype Go aggregator (hybrid fuzzy + substring + weight + tie-break)
- `prototype_test.go` — parity assertion; exits non-zero on any top-5 divergence
- `go.mod` / `go.sum` — pinned to `github.com/sahilm/fuzzy v0.1.1` (same as `go/go.mod`)
- `FINDINGS.md` — this file
