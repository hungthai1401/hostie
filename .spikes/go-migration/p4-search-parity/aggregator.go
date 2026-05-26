package parity

// Score-parity prototype: aggregator over sahilm/fuzzy that targets byte-equal
// top-5 ordering against fuse.js (see fuse-baseline.json in spike root).
//
// Design (per D8 + spike findings):
//   - hostname/alias fields: fuzzy match via sahilm/fuzzy (subsequence match,
//     tolerant of typos like "gatway"→"gateway").
//   - ip/group fields: case-insensitive substring (contiguous). Structural
//     fields where fuse.js with threshold 0.3 rejects 10.0 ↔ 10.99.0.1 even
//     though it is a fuzzy subsequence match; substring matches fuse here.
//   - Aggregate score = host×2 + (anyAlias?1.5:0) + ip×1 + group×0.5.
//   - Tie-break = original insertion order (flattened group walk, same as
//     fuse.js item ordering on tied scores).
//   - Filter: drop entries with aggregate == 0.

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

type Entry struct {
	ID       string   `yaml:"id"`
	IP       string   `yaml:"ip"`
	Hostname string   `yaml:"hostname"`
	Aliases  []string `yaml:"aliases"`
	Enabled  bool     `yaml:"enabled"`
}

type Group struct {
	Name    string  `yaml:"name"`
	Entries []Entry `yaml:"entries"`
	Groups  []Group `yaml:"groups"`
}

type HostsFile struct {
	Version int     `yaml:"version"`
	Groups  []Group `yaml:"groups"`
}

type Searchable struct {
	Entry     Entry
	GroupPath []string
	Order     int // insertion order, tie-break key
}

// Flatten walks groups in the same order as v1 useSearch.ts/flattenEntriesWithPaths.
func Flatten(groups []Group) []Searchable {
	var out []Searchable
	var walk func(gs []Group, parent []string)
	walk = func(gs []Group, parent []string) {
		for _, g := range gs {
			path := append([]string{}, parent...)
			path = append(path, g.Name)
			for _, e := range g.Entries {
				out = append(out, Searchable{Entry: e, GroupPath: path, Order: len(out)})
			}
			walk(g.Groups, path)
		}
	}
	walk(groups, nil)
	return out
}

// fuzzyMatches returns true if `query` matches `target` as a fuzzy subsequence
// (case-insensitive). Empty query never matches a field.
func fuzzyMatches(query, target string) bool {
	if query == "" || target == "" {
		return false
	}
	// fuzzy.Find requires a slice; we supply a single-element list.
	matches := fuzzy.Find(query, []string{target})
	return len(matches) > 0
}

// substringMatches returns true on a case-insensitive contiguous substring.
func substringMatches(query, target string) bool {
	if query == "" || target == "" {
		return false
	}
	return strings.Contains(strings.ToLower(target), strings.ToLower(query))
}

type Result struct {
	Entry     Entry
	GroupPath []string
	Score     float64
	Order     int
}

// Search runs the weighted aggregator over the flattened corpus.
// Returns results sorted by aggregate score desc, tie-broken by Order asc.
func Search(items []Searchable, query string) []Result {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	var out []Result
	for _, it := range items {
		var score float64
		if fuzzyMatches(query, it.Entry.Hostname) {
			score += 2.0
		}
		aliasHit := false
		for _, a := range it.Entry.Aliases {
			if fuzzyMatches(query, a) {
				aliasHit = true
				break
			}
		}
		if aliasHit {
			score += 1.5
		}
		if substringMatches(query, it.Entry.IP) {
			score += 1.0
		}
		groupPathStr := strings.Join(it.GroupPath, "/")
		if substringMatches(query, groupPathStr) {
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

// TopN returns up to the first n results.
func TopN(results []Result, n int) []Result {
	if len(results) <= n {
		return results
	}
	return results[:n]
}
