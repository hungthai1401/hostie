// runner_test.go — unit tests for apply.Runner additions wired by
// hosts-cli-review-p1-sudo-merge-boundary-bre.
//
// The pre-existing Runner.Apply pipeline is covered indirectly via the
// integration suite in go/internal/cmd. This file adds:
//
//   - TestRunner_PrepareSudoHandoff_*: locks down the unprivileged-side
//     prep step's behavior (writes ~/.hosts, writes payload tempfile,
//     payload contains ONLY the rendered managed block).
//   - TestApply_DirectAndSudoPathsProduceIdenticalEtc: the conformance
//     "pin" from critical-patterns §17. The direct path (Runner.Apply)
//     and the simulated sudo path (PrepareSudoHandoff → privileged
//     merge) MUST produce byte-identical /etc/hosts content for the same
//     input fixture. Drift here is a hard failure.

package apply

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
	"github.com/hungthai1401/hostie/go/internal/core/render"
	"github.com/hungthai1401/hostie/go/internal/domain"
)

func conformanceFixture() domain.HostsFile {
	return domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name: "default",
				Entries: []domain.Entry{
					{ID: "e1", IP: "10.0.0.1", Hostname: "example.test", Enabled: true},
					{ID: "e2", IP: "10.0.0.2", Hostname: "two.test", Enabled: false},
				},
			},
		},
	}
}

func TestRunner_PrepareSudoHandoff_WritesHostsAndPayload(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts.yaml")

	r := NewRunner(hostsPath, false)
	hf := conformanceFixture()

	payloadPath, cleanup, err := r.PrepareSudoHandoff(hf)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	defer cleanup()

	// ~/.hosts must exist (D13 — YAML write independent of /etc/hosts).
	hostsStat, err := os.Stat(hostsPath)
	require.NoError(t, err, "PrepareSudoHandoff must write ~/.hosts")
	require.Greater(t, hostsStat.Size(), int64(0))

	// Payload must exist with 0600 perms.
	info, err := os.Lstat(payloadPath)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())

	// Payload must contain ONLY the rendered managed block (markers + body),
	// nothing else. This is the threat-model §3.3 invariant — the
	// unprivileged side hands the privileged side only what is between the
	// markers (plus the markers themselves).
	got, err := os.ReadFile(payloadPath)
	require.NoError(t, err)
	want := render.RenderManagedBlock(&hf)
	require.Equal(t, want, string(got),
		"payload must be byte-identical to render.RenderManagedBlock output")

	// ValidatePayloadFile must accept it (this proves the renderer's output
	// satisfies the privileged-side receiver's invariant — the pin).
	validated, err := ValidatePayloadFile(payloadPath)
	require.NoError(t, err, "renderer output must satisfy ValidatePayloadFile invariant")
	require.Equal(t, got, validated)
}

func TestRunner_PrepareSudoHandoff_HostsWriteFailure(t *testing.T) {
	// Pointing the runner at a non-existent directory forces the ~/.hosts
	// write to fail and proves PrepareSudoHandoff does NOT leak a payload
	// tempfile on the error path.
	r := NewRunner("/nonexistent/dir/hosts.yaml", false)
	_, cleanup, err := r.PrepareSudoHandoff(conformanceFixture())
	require.Error(t, err)
	require.Nil(t, cleanup)
}

// TestApply_DirectAndSudoPathsProduceIdenticalEtc is the conformance pin
// from docs/learnings/critical-patterns.md §17 ("One Renderer, One Parser
// — Share or Pin"). The direct apply path and the sudo handoff path both
// produce /etc/hosts content; if their outputs ever diverge for the same
// fixture, the merge logic has split into two implementations again.
//
// Path 1 (direct): Runner.Apply against a writable etc-hosts file.
// Path 2 (sudo simulated): Runner.PrepareSudoHandoff to obtain the
//
//	payload, then perform the merge the way apply_privileged.go's RunE
//	would (ReadFile + ReplaceManagedBlock + WriteEtcHosts).
//
// We assert the resulting /etc/hosts bytes are byte-identical.
func TestApply_DirectAndSudoPathsProduceIdenticalEtc(t *testing.T) {
	hf := conformanceFixture()
	seedEtc := []byte("127.0.0.1 localhost\n::1 localhost\n# existing comment\n")

	// --- Path 1: direct -----------------------------------------------------
	dir1 := t.TempDir()
	hostsPath1 := filepath.Join(dir1, "hosts.yaml")
	etcPath1 := filepath.Join(dir1, "etc-hosts")
	require.NoError(t, os.WriteFile(etcPath1, seedEtc, 0o644))

	prevEtc := ETC_HOSTS_PATH
	ETC_HOSTS_PATH = etcPath1
	r1 := NewRunner(hostsPath1, false)
	_, err := r1.Apply(hf)
	ETC_HOSTS_PATH = prevEtc
	require.NoError(t, err)

	directEtc, err := os.ReadFile(etcPath1)
	require.NoError(t, err)

	// --- Path 2: simulated sudo --------------------------------------------
	dir2 := t.TempDir()
	hostsPath2 := filepath.Join(dir2, "hosts.yaml")
	etcPath2 := filepath.Join(dir2, "etc-hosts")
	require.NoError(t, os.WriteFile(etcPath2, seedEtc, 0o644))

	r2 := NewRunner(hostsPath2, false)
	payloadPath, cleanup, err := r2.PrepareSudoHandoff(hf)
	require.NoError(t, err)
	defer cleanup()

	// Reproduce exactly what cmd/apply_privileged.go's RunE does under root:
	//   1. ValidatePayloadFile (asserts marker invariants)
	//   2. ReadFile(/etc/hosts)
	//   3. ReplaceManagedBlock(etc, payload)
	//   4. WriteEtcHosts(merged)
	payloadBytes, err := ValidatePayloadFile(payloadPath)
	require.NoError(t, err)
	etcBytes, err := os.ReadFile(etcPath2)
	require.NoError(t, err)
	merged, err := etchosts.ReplaceManagedBlock(etcBytes, payloadBytes)
	require.NoError(t, err)
	require.NoError(t, etchosts.WriteEtcHosts(etcPath2, string(merged)))

	sudoEtc, err := os.ReadFile(etcPath2)
	require.NoError(t, err)

	require.Equal(t, string(directEtc), string(sudoEtc),
		"direct and sudo paths MUST produce byte-identical /etc/hosts (critical-patterns §17)")
}
