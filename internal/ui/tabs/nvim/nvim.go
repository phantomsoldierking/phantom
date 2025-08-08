package nvim

import (
	"phantom/internal/ui/components/launcher"
)

// New creates a new nvim launcher tab.
func New() launcher.Model {
	// Launch nvim as a plain terminal command to ensure it loads user configs.
	return launcher.New("Neovim", "nvim")
}
