package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/stretchr/testify/require"
)

// testHarness sets up an isolated environment for CLI integration tests:
//   - temp ~/.hosts file (path passed via --file flag)
//   - temp /etc/hosts override (via apply.ETC_HOSTS_PATH var)
//   - resets all command flag global vars to defaults
//   - returns helper to run cmds and read the resulting files
type testHarness struct {
	t           *testing.T
	hostsPath   string
	etcHostPath string
	origEtcPath string
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	tmp := t.TempDir()
	h := &testHarness{
		t:           t,
		hostsPath:   filepath.Join(tmp, "hosts.yaml"),
		etcHostPath: filepath.Join(tmp, "etc-hosts"),
		origEtcPath: apply.ETC_HOSTS_PATH,
	}
	// Override /etc/hosts so apply() never touches the real file.
	apply.ETC_HOSTS_PATH = h.etcHostPath
	// Seed empty /etc/hosts so apply.Apply succeeds.
	require.NoError(t, os.WriteFile(h.etcHostPath, []byte("127.0.0.1 localhost\n"), 0o644))

	// Reset all CLI flag globals (they persist across tests).
	resetGlobals()

	t.Cleanup(func() {
		apply.ETC_HOSTS_PATH = h.origEtcPath
		resetGlobals()
	})
	return h
}

func resetGlobals() {
	hostsFilePath = "~/.hosts"
	addDisabled = false
	addComment = ""
	addGroup = ""
	addDryRun = false
	rmDryRun = false
	enableDryRun = false
	disableDryRun = false
	listJSON = false
	applyDryRun = false
	groupDescription = ""
	groupAddDryRun = false
}

// run executes a cobra command with the given args and captures combined output.
func (h *testHarness) run(args ...string) (string, error) {
	h.t.Helper()
	resetGlobals()
	root := NewRootCmd("test")
	// Always inject --file pointing to the harness's hosts file.
	full := append([]string{"--file", h.hostsPath}, args...)
	root.SetArgs(full)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	return buf.String(), err
}

// runCapture executes the command but also captures os.Stdout writes
// (commands that fmt.Println go to real stdout, not cobra's writer).
func (h *testHarness) runCapture(args ...string) (string, error) {
	h.t.Helper()
	resetGlobals()
	// Redirect os.Stdout to a pipe.
	origStdout := os.Stdout
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(h.t, err)
	os.Stdout = w
	os.Stderr = w
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	root := NewRootCmd("test")
	full := append([]string{"--file", h.hostsPath}, args...)
	root.SetArgs(full)
	cobraBuf := &bytes.Buffer{}
	root.SetOut(cobraBuf)
	root.SetErr(cobraBuf)
	cmdErr := root.Execute()

	// Close writer and drain.
	require.NoError(h.t, w.Close())
	out, _ := readAll(r)
	return cobraBuf.String() + string(out), cmdErr
}

func readAll(r *os.File) ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.Bytes(), nil
}

func (h *testHarness) readHosts() domain.HostsFile {
	h.t.Helper()
	hf, err := fileio.ReadHostsFile(h.hostsPath)
	require.NoError(h.t, err)
	return hf
}

func (h *testHarness) writeHosts(hf domain.HostsFile) {
	h.t.Helper()
	require.NoError(h.t, fileio.WriteHostsFile(h.hostsPath, hf))
}

func seedHostsFile(h *testHarness, entries ...domain.Entry) {
	h.t.Helper()
	hf := domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name:    "default",
				Entries: append([]domain.Entry{}, entries...),
				Groups:  []domain.Group{},
			},
		},
	}
	// Ensure entries have IDs.
	for i := range hf.Groups[0].Entries {
		if hf.Groups[0].Entries[i].ID == "" {
			hf.Groups[0].Entries[i].ID = domain.NewID()
		}
	}
	h.writeHosts(hf)
}

// ============================================================================
// add command
// ============================================================================

func TestAddCmd_Success_DefaultGroup(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test")
	require.NoError(t, err)

	hf := h.readHosts()
	require.Len(t, hf.Groups, 1)
	require.Equal(t, "default", hf.Groups[0].Name)
	require.Len(t, hf.Groups[0].Entries, 1)
	e := hf.Groups[0].Entries[0]
	require.Equal(t, "10.0.0.1", e.IP)
	require.Equal(t, "example.test", e.Hostname)
	require.True(t, e.Enabled)
	require.NotEmpty(t, e.ID)
}

func TestAddCmd_Success_WithAliases(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "alias1.test", "alias2.test")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Equal(t, []string{"alias1.test", "alias2.test"}, hf.Groups[0].Entries[0].Aliases)
}

