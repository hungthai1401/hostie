// Package fileio is the single seam for ~/.hosts file I/O.
//
// Critical Pattern: One I/O Funnel. All ~/.hosts read/write operations MUST
// go through ReadHostsFile and WriteHostsFile in this package. No other
// internal package should call os.UserHomeDir directly — this ensures tilde
// expansion is consistent and testable via HOME env injection.
package fileio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hungthai1401/hostie/go/internal/core/yaml"
	"github.com/hungthai1401/hostie/go/internal/domain"
)

// ReadHostsFile reads and parses a hosts file from the given path.
// Tilde (~) is expanded to the user's home directory at call time.
// Returns ENOENT if the file does not exist, or a parse error if the YAML is malformed.
func ReadHostsFile(path string) (domain.HostsFile, error) {
	expanded, err := expandTilde(path)
	if err != nil {
		return domain.HostsFile{}, fmt.Errorf("fileio.ReadHostsFile: %w", err)
	}

	data, err := os.ReadFile(expanded)
	if err != nil {
		return domain.HostsFile{}, fmt.Errorf("fileio.ReadHostsFile: %w", err)
	}

	hf, err := yaml.Unmarshal(data)
	if err != nil {
		return domain.HostsFile{}, fmt.Errorf("fileio.ReadHostsFile: %w", err)
	}

	return *hf, nil
}

// WriteHostsFile marshals and writes a HostsFile to the given path.
// Tilde (~) is expanded to the user's home directory at call time.
// The file is created with mode 0o644. Parent directories must already exist.
func WriteHostsFile(path string, hf domain.HostsFile) error {
	expanded, err := expandTilde(path)
	if err != nil {
		return fmt.Errorf("fileio.WriteHostsFile: %w", err)
	}

	data, err := yaml.Marshal(&hf)
	if err != nil {
		return fmt.Errorf("fileio.WriteHostsFile: %w", err)
	}

	if err := os.WriteFile(expanded, data, 0o644); err != nil {
		return fmt.Errorf("fileio.WriteHostsFile: %w", err)
	}

	return nil
}

// expandTilde expands a leading ~ to the user's home directory.
// Calls os.UserHomeDir() at call time (NOT cached) so tests can inject HOME via t.Setenv.
// Paths without a leading ~ are returned unchanged.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expandTilde: failed to get home directory: %w", err)
	}

	if path == "~" {
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}

	// ~username is not supported
	return "", fmt.Errorf("expandTilde: ~username syntax not supported: %s", path)
}
