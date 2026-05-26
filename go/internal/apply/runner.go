package apply

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/core/render"
	"github.com/hungthai1401/hostie/go/internal/core/yaml"
	"github.com/hungthai1401/hostie/go/internal/domain"
)

// ApplyResult represents the result of applying changes to /etc/hosts
type ApplyResult struct {
	Changed bool
	Message string
}

// Runner orchestrates the apply workflow: YAML write → render → /etc/hosts write
type Runner struct {
	hostsFilePath string
	dryRun        bool
}

// NewRunner creates a new apply runner
func NewRunner(hostsFilePath string, dryRun bool) *Runner {
	return &Runner{
		hostsFilePath: hostsFilePath,
		dryRun:        dryRun,
	}
}

// Apply executes the full apply workflow
func (r *Runner) Apply(hostsFile domain.HostsFile) (*ApplyResult, error) {
	// Step 1: Write to ~/.hosts (unless dry-run)
	if !r.dryRun {
		if err := fileio.WriteHostsFile(r.hostsFilePath, hostsFile); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", r.hostsFilePath, err)
		}
	}

	// Step 2: Render the managed block
	newBlock := render.RenderManagedBlock(&hostsFile)

	// Step 3: Read current /etc/hosts content
	currentContent, err := os.ReadFile(ETC_HOSTS_PATH)
	if err != nil {
		if os.IsNotExist(err) {
			return &ApplyResult{
				Changed: false,
				Message: fmt.Sprintf("/etc/hosts not found: %v", err),
			}, nil
		}
		if os.IsPermission(err) {
			return &ApplyResult{
				Changed: false,
				Message: fmt.Sprintf("Permission denied reading /etc/hosts (may need sudo): %v", err),
			}, nil
		}
		return nil, fmt.Errorf("failed to read /etc/hosts: %w", err)
	}

	// Step 4: Build new content by replacing managed block
	newContent, err := etchosts.ReplaceManagedBlock(currentContent, []byte(newBlock))
	if err != nil {
		return nil, fmt.Errorf("failed to replace managed block: %w", err)
	}

	// Step 5: Check if content changed (idempotency)
	if string(newContent) == string(currentContent) {
		return &ApplyResult{
			Changed: false,
			Message: "/etc/hosts is already up to date (no changes needed)",
		}, nil
	}

	// Step 6: Dry-run mode - show preview without writing
	if r.dryRun {
		return &ApplyResult{
			Changed: false,
			Message: "Dry-run mode: would update /etc/hosts",
		}, nil
	}

	// Step 7: Write to /etc/hosts (with privilege escalation if needed)
	if err := r.writeEtcHosts(newContent); err != nil {
		return nil, err
	}

	return &ApplyResult{
		Changed: true,
		Message: "/etc/hosts updated successfully",
	}, nil
}

// writeEtcHosts writes content to /etc/hosts, escalating with sudo if needed
func (r *Runner) writeEtcHosts(content []byte) error {
	// Try direct write first
	if CanWriteEtcHosts() {
		return etchosts.WriteEtcHosts(ETC_HOSTS_PATH, string(content))
	}

	// Need privilege escalation - use sudo reexec
	// Write payload to tempfile
	if _, cleanup, err := WritePayloadToTempfile(content); err != nil {
		return fmt.Errorf("failed to create payload tempfile: %w", err)
	} else {
		defer cleanup()
	}

	// Re-exec with sudo, passing tempfile path
	return ReexecWithSudo()
}

// ApplyFromFile reads a hosts file and applies it
func ApplyFromFile(hostsFilePath string, dryRun bool) (*ApplyResult, error) {
	// Read the hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", hostsFilePath, err)
	}

	// Create runner and apply
	runner := NewRunner(hostsFilePath, dryRun)
	return runner.Apply(hostsFile)
}

// RenderPreview renders a preview of what would be written to /etc/hosts
func RenderPreview(hostsFile domain.HostsFile) string {
	return render.RenderManagedBlock(&hostsFile)
}

// MarshalHostsFile converts a HostsFile to YAML bytes
func MarshalHostsFile(hostsFile domain.HostsFile) ([]byte, error) {
	return yaml.Marshal(&hostsFile)
}
