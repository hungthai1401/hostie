package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// hostsFilePath is the path to the ~/.hosts file (can be overridden with --file flag)
	hostsFilePath string
)

// NewRootCmd creates the root command for hostie
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hostie",
		Short:         "hostie — hosts file manager",
		Long:          "Manage /etc/hosts entries through a YAML source file with groups and validation",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Match the prior output shape: "hostie v<version>\n".
	// Tag refs already strip leading "v" in the release workflow, so a tag
	// like "v1.0.0" yields version="1.0.0" → "hostie v1.0.0".
	cmd.SetVersionTemplate("hostie v{{.Version}}\n")

	// Global flags
	cmd.PersistentFlags().StringVarP(&hostsFilePath, "file", "f", "~/.hosts", "path to hosts file")

	// Add subcommands
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRmCmd())
	cmd.AddCommand(newEnableCmd())
	cmd.AddCommand(newDisableCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newGroupCmd())

	return cmd
}

// Execute runs the root command
func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
