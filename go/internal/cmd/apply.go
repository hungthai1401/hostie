package cmd

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/core/render"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

var (
	applyDryRun bool
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply hosts entries to /etc/hosts",
		Long:  "Apply enabled entries from the hosts file to /etc/hosts with managed block",
		Args:  cobra.NoArgs,
		RunE:  runApply,
	}

	cmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "show what would be applied without writing")

	return cmd
}

func runApply(cmd *cobra.Command, args []string) error {
	// Read hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Render managed block
	managedBlock := render.RenderManagedBlock(&hostsFile)

	if applyDryRun {
		fmt.Println("# Dry run - would apply the following to /etc/hosts:")
		fmt.Println(managedBlock)
		return nil
	}

	// Read current /etc/hosts
	etcHostsPath := "/etc/hosts"
	currentContent, err := os.ReadFile(etcHostsPath)
	if err != nil {
		return fmt.Errorf("failed to read /etc/hosts: %w", err)
	}

	// Extract managed block to check if update is needed
	_, oldManaged, _, err := etchosts.ExtractManagedBlock(currentContent)
	if err != nil {
		return fmt.Errorf("failed to extract managed block: %w", err)
	}

	// Check if content has changed (idempotency)
	newManagedBytes := []byte(managedBlock)
	if string(oldManaged) == managedBlock {
		fmt.Println("✓ /etc/hosts is already up to date")
		return nil
	}

	// Replace managed block
	newContent, err := etchosts.ReplaceManagedBlock(currentContent, newManagedBytes)
	if err != nil {
		return fmt.Errorf("failed to replace managed block: %w", err)
	}

	// Write atomically
	if err := etchosts.WriteEtcHosts(etcHostsPath, string(newContent)); err != nil {
		// Check if it's a permission error
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: try running with sudo")
		}
		return fmt.Errorf("failed to write /etc/hosts: %w", err)
	}

	// Count enabled entries
	enabledCount := countEnabledEntries(&hostsFile)
	fmt.Printf("✓ Applied %d entries to /etc/hosts\n", enabledCount)
	return nil
}

// countEnabledEntries counts the number of enabled entries in the hosts file
func countEnabledEntries(hf *domain.HostsFile) int {
	count := 0
	for _, g := range hf.Groups {
		count += countGroupEnabledEntries(&g)
	}
	return count
}

// countGroupEnabledEntries recursively counts enabled entries in a group
func countGroupEnabledEntries(g *domain.Group) int {
	count := 0
	for _, entry := range g.Entries {
		if entry.Enabled {
			count++
		}
	}
	for i := range g.Groups {
		count += countGroupEnabledEntries(&g.Groups[i])
	}
	return count
}
