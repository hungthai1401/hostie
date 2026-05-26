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
	groupDescription string
	groupAddDryRun   bool
)

func newGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage groups",
		Long:  "Create and manage groups for organizing hosts entries",
	}

	cmd.AddCommand(newGroupCreateCmd())
	cmd.AddCommand(newGroupAddCmd())

	return cmd
}

func newGroupCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new group",
		Long:  "Create a new group for organizing hosts entries",
		Args:  cobra.ExactArgs(1),
		RunE:  runGroupCreate,
	}

	cmd.Flags().StringVarP(&groupDescription, "description", "d", "", "group description")

	return cmd
}

func newGroupAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <group> <hostname>",
		Short: "Add an entry to a group",
		Long:  "Move an existing entry to a specified group",
		Args:  cobra.ExactArgs(2),
		RunE:  runGroupAdd,
	}

	cmd.Flags().BoolVar(&groupAddDryRun, "dry-run", false, "preview changes without writing")

	return cmd
}

func runGroupCreate(cmd *cobra.Command, args []string) error {
	groupName := args[0]

	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Check if group already exists
	for _, g := range hostsFile.Groups {
		if g.Name == groupName {
			return fmt.Errorf("group already exists: %s", groupName)
		}
	}

	// Create new group
	newGroup := domain.Group{
		Name:        groupName,
		Description: groupDescription,
		Entries:     []domain.Entry{},
		Groups:      []domain.Group{},
	}

	hostsFile.Groups = append(hostsFile.Groups, newGroup)

	// Write back to file
	if err := fileio.WriteHostsFile(hostsFilePath, hostsFile); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	fmt.Printf("✓ Created group: %s\n", groupName)
	return nil
}

func runGroupAdd(cmd *cobra.Command, args []string) error {
	groupName := args[0]
	hostname := args[1]

	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	// Find the entry
	entry, found := findAndRemoveEntry(&hostsFile, hostname)
	if !found {
		return fmt.Errorf("entry not found: %s", hostname)
	}

	// Find or create the target group
	targetGroup := findGroupByName(&hostsFile, groupName)
	if targetGroup == nil {
		// Create the group if it doesn't exist
		newGroup := domain.Group{
			Name:        groupName,
			Description: "",
			Entries:     []domain.Entry{entry},
			Groups:      []domain.Group{},
		}
		hostsFile.Groups = append(hostsFile.Groups, newGroup)
	} else {
		// Add to existing group
		targetGroup.Entries = append(targetGroup.Entries, entry)
	}

	// Dry-run: show what would be moved
	if groupAddDryRun {
		fmt.Printf("[DRY RUN] Would move %s to group: %s\n", hostname, groupName)
		
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

	fmt.Printf("✓ Moved %s to group: %s\n", hostname, groupName)

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

// findAndRemoveEntry finds an entry by hostname and removes it from the hosts file
// Returns the entry and true if found, zero value and false otherwise
func findAndRemoveEntry(hf *domain.HostsFile, hostname string) (domain.Entry, bool) {
	for i := range hf.Groups {
		if entry, found := findAndRemoveFromGroup(&hf.Groups[i], hostname); found {
			return entry, true
		}
	}
	return domain.Entry{}, false
}

// findAndRemoveFromGroup recursively finds and removes an entry from a group
func findAndRemoveFromGroup(g *domain.Group, hostname string) (domain.Entry, bool) {
	// Check entries in this group
	for i, entry := range g.Entries {
		if entry.Hostname == hostname {
			// Remove and return the entry
			g.Entries = append(g.Entries[:i], g.Entries[i+1:]...)
			return entry, true
		}
	}

	// Check subgroups
	for i := range g.Groups {
		if entry, found := findAndRemoveFromGroup(&g.Groups[i], hostname); found {
			return entry, true
		}
	}

	return domain.Entry{}, false
}

// findGroupByName finds a group by name (non-recursive, top-level only)
func findGroupByName(hf *domain.HostsFile, name string) *domain.Group {
	for i := range hf.Groups {
		if hf.Groups[i].Name == name {
			return &hf.Groups[i]
		}
	}
	return nil
}
