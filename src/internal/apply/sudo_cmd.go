// sudo_cmd.go — pure-stdlib sudo handoff helpers for the privileged
// /etc/hosts write.
//
// Bead: hosts-cli-go-mig-p4-sudo-wire-jpr
// Review: hosts-cli-review-p1-apply-bubbletea-dep-p40
//
// Why this exists separate from privilege.go:
//
//   - privilege.go::ReexecWithSudo replaces the current process via os.Exit on
//     the child's return. That is correct for the CLI path (the CLI has no
//     terminal state to preserve), but it would tear down a Bubble Tea Program
//     mid-render, leaving the terminal in altscreen + raw mode.
//   - The TUI sudo branch must instead release the TTY (so the sudo prompt is
//     visible in the user's normal scrollback), run the child, then re-acquire
//     the TTY. That lifecycle is provided by tea.ExecProcess, which lives in
//     the TUI layer (go/internal/tui/app/sudo_exec.go) — apply/ stays free of
//     any bubbletea dependency so the CLI does not transitively link the TUI
//     runtime (review P1).
//
// This file therefore exposes only pure-stdlib helpers:
//
//   - BuildSudoCmd: pure constructor for the *exec.Cmd that will be handed to
//     tea.ExecProcess (in the TUI layer). Validates inputs so a malformed exe
//     path or a payload tempfile outside $TMPDIR is rejected before sudo runs
//     (defence in depth for the threat model in approach.md §8
//     "__apply-privileged threat model").
//   - ResolveSelfExe: absolute, symlink-resolved path to the running binary,
//     used by both CLI and TUI sudo paths so they agree on what "self" means.
//
// The bubbletea-facing wrappers (SudoApplyCmd, SudoFinishedMsg) live in
// go/internal/tui/app/sudo_exec.go and consume BuildSudoCmd from here.
//
// Design references:
//
//   - design.md D12: privileged write uses `sudo $0 __apply-privileged
//     --payload-path=<f>`; payload is a 0600 tempfile under $TMPDIR owned by
//     the invoking uid.
//   - design.md D13: tempfile is unlinked on every exit path (success,
//     failure, signal). The caller owns the cleanup (see
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
)

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
