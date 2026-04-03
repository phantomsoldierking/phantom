package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"phantom/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Setup logging
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		f, err = tea.LogToFile("/tmp/phantom-debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
	}
	defer f.Close()

	opts, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if opts.WorkDir == "" {
		if wd, err := os.Getwd(); err == nil {
			opts.WorkDir = wd
		}
	}
	if opts.ExplorerDir == "" {
		opts.ExplorerDir = opts.WorkDir
	}
	if opts.WorkDir != "" {
		if err := os.Chdir(opts.WorkDir); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
	model := ui.InitialModelWithOptions(opts)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func parseCLIArgs(args []string) (ui.StartOptions, error) {
	opts := ui.StartOptions{}
	if len(args) == 0 {
		return opts, nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	case "explore":
		if len(args) < 2 {
			return ui.StartOptions{}, fmt.Errorf("usage: phantom explore <file|->")
		}
		source := strings.TrimSpace(args[1])
		var data []byte
		var err error
		if source == "-" {
			data, err = os.ReadFile("/dev/stdin")
		} else {
			data, err = os.ReadFile(source)
			if err == nil {
				if abs, absErr := filepath.Abs(source); absErr == nil {
					opts.WorkDir = filepath.Dir(abs)
					opts.ExplorerDir = opts.WorkDir
				}
			}
		}
		if err != nil {
			return ui.StartOptions{}, err
		}
		opts.StartTab = "Explorer"
		opts.ExplorerRaw = string(data)
		return opts, nil
	default:
		target := args[0]
		info, err := os.Stat(target)
		if err != nil {
			return ui.StartOptions{}, fmt.Errorf("unknown command or path not found: %s", target)
		}
		abs, err := filepath.Abs(target)
		if err != nil {
			return ui.StartOptions{}, err
		}
		if info.IsDir() {
			opts.WorkDir = abs
			opts.ExplorerDir = abs
			return opts, nil
		}
		opts.TargetFile = abs
		opts.WorkDir = filepath.Dir(abs)
		opts.ExplorerDir = opts.WorkDir
		return opts, nil
	}
	return opts, nil
}

func printUsage() {
	fmt.Println("Phantom")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  phantom")
	fmt.Println("  phantom explore <file>")
	fmt.Println("  cat payload.json | phantom explore -")
}
