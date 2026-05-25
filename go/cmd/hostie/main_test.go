package main_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestVersionFlag exercises the --version flag against the compiled binary.
// Subtests verify:
//   1. Default 'dev' fallback when version is not injected via ldflags
//   2. Format contract: output matches "hostie v<version>\n" (exact shape)
func TestVersionFlag(t *testing.T) {
	t.Parallel()

	t.Run("default_dev_fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bin := filepath.Join(dir, "hostie")

		// Build WITHOUT ldflags — version should default to "dev"
		build := exec.Command("go", "build", "-o", bin, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build failed: %v\n%s", err, out)
		}

		run := exec.Command(bin, "--version")
		out, err := run.Output()
		if err != nil {
			t.Fatalf("running %s --version failed: %v", bin, err)
		}

		const want = "hostie vdev\n"
		if got := string(out); got != want {
			t.Fatalf("unexpected stdout:\n  got:  %q\n  want: %q", got, want)
		}
	})

	t.Run("format_contract", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bin := filepath.Join(dir, "hostie")

		// Build WITH ldflags — version should be injected
		build := exec.Command("go", "build",
			"-ldflags=-X main.version=1.2.3",
			"-o", bin,
			".",
		)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build failed: %v\n%s", err, out)
		}

		run := exec.Command(bin, "--version")
		out, err := run.Output()
		if err != nil {
			t.Fatalf("running %s --version failed: %v", bin, err)
		}

		// Assert exact format: "hostie v<version>\n"
		// If format string changes (e.g., drops the 'v'), this test fails
		const want = "hostie v1.2.3\n"
		if got := string(out); got != want {
			t.Fatalf("unexpected stdout:\n  got:  %q\n  want: %q", got, want)
		}
	})
}
