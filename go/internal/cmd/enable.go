package cmd

import (
	"fmt"

	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

func newEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <hostname>",
		Short: "Enable a hosts entry",
		Long:  "Enable an entry in the hosts file by hostname",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnable,
	}

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

	return cmd
}

func runEnable(cmd *cobra.Command, args []string) error {
	return setEntryEnabled(args[0], true)
}

func runDisable(cmd *cobra.Command, args []string) error {
	return setEntryEnabled(args[0], false)
}

func setEntryEnabled(hostname string, enabled bool) error {
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

	// Write back to file
	if err := fileio.WriteHostsFile(hostsFilePath, hostsFile); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	fmt.Printf("✓ %s %s\n", hostname, action)
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
