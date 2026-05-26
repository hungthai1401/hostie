package cmd

import (
	"fmt"
	"os"

	"github.com/hungthai1401/hostie/go/internal/apply"
	"github.com/hungthai1401/hostie/go/internal/core/etchosts"
	"github.com/spf13/cobra"
)

// applyPrivilegedCmd is a hidden subcommand used internally for privileged apply
// This command is invoked by sudo when privilege escalation is needed
var applyPrivilegedCmd = &cobra.Command{
	Use:    apply.APPLY_PRIVILEGED_CMD + " <payload-file>",
	Short:  "Internal command for privileged /etc/hosts write (do not call directly)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payloadPath := args[0]

		// Validate and read payload file
		content, err := apply.ValidatePayloadFile(payloadPath)
		if err != nil {
			return fmt.Errorf("payload validation failed: %w", err)
		}

		// Ensure cleanup happens on all exit paths
		defer func() {
			if err := os.Remove(payloadPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove payload file: %v\n", err)
			}
		}()

		// Write to /etc/hosts atomically
		if err := etchosts.WriteEtcHosts(apply.ETC_HOSTS_PATH, string(content)); err != nil {
			return fmt.Errorf("failed to write /etc/hosts: %w", err)
		}

		return nil
	},
}

