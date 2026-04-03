package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"phantom/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Setup logging
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	model := ui.InitialModel()
	if opts, handled, err := parseCLIArgs(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		model = ui.InitialModelWithOptions(opts)
	}

	// Create and run the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func parseCLIArgs(args []string) (ui.StartOptions, bool, error) {
	if len(args) == 0 {
		return ui.StartOptions{}, false, nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	case "explore":
		if len(args) < 2 {
			return ui.StartOptions{}, true, fmt.Errorf("usage: phantom explore <file|- >")
		}
		source := strings.TrimSpace(args[1])
		var data []byte
		var err error
		if source == "-" {
			data, err = os.ReadFile("/dev/stdin")
		} else {
			data, err = os.ReadFile(source)
		}
		if err != nil {
			return ui.StartOptions{}, true, err
		}
		return ui.StartOptions{
			StartTab:    "Explorer",
			ExplorerRaw: string(data),
		}, true, nil
	default:
		return ui.StartOptions{}, false, nil
	}
	return ui.StartOptions{}, false, nil
}

func printUsage() {
	fmt.Println("Phantom")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  phantom")
	fmt.Println("  phantom explore <file>")
	fmt.Println("  cat payload.json | phantom explore -")
}
