package fileio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

func TestReadHostsFile(t *testing.T) {
	t.Run("tilde expansion with valid file", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		hostsPath := filepath.Join(tmpHome, ".hosts")
		validYAML := `version: 1
groups:
  - name: test-group
    entries:
      - id: 01ARZ3NDEKTSV4RRFFQ69G5FAV
        ip: 127.0.0.1
        hostname: localhost
        enabled: true
`
		if err := os.WriteFile(hostsPath, []byte(validYAML), 0o644); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		hf, err := ReadHostsFile("~/.hosts")
		if err != nil {
			t.Fatalf("ReadHostsFile failed: %v", err)
		}

		if hf.Version != 1 {
			t.Errorf("expected version 1, got %d", hf.Version)
		}
		if len(hf.Groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(hf.Groups))
		}
		if hf.Groups[0].Name != "test-group" {
			t.Errorf("expected group name 'test-group', got %q", hf.Groups[0].Name)
		}
	})

	t.Run("absolute path passes through unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "hosts.yaml")
		validYAML := `version: 1
groups: []
`
		if err := os.WriteFile(hostsPath, []byte(validYAML), 0o644); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		hf, err := ReadHostsFile(hostsPath)
		if err != nil {
			t.Fatalf("ReadHostsFile failed: %v", err)
		}

		if hf.Version != 1 {
			t.Errorf("expected version 1, got %d", hf.Version)
		}
	})

	t.Run("ENOENT on missing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		missingPath := filepath.Join(tmpDir, "nonexistent.yaml")

		_, err := ReadHostsFile(missingPath)
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("expected ENOENT error, got: %v", err)
		}
	})

	t.Run("malformed YAML returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "bad.yaml")
		badYAML := `version: 1
groups:
  - name: test
    entries: "not a list"
`
		if err := os.WriteFile(hostsPath, []byte(badYAML), 0o644); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		_, err := ReadHostsFile(hostsPath)
		if err == nil {
			t.Fatal("expected error for malformed YAML, got nil")
		}
		if !strings.Contains(err.Error(), "yaml") && !strings.Contains(err.Error(), "Unmarshal") {
			t.Errorf("expected YAML parse error, got: %v", err)
		}
	})

	t.Run("missing version field returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "no-version.yaml")
		noVersionYAML := `groups: []
`
		if err := os.WriteFile(hostsPath, []byte(noVersionYAML), 0o644); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		_, err := ReadHostsFile(hostsPath)
		if err == nil {
			t.Fatal("expected error for missing version, got nil")
		}
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("expected version error, got: %v", err)
		}
	})
}

func TestWriteHostsFile(t *testing.T) {
	t.Run("write creates file with mode 0o644", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "hosts.yaml")

		hf := domain.HostsFile{
			Version: 1,
			Groups: []domain.Group{
				{
					Name: "test-group",
					Entries: []domain.Entry{
						{
							ID:       "01ARZ3NDEKTSV4RRFFQ69G5FAV",
							IP:       "192.168.1.1",
							Hostname: "example.local",
							Enabled:  true,
						},
					},
				},
			},
		}

		if err := WriteHostsFile(hostsPath, hf); err != nil {
			t.Fatalf("WriteHostsFile failed: %v", err)
		}

		info, err := os.Stat(hostsPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}

		if info.Mode().Perm() != 0o644 {
			t.Errorf("expected mode 0o644, got %o", info.Mode().Perm())
		}

		// Verify content is valid YAML
		data, err := os.ReadFile(hostsPath)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if !strings.Contains(string(data), "version: 1") {
			t.Errorf("expected 'version: 1' in output, got: %s", string(data))
		}
	})

	t.Run("tilde expansion on write", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		hf := domain.HostsFile{
			Version: 1,
			Groups:  []domain.Group{},
		}

		if err := WriteHostsFile("~/.hosts", hf); err != nil {
			t.Fatalf("WriteHostsFile failed: %v", err)
		}

		expectedPath := filepath.Join(tmpHome, ".hosts")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("file not created at expected path %s: %v", expectedPath, err)
		}
	})

	t.Run("write to subdir without parent returns ENOENT", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "nonexistent", "hosts.yaml")

		hf := domain.HostsFile{
			Version: 1,
			Groups:  []domain.Group{},
		}

		err := WriteHostsFile(hostsPath, hf)
		if err == nil {
			t.Fatal("expected error for missing parent directory, got nil")
		}
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("expected ENOENT error, got: %v", err)
		}
	})

	t.Run("write absolute path", func(t *testing.T) {
		tmpDir := t.TempDir()
		hostsPath := filepath.Join(tmpDir, "absolute.yaml")

		hf := domain.HostsFile{
			Version: 1,
			Groups:  []domain.Group{},
		}

		if err := WriteHostsFile(hostsPath, hf); err != nil {
			t.Fatalf("WriteHostsFile failed: %v", err)
		}

		if _, err := os.Stat(hostsPath); err != nil {
			t.Errorf("file not created: %v", err)
		}
	})
}

func TestExpandTilde(t *testing.T) {
	t.Run("tilde alone expands to HOME", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		expanded, err := expandTilde("~")
		if err != nil {
			t.Fatalf("expandTilde failed: %v", err)
		}
		if expanded != tmpHome {
			t.Errorf("expected %q, got %q", tmpHome, expanded)
		}
	})

	t.Run("tilde slash expands correctly", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		expanded, err := expandTilde("~/.hosts")
		if err != nil {
			t.Fatalf("expandTilde failed: %v", err)
		}

		expected := filepath.Join(tmpHome, ".hosts")
		if expanded != expected {
			t.Errorf("expected %q, got %q", expected, expanded)
		}
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		absPath := "/etc/hosts"
		expanded, err := expandTilde(absPath)
		if err != nil {
			t.Fatalf("expandTilde failed: %v", err)
		}
		if expanded != absPath {
			t.Errorf("expected %q, got %q", absPath, expanded)
		}
	})

	t.Run("relative path unchanged", func(t *testing.T) {
		relPath := "config/hosts.yaml"
		expanded, err := expandTilde(relPath)
		if err != nil {
			t.Fatalf("expandTilde failed: %v", err)
		}
		if expanded != relPath {
			t.Errorf("expected %q, got %q", relPath, expanded)
		}
	})

	t.Run("tilde username syntax not supported", func(t *testing.T) {
		_, err := expandTilde("~otheruser/.hosts")
		if err == nil {
			t.Fatal("expected error for ~username syntax, got nil")
		}
		if !strings.Contains(err.Error(), "not supported") {
			t.Errorf("expected 'not supported' error, got: %v", err)
		}
	})
}
