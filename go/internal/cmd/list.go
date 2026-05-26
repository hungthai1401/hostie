package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/core/fileio"
	"github.com/hungthai1401/hostie/go/internal/domain"
	"github.com/spf13/cobra"
)

var (
	listJSON bool
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all hosts entries",
		Long:  "List all entries in the hosts file with optional JSON output",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}

	cmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	// Read existing hosts file
	hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts file: %w", err)
	}

	if listJSON {
		return outputJSON(hostsFile)
	}

	return outputHuman(hostsFile)
}

func outputJSON(hf domain.HostsFile) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(hf); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func outputHuman(hf domain.HostsFile) error {
	if len(hf.Groups) == 0 {
		fmt.Println("No entries found")
		return nil
	}

	for _, group := range hf.Groups {
		printGroup(&group, 0)
	}

	return nil
}

func printGroup(g *domain.Group, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	// Print group header
	if g.Name != "" {
		fmt.Printf("%s[%s]\n", prefix, g.Name)
		if g.Description != "" {
			fmt.Printf("%s  %s\n", prefix, g.Description)
		}
	}

	// Print entries
	for _, entry := range g.Entries {
		status := "✓"
		if !entry.Enabled {
			status = "✗"
		}

		aliasStr := ""
		if len(entry.Aliases) > 0 {
			aliasStr = fmt.Sprintf(" (aliases: %v)", entry.Aliases)
		}

		commentStr := ""
		if entry.Comment != "" {
			commentStr = fmt.Sprintf(" # %s", entry.Comment)
		}

		fmt.Printf("%s  %s %s → %s%s%s\n", prefix, status, entry.Hostname, entry.IP, aliasStr, commentStr)
	}

	// Print subgroups
	for i := range g.Groups {
		printGroup(&g.Groups[i], indent+1)
	}
}
