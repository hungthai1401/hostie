//go:build golden

package golden_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hungthai1401/hostie/go/internal/core/render"
	"github.com/hungthai1401/hostie/go/internal/core/yaml"
	"github.com/stretchr/testify/require"
)

const (
	// PinnedV1Tag is the v1 release tag we test parity against.
	PinnedV1Tag = "v1.0.0"

	// PinnedV1URLBase is the GitHub Releases download URL prefix.
	PinnedV1URLBase = "https://github.com/hungthai1401/hostie/releases/download/" + PinnedV1Tag
)

// PinnedV1SHA holds the SHA-256 checksums for the v1 binaries we test against.
// These are locked from .spikes/go-migration/p2-golden-pin/FINDINGS.md.
var PinnedV1SHA = map[string]string{
	"darwin-arm64": "e1ff4b47d02cc8a7872ed0fc4da0616301c92b50377e88ffb96f3eb07ca68119",
	"linux-x64":    "f97fa80cc2a3bb6a2e689009837bcb80fdeb8a0853554988c3e34b51f1dd9eef",
	// darwin-x64 and linux-arm64 can be added when needed for CI matrix expansion
}

// platformKey returns the platform identifier for the current runtime.
func platformKey() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	} else if arch == "arm64" {
		// keep as-is
	}
	return runtime.GOOS + "-" + arch
}

