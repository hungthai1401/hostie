package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hungthai1401/hostie/src/internal/apply"
	"github.com/stretchr/testify/require"
)

// resetApplyPrivilegedFlags clears the package-level globals bound to
// applyPrivilegedCmd's flags so each table case starts from a known state.
// (The cobra command is a singleton shared across tests, so flag values
// persist across invocations unless explicitly reset.)
func resetApplyPrivilegedFlags() {
	applyPrivilegedPayloadPath = ""
	applyPrivilegedOwnerUID = 0
}

// writePayload creates a tempfile in dir with the given mode and content
// and returns its absolute path. Owner is the current process uid.
func writePayload(t *testing.T, dir, name string, mode os.FileMode, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), mode))
	// os.WriteFile honors umask; force the exact mode we asked for.
	require.NoError(t, os.Chmod(p, mode))
	return p
}

// TestApplyPrivileged exercises the hidden __apply-privileged subcommand's
// RunE owner-uid validation and payload-handling contract.
//
// Threat model: the hidden cmd runs as root (sudo). A regression that
// inverts the uid check or skips Lstat could let a non-root process trick
// root into copying an attacker-controlled payload into /etc/hosts. These
// table cases lock down the negative paths and the happy path.
func TestApplyPrivileged(t *testing.T) {
	// Payloads handed to __apply-privileged must contain ONLY the rendered
	// managed block (BEGIN/END markers + entries between). The privileged
	// side re-derives the merge against /etc/hosts under root — see
	// threat-model §3.3 and apply.ValidatePayloadFile.
	validBody := "# BEGIN HOSTIE\n127.0.0.1 example.test\n# END HOSTIE\n"
	uid := os.Getuid()

	type caseSpec struct {
		name string
		// build returns (payloadPath, ownerUID). payloadPath may be empty
		// to exercise the "missing flag" branch.
		build         func(t *testing.T, dir string) (string, int)
		wantErr       bool
		wantErrSubstr string
		// wantEtcHostsWritten asserts whether /etc/hosts (tempfile) was
		// modified. Only meaningful on the happy path.
		wantEtcHostsWritten bool
		// preRemovePayload removes the payload file before running, to
		// prove the deferred cleanup tolerates ENOENT.
		preRemovePayload bool
	}

	cases := []caseSpec{
		{
			name: "missing --payload-path",
			build: func(t *testing.T, dir string) (string, int) {
				return "", uid
			},
			wantErr:       true,
			wantErrSubstr: "--payload-path is required",
		},
		{
			name: "owner-uid zero rejected",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o600, validBody), 0
			},
			wantErr:       true,
			wantErrSubstr: "--owner-uid must be > 0",
		},
		{
			name: "owner-uid negative rejected",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o600, validBody), -1
			},
			wantErr:       true,
			wantErrSubstr: "--owner-uid must be > 0",
		},
		{
			name: "lstat failure on nonexistent payload",
			build: func(t *testing.T, dir string) (string, int) {
				return filepath.Join(dir, "does-not-exist"), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload not accessible",
		},
		{
			name: "owner-uid mismatch refuses to write",
			build: func(t *testing.T, dir string) (string, int) {
				p := writePayload(t, dir, "p", 0o600, validBody)
				// The file is owned by `uid`. Assert a deliberately
				// wrong owner-uid; the mismatch branch must fire
				// regardless of the running test user.
				return p, uid + 1
			},
			wantErr:       true,
			wantErrSubstr: "payload owner uid mismatch",
		},
		{
			name: "validate fails: wrong permissions (0644)",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o644, validBody), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "validate fails: non-regular file (directory)",
			build: func(t *testing.T, dir string) (string, int) {
				p := filepath.Join(dir, "not-a-file")
				require.NoError(t, os.Mkdir(p, 0o700))
				return p, uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "validate fails: missing managed-block markers",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o600, "no marker here\n"), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "validate fails: payload has bytes outside markers",
			build: func(t *testing.T, dir string) (string, int) {
				body := "leading garbage\n# BEGIN HOSTIE\n127.0.0.1 example.test\n# END HOSTIE\n"
				return writePayload(t, dir, "p", 0o600, body), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "validate fails: duplicate BEGIN markers",
			build: func(t *testing.T, dir string) (string, int) {
				body := "# BEGIN HOSTIE\n# BEGIN HOSTIE\n# END HOSTIE\n"
				return writePayload(t, dir, "p", 0o600, body), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "validate fails: END before BEGIN",
			build: func(t *testing.T, dir string) (string, int) {
				body := "# END HOSTIE\n# BEGIN HOSTIE\n"
				return writePayload(t, dir, "p", 0o600, body), uid
			},
			wantErr:       true,
			wantErrSubstr: "payload validation failed",
		},
		{
			name: "happy path writes /etc/hosts",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o600, validBody), uid
			},
			wantErr:             false,
			wantEtcHostsWritten: true,
		},
		{
			name: "defer cleanup tolerates already-gone payload",
			build: func(t *testing.T, dir string) (string, int) {
				return writePayload(t, dir, "p", 0o600, validBody), uid
			},
			// Remove the payload AFTER ValidatePayloadFile would run —
			// but actually we need it present for the happy path. To
			// exercise idempotent cleanup we instead pre-remove and
			// expect the lstat-not-accessible branch; the defer still
			// runs and must not panic / must not surface a second error.
			preRemovePayload:    true,
			wantErr:             true,
			wantErrSubstr:       "payload not accessible",
			wantEtcHostsWritten: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			resetApplyPrivilegedFlags()
			t.Cleanup(resetApplyPrivilegedFlags)

			dir := t.TempDir()
			payloadPath, ownerUID := tc.build(t, dir)

			if tc.preRemovePayload && payloadPath != "" {
				_ = os.Remove(payloadPath)
			}

			// Snapshot /etc/hosts (tempfile) so we can verify it was
			// or was not modified depending on the case.
			origEtc, err := os.ReadFile(h.etcHostPath)
			require.NoError(t, err)

			args := []string{apply.APPLY_PRIVILEGED_CMD}
			if payloadPath != "" {
				args = append(args, "--payload-path="+payloadPath)
			}
			args = append(args, "--owner-uid="+strconv.Itoa(ownerUID))

			out, runErr := h.run(args...)

			if tc.wantErr {
				require.Error(t, runErr, "expected error, got nil. output=%q", out)
				if tc.wantErrSubstr != "" {
					require.Contains(t, runErr.Error(), tc.wantErrSubstr,
						"error message did not contain expected substring")
				}
			} else {
				require.NoError(t, runErr, "unexpected error. output=%q", out)
			}

			afterEtc, err := os.ReadFile(h.etcHostPath)
			require.NoError(t, err)
			if tc.wantEtcHostsWritten {
				require.NotEqual(t, string(origEtc), string(afterEtc),
					"/etc/hosts should have been written on happy path")
				require.Contains(t, string(afterEtc), "example.test",
					"/etc/hosts should contain payload content")
			} else {
				require.Equal(t, string(origEtc), string(afterEtc),
					"/etc/hosts must NOT be written when RunE rejects the request")
			}

			// On success, the defer cleanup must remove the payload.
			if !tc.wantErr && payloadPath != "" {
				_, statErr := os.Lstat(payloadPath)
				require.True(t, os.IsNotExist(statErr),
					"payload tempfile must be removed by deferred cleanup, got err=%v", statErr)
			}
		})
	}
}