func TestAddCmd_Success_WithComment(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "-c", "test comment")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Equal(t, "test comment", hf.Groups[0].Entries[0].Comment)
}

func TestAddCmd_Success_DisabledFlag(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "--disabled")
	require.NoError(t, err)
	hf := h.readHosts()
	require.False(t, hf.Groups[0].Entries[0].Enabled)
}

func TestAddCmd_Success_NewGroup(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "-g", "work")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups, 1)
	require.Equal(t, "work", hf.Groups[0].Name)
}

func TestAddCmd_Success_ExistingGroup(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
		},
	})
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "-g", "work")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups, 1)
	require.Len(t, hf.Groups[0].Entries, 1)
}

func TestAddCmd_Error_InvalidIP(t *testing.T) {
	h := newHarness(t)
	out, err := h.runCapture("add", "not-an-ip", "example.test")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(out+err.Error()), "invalid ip")
}

func TestAddCmd_Error_InvalidHostname(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "-bad-hostname")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "hostname")
}

func TestAddCmd_Error_InvalidAlias(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1", "example.test", "bad_alias")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "alias")
}

func TestAddCmd_Error_DuplicateHostname(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "dup.test", Enabled: true})
	_, err := h.runCapture("add", "10.0.0.2", "dup.test")
	require.Error(t, err)
}

func TestAddCmd_DryRun_NoWrite(t *testing.T) {
	h := newHarness(t)
	// Seed an empty hosts file so apply preview works.
	h.writeHosts(domain.HostsFile{Version: 1, Groups: []domain.Group{}})
	out, err := h.runCapture("add", "10.0.0.1", "example.test", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "DRY RUN")
	// File contents unchanged (still no entries).
	hf := h.readHosts()
	require.Empty(t, hf.Groups, "dry-run must not write entries")
}

func TestAddCmd_MissingArgs(t *testing.T) {
	h := newHarness(t)
	_, err := h.runCapture("add", "10.0.0.1")
	require.Error(t, err)
}

// ============================================================================
// rm command
// ============================================================================

func TestRmCmd_Success(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h,
		domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true},
		domain.Entry{IP: "10.0.0.2", Hostname: "other.test", Enabled: true},
	)
	_, err := h.runCapture("rm", "example.test")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups[0].Entries, 1)
	require.Equal(t, "other.test", hf.Groups[0].Entries[0].Hostname)
}

func TestRmCmd_NotFound(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	_, err := h.runCapture("rm", "nonexistent.test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestRmCmd_DryRun_NoWrite(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	out, err := h.runCapture("rm", "example.test", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "DRY RUN")
	hf := h.readHosts()
	require.Len(t, hf.Groups[0].Entries, 1, "entry should still exist after dry-run")
}

func TestRmCmd_NestedGroup(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name:    "work",
				Entries: []domain.Entry{},
				Groups: []domain.Group{
					{
						Name:    "prod",
						Entries: []domain.Entry{{ID: domain.NewID(), IP: "10.0.0.1", Hostname: "prod.test", Enabled: true}},
						Groups:  []domain.Group{},
					},
				},
			},
		},
	})
	_, err := h.runCapture("rm", "prod.test")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups[0].Groups[0].Entries, 0)
}

// ============================================================================
// enable / disable commands
// ============================================================================

func TestEnableCmd_Success(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: false})
	_, err := h.runCapture("enable", "example.test")
	require.NoError(t, err)
	require.True(t, h.readHosts().Groups[0].Entries[0].Enabled)
}

func TestDisableCmd_Success(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	_, err := h.runCapture("disable", "example.test")
	require.NoError(t, err)
	require.False(t, h.readHosts().Groups[0].Entries[0].Enabled)
}

func TestEnableCmd_NotFound(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	_, err := h.runCapture("enable", "missing.test")
	require.Error(t, err)
}

func TestDisableCmd_NotFound(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	_, err := h.runCapture("disable", "missing.test")
	require.Error(t, err)
}

func TestEnableCmd_DryRun(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: false})
	out, err := h.runCapture("enable", "example.test", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "DRY RUN")
	require.False(t, h.readHosts().Groups[0].Entries[0].Enabled, "should not change on dry-run")
}

func TestDisableCmd_DryRun(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "example.test", Enabled: true})
	out, err := h.runCapture("disable", "example.test", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "DRY RUN")
	require.True(t, h.readHosts().Groups[0].Entries[0].Enabled, "should not change on dry-run")
}

// ============================================================================
// list command
// ============================================================================

func TestListCmd_Empty(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{Version: 1, Groups: []domain.Group{}})
	out, err := h.runCapture("list")
	require.NoError(t, err)
	require.Contains(t, out, "No entries found")
}

