// sudo_cmd.go — TUI-safe sudo handoff helper for the privileged /etc/hosts write.
//
// Bead: hosts-cli-go-mig-p4-sudo-wire-jpr
//
// Why this exists separate from privilege.go:
//
//   - privilege.go::ReexecWithSudo replaces the current process via os.Exit on
//     the child's return. That is correct for the CLI path (the CLI has no
//     terminal state to preserve), but it would tear down a Bubble Tea Program
//     mid-render, leaving the terminal in altscreen + raw mode.
//   - The TUI sudo branch must instead release the TTY (so the sudo prompt is
//     visible in the user's normal scrollback), run the child, then re-acquire
//     the TTY. That lifecycle is exactly what tea.ExecProcess provides.
//
// This file therefore exposes:
//
//   - BuildSudoCmd: pure constructor for the *exec.Cmd that will be handed to
//     tea.ExecProcess. Validates inputs so a malformed exe path or a payload
//     tempfile outside $TMPDIR is rejected before sudo runs (defence in depth
//     for the threat model in approach.md §8 "__apply-privileged threat model").
//   - SudoFinishedMsg: the tea.Msg the callback delivers back to the Update
//     loop after the child exits (success or failure).
//   - SudoApplyCmd: convenience tea.Cmd factory that builds the command,
//     invokes tea.ExecProcess, and wires SudoFinishedMsg as the callback
//     payload. The TUI consumes this directly; tests can use BuildSudoCmd
//     alone to avoid the bubbletea runtime dependency.
//
// Design references:
//
//   - design.md D12: privileged write uses `sudo $0 __apply-privileged
//     --payload-path=<f>`; payload is a 0600 tempfile under $TMPDIR owned by
//     the invoking uid.
//   - design.md D13: tempfile is unlinked on every exit path (success,
//     failure, signal). The caller of SudoApplyCmd owns the cleanup (see
//     apply/privilege.go::WritePayloadToTempfile for the signal handler).
//   - phase-4-contract.md clause 6: TUI uses tea.ExecProcess for the handoff.
//   - .spikes/go-migration/sudo-spike-asr/FINDINGS.md: integration sketch.

package apply

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SudoFinishedMsg is delivered to the Bubble Tea Update loop after the sudo
// child process exits.
//
// Field semantics:
//
//   - Err == nil          → child exited 0; /etc/hosts has been rewritten.
//   - Err != nil          → either the child exited non-zero (wrong password,
//                           validation failure, write failure) or the exec
//                           itself failed (sudo not on PATH, TTY release
//                           failed). ExitCode disambiguates.
//   - ExitCode == 0       → success (matches Err == nil).
//   - ExitCode > 0        → child exited non-zero with this status.
//   - ExitCode == -1      → exec never produced an exit status (sudo missing,
//                           TTY release failed, etc.).
type SudoFinishedMsg struct {
	Err      error
	ExitCode int
}

