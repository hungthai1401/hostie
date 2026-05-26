package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

// mockHostsFile mirrors the fixture from
// src/tui/hooks/__tests__/useSearch.test.ts.
func mockHostsFile() domain.HostsFile {
	return domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name: "work",
				Entries: []domain.Entry{
					{
						ID:       "entry-1",
						IP:       "10.0.2.10",
						Hostname: "db.prod.work",
						Aliases:  []string{"database", "postgres"},
						Enabled:  true,
					},
					{
						ID:       "entry-2",
						IP:       "10.0.2.11",
						Hostname: "api.prod.work",
						Aliases:  []string{"api-server"},
						Enabled:  true,
					},
				},
				Groups: []domain.Group{
					{
						Name: "staging",
						Entries: []domain.Entry{
							{
								ID:       "entry-3",
								IP:       "10.0.3.10",
								Hostname: "staging-api.work",
								Aliases:  []string{},
								Enabled:  true,
							},
						},
					},
				},
			},
			{
				Name: "personal",
				Entries: []domain.Entry{
					{
						ID:       "entry-4",
						IP:       "192.168.1.100",
						Hostname: "homelab.local",
						Aliases:  []string{"lab"},
						Enabled:  false,
					},
				},
			},
		},
	}
}

func containsHostname(rs []Result, h string) bool {
	for _, r := range rs {
		if r.Entry.Hostname == h {
			return true
		}
	}
	return false
}

func findByHostname(rs []Result, h string) (Result, bool) {
	for _, r := range rs {
		if r.Entry.Hostname == h {
			return r, true
		}
	}
	return Result{}, false
}

// --- Ported v1 useSearch.test.ts cases (parity in spirit, not byte) -----------

func TestEngine_EmptyQueryReturnsNoResults(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	if got := e.Query(""); got != nil {
		t.Fatalf("expected nil for empty query, got %v", got)
	}
	if got := e.Query("   "); got != nil {
		t.Fatalf("expected nil for whitespace query, got %v", got)
	}
}

func TestEngine_SearchByHostname(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("api")
	if len(results) == 0 {
		t.Fatal("expected matches for 'api'")
	}
	if !containsHostname(results, "api.prod.work") {
		t.Error("expected api.prod.work in results")
	}
	if !containsHostname(results, "staging-api.work") {
		t.Error("expected staging-api.work in results")
	}
}

func TestEngine_SearchByIPAddress(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("10.0.2.10")
	if len(results) == 0 {
		t.Fatal("expected matches for IP '10.0.2.10'")
	}
	if results[0].Entry.IP != "10.0.2.10" {
		t.Errorf("expected first IP=10.0.2.10, got %q", results[0].Entry.IP)
	}
	if results[0].Entry.Hostname != "db.prod.work" {
		t.Errorf("expected first hostname db.prod.work, got %q", results[0].Entry.Hostname)
	}
}

func TestEngine_SearchByAlias(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("postgres")
	if len(results) == 0 {
		t.Fatal("expected matches for alias 'postgres'")
	}
	found := false
	for _, a := range results[0].Entry.Aliases {
		if a == "postgres" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected first result aliases to contain 'postgres', got %v", results[0].Entry.Aliases)
	}
}

func TestEngine_SearchByGroupPath(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("staging")
	if len(results) == 0 {
		t.Fatal("expected matches for 'staging'")
	}
	r, ok := findByHostname(results, "staging-api.work")
	if !ok {
		t.Fatal("expected staging-api.work in results")
	}
	want := []string{"work", "staging"}
	if len(r.GroupPath) != len(want) || r.GroupPath[0] != want[0] || r.GroupPath[1] != want[1] {
		t.Errorf("expected groupPath %v, got %v", want, r.GroupPath)
	}
}

func TestEngine_FuzzyMatchingWithTypos(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("homelb")
	if len(results) == 0 {
		t.Fatal("expected fuzzy match for 'homelb'")
	}
	if results[0].Entry.Hostname != "homelab.local" {
		t.Errorf("expected first hostname homelab.local, got %q", results[0].Entry.Hostname)
	}
}

func TestEngine_ResultsSortedByRelevance(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("api")
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// All top hits should themselves contain "api" in hostname.
	first := results[0].Entry.Hostname
	if !containsLower(first, "api") {
		t.Errorf("expected first hostname to contain 'api', got %q", first)
	}
}

func TestEngine_TopNLimits(t *testing.T) {
	entries := make([]domain.Entry, 15)
	for i := 0; i < 15; i++ {
		entries[i] = domain.Entry{
			ID:       "entry-" + itoa(i),
			IP:       "10.0.0." + itoa(i),
			Hostname: "server" + itoa(i) + ".test",
			Aliases:  []string{},
			Enabled:  true,
		}
	}
	large := domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "test", Entries: entries},
		},
	}
	e := NewEngine(large.Groups)
	results := TopN(e.Query("server"), 10)
	if len(results) > 10 {
		t.Errorf("expected ≤10 results, got %d", len(results))
	}
	// TopN with n >= len returns full slice
	all := e.Query("server")
	if got := TopN(all, 100); len(got) != len(all) {
		t.Errorf("TopN(all, 100) should equal all (len=%d), got len=%d", len(all), len(got))
	}
	if got := TopN(all, -1); len(got) != 0 {
		t.Errorf("TopN(all, -1) should be empty, got len=%d", len(got))
	}
}

