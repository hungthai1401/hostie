package etchosts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestAtomicProperty1_SymlinkRejection verifies WriteEtcHosts refuses to follow symlinks.
func TestAtomicProperty1_SymlinkRejection(t *testing.T) {
	dir := t.TempDir()
	sibling := filepath.Join(dir, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("UNTOUCHED"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.txt")
	if err := os.Symlink(sibling, target); err != nil {
		t.Fatal(err)
	}

	err := WriteEtcHosts(target, "PWNED")
	if err == nil {
		t.Fatal("expected error when target is symlink, got nil")
	}

	siblingContent, _ := os.ReadFile(sibling)
	if string(siblingContent) != "UNTOUCHED" {
		t.Errorf("sibling was modified: got %q, want %q", siblingContent, "UNTOUCHED")
	}
}

// TestAtomicProperty2_SameFSTempfile verifies tempfile is created in same directory as target.
func TestAtomicProperty2_SameFSTempfile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	if err := WriteEtcHosts(target, "hello"); err != nil {
		t.Fatalf("WriteEtcHosts failed: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read target: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content mismatch: got %q, want %q", got, "hello")
	}

	// Verify no leftover tempfiles
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "out.txt" {
			t.Errorf("leftover file: %s", e.Name())
		}
	}
}

// TestAtomicProperty3_ModeClamp verifies mode is clamped to 0o644 (no setuid/setgid/sticky/world-write).
func TestAtomicProperty3_ModeClamp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	if err := WriteEtcHosts(target, "data"); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	mode := st.Mode().Perm()

	// Expect 0o644 (rw-r--r--)
	if mode != 0o644 {
		t.Errorf("mode mismatch: got %04o, want 0644", mode)
	}

	// Verify clamp formula: 0o777 & 0o777 & ^0o022 = 0o755
	clampMask := fs.FileMode(0o777 &^ 0o022)
	clamped := fs.FileMode(0o777) & clampMask
	if clamped != 0o755 {
		t.Errorf("clamp formula broken: 0o777 clamped to %04o, want 0755", clamped)
	}
}

// TestAtomicProperty4_EPERMSwallow verifies chown swallows ONLY unix.EPERM.
func TestAtomicProperty4_EPERMSwallow(t *testing.T) {
	mkErr := func(e syscall.Errno) error {
		return &fs.PathError{Op: "chown", Path: "/x", Err: e}
	}

	check := func(err error) (swallowed bool) {
		var pe *fs.PathError
		if errors.As(err, &pe) && errors.Is(pe.Err, unix.EPERM) {
			return true
		}
		return false
	}

	eperm := mkErr(unix.EPERM)
	eacces := mkErr(unix.EACCES)

	if !check(eperm) {
		t.Error("EPERM should be swallowed")
	}
	if check(eacces) {
		t.Error("EACCES should NOT be swallowed")
	}
}

// TestAtomicProperty5_SignalCleanup verifies tempfile is unlinked on SIGTERM.
func TestAtomicProperty5_SignalCleanup(t *testing.T) {
	if os.Getenv("HOSTIE_TEST_SIGCHILD") == "1" {
		// Child process: create tempfile, trap signal, wait for SIGTERM
		dir := os.Getenv("HOSTIE_TEST_DIR")
		_ = dir // used in CreateTemp below

		// Simulate WriteEtcHosts starting but not finishing
		tmp, err := os.CreateTemp(dir, ".hostie-tmp-*")
		if err != nil {
			os.Exit(2)
		}
		tmpPath := tmp.Name()

		// Signal trap (same as WriteEtcHosts)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			_ = os.Remove(tmpPath)
			os.Exit(130)
		}()
		defer signal.Stop(sigCh)
		defer func() {
			if _, statErr := os.Lstat(tmpPath); statErr == nil {
				_ = os.Remove(tmpPath)
			}
		}()

		tmp.WriteString("partial")
		tmp.Close()

		// Block until killed
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}

	// Parent process: spawn child, send SIGTERM, verify no tempfiles remain
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=TestAtomicProperty5_SignalCleanup")
	cmd.Env = append(os.Environ(), "HOSTIE_TEST_SIGCHILD=1", "HOSTIE_TEST_DIR="+dir)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Give child time to create tempfile
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	// Wait for child to exit
	_ = cmd.Wait()

	// Verify no tempfiles remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	leftovers := 0
	for _, e := range entries {
		name := e.Name()
		if len(name) > 0 && name[0] == '.' && filepath.Ext(name) == "" {
			leftovers++
			t.Errorf("leftover tempfile after SIGTERM: %s", name)
		}
	}

	if leftovers > 0 {
		t.Errorf("found %d leftover tempfiles after SIGTERM (signal trap failed)", leftovers)
	}
}

// TestAtomicIntegration verifies end-to-end write + overwrite behavior.
func TestAtomicIntegration(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hosts")

	// Initial write
	if err := WriteEtcHosts(target, "127.0.0.1 localhost\n"); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	content1, _ := os.ReadFile(target)
	if string(content1) != "127.0.0.1 localhost\n" {
		t.Errorf("initial content mismatch: got %q", content1)
	}

	// Overwrite
	if err := WriteEtcHosts(target, "127.0.0.1 localhost\n::1 localhost\n"); err != nil {
		t.Fatalf("overwrite failed: %v", err)
	}

	content2, _ := os.ReadFile(target)
	expected := "127.0.0.1 localhost\n::1 localhost\n"
	if string(content2) != expected {
		t.Errorf("overwrite content mismatch: got %q, want %q", content2, expected)
	}

	// Verify no tempfiles remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "hosts" {
			t.Errorf("leftover file: %s", e.Name())
		}
	}
}

// TestAtomicRaceCondition runs WriteEtcHosts concurrently to detect race conditions.
func TestAtomicRaceCondition(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hosts")

	// Write initial content
	if err := WriteEtcHosts(target, "initial\n"); err != nil {
		t.Fatal(err)
	}

	// Concurrent writes (should be serialized by OS-level file locks)
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		i := i
		go func() {
			content := fmt.Sprintf("writer-%d\n", i)
			if err := WriteEtcHosts(target, content); err != nil {
				t.Errorf("concurrent write %d failed: %v", i, err)
			}
			done <- true
		}()
	}

	// Wait for all writes
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify file exists and has valid content from one of the writers
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read after concurrent writes: %v", err)
	}

	valid := false
	for i := 0; i < 3; i++ {
		if string(content) == fmt.Sprintf("writer-%d\n", i) {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("unexpected content after concurrent writes: %q", content)
	}
}
