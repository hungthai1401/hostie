// sudo_exec_test.go — covers SudoApplyCmd error paths (moved from
// go/internal/apply/sudo_cmd_test.go as part of review P1
// hosts-cli-review-p1-apply-bubbletea-dep-p40).

package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSudoApplyCmd_BuildErrorYieldsMsg verifies that a build failure (bad
// inputs) surfaces as a SudoFinishedMsg via the returned Cmd instead of a
// nil Cmd or a panic. The TUI relies on always getting a message back so
// the Update loop can clean up the tempfile.
func TestSudoApplyCmd_BuildErrorYieldsMsg(t *testing.T) {
	cmd := SudoApplyCmd("", "", 0)
	require.NotNil(t, cmd)
	msg := cmd()
	finished, ok := msg.(SudoFinishedMsg)
	require.True(t, ok, "expected SudoFinishedMsg, got %T", msg)
	require.Error(t, finished.Err)
	require.Equal(t, -1, finished.ExitCode)
	// Error should originate from apply.BuildSudoCmd validation.
	require.True(t,
		strings.Contains(finished.Err.Error(), "exePath") ||
			strings.Contains(finished.Err.Error(), "payloadPath"),
		"want validation error, got %v", finished.Err)
}
