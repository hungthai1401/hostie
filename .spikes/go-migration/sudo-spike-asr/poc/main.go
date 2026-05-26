// POC: tea.ExecProcess sudo TTY handoff
//
// Demonstrates the pattern used for Phase 4 sudo branch:
//   1. TUI is in altscreen
//   2. User triggers an action (here: pressing 's')
//   3. We return tea.ExecProcess(...) which releases the TTY, runs an
//      external command (here `id` for safety; swap for `sudo -v` to
//      manually verify password prompt), then re-acquires TTY.
//   4. On callback, we render the result.
//
// Run modes:
//   go run .                 # default: runs `id` (non-interactive, safe)
//   SPIKE_CMD=sudo go run .  # runs `sudo -v` to test real sudo prompt
//   SPIKE_CMD=true go run .  # trivial true (control)
//
// Press 's' to spawn the subcommand. Press 'q' to quit.
package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type execFinishedMsg struct{ err error }

type model struct {
	count    int
	lastErr  error
	lastDone bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			m.count++
			return m, spawnCmd()
		}
	case execFinishedMsg:
		m.lastDone = true
		m.lastErr = msg.err
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	out := "tea.ExecProcess TTY-handoff POC\n\n"
	out += "Press [s] to spawn external command. [q] to quit.\n\n"
	out += fmt.Sprintf("Spawn count: %d\n", m.count)
	if m.lastDone {
		if m.lastErr != nil {
			out += fmt.Sprintf("Last result: ERROR %v\n", m.lastErr)
		} else {
			out += "Last result: OK (TTY reacquired, this line proves redraw)\n"
		}
	}
	return out
}

// spawnCmd builds the tea.Cmd that does the TTY release/run/reacquire dance.
func spawnCmd() tea.Cmd {
	choice := os.Getenv("SPIKE_CMD")
	var c *exec.Cmd
	switch choice {
	case "sudo":
		// Real sudo prompt — manual test only.
		c = exec.Command("sudo", "-v")
	case "true":
		c = exec.Command("true")
	default:
		// Default: `id` — prints uid info, no auth, non-interactive.
		c = exec.Command("id")
	}
	// Inherit stdio so user sees output of `id` / sudo prompt.
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tea run error:", err)
		os.Exit(1)
	}
}
