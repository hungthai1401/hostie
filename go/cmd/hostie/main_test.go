package main_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestVersionFlag_CompiledBinary builds the hostie binary with an
// ldflags-injected version, executes it with --version, and asserts the
// stdout and exit code. The test hits the compiled binary (not `go run`)
// to mirror how users will invoke it.
func TestVersionFlag_CompiledBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := filepath.Join(dir, "hostie")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	build := exec.Command("go", "build",
		"-ldflags=-X main.version=test-0.0.0",
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

	const want = "hostie vtest-0.0.0\n"
	if got := string(out); got != want {
		t.Fatalf("unexpected stdout:\n  got:  %q\n  want: %q", got, want)
	}
}