func TestEngine_IncludesGroupPathInResults(t *testing.T) {
	e := NewEngine(mockHostsFile().Groups)
	results := e.Query("staging-api")
	if len(results) == 0 {
		t.Fatal("expected results for 'staging-api'")
	}
	gp := results[0].GroupPath
	if len(gp) != 2 || gp[0] != "work" || gp[1] != "staging" {
		t.Errorf("expected groupPath [work staging], got %v", gp)
	}
}

func TestFlatten_NestedGroups(t *testing.T) {
	flat := Flatten(mockHostsFile().Groups)
	if len(flat) != 4 {
		t.Fatalf("expected 4 flattened entries, got %d", len(flat))
	}
	// Order: work entries (2), work/staging entry (1), personal entry (1).
	wantHostnames := []string{"db.prod.work", "api.prod.work", "staging-api.work", "homelab.local"}
	for i, want := range wantHostnames {
		if flat[i].Entry.Hostname != want {
			t.Errorf("flat[%d]: expected hostname %q, got %q", i, want, flat[i].Entry.Hostname)
		}
		if flat[i].Order != i {
			t.Errorf("flat[%d]: expected Order=%d, got %d", i, i, flat[i].Order)
		}
	}
	// staging entry has correct group path
	if got := flat[2].GroupPath; len(got) != 2 || got[0] != "work" || got[1] != "staging" {
		t.Errorf("flat[2].GroupPath: expected [work staging], got %v", got)
	}
}

// TestEngine_TieBreakInsertionOrder verifies the locked tie-break rule: when
// aggregate scores are equal, results MUST come out in flatten / insertion
// order. This is the spike-ratified contract behind score-parity.
func TestEngine_TieBreakInsertionOrder(t *testing.T) {
	// Three entries, all match hostname "node" → all score 2.0 → tie.
	hf := domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name: "g",
				Entries: []domain.Entry{
					{ID: "a", IP: "1.1.1.1", Hostname: "node-a", Enabled: true},
					{ID: "b", IP: "2.2.2.2", Hostname: "node-b", Enabled: true},
					{ID: "c", IP: "3.3.3.3", Hostname: "node-c", Enabled: true},
				},
			},
		},
	}
	e := NewEngine(hf.Groups)
	results := e.Query("node")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	wantIDs := []string{"a", "b", "c"}
	for i, want := range wantIDs {
		if results[i].Entry.ID != want {
			t.Errorf("results[%d]: expected ID %q, got %q (tie-break broken)", i, want, results[i].Entry.ID)
		}
		if results[i].Score != 2.0 {
			t.Errorf("results[%d]: expected score 2.0, got %v", i, results[i].Score)
		}
	}
}

// --- Score-parity tests against fuse.js baseline (12 queries × ≤5 hits) ------

type baselineHit struct {
	ID       string   `json:"id"`
	Hostname string   `json:"hostname"`
	Score    *float64 `json:"score"`
}

func loadParityCorpus(t *testing.T) domain.HostsFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus.yaml"))
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var hf domain.HostsFile
	if err := yaml.Unmarshal(b, &hf); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}
	return hf
}

func loadParityBaseline(t *testing.T) map[string][]baselineHit {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "fuse-baseline.json"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var m map[string][]baselineHit
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	return m
}

func loadParityQueries(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "queries.json"))
	if err != nil {
		t.Fatalf("read queries: %v", err)
	}
	var qs []string
	if err := json.Unmarshal(b, &qs); err != nil {
		t.Fatalf("parse queries: %v", err)
	}
	return qs
}

// TestScoreParityTop5 is the spike regression oracle: for every query in the
// 12-query fixture, the Go engine's top-5 results must be byte-equal (by ID,
// in order) to the fuse.js baseline. If this test ever fails, either v1
// fuse.js config changed (regenerate fuse-baseline.json) or the aggregator
// drifted from D8 (FIX the aggregator, don't regenerate the baseline).
func TestScoreParityTop5(t *testing.T) {
	hf := loadParityCorpus(t)
	baseline := loadParityBaseline(t)
	queries := loadParityQueries(t)

	e := NewEngine(hf.Groups)

	var failed []string
	for _, q := range queries {
		want := baseline[q]
		got := TopN(e.Query(q), 5)

		ok := len(want) == len(got)
		if ok {
			for i := range want {
				if want[i].ID != got[i].Entry.ID {
					ok = false
					break
				}
			}
		}
		if !ok {
			failed = append(failed, q)
			t.Logf("DIFF on q=%q: want=%d got=%d", q, len(want), len(got))
			for i := 0; i < maxInt(len(want), len(got)); i++ {
				var w, g string
				if i < len(want) {
					w = want[i].ID
				}
				if i < len(got) {
					g = got[i].Entry.ID
				}
				mark := "ok"
				if w != g {
					mark = "DIFF"
				}
				t.Logf("  [%d] want=%-32s got=%-32s %s", i, w, g, mark)
			}
		}
	}
	if len(failed) > 0 {
		t.Fatalf("top-5 divergence on %d/%d queries: %v", len(failed), len(queries), failed)
	}
}

// --- helpers -----------------------------------------------------------------

func containsLower(s, sub string) bool {
	// inlined to avoid importing strings in tests file (we already do)
	return len(sub) == 0 || (len(s) >= len(sub) && indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	// minimal case-insensitive index for ASCII; tests use ASCII fixtures.
	ls := toLowerASCII(s)
	lp := toLowerASCII(sub)
	for i := 0; i+len(lp) <= len(ls); i++ {
		if ls[i:i+len(lp)] == lp {
			return i
		}
	}
	return -1
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
