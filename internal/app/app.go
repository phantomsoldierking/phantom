package app

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// CheckBinary checks if a binary exists in the system's PATH.
func CheckBinary(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := exec.LookPath(name)
		return CheckBinaryMsg{AppName: name, Found: err == nil}
	}
}

// CheckBinaryMsg is the message sent when a binary check is complete.
type CheckBinaryMsg struct {
	AppName string
	Found   bool
}