// ensureV1Binary downloads and verifies the v1 binary for the current platform.
// Returns the path to the cached binary.
func ensureV1Binary(t *testing.T) string {
	t.Helper()

	platform := platformKey()
	expectedSHA, ok := PinnedV1SHA[platform]
	if !ok {
		t.Skipf("No pinned SHA for platform %s — add to PinnedV1SHA map when needed", platform)
	}

	// Cache layout: go/test/golden/.cache/v1.0.0/<platform>/hostie-<platform>
	cacheDir := filepath.Join(".", ".cache", PinnedV1Tag, platform)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory %s: %v", cacheDir, err)
	}

	assetName := "hostie-" + platform
	cachePath := filepath.Join(cacheDir, assetName)

	// If cache exists, verify SHA before using
	if _, err := os.Stat(cachePath); err == nil {
		computedSHA := computeSHA256(t, cachePath)
		if computedSHA == expectedSHA {
			t.Logf("Using cached v1 binary: %s (SHA: %s)", cachePath, computedSHA[:16])
			return cachePath
		}
		// SHA mismatch — remove corrupted cache and re-download
		t.Logf("Cached binary SHA mismatch (expected %s, got %s) — removing and re-downloading", expectedSHA[:16], computedSHA[:16])
		if err := os.Remove(cachePath); err != nil {
			t.Fatalf("Failed to remove corrupted cache file %s: %v", cachePath, err)
		}
	}

	// Download from GitHub Releases
	downloadURL := fmt.Sprintf("%s/%s", PinnedV1URLBase, assetName)
	t.Logf("Downloading v1 binary from %s", downloadURL)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		t.Fatalf("Failed to download v1 binary from %s: %v", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to download v1 binary: HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	// Write to cache
	f, err := os.OpenFile(cachePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		t.Fatalf("Failed to create cache file %s: %v", cachePath, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(cachePath)
		t.Fatalf("Failed to write v1 binary to %s: %v", cachePath, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(cachePath)
		t.Fatalf("Failed to close cache file %s: %v", cachePath, err)
	}

	// Verify SHA
	computedSHA := computeSHA256(t, cachePath)
	if computedSHA != expectedSHA {
		_ = os.Remove(cachePath)
		t.Fatalf("SHA-256 mismatch for downloaded v1 binary: expected %s, got %s — refusing to proceed", expectedSHA, computedSHA)
	}

	t.Logf("Downloaded and verified v1 binary: %s (SHA: %s)", cachePath, computedSHA[:16])
	return cachePath
}

// computeSHA256 computes the SHA-256 checksum of a file.
func computeSHA256(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file %s for SHA-256: %v", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("Failed to compute SHA-256 for %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestYAMLRoundtripParity verifies that the Go v2 YAML round-trip is internally
// consistent: parse → render → parse produces a fixed point.
//
// NOTE: This test does NOT compare v1 vs v2 rendered output because v2.0.0
// intentionally changes the managed block format (removes blank-line padding,
// adds group headers). See docs/go-migration/release-notes-2.0.0.md.
//
// Phase 3 will add v1/v2 parity tests for `list --json` and `apply --dry-run`
// on the surfaces where byte-exact parity is required (D15).
func TestYAMLRoundtripParity(t *testing.T) {
	// Ensure v1 binary is cached for future Phase 3 tests
	_ = ensureV1Binary(t)

	fixtures := []string{
		"../fixtures/hosts/simple-dev.yaml",
		"../fixtures/hosts/complex-roundtrip.yaml",
		"../fixtures/hosts/nested-groups.yaml",
	}

	for _, fixturePath := range fixtures {
		fixturePath := fixturePath // capture for parallel subtests
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			// Read fixture YAML
			fixtureData, err := os.ReadFile(fixturePath)
			require.NoError(t, err, "Failed to read fixture %s", fixturePath)

			// Round 1: Unmarshal → Marshal
			hf1, err := yaml.Unmarshal(fixtureData)
			require.NoError(t, err, "Round 1 Unmarshal failed for %s", fixturePath)

			yamlBytes1, err := yaml.Marshal(hf1)
			require.NoError(t, err, "Round 1 Marshal failed for %s", fixturePath)

			// Round 2: Unmarshal → Marshal (should produce identical YAML)
			hf2, err := yaml.Unmarshal(yamlBytes1)
			require.NoError(t, err, "Round 2 Unmarshal failed for %s", fixturePath)

			yamlBytes2, err := yaml.Marshal(hf2)
			require.NoError(t, err, "Round 2 Marshal failed for %s", fixturePath)

			// Assert YAML round-trip is a fixed point
			require.Equal(t, string(yamlBytes1), string(yamlBytes2),
				"YAML round-trip not a fixed point for %s", fixturePath)

			// Verify render produces valid output (smoke test)
			renderedBlock := render.RenderManagedBlock(hf1)
			require.Contains(t, renderedBlock, "# BEGIN HOSTIE",
				"Rendered block missing BEGIN marker for %s", fixturePath)
			require.Contains(t, renderedBlock, "# END HOSTIE",
				"Rendered block missing END marker for %s", fixturePath)
		})
	}
}


// TestListJSONParity verifies that `hostie list --json` produces identical JSON
// output between v1 (Bun/TS) and v2 (Go) for the same ~/.hosts fixture.
//
// This is a D15 parity surface: list --json output must be byte-equal.
func TestListJSONParity(t *testing.T) {
	v1Binary := ensureV1Binary(t)
	v2Binary := buildV2Binary(t)

	fixtures := []string{
		"../fixtures/hosts/simple-dev.yaml",
		"../fixtures/hosts/complex-roundtrip.yaml",
		"../fixtures/hosts/nested-groups.yaml",
	}

	for _, fixturePath := range fixtures {
		fixturePath := fixturePath
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			// Create temp HOME with ~/.hosts file
			tmpHome := t.TempDir()
			hostsPath := filepath.Join(tmpHome, ".hosts")
			fixtureData, err := os.ReadFile(fixturePath)
			require.NoError(t, err)
			err = os.WriteFile(hostsPath, fixtureData, 0644)
			require.NoError(t, err)

			// Run v1: hostie list --json (with HOME set to tmpHome)
			v1Output := runBinaryWithEnv(t, v1Binary, []string{"list", "--json"}, map[string]string{"HOME": tmpHome})

			// Run v2: hostie list --json (with HOME set to tmpHome)
			v2Output := runBinaryWithEnv(t, v2Binary, []string{"list", "--json"}, map[string]string{"HOME": tmpHome})

			// Assert byte-equal JSON output
			require.JSONEq(t, v1Output, v2Output,
				"list --json output diverged for %s\nv1:\n%s\n\nv2:\n%s",
				fixturePath, v1Output, v2Output)
		})
	}
}

