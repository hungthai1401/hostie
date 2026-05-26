package cmd

import (
	"fmt"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/fileio"
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
	// Use the apply.Runner to orchestrate the workflow
	result, err := apply.ApplyFromFile(hostsFilePath, applyDryRun)
	if err != nil {
		return err
	}

	if applyDryRun {
		// Read hosts file for preview
		hostsFile, err := fileio.ReadHostsFile(hostsFilePath)
		if err != nil {
			return fmt.Errorf("failed to read hosts file: %w", err)
		}
		preview := apply.RenderPreview(hostsFile)
		fmt.Println("# Dry run - would apply the following to /etc/hosts:")
		fmt.Println(preview)
		return nil
	}

	// Print result message
	if result.Changed {
		fmt.Printf("✓ %s\n", result.Message)
	} else {
		fmt.Printf("○ %s\n", result.Message)
	}

	return nil
}
