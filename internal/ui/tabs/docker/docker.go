package docker

import (
	"phantom/internal/ui/components/launcher"
)

// New creates a new Docker launcher tab.
func New() launcher.Model {
	return launcher.New("LazyDocker", "lazydocker")
}
