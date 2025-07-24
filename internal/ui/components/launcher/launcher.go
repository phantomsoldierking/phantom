package launcher

import (
	"fmt"
	"os/exec"
	"strings"

	"phantom/internal/ui/components/styles"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents a launcher for an external application.
type Model struct {
	Width, Height int
	AppName       string
	BinaryName    string
	IsInstalled   bool
}

// New creates a new launcher model.
func New(appName, binaryName string) Model {
	return Model{AppName: appName, BinaryName: binaryName}
}

// Init initializes the launcher model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the launcher model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, key.NewBinding(key.WithKeys("enter"))) && m.IsInstalled {
		return m, tea.ExecProcess(exec.Command(m.BinaryName), nil)
	}
	return m, nil
}

// View renders the launcher model.
func (m Model) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render(m.AppName + " Launcher")
	b.WriteString(title + "\n\n")
	if m.IsInstalled {
		b.WriteString(fmt.Sprintf("%s is installed.\n\n", m.AppName))
		b.WriteString(styles.SuccessStyle.Render("Press [Enter] to launch."))
	} else {
		b.WriteString(fmt.Sprintf("%s not found in PATH.\n\n", m.AppName))
		b.WriteString(styles.ErrorStyle.Render("Please install it to use this feature."))
	}
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, b.String())
}