func TestListCmd_Human(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h,
		domain.Entry{IP: "10.0.0.1", Hostname: "a.test", Enabled: true},
		domain.Entry{IP: "10.0.0.2", Hostname: "b.test", Enabled: false},
	)
	out, err := h.runCapture("list")
	require.NoError(t, err)
	require.Contains(t, out, "a.test")
	require.Contains(t, out, "b.test")
	require.Contains(t, out, "[default]")
}

func TestListCmd_JSON(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h,
		domain.Entry{IP: "10.0.0.1", Hostname: "a.test", Aliases: []string{"alias.test"}, Enabled: true},
	)
	out, err := h.runCapture("list", "--json")
	require.NoError(t, err)

	// Find the JSON portion (skip any leading non-JSON output).
	idx := strings.Index(out, "[")
	require.GreaterOrEqual(t, idx, 0, "expected JSON array in output")
	jsonStr := out[idx:]

	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "10.0.0.1", entries[0]["ip"])
	require.Equal(t, "a.test", entries[0]["hostname"])
	require.Equal(t, "default", entries[0]["group"])
	require.Equal(t, true, entries[0]["enabled"])
}

func TestListCmd_JSON_NestedGroupPath(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name:    "work",
				Entries: []domain.Entry{},
				Groups: []domain.Group{
					{
						Name:    "prod",
						Entries: []domain.Entry{{ID: domain.NewID(), IP: "10.0.0.1", Hostname: "p.test", Enabled: true}},
						Groups:  []domain.Group{},
					},
				},
			},
		},
	})
	out, err := h.runCapture("list", "--json")
	require.NoError(t, err)
	idx := strings.Index(out, "[")
	require.GreaterOrEqual(t, idx, 0)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out[idx:]), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "work/prod", entries[0]["group"])
}

// ============================================================================
// group create / group add
// ============================================================================

func TestGroupCreateCmd_Success(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{Version: 1, Groups: []domain.Group{}})
	_, err := h.runCapture("group", "create", "work")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups, 1)
	require.Equal(t, "work", hf.Groups[0].Name)
}

func TestGroupCreateCmd_WithDescription(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{Version: 1, Groups: []domain.Group{}})
	_, err := h.runCapture("group", "create", "work", "-d", "Work hosts")
	require.NoError(t, err)
	require.Equal(t, "Work hosts", h.readHosts().Groups[0].Description)
}

func TestGroupCreateCmd_Duplicate(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups:  []domain.Group{{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}}},
	})
	_, err := h.runCapture("group", "create", "work")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestGroupAddCmd_MoveToExistingGroup(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "default", Entries: []domain.Entry{{ID: domain.NewID(), IP: "10.0.0.1", Hostname: "e.test", Enabled: true}}, Groups: []domain.Group{}},
			{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
		},
	})
	_, err := h.runCapture("group", "add", "work", "e.test")
	require.NoError(t, err)
	hf := h.readHosts()
	require.Len(t, hf.Groups[0].Entries, 0, "should be removed from default")
	require.Len(t, hf.Groups[1].Entries, 1, "should be added to work")
}

func TestGroupAddCmd_NotFound(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "e.test", Enabled: true})
	_, err := h.runCapture("group", "add", "work", "missing.test")
	require.Error(t, err)
}

func TestGroupAddCmd_DryRun(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "default", Entries: []domain.Entry{{ID: domain.NewID(), IP: "10.0.0.1", Hostname: "e.test", Enabled: true}}, Groups: []domain.Group{}},
			{Name: "work", Entries: []domain.Entry{}, Groups: []domain.Group{}},
		},
	})
	out, err := h.runCapture("group", "add", "work", "e.test", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "DRY RUN")
	hf := h.readHosts()
	require.Len(t, hf.Groups[0].Entries, 1, "entry should still be in default on dry-run")
}

// ============================================================================
// apply command
// ============================================================================

func TestApplyCmd_DryRun(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "e.test", Enabled: true})
	out, err := h.runCapture("apply", "--dry-run")
	require.NoError(t, err)
	require.NotEmpty(t, out)
	// /etc/hosts should not contain managed block (was just seeded with localhost)
	etc, _ := os.ReadFile(h.etcHostPath)
	require.NotContains(t, string(etc), "e.test", "dry-run should not modify /etc/hosts")
}

