package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/hungthai1401/hostie/go/internal/tui/app"
)

var (
	// hostsFilePath is the path to the ~/.hosts file (can be overridden with --file flag)
	hostsFilePath string

	// tuiRunner launches the Bubble Tea TUI. It is a package-level variable so
	// tests can swap in a fake runner and verify the root command wires no-arg
	// invocation through to the TUI without spinning up a real terminal program.
	tuiRunner = defaultTUIRunner
)

// defaultTUIRunner is the production TUI launcher: it constructs the root
// app.Model rooted at hostsPath and runs a Bubble Tea program in altscreen
// mode (matches the v1 Ink behavior of taking over the terminal).
func defaultTUIRunner(hostsPath string) error {
	p := tea.NewProgram(app.NewModel(hostsPath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// NewRootCmd creates the root command for hostie
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hostie",
		Short:         "hostie — hosts file manager",
		Long:          "Manage /etc/hosts entries through a YAML source file with groups and validation",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// No positional args allowed at the root — anything that isn't a known
		// subcommand should surface a clean error rather than silently dropping
		// into the TUI with the operator's mistyped argument ignored.
		Args: cobra.NoArgs,
		// RunE fires only when the user invokes `hostie` with no subcommand
		// (cobra dispatches matching subcommands before reaching this hook).
		// --help and --version are handled by cobra before RunE as well, so
		// they continue to behave exactly as they did pre-wire.
		RunE: func(_ *cobra.Command, _ []string) error {
			return tuiRunner(hostsFilePath)
		},
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
	cmd.AddCommand(applyPrivilegedCmd)

	return cmd
}

// Execute runs the root command
func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