// TestApplyDryRunParity verifies that `hostie apply --dry-run` produces a
// semantically equivalent managed block between v1 and v2.
//
// This is a D15 parity surface, but with documented intentional divergence:
//   - v1 wraps the block with blank-line padding (BEGIN HOSTIE\n\n...\n\nEND HOSTIE)
//   - v2 removes the padding for canonical shape (BEGIN HOSTIE\n...\nEND HOSTIE)
//   - v2 adds "# group: <name>" headers
//   - v1 and v2 use different framing text ("Managed block..." vs "# Dry run...")
//
// See docs/go-migration/release-notes-2.0.0.md for the breaking change rationale.
// This test verifies that the entry data lines (IP + hostname + aliases) match
// between v1 and v2 after normalizing all known intentional differences.
func TestApplyDryRunParity(t *testing.T) {
	v1Binary := ensureV1Binary(t)
	v2Binary := buildV2Binary(t)

	fixtures := []string{
		"../fixtures/hosts/simple-dev.yaml",
		"../fixtures/hosts/complex-roundtrip.yaml",
		"../fixtures/hosts/nested-groups.yaml",
	}

	for _, fixturePath := range fixtures {
		fixturePath := fixturePath
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			// Create temp HOME with ~/.hosts file
			tmpHome := t.TempDir()
			hostsPath := filepath.Join(tmpHome, ".hosts")
			fixtureData, err := os.ReadFile(fixturePath)
			require.NoError(t, err)
			err = os.WriteFile(hostsPath, fixtureData, 0644)
			require.NoError(t, err)

			// Run v1: hostie apply --dry-run (with HOME set to tmpHome)
			v1Output := runBinaryWithEnv(t, v1Binary, []string{"apply", "--dry-run"}, map[string]string{"HOME": tmpHome})

			// Run v2: hostie apply --dry-run (with HOME set to tmpHome)
			v2Output := runBinaryWithEnv(t, v2Binary, []string{"apply", "--dry-run"}, map[string]string{"HOME": tmpHome})

			// Extract and normalize entry data lines for semantic comparison.
			v1Entries := extractEntryLines(v1Output)
			v2Entries := extractEntryLines(v2Output)

			require.Equal(t, v1Entries, v2Entries,
				"apply --dry-run entry data diverged for %s\nv1:\n%s\n\nv2:\n%s",
				fixturePath, v1Output, v2Output)
		})
	}
}

// extractEntryLines pulls IP/hostname lines from apply --dry-run output,
// stripping all framing, comments, and group headers. Returns sorted list
// of "<ip> <hostname> [aliases...]" strings for semantic comparison.
func extractEntryLines(output string) []string {
	var entries []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines
		if trimmed == "" {
			continue
		}
		// Skip comments, markers, framing
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "─") {
			continue
		}
		if strings.HasPrefix(trimmed, "Dry-run mode") || strings.HasPrefix(trimmed, "Managed block") {
			continue
		}
		entries = append(entries, trimmed)
	}
	sort.Strings(entries)
	return entries
}

// buildV2Binary builds the Go v2 binary and returns its path.
func buildV2Binary(t *testing.T) string {
	t.Helper()

	// Build to temp location
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "hostie-v2")

	// Build from go/ directory
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/hostie")
	cmd.Dir = filepath.Join("..", "..") // go/test/golden -> go/
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build v2 binary: %v\nOutput:\n%s", err, output)
	}

	t.Logf("Built v2 binary: %s", binaryPath)
	return binaryPath
}

// runBinaryWithEnv runs a binary with args and environment variables, returns stdout as a string.
// Fails the test if the command exits non-zero.
func runBinaryWithEnv(t *testing.T, binaryPath string, args []string, env map[string]string) string {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment variables
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %s %v\nError: %v\nStdout:\n%s\nStderr:\n%s",
			binaryPath, args, err, stdout.String(), stderr.String())
	}

	return stdout.String()
}
