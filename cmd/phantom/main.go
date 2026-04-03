package main

import (
	"fmt"
	"log"
	"os"

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

	// Create and run the Bubble Tea program
	p := tea.NewProgram(ui.InitialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
