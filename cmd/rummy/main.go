package main

import (
	"fmt"
	"github.com/DukeAvi/rummy-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"os"
)

func main() {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "rummy needs an interactive terminal; run go run ./cmd/rummy inside WSL.")
		os.Exit(1)
	}
	app := tui.NewApp()
	defer app.Close()
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rummy:", err)
		os.Exit(1)
	}
}
