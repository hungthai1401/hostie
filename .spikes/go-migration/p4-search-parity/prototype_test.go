package parity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type baselineHit struct {
	ID       string   `json:"id"`
	Hostname string   `json:"hostname"`
	Score    *float64 `json:"score"`
}

func loadCorpus(t *testing.T) HostsFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures", "corpus.yaml"))
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var hf HostsFile
	if err := yaml.Unmarshal(b, &hf); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}
	return hf
}

func loadBaseline(t *testing.T) map[string][]baselineHit {
	t.Helper()
	b, err := os.ReadFile("fuse-baseline.json")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var m map[string][]baselineHit
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	return m
}

func loadQueries(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures", "queries.json"))
	if err != nil {
		t.Fatalf("read queries: %v", err)
	}
	var qs []string
	if err := json.Unmarshal(b, &qs); err != nil {
		t.Fatalf("parse queries: %v", err)
	}
	return qs
}

func TestScoreParityTop5(t *testing.T) {
	hf := loadCorpus(t)
	items := Flatten(hf.Groups)
	baseline := loadBaseline(t)
	queries := loadQueries(t)

	var failed []string
	for _, q := range queries {
		want := baseline[q]
		got := TopN(Search(items, q), 5)

		t.Logf("Q=%q  want=%d got=%d", q, len(want), len(got))
		for i := range max(len(want), len(got)) {
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
		}
	}

	if len(failed) > 0 {
		t.Fatalf("top-5 divergence on %d/%d queries: %v", len(failed), len(queries), failed)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
