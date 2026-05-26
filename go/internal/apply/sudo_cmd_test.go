// sudo_cmd_test.go — covers BuildSudoCmd input validation for
// hosts-cli-go-mig-p4-sudo-wire-jpr.
//
// The success path (tea.ExecProcess actually releasing the TTY and running
// sudo) is exercised in UAT — these unit tests stay disk- and TTY-free.
// The SudoApplyCmd error-path test lives in go/internal/tui/app/sudo_exec_test.go
// since SudoApplyCmd moved there as part of review P1
// (hosts-cli-review-p1-apply-bubbletea-dep-p40).

package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildSudoCmd_Happy verifies the well-formed inputs produce a sudo argv
// in the exact order the design contract requires: sudo, exe,
// __apply-privileged, --payload-path=, --owner-uid=.
func TestBuildSudoCmd_Happy(t *testing.T) {
	exe := "/usr/local/bin/hostie"
	payload := filepath.Join(os.TempDir(), "hostie-payload-abcd")

	cmd, err := BuildSudoCmd(exe, payload, 501)
	require.NoError(t, err)
	require.NotNil(t, cmd)

	args := cmd.Args
	require.Equal(t, "sudo", args[0])
	require.Equal(t, exe, args[1])
	require.Equal(t, APPLY_PRIVILEGED_CMD, args[2])
	require.Equal(t, "--payload-path="+payload, args[3])
	require.Equal(t, "--owner-uid=501", args[4])

	// Tea owns Stdin/Stdout/Stderr — we must not pre-wire them.
	require.Nil(t, cmd.Stdin)
	require.Nil(t, cmd.Stdout)
	require.Nil(t, cmd.Stderr)
}

func TestBuildSudoCmd_EmptyExe(t *testing.T) {
	_, err := BuildSudoCmd("", filepath.Join(os.TempDir(), "p"), 501)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exePath is empty")
}

func TestBuildSudoCmd_RelativeExe(t *testing.T) {
	_, err := BuildSudoCmd("./hostie", filepath.Join(os.TempDir(), "p"), 501)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be absolute")
}

func TestBuildSudoCmd_ShellMetaInExe(t *testing.T) {
	bad := []string{
		"/tmp/hostie;rm",
		"/tmp/hostie|cat",
		"/tmp/hostie`id`",
		"/tmp/hostie$HOME",
		"/tmp/hostie\nrm",
	}
	for _, b := range bad {
		_, err := BuildSudoCmd(b, filepath.Join(os.TempDir(), "p"), 501)
		require.Error(t, err, "input %q must be rejected", b)
		require.Contains(t, err.Error(), "forbidden characters", "input %q: %v", b, err)
	}
}

func TestBuildSudoCmd_EmptyPayload(t *testing.T) {
	_, err := BuildSudoCmd("/usr/local/bin/hostie", "", 501)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payloadPath is empty")
}

func TestBuildSudoCmd_RelativePayload(t *testing.T) {
	_, err := BuildSudoCmd("/usr/local/bin/hostie", "relative/path", 501)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be absolute")
}

func TestBuildSudoCmd_PayloadOutsideTmpdir(t *testing.T) {
	// Pick something definitely not under $TMPDIR.
	_, err := BuildSudoCmd("/usr/local/bin/hostie", "/etc/passwd", 501)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must live under")
}

func TestBuildSudoCmd_InvalidUID(t *testing.T) {
	cases := []int{0, -1, -500}
	for _, uid := range cases {
		_, err := BuildSudoCmd("/usr/local/bin/hostie", filepath.Join(os.TempDir(), "p"), uid)
		require.Error(t, err, "uid %d must be rejected", uid)
		require.Contains(t, err.Error(), "ownerUID must be > 0")
	}
}

// TestBuildSudoCmd_MacOSPrivateTmp guards the EvalSymlinks fallback: on
// macOS os.TempDir() returns /var/folders/... but a literal /tmp file should
// also be accepted because /tmp is symlinked to /private/tmp. We don't
// require this path to exist; we just verify the prefix logic works on
// resolved + literal forms.
func TestBuildSudoCmd_PathPrefixResolvesSymlinks(t *testing.T) {
	tmp := os.TempDir()
	payload := filepath.Join(tmp, "hostie-payload-xyz")
	_, err := BuildSudoCmd("/usr/local/bin/hostie", payload, 501)
	require.NoError(t, err)
}


// TestResolveSelfExe smoke-tests the helper: it must return a non-empty
// absolute path. The exact value depends on the test binary location.
func TestResolveSelfExe(t *testing.T) {
	exe, err := ResolveSelfExe()
	require.NoError(t, err)
	require.NotEmpty(t, exe)
	require.True(t, filepath.IsAbs(exe), "want absolute path, got %q", exe)
}

// TestHasPathPrefix covers the prefix helper that backs the TMPDIR
// containment check.
func TestHasPathPrefix(t *testing.T) {
	cases := []struct {
		name           string
		path, prefix   string
		want           bool
	}{
		{"identical", "/tmp", "/tmp", true},
		{"child", "/tmp/foo", "/tmp", true},
		{"deep child", "/tmp/a/b/c", "/tmp", true},
		{"sibling not prefix", "/tmpfoo", "/tmp", false},
		{"outside", "/etc/passwd", "/tmp", false},
		{"trailing slash on prefix", "/tmp/foo", "/tmp/", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, hasPathPrefix(c.path, c.prefix))
		})
	}
}
