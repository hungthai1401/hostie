package cmd

import (
	"fmt"

	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <hostname>",
		Short: "Remove a hosts entry",
		Long:  "Remove an entry from the hosts file by hostname",
		Args:  cobra.ExactArgs(1),
		RunE:  runRm,
	}

	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	hostname := args[0]

	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Remove entry by hostname
	found := removeEntryByHostname(&hostsFile, hostname)
	if !found {
		return fmt.Errorf("entry not found: %s", hostname)
	}

	// Write back to file
	if err := fileio.WriteHostsFile(hostsFilePath, hostsFile); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	fmt.Printf("✓ Removed %s\n", hostname)
	return nil
}

// removeEntryByHostname removes an entry from the hosts file by hostname
// Returns true if the entry was found and removed, false otherwise
func removeEntryByHostname(hf *domain.HostsFile, hostname string) bool {
	for i := range hf.Groups {
		if removeEntryFromGroup(&hf.Groups[i], hostname) {
			return true
		}
	}
	return false
}

// removeEntryFromGroup recursively removes an entry from a group and its subgroups
func removeEntryFromGroup(g *domain.Group, hostname string) bool {
	// Check entries in this group
	for i, entry := range g.Entries {
		if entry.Hostname == hostname {
			// Remove entry by slicing
			g.Entries = append(g.Entries[:i], g.Entries[i+1:]...)
			return true
		}
	}

	// Check subgroups
	for i := range g.Groups {
		if removeEntryFromGroup(&g.Groups[i], hostname) {
			return true
		}
	}

	return false
}
