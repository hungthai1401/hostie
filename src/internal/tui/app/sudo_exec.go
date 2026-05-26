// sudo_exec.go — bubbletea wrapper around apply.BuildSudoCmd.
//
// Bead: hosts-cli-go-mig-p4-sudo-wire-jpr
// Review: hosts-cli-review-p1-apply-bubbletea-dep-p40
//
// This file owns the Bubble Tea-facing side of the sudo handoff:
//
//   - SudoFinishedMsg: the tea.Msg the ExecProcess callback delivers back to
//     the Update loop after the child exits.
//   - SudoApplyCmd: convenience tea.Cmd factory that builds the sudo *exec.Cmd
//     via apply.BuildSudoCmd, invokes tea.ExecProcess (which releases the TTY,
//     runs sudo, then re-acquires the TTY), and wires SudoFinishedMsg as the
//     callback payload.
//
// These symbols used to live in go/internal/apply/sudo_cmd.go but were moved
// here so apply/ stays pure stdlib (Layer 4) and the CLI does not transitively
// link the full bubbletea runtime. apply.BuildSudoCmd remains the single
// source of truth for argv construction and input validation.
//
// Design references:
//
//   - design.md D12, D13: privileged write semantics + tempfile cleanup.
//   - phase-4-contract.md clause 6: TUI uses tea.ExecProcess for the handoff.
//   - .spikes/go-migration/sudo-spike-asr/FINDINGS.md: integration sketch.

package app

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hungthai1401/hostie/src/internal/apply"
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

// SudoApplyCmd returns a tea.Cmd suitable for return from the TUI Update
// loop. It builds the sudo command via apply.BuildSudoCmd, invokes
// tea.ExecProcess (which releases the TTY, runs sudo, then re-acquires the
// TTY), and dispatches SudoFinishedMsg back to Update on completion.
//
// The caller is responsible for:
//
//   - writing the payload tempfile (apply.WritePayloadToTempfile) BEFORE
//     calling this, and
//   - cleaning up the tempfile when SudoFinishedMsg is received (success or
//     failure). See sudo_handoff.go for the integration.
//
// On build failure (bad exePath, payload outside TMPDIR, invalid uid) the
// returned Cmd yields SudoFinishedMsg{Err: ..., ExitCode: -1} immediately so
// the caller never gets a nil callback.
func SudoApplyCmd(exePath, payloadPath string, ownerUID int) tea.Cmd {
	cmd, err := apply.BuildSudoCmd(exePath, payloadPath, ownerUID)
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
