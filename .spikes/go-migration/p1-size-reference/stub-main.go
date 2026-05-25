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

var version = "dev"

var _ = textinput.New
var _ tea.Model = nil
var _ = lipgloss.NewStyle
var _ = ulid.Make
var _ = fuzzy.Find
var _ = yaml.Marshal

func main() {
	root := &cobra.Command{
		Use: "hostie",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("hostie v%s\n", version)
		},
	}
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
