// Package search implements the TUI weighted search engine.
//
// This is the production port of v1 src/tui/hooks/useSearch.ts, productionized
// from the .spikes/go-migration/p4-search-parity prototype after the search
// parity spike CONFIRMED top-5 byte-equal parity with fuse.js across the spike
// fixture corpus.
//
// Design (locked decision D8 in docs/go-migration/design.md):
//   - hostname / alias fields → fuzzy subsequence via github.com/sahilm/fuzzy
//     (typo tolerant: "gatway" → "gateway").
//   - ip / group-path fields → case-insensitive contiguous substring (matches
//     fuse.js threshold:0.3 + ignoreLocation:true behavior on structural data).
//   - Weights: host × 2.0, alias × 1.5 (capped at one hit per entry),
//     ip × 1.0, group × 0.5.
//   - Tie-break: original insertion order (depth-first walk of
//     HostsFile.Groups, identical to v1 flattenEntriesWithPaths).
//   - Filter: entries with aggregate score == 0 are dropped.
//
// See .spikes/go-migration/p4-search-parity/FINDINGS.md for the parity report
// that ratified this design.
package search

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// Searchable is a flattened view of an Entry with its group path and the
// insertion-order index used as the secondary sort key.
type Searchable struct {
	Entry     domain.Entry
	GroupPath []string
	Order     int // depth-first insertion order, tie-break key
}

// Result is a single search hit. Order is exposed so callers (and tests) can
// reason about tie-break behavior; UI consumers may ignore it.
type Result struct {
	Entry     domain.Entry
	GroupPath []string
	Score     float64
	Order     int
}

// Engine is the production weighted-search engine. Construct with NewEngine
// over the current HostsFile; call Query for each user input. Engine is
// immutable — rebuild it whenever the underlying hosts file changes.
type Engine struct {
	items []Searchable
}

// NewEngine flattens the given groups depth-first and returns an Engine ready
// for queries. Insertion order is stamped at flatten time and is the sole
// tie-break key — callers MUST rebuild the engine after any mutation that
// could reorder entries.
func NewEngine(groups []domain.Group) *Engine {
	return &Engine{items: Flatten(groups)}
}

// Flatten walks groups depth-first in the same order as v1
// flattenEntriesWithPaths in src/tui/hooks/useSearch.ts. The Order field on
// each returned Searchable is the index assigned during the walk.
func Flatten(groups []domain.Group) []Searchable {
	var out []Searchable
	var walk func(gs []domain.Group, parent []string)
	walk = func(gs []domain.Group, parent []string) {
		for _, g := range gs {
			path := append([]string{}, parent...)
			path = append(path, g.Name)
			for _, e := range g.Entries {
				out = append(out, Searchable{
					Entry:     e,
					GroupPath: path,
					Order:     len(out),
				})
			}
			walk(g.Groups, path)
		}
	}
	walk(groups, nil)
	return out
}

// Query runs the weighted aggregator and returns matching results sorted by
// aggregate score descending, tie-broken by insertion order ascending. An
// empty / whitespace-only query returns nil (matches v1 behavior).
//
// The engine does NOT pre-truncate results; v1 returns the top 10 in
// useSearch.ts via .slice(0, 10). Callers should slice as needed (see TopN).
func (e *Engine) Query(q string) []Result {
	if strings.TrimSpace(q) == "" {
		return nil
	}
	var out []Result
	for _, it := range e.items {
		var score float64
		if fuzzyMatches(q, it.Entry.Hostname) {
			score += 2.0
		}
		aliasHit := false
		for _, a := range it.Entry.Aliases {
			if fuzzyMatches(q, a) {
				aliasHit = true
				break
			}
		}
		if aliasHit {
			score += 1.5
		}
		if substringMatches(q, it.Entry.IP) {
			score += 1.0
		}
		groupPathStr := strings.Join(it.GroupPath, "/")
		if substringMatches(q, groupPathStr) {
			score += 0.5
		}
		if score > 0 {
			out = append(out, Result{
				Entry:     it.Entry,
				GroupPath: it.GroupPath,
				Score:     score,
				Order:     it.Order,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Order < out[j].Order
	})
	return out
}

// TopN returns up to the first n results. Convenience for the UI consumer; v1
// uses n=10.
func TopN(results []Result, n int) []Result {
	if n < 0 {
		n = 0
	}
	if len(results) <= n {
		return results
	}
	return results[:n]
}

// fuzzyMatches reports whether query matches target as a fuzzy subsequence
// (case-insensitive). Empty query or empty target never matches.
func fuzzyMatches(query, target string) bool {
	if query == "" || target == "" {
		return false
	}
	matches := fuzzy.Find(query, []string{target})
	return len(matches) > 0
}

// substringMatches reports whether query appears as a case-insensitive
// contiguous substring of target.
func substringMatches(query, target string) bool {
	if query == "" || target == "" {
		return false
	}
	return strings.Contains(strings.ToLower(target), strings.ToLower(query))
}
