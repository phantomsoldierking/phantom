package ui

import (
	"fmt"
	"time"

	"phantom/internal/app"
	"phantom/internal/config"
	"phantom/internal/ui/components/launcher"
	"phantom/internal/ui/components/styles"
	"phantom/internal/ui/tabs/dashboard"
	"phantom/internal/ui/tabs/docker"
	"phantom/internal/ui/tabs/git"
	"phantom/internal/ui/tabs/http"
	"phantom/internal/ui/tabs/kind"
	"phantom/internal/ui/tabs/nvim"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the main model for the TUI application.
type Model struct {
	Tabs           []string
	ActiveTab      int
	Width, Height  int
	Ready          bool
	DashboardModel dashboard.Model
	HTTPModel      http.Model
	GitModel       launcher.Model
	DockerModel    launcher.Model
	KindModel      kind.Model
	NvimModel      launcher.Model
}

// InitialModel creates the initial state of the application.
func InitialModel() Model {
	m := Model{
		Tabs:           []string{"Dashboard", "HTTP", "Git", "Docker", "Kind", "Nvim"},
		ActiveTab:      0,
		DashboardModel: dashboard.Model{},
		HTTPModel:      http.New(),
		GitModel:       git.New(),
		DockerModel:    docker.New(),
		KindModel:      kind.New(),
		NvimModel:      nvim.New(),
	}
	return m
}

// Init initializes the application.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.DashboardModel.Init(),
		m.HTTPModel.Init(),
		m.KindModel.Init(),
		app.CheckBinary(m.GitModel.BinaryName),
		app.CheckBinary(m.DockerModel.BinaryName),
		app.CheckBinary("kind"), // Check for the kind binary directly
		app.CheckBinary(m.NvimModel.BinaryName),
		config.LoadConfig(),
	)
}

// Update handles all messages for the application.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			m.ActiveTab = (m.ActiveTab + 1) % len(m.Tabs)
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			m.ActiveTab--
			if m.ActiveTab < 0 {
				m.ActiveTab = len(m.Tabs) - 1
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		modelHeight := m.Height - 5 // Account for header and footer
		m.DashboardModel.Width, m.DashboardModel.Height = m.Width, modelHeight
		m.HTTPModel.SetSize(m.Width, modelHeight)
		m.GitModel.Width, m.GitModel.Height = m.Width, modelHeight
		m.DockerModel.Width, m.DockerModel.Height = m.Width, modelHeight
		m.KindModel.Width, m.KindModel.Height = m.Width, modelHeight
		m.NvimModel.Width, m.NvimModel.Height = m.Width, modelHeight

	// Custom messages
	case app.CheckBinaryMsg:
		switch msg.AppName {
		case m.GitModel.BinaryName:
			m.GitModel.IsInstalled = msg.Found
		case m.DockerModel.BinaryName:
			m.DockerModel.IsInstalled = msg.Found
		case "kind":
			m.KindModel.SetInstalled(msg.Found)
		case m.NvimModel.BinaryName:
			m.NvimModel.IsInstalled = msg.Found
		}
	case config.ConfigLoadedMsg:
		m.HTTPModel.Collections.SetItems(msg.Templates)
		m.HTTPModel.Environment = msg.Environment
	}

	// Delegate updates to the active model
	switch m.Tabs[m.ActiveTab] {
	case "Dashboard":
		m.DashboardModel, cmd = m.DashboardModel.Update(msg)
	case "HTTP":
		m.HTTPModel, cmd = m.HTTPModel.Update(msg)
	case "Git":
		m.GitModel, cmd = m.GitModel.Update(msg)
	case "Docker":
		m.DockerModel, cmd = m.DockerModel.Update(msg)
	case "Kind":
		var updated tea.Model
		updated, cmd = m.KindModel.Update(msg)
		if km, ok := updated.(kind.Model); ok {
			m.KindModel = km
		}
	case "Nvim":
		m.NvimModel, cmd = m.NvimModel.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the application's UI.
func (m Model) View() string {
	if !m.Ready {
		return "Initializing..."
	}

	var renderedTabs []string
	for i, t := range m.Tabs {
		style := styles.InactiveTabStyle
		if i == m.ActiveTab {
			style = styles.ActiveTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(t))
	}
	tabHeader := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	var tabContent string
	switch m.Tabs[m.ActiveTab] {
	case "Dashboard":
		tabContent = m.DashboardModel.View()
	case "HTTP":
		tabContent = m.HTTPModel.View()
	case "Git":
		tabContent = m.GitModel.View()
	case "Docker":
		tabContent = m.DockerModel.View()
	case "Kind":
		tabContent = m.KindModel.View()
	case "Nvim":
		tabContent = m.NvimModel.View()
	}

	statusBar := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("57")).Width(m.Width).Render(fmt.Sprintf("Phantom | Tab/Shift+Tab: Switch | q: Quit | Time: %s", time.Now().Format("15:04:05")))

	return lipgloss.JoinVertical(lipgloss.Left, tabHeader, styles.DocStyle.Render(tabContent), statusBar)
}
