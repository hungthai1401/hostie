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
	cmd := &cobra.Command{
		Use:           "hostie",
		Short:         "hostie — hosts file manager",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Match the prior output shape: "hostie v<version>\n".
	// Tag refs already strip leading "v" in the release workflow, so a tag
	// like "v1.0.0" yields version="1.0.0" → "hostie v1.0.0".
	cmd.SetVersionTemplate("hostie v{{.Version}}\n")
	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