// BuildSudoCmd constructs the *exec.Cmd that re-launches the current binary
// under sudo to perform the privileged /etc/hosts write.
//
// Inputs are validated so a malformed call surfaces a clean error rather than
// invoking sudo with a bogus argv:
//
//   - exePath must be a non-empty absolute path with no shell metacharacters.
//     (Callers should pass the result of os.Executable + filepath.EvalSymlinks.
//     sudo will refuse to run a relative path anyway, but we reject it here
//     so the failure is attributable.)
//   - payloadPath must live under os.TempDir(). The hidden subcommand
//     re-validates this on the receiving side (per the threat model in
//     approach.md §8), but rejecting early keeps sudo from being asked to
//     resolve attacker-controlled paths.
//   - ownerUID must be > 0 (the design forbids passing uid 0 — the whole
//     point of the SUDO_UID round-trip is to keep the resulting tempfile
//     owned by the invoking user, not root).
//
// The returned *exec.Cmd has Stdin/Stdout/Stderr unset; tea.ExecProcess wires
// those to the real TTY when it releases the terminal. Tests that exercise
// BuildSudoCmd directly can leave them unset.
func BuildSudoCmd(exePath, payloadPath string, ownerUID int) (*exec.Cmd, error) {
	if exePath == "" {
		return nil, fmt.Errorf("BuildSudoCmd: exePath is empty")
	}
	if !filepath.IsAbs(exePath) {
		return nil, fmt.Errorf("BuildSudoCmd: exePath must be absolute: %q", exePath)
	}
	if strings.ContainsAny(exePath, "\x00\n;&|`$<>") {
		return nil, fmt.Errorf("BuildSudoCmd: exePath contains forbidden characters: %q", exePath)
	}

	if payloadPath == "" {
		return nil, fmt.Errorf("BuildSudoCmd: payloadPath is empty")
	}
	if !filepath.IsAbs(payloadPath) {
		return nil, fmt.Errorf("BuildSudoCmd: payloadPath must be absolute: %q", payloadPath)
	}
	tmpDir := os.TempDir()
	// Compare resolved-symlink prefixes so /tmp vs /private/tmp on macOS is
	// not a false negative.
	resolvedTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		resolvedTmp = tmpDir
	}
	resolvedPayloadDir, err := filepath.EvalSymlinks(filepath.Dir(payloadPath))
	if err != nil {
		// Tempfile may have been removed between WritePayloadToTempfile and
		// here — fall back to the literal parent directory and let the
		// subcommand reject it. We only fail closed on prefix mismatch.
		resolvedPayloadDir = filepath.Dir(payloadPath)
	}
	if !hasPathPrefix(resolvedPayloadDir, resolvedTmp) && !hasPathPrefix(filepath.Dir(payloadPath), tmpDir) {
		return nil, fmt.Errorf("BuildSudoCmd: payloadPath must live under %q, got %q", tmpDir, payloadPath)
	}

	if ownerUID <= 0 {
		return nil, fmt.Errorf("BuildSudoCmd: ownerUID must be > 0, got %d", ownerUID)
	}

	cmd := exec.Command("sudo",
		exePath,
		APPLY_PRIVILEGED_CMD,
		"--payload-path="+payloadPath,
		"--owner-uid="+strconv.Itoa(ownerUID),
	)
	return cmd, nil
}

// hasPathPrefix reports whether path is equal to or rooted under prefix.
// Both arguments must already be cleaned absolute paths.
func hasPathPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	if path == prefix {
		return true
	}
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(path, prefix)
}

// ResolveSelfExe returns the absolute path to the currently-running binary
// with symlinks resolved. Mirrors the resolution ReexecWithSudo performs so
// the TUI and CLI paths agree on what "self" means.
func ResolveSelfExe() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("ResolveSelfExe: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exePath); err == nil {
		return real, nil
	}
	return exePath, nil
}

// SudoApplyCmd returns a tea.Cmd suitable for return from the TUI Update
// loop. It builds the sudo command via BuildSudoCmd, invokes tea.ExecProcess
// (which releases the TTY, runs sudo, then re-acquires the TTY), and
// dispatches SudoFinishedMsg back to Update on completion.
//
// The caller is responsible for:
//
//   - writing the payload tempfile (apply.WritePayloadToTempfile) BEFORE
//     calling this, and
//   - cleaning up the tempfile when SudoFinishedMsg is received (success or
//     failure). See app/sudo_handoff.go for the integration.
//
// On build failure (bad exePath, payload outside TMPDIR, invalid uid) the
// returned Cmd yields SudoFinishedMsg{Err: ..., ExitCode: -1} immediately so
// the caller never gets a nil callback.
func SudoApplyCmd(exePath, payloadPath string, ownerUID int) tea.Cmd {
	cmd, err := BuildSudoCmd(exePath, payloadPath, ownerUID)
	if err != nil {
		return func() tea.Msg {
			return SudoFinishedMsg{Err: err, ExitCode: -1}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		return SudoFinishedMsg{Err: err, ExitCode: code}
	})
}
