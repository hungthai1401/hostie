package cmd

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/src/internal/apply"
	"github.com/hungthai1401/hostie/src/internal/core/fileio"
	"github.com/hungthai1401/hostie/src/internal/domain"
	"github.com/spf13/cobra"
)

var rmDryRun bool

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <hostname>",
		Short: "Remove a hosts entry",
		Long:  "Remove an entry from the hosts file by hostname",
		Args:  cobra.ExactArgs(1),
		RunE:  runRm,
	}

	cmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "preview changes without writing")

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

	// Dry-run: show what would be removed
	if rmDryRun {
		fmt.Printf("[DRY RUN] Would remove: %s\n", hostname)
		
		// Show apply preview
		result, err := apply.ApplyFromFile(hostsFilePath, true)
		if err != nil {
			return fmt.Errorf("failed to preview apply: %w", err)
		}
		if result.Changed {
			fmt.Println("\n[DRY RUN] Would apply to /etc/hosts:")
			fmt.Println(result.Message)
		} else {
			fmt.Println("\n[DRY RUN] No changes to /etc/hosts (already up to date)")
		}
		return nil
	}

	// Write to ~/.hosts
	if err := fileio.WriteHostsFile(hostsFilePath, hostsFile); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	fmt.Printf("✓ Removed %s\n", hostname)

	// Auto-apply to /etc/hosts (D11)
	result, err := apply.ApplyFromFile(hostsFilePath, false)
	if err != nil {
		// D13: Keep YAML change, print error, exit with appropriate code
		fmt.Fprintf(os.Stderr, "Warning: Failed to apply to /etc/hosts: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'hostie apply' to retry.\n")
		return err
	}

	if result.Changed {
		fmt.Println("✓ Applied to /etc/hosts")
	} else {
		fmt.Println("✓ /etc/hosts already up to date")
	}

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
