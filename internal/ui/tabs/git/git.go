package git

import (
	"phantom/internal/ui/components/launcher"
)

// New creates a new Git launcher tab.
func New() launcher.Model {
	return launcher.New("LazyGit", "lazygit")
}
