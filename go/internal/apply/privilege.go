package apply

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
)

const (
	// APPLY_PRIVILEGED_CMD is the hidden subcommand for privileged apply
	APPLY_PRIVILEGED_CMD = "__apply-privileged"
)

// ETC_HOSTS_PATH is the standard location of the hosts file.
// Declared as var (not const) so tests can override it via test helpers.
var ETC_HOSTS_PATH = "/etc/hosts"

// CanWriteEtcHosts checks if the current process can write to /etc/hosts directly
func CanWriteEtcHosts() bool {
	// Try to open /etc/hosts for writing (without actually writing)
	f, err := os.OpenFile(ETC_HOSTS_PATH, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// ReexecWithSudo re-executes the current binary with sudo, passing through all arguments
func ReexecWithSudo() error {
	// Check if already running as root
	if os.Geteuid() == 0 {
		return fmt.Errorf("cannot write /etc/hosts even as root")
	}

	// Get the real path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		// If we can't resolve symlinks, use the original path
		realPath = execPath
	}

	// Build sudo command with original arguments
	args := append([]string{realPath}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run and wait for completion
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("sudo reexec failed: %w", err)
	}

	os.Exit(0)
	return nil // unreachable
}

// WritePayloadToTempfile writes the rendered hosts block to a secure tempfile
// Returns the path to the tempfile and a cleanup function
func WritePayloadToTempfile(payload []byte) (string, func(), error) {
	// Generate a random filename to avoid predictable paths
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random filename: %w", err)
	}
	filename := fmt.Sprintf("hostie-payload-%s", hex.EncodeToString(randomBytes))

	// Create tempfile with 0600 permissions (owner read/write only)
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, filename)
	
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create tempfile: %w", err)
	}

	// Write payload
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("failed to write payload: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("failed to close tempfile: %w", err)
	}

	// Setup cleanup function with signal handling
	cleanup := func() {
		os.Remove(tmpPath)
	}

	// Register signal handler for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(1)
	}()

	return tmpPath, cleanup, nil
}

// ValidatePayloadFile validates that a tempfile contains a well-formed hostie managed block
func ValidatePayloadFile(path string) ([]byte, error) {
	// Check file ownership and permissions
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat payload file: %w", err)
	}

	// Verify it's a regular file (not a symlink or device)
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("payload file is not a regular file")
	}

	// Verify permissions are 0600 (owner read/write only)
	if info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("payload file has incorrect permissions: %o (expected 0600)", info.Mode().Perm())
	}

	// Verify owner is the invoking user (not root)
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("failed to get file stat")
	}

	// Get the real UID (the user who invoked sudo)
	realUID := os.Getuid()
	sudoUID := unix.Getuid()
	
	// If running under sudo, SUDO_UID env var contains the real user's UID
	if sudoUIDStr := os.Getenv("SUDO_UID"); sudoUIDStr != "" {
		var parsedUID int
		if _, err := fmt.Sscanf(sudoUIDStr, "%d", &parsedUID); err == nil {
			realUID = parsedUID
		}
	}

	// Verify file is owned by the invoking user (not root)
	if int(stat.Uid) != realUID && int(stat.Uid) != sudoUID {
		return nil, fmt.Errorf("payload file is not owned by invoking user")
	}

	// Read and validate content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read payload file: %w", err)
	}

	// Basic validation: payload must be non-empty
	if len(content) == 0 {
		return nil, fmt.Errorf("payload file is empty")
	}

	// Marker invariant (threat-model §3.3): the payload MUST be a rendered
	// managed block with exactly one BEGIN marker, exactly one END marker,
	// and BEGIN strictly before END. The privileged side re-derives the
	// /etc/hosts merge from this block — it does NOT write the bytes
	// verbatim — so garbage cannot escape the managed region even if the
	// unprivileged side (or an attacker holding the tempfile) misbehaves.
	//
	// Marker lines are matched full-line after right-trimming ASCII
	// whitespace, matching etchosts.ExtractManagedBlock's parser. We reuse
	// ExtractManagedBlock here so "what the receiver accepts" is pinned to
	// "what the parser splits" — a single seam, no drift.
	preamble, managed, suffix, err := etchosts.ExtractManagedBlock(content)
	if err != nil {
		return nil, fmt.Errorf("payload marker validation failed: %w", err)
	}
	if managed == nil {
		return nil, fmt.Errorf("payload does not contain hostie managed block markers")
	}
	if len(preamble) != 0 || len(suffix) != 0 {
		return nil, fmt.Errorf("payload must contain ONLY the managed block (no preamble or suffix bytes)")
	}

	return content, nil
}
