package yaml

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFixtureRoundTrip verifies that v1 YAML fixtures achieve fixed-point round-trip:
// Unmarshal → Marshal → Unmarshal MUST produce byte-equal second Marshal AND deep-equal HostsFile.
// This is Pivot Signal #1: any fixture failure escalates to schema diagnostic.
func TestFixtureRoundTrip(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "..", "test", "fixtures", "hosts")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("failed to read fixtures directory %s: %v", fixturesDir, err)
	}

	fixtures := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			fixtures = append(fixtures, entry.Name())
		}
	}

	if len(fixtures) < 3 {
		t.Fatalf("expected at least 3 YAML fixtures in %s, found %d", fixturesDir, len(fixtures))
	}

	for _, fixtureName := range fixtures {
		fixtureName := fixtureName
		t.Run(fixtureName, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, fixtureName)
			originalBytes, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", fixturePath, err)
			}

			// Round 1: Unmarshal original
			hf1, err := Unmarshal(originalBytes)
			if err != nil {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed Unmarshal (round 1): %v\nOriginal YAML:\n%s",
					fixtureName, err, string(originalBytes))
			}

			// Round 1: Marshal
			marshaled1, err := Marshal(hf1)
			if err != nil {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed Marshal (round 1): %v\nHostsFile: %#v",
					fixtureName, err, hf1)
			}

			// Round 2: Unmarshal marshaled output
			hf2, err := Unmarshal(marshaled1)
			if err != nil {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed Unmarshal (round 2): %v\nMarshaled YAML:\n%s",
					fixtureName, err, string(marshaled1))
			}

			// Round 2: Marshal again
			marshaled2, err := Marshal(hf2)
			if err != nil {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed Marshal (round 2): %v\nHostsFile: %#v",
					fixtureName, err, hf2)
			}

			// Verify byte-equal Marshal output (fixed-point)
			if string(marshaled1) != string(marshaled2) {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed byte-equal Marshal fixed-point check\nRound 1 Marshal:\n%s\n\nRound 2 Marshal:\n%s",
					fixtureName, string(marshaled1), string(marshaled2))
			}

			// Verify deep-equal HostsFile across rounds
			if !reflect.DeepEqual(hf1, hf2) {
				t.Fatalf("[PIVOT SIGNAL #1] fixture %s failed deep-equal HostsFile check\nRound 1: %#v\n\nRound 2: %#v",
					fixtureName, hf1, hf2)
			}

			t.Logf("✓ fixture %s: round-trip fixed-point verified (%d bytes)", fixtureName, len(marshaled2))
		})
	}
}
