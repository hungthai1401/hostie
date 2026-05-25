package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oklog/ulid/v2"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// version is injected at build time via -ldflags="-X main.version=<value>".
var version = "dev"

// Dep-usage proofs: reference each primary dep so the linker keeps them
// in the binary, producing realistic size measurements for Phase 1.
var (
	_ = textinput.New
	_ tea.Model
	_ = lipgloss.NewStyle
	_ = ulid.Make
	_ = fuzzy.Find
	_ = yaml.Marshal
)

func newRootCmd() *cobra.Command {
	var showVersion bool
	cmd := &cobra.Command{
		Use:           "hostie",
		Short:         "hostie — hosts file manager",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "hostie v%s\n", version)
				return nil
			}
			return cmd.Help()
		},
	}
	cmd.Flags().BoolVar(&showVersion, "version", false, "print version and exit")
	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
