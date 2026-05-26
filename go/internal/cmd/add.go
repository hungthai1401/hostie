package cmd

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

var (
	addDisabled bool
	addComment  string
	addGroup    string
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <ip> <hostname> [aliases...]",
		Short: "Add a new hosts entry",
		Long:  "Add a new entry to the hosts file with optional aliases and group assignment",
		Args:  cobra.MinimumNArgs(2),
		RunE:  runAdd,
	}

	cmd.Flags().BoolVar(&addDisabled, "disabled", false, "add entry in disabled state")
	cmd.Flags().StringVarP(&addComment, "comment", "c", "", "comment for the entry")
	cmd.Flags().StringVarP(&addGroup, "group", "g", "", "group to add entry to")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	ip := args[0]
	hostname := args[1]
	aliases := args[2:]

	// Validate IP address
	if err := domain.ValidateIP(ip); err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	// Validate hostname
	if err := domain.ValidateHostname(hostname); err != nil {
		return fmt.Errorf("invalid hostname: %w", err)
	}

	// Validate aliases
	if err := domain.ValidateAliases(aliases); err != nil {
		return err
	}

	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		// If file doesn't exist, create a new one
		if os.IsNotExist(err) {
			hostsFile = domain.HostsFile{
				Version: 1,
				Groups:  []domain.Group{},
			}
		} else {
			return fmt.Errorf("failed to read hosts file: %w", err)
		}
	}

	// Create new entry
	newEntry := domain.Entry{
		ID:       domain.NewID(),
		IP:       ip,
		Hostname: hostname,
		Aliases:  aliases,
		Enabled:  !addDisabled,
		Comment:  addComment,
	}

	// Check for duplicate hostnames
	allEntries := collectAllEntries(&hostsFile)
	allEntries = append(allEntries, newEntry)
	if err := domain.ValidateNoDuplicates(allEntries); err != nil {
		return err
	}

	// Add entry to the appropriate location
	if addGroup != "" {
		if err := addEntryToGroup(&hostsFile, newEntry, addGroup); err != nil {
			return err
		}
	} else {
		addEntryToRoot(&hostsFile, newEntry)
	}

	// Write back to file
	if err := fileio.WriteHostsFile(hostsFilePath, hostsFile); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	fmt.Printf("✓ Added %s (%s)\n", hostname, ip)
	return nil
}

// collectAllEntries recursively collects all entries from the hosts file
func collectAllEntries(hf *domain.HostsFile) []domain.Entry {
	var entries []domain.Entry
	for _, g := range hf.Groups {
		entries = append(entries, collectGroupEntries(&g)...)
	}
	return entries
}

// collectGroupEntries recursively collects entries from a group and its subgroups
func collectGroupEntries(g *domain.Group) []domain.Entry {
	entries := make([]domain.Entry, len(g.Entries))
	copy(entries, g.Entries)
	for _, subgroup := range g.Groups {
		entries = append(entries, collectGroupEntries(&subgroup)...)
	}
	return entries
}

// addEntryToRoot adds an entry to the root level (creates a default group if needed)
func addEntryToRoot(hf *domain.HostsFile, entry domain.Entry) {
	// If there are no groups, create a default one
	if len(hf.Groups) == 0 {
		hf.Groups = []domain.Group{
			{
				Name:        "default",
				Description: "Default group",
				Entries:     []domain.Entry{entry},
				Groups:      []domain.Group{},
			},
		}
		return
	}

	// Add to the first group (default behavior)
	hf.Groups[0].Entries = append(hf.Groups[0].Entries, entry)
}

// addEntryToGroup adds an entry to a specified group (creates group if it doesn't exist)
func addEntryToGroup(hf *domain.HostsFile, entry domain.Entry, groupName string) error {
	// Find or create the group
	for i := range hf.Groups {
		if hf.Groups[i].Name == groupName {
			hf.Groups[i].Entries = append(hf.Groups[i].Entries, entry)
			return nil
		}
	}

	// Group doesn't exist, create it
	newGroup := domain.Group{
		Name:        groupName,
		Description: "",
		Entries:     []domain.Entry{entry},
		Groups:      []domain.Group{},
	}
	hf.Groups = append(hf.Groups, newGroup)
	return nil
}
