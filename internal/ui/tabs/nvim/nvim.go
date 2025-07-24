package nvim

import (
	"phantom/internal/ui/components/launcher" // Corrected import path
)

// New creates a new nvim launcher tab.
func New() launcher.Model {
	// Using "nvim" assumes it will open in the current terminal.
	// For a more robust solution, you might need to handle terminal specifics.
	return launcher.New("Neovim", "nvim")
}