func TestApplyCmd_Idempotent(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "e.test", Enabled: true})
	_, err := h.runCapture("apply")
	require.NoError(t, err)
	first, err := os.ReadFile(h.etcHostPath)
	require.NoError(t, err)

	// Apply again
	_, err = h.runCapture("apply")
	require.NoError(t, err)
	second, err := os.ReadFile(h.etcHostPath)
	require.NoError(t, err)
	require.Equal(t, string(first), string(second), "apply should be idempotent")
}

func TestApplyCmd_WritesManagedBlock(t *testing.T) {
	h := newHarness(t)
	seedHostsFile(h, domain.Entry{IP: "10.0.0.1", Hostname: "e.test", Enabled: true})
	_, err := h.runCapture("apply")
	require.NoError(t, err)
	etc, err := os.ReadFile(h.etcHostPath)
	require.NoError(t, err)
	require.Contains(t, string(etc), "e.test")
	require.Contains(t, string(etc), "10.0.0.1")
}

// ============================================================================
// root / version
// ============================================================================

func TestRoot_Version(t *testing.T) {
	h := newHarness(t)
	out, err := h.run("--version")
	require.NoError(t, err)
	require.Contains(t, out, "hostie vtest")
}

func TestRoot_Help(t *testing.T) {
	h := newHarness(t)
	out, err := h.run("--help")
	require.NoError(t, err)
	require.Contains(t, out, "add")
	require.Contains(t, out, "rm")
	require.Contains(t, out, "list")
	require.Contains(t, out, "apply")
	require.Contains(t, out, "group")
}

// ============================================================================
// TUI launch wiring (bead hosts-cli-go-mig-p4-app-wire-i6k)
// ============================================================================

// withFakeTUIRunner swaps the package-level tuiRunner with a recorder for the
// duration of a single test. The recorder captures every invocation's
// hostsPath argument so tests can assert on what the root command passed in.
func withFakeTUIRunner(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	orig := tuiRunner
	tuiRunner = func(hostsPath string) error {
		calls = append(calls, hostsPath)
		return nil
	}
	t.Cleanup(func() { tuiRunner = orig })
	return &calls
}

// TestRoot_NoSubcommand_LaunchesTUI verifies that invoking `hostie` with no
// subcommand drops into the TUI. The fake runner records the hostsPath that
// was passed in so we also assert that --file flows through to the TUI model.
func TestRoot_NoSubcommand_LaunchesTUI(t *testing.T) {
	h := newHarness(t)
	calls := withFakeTUIRunner(t)

	_, err := h.run() // no subcommand, no args
	require.NoError(t, err)
	require.Len(t, *calls, 1, "TUI runner must be invoked exactly once on bare `hostie`")
	require.Equal(t, h.hostsPath, (*calls)[0], "TUI must receive the --file path")
}

// TestRoot_Subcommand_DoesNotLaunchTUI verifies that running a subcommand
// (e.g. `hostie list`) executes the subcommand and does NOT fall through to
// the TUI runner — the existing CLI behavior must be preserved.
func TestRoot_Subcommand_DoesNotLaunchTUI(t *testing.T) {
	h := newHarness(t)
	h.writeHosts(domain.HostsFile{Version: 1, Groups: []domain.Group{}})
	calls := withFakeTUIRunner(t)

	out, err := h.runCapture("list")
	require.NoError(t, err)
	require.Empty(t, *calls, "TUI runner must NOT be invoked when a subcommand is given")
	require.Contains(t, out, "No entries found")
}

// TestRoot_HelpDoesNotLaunchTUI verifies --help short-circuits cobra's RunE
// dispatch (cobra prints help and returns before RunE fires).
func TestRoot_HelpDoesNotLaunchTUI(t *testing.T) {
	h := newHarness(t)
	calls := withFakeTUIRunner(t)

	_, err := h.run("--help")
	require.NoError(t, err)
	require.Empty(t, *calls, "--help must not launch the TUI")
}

// TestRoot_VersionDoesNotLaunchTUI verifies --version short-circuits RunE.
func TestRoot_VersionDoesNotLaunchTUI(t *testing.T) {
	h := newHarness(t)
	calls := withFakeTUIRunner(t)

	_, err := h.run("--version")
	require.NoError(t, err)
	require.Empty(t, *calls, "--version must not launch the TUI")
}

// TestRoot_UnknownArg_DoesNotLaunchTUI verifies that an unrecognized
// positional argument surfaces a clean error instead of being swallowed by
// the TUI launcher (Args: cobra.NoArgs at the root enforces this).
func TestRoot_UnknownArg_DoesNotLaunchTUI(t *testing.T) {
	h := newHarness(t)
	calls := withFakeTUIRunner(t)

	_, err := h.run("not-a-subcommand")
	require.Error(t, err)
	require.Empty(t, *calls, "unknown positional arg must not silently launch the TUI")
}
