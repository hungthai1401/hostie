package cmd

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

var (
	enableDryRun  bool
	disableDryRun bool
)

func newEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <hostname>",
		Short: "Enable a hosts entry",
		Long:  "Enable an entry in the hosts file by hostname",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnable,
	}

	cmd.Flags().BoolVar(&enableDryRun, "dry-run", false, "preview changes without writing")

	return cmd
}

func newDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <hostname>",
		Short: "Disable a hosts entry",
		Long:  "Disable an entry in the hosts file by hostname",
		Args:  cobra.ExactArgs(1),
		RunE:  runDisable,
	}

	cmd.Flags().BoolVar(&disableDryRun, "dry-run", false, "preview changes without writing")

	return cmd
}

func runEnable(cmd *cobra.Command, args []string) error {
	return setEntryEnabled(args[0], true, enableDryRun)
}

func runDisable(cmd *cobra.Command, args []string) error {
	return setEntryEnabled(args[0], false, disableDryRun)
}

func setEntryEnabled(hostname string, enabled bool, dryRun bool) error {
	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Set entry enabled state
	found := setEntryEnabledByHostname(&hostsFile, hostname, enabled)
	if !found {
		return fmt.Errorf("entry not found: %s", hostname)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	// Dry-run: show what would be changed
	if dryRun {
		fmt.Printf("[DRY RUN] Would %s: %s\n", action, hostname)
		
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

	fmt.Printf("✓ %s %s\n", hostname, action)

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

// setEntryEnabledByHostname sets the enabled state of an entry by hostname
// Returns true if the entry was found, false otherwise
func setEntryEnabledByHostname(hf *domain.HostsFile, hostname string, enabled bool) bool {
	for i := range hf.Groups {
		if setGroupEntryEnabled(&hf.Groups[i], hostname, enabled) {
			return true
		}
	}
	return false
}

// setGroupEntryEnabled recursively sets the enabled state in a group and its subgroups
func setGroupEntryEnabled(g *domain.Group, hostname string, enabled bool) bool {
	// Check entries in this group
	for i := range g.Entries {
		if g.Entries[i].Hostname == hostname {
			g.Entries[i].Enabled = enabled
			return true
		}
	}

	// Check subgroups
	for i := range g.Groups {
		if setGroupEntryEnabled(&g.Groups[i], hostname, enabled) {
			return true
		}
	}

	return false
}
