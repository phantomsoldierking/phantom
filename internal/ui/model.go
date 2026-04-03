package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"phantom/internal/app"
	"phantom/internal/config"
	"phantom/internal/ui/components/launcher"
	"phantom/internal/ui/components/styles"
	"phantom/internal/ui/tabs/dashboard"
	"phantom/internal/ui/tabs/docker"
	"phantom/internal/ui/tabs/explorer"
	"phantom/internal/ui/tabs/git"
	"phantom/internal/ui/tabs/http"
	"phantom/internal/ui/tabs/kind"
	"phantom/internal/ui/tabs/logs"
	"phantom/internal/ui/tabs/nvim"
	"phantom/internal/ui/tabs/ports"
	"phantom/internal/ui/tabs/processes"
	"phantom/internal/utils"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paletteCommand struct {
	Label string
	Kind  string
	Index int
}

type rootTickMsg time.Time

type shellResultMsg struct {
	Command string
	Output  string
	Err     error
}

type StartOptions struct {
	StartTab    string
	ExplorerRaw string
	ExplorerDir string
	WorkDir     string
	TargetFile  string
}

// Model is the main model for the TUI application.
type Model struct {
	Tabs      []string
	ActiveTab int

	Width, Height int
	Ready         bool

	DashboardModel dashboard.Model
	HTTPModel      http.Model
	GitModel       launcher.Model
	DockerModel    launcher.Model
	KindModel      kind.Model
	NvimModel      launcher.Model
	ExplorerModel  explorer.Model
	LogsModel      logs.Model
	ProcessesModel processes.Model
	PortsModel     ports.Model

	PaletteOpen   bool
	PaletteInput  textinput.Model
	PaletteCursor int

	PaletteOverlayOpen   bool
	PaletteOverlayTitle  string
	PaletteOverlayLines  []string
	PaletteOverlayOffset int

	ShellHistory     []string
	ShellHistoryIdx  int
	LastShellCommand string

	StatusText  string
	StatusUntil time.Time
	ConfigPath  string
	WorkDir     string
}

// InitialModel creates the initial state of the application.
func InitialModel() Model {
	palette := textinput.New()
	palette.Placeholder = "type to search commands and tabs"
	palette.Prompt = ": "

	m := Model{
		Tabs:           []string{"Dashboard", "Explorer", "Logs", "Processes", "Ports", "HTTP", "Git", "Docker", "Kind", "Nvim"},
		ActiveTab:      0,
		DashboardModel: dashboard.Model{},
		HTTPModel:      http.New(),
		GitModel:       git.New(),
		DockerModel:    docker.New(),
		KindModel:      kind.New(),
		NvimModel:      nvim.New(),
		ExplorerModel:  explorer.New(),
		LogsModel:      logs.New("debug.log"),
		ProcessesModel: processes.New(),
		PortsModel:     ports.New(),
		PaletteInput:   palette,
	}
	return m
}

func InitialModelWithOptions(opts StartOptions) Model {
	m := InitialModel()
	if opts.WorkDir != "" {
		m.WorkDir = opts.WorkDir
		m.GitModel.WorkDir = opts.WorkDir
		m.DockerModel.WorkDir = opts.WorkDir
		m.NvimModel.WorkDir = opts.WorkDir
	}
	m.GitModel.AutoLaunch = true
	m.DockerModel.AutoLaunch = true
	m.NvimModel.AutoLaunch = true
	if opts.TargetFile != "" {
		m.NvimModel.Args = []string{opts.TargetFile}
	}
	if opts.StartTab != "" {
		if idx := m.tabIndex(opts.StartTab); idx >= 0 {
			m.ActiveTab = idx
		}
	}
	if strings.TrimSpace(opts.ExplorerDir) != "" {
		m.ExplorerModel = m.ExplorerModel.LoadRootConfigs(opts.ExplorerDir)
	}
	if strings.TrimSpace(opts.ExplorerRaw) != "" {
		m.ExplorerModel = m.ExplorerModel.LoadRaw(opts.ExplorerRaw)
		if idx := m.tabIndex("Explorer"); idx >= 0 {
			m.ActiveTab = idx
		}
	}
	return m
}

// Init initializes the application.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.DashboardModel.Init(),
		m.LogsModel.Init(),
		m.ExplorerModel.Init(),
		m.ProcessesModel.Init(),
		m.PortsModel.Init(),
		m.HTTPModel.Init(),
		m.KindModel.Init(),
		app.CheckBinary(m.GitModel.BinaryName),
		app.CheckBinary(m.DockerModel.BinaryName),
		app.CheckBinary("kind"),
		app.CheckBinary(m.NvimModel.BinaryName),
		config.LoadConfig(),
		rootTickCmd(),
	)
}

// Update handles all messages for the application.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case rootTickMsg:
		return m, rootTickCmd()
	case app.StatusMsg:
		m.StatusText = msg.Text
		m.StatusUntil = time.Now().Add(2 * time.Second)
		return m, nil
	case app.NavigateMsg:
		if idx := m.tabIndex(msg.Tab); idx >= 0 {
			m.ActiveTab = idx
			if msg.Tab == "Processes" && msg.PID > 0 {
				m.ProcessesModel.FocusPID(msg.PID)
			}
		}
		return m, nil
	case shellResultMsg:
		m.PaletteOverlayOpen = true
		m.PaletteOverlayOffset = 0
		m.PaletteOverlayTitle = "Shell Output: " + msg.Command
		if msg.Err != nil {
			m.PaletteOverlayTitle += " (error)"
		}
		m.PaletteOverlayLines = splitLines(msg.Output)
		if len(m.PaletteOverlayLines) == 0 {
			m.PaletteOverlayLines = []string{"(no output)"}
		}
		return m, nil
	case tea.KeyMsg:
		if m.PaletteOpen {
			return m.updatePalette(msg)
		}
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys(":"))):
			m.PaletteOpen = true
			m.PaletteCursor = 0
			m.PaletteInput.SetValue("")
			m.PaletteInput.Focus()
			m.PaletteOverlayOpen = false
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			m.ActiveTab = (m.ActiveTab + 1) % len(m.Tabs)
			return m, m.tabEnterCmd()
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			m.ActiveTab--
			if m.ActiveTab < 0 {
				m.ActiveTab = len(m.Tabs) - 1
			}
			return m, m.tabEnterCmd()
		default:
			if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= len(m.Tabs) {
				m.ActiveTab = n - 1
				return m, m.tabEnterCmd()
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		modelHeight := m.Height - 5
		m.DashboardModel.Width, m.DashboardModel.Height = m.Width, modelHeight
		m.LogsModel.Width, m.LogsModel.Height = m.Width, modelHeight
		m.ExplorerModel.Width, m.ExplorerModel.Height = m.Width, modelHeight
		m.ProcessesModel.Width, m.ProcessesModel.Height = m.Width, modelHeight
		m.PortsModel.Width, m.PortsModel.Height = m.Width, modelHeight
		m.HTTPModel.SetSize(m.Width, modelHeight)
		m.GitModel.Width, m.GitModel.Height = m.Width, modelHeight
		m.DockerModel.Width, m.DockerModel.Height = m.Width, modelHeight
		m.KindModel.Width, m.KindModel.Height = m.Width, modelHeight
		m.NvimModel.Width, m.NvimModel.Height = m.Width, modelHeight
		m.PaletteInput.Width = max(20, m.Width-10)

	case app.CheckBinaryMsg:
		var launch tea.Cmd
		switch msg.AppName {
		case m.GitModel.BinaryName:
			m.GitModel.IsInstalled = msg.Found
			if msg.Found && m.Tabs[m.ActiveTab] == "Git" {
				launch = m.GitModel.LaunchCmd()
			}
		case m.DockerModel.BinaryName:
			m.DockerModel.IsInstalled = msg.Found
			if msg.Found && m.Tabs[m.ActiveTab] == "Docker" {
				launch = m.DockerModel.LaunchCmd()
			}
		case "kind":
			m.KindModel.SetInstalled(msg.Found)
		case m.NvimModel.BinaryName:
			m.NvimModel.IsInstalled = msg.Found
			if msg.Found && m.Tabs[m.ActiveTab] == "Nvim" {
				launch = m.NvimModel.LaunchCmd()
			}
		}
		if launch != nil {
			return m, launch
		}

	case config.ConfigLoadedMsg:
		m.HTTPModel.Collections.SetItems(msg.Templates)
		m.HTTPModel.Environment = msg.Environment
		m.LogsModel.ApplyConfig(msg.LogFile, msg.LogSources)
		m.ConfigPath = msg.ConfigPath
	}

	switch m.Tabs[m.ActiveTab] {
	case "Dashboard":
		m.DashboardModel, cmd = m.DashboardModel.Update(msg)
	case "Logs":
		m.LogsModel, cmd = m.LogsModel.Update(msg)
	case "Explorer":
		m.ExplorerModel, cmd = m.ExplorerModel.Update(msg)
	case "Processes":
		m.ProcessesModel, cmd = m.ProcessesModel.Update(msg)
	case "Ports":
		m.PortsModel, cmd = m.PortsModel.Update(msg)
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

	return m, cmd
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.PaletteOverlayOpen {
		switch msg.String() {
		case "esc":
			m.PaletteOverlayOpen = false
			return m, nil
		case "j", "down":
			m.PaletteOverlayOffset++
			maxOffset := max(0, len(m.PaletteOverlayLines)-m.overlayHeight())
			m.PaletteOverlayOffset = min(m.PaletteOverlayOffset, maxOffset)
			return m, nil
		case "k", "up":
			m.PaletteOverlayOffset = max(0, m.PaletteOverlayOffset-1)
			return m, nil
		case "y":
			all := strings.Join(m.PaletteOverlayLines, "\n")
			_, detail := utils.Yank(all)
			return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked output: " + detail} }
		}
		return m, nil
	}

	commands := m.filteredPaletteCommands()
	switch msg.String() {
	case "esc":
		m.PaletteOpen = false
		m.PaletteInput.Blur()
		m.PaletteOverlayOpen = false
		return m, nil
	case "down", "ctrl+n":
		if len(commands) > 0 {
			m.PaletteCursor = (m.PaletteCursor + 1) % len(commands)
		}
		return m, nil
	case "up", "ctrl+p":
		if strings.HasPrefix(strings.TrimSpace(m.PaletteInput.Value()), "!") && len(m.ShellHistory) > 0 {
			if m.ShellHistoryIdx <= 0 {
				m.ShellHistoryIdx = len(m.ShellHistory) - 1
			} else {
				m.ShellHistoryIdx--
			}
			m.PaletteInput.SetValue("!" + m.ShellHistory[m.ShellHistoryIdx])
			return m, nil
		}
		if len(commands) > 0 {
			m.PaletteCursor--
			if m.PaletteCursor < 0 {
				m.PaletteCursor = len(commands) - 1
			}
		}
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.PaletteInput.Value())
		if strings.HasPrefix(input, "!") {
			command := strings.TrimSpace(strings.TrimPrefix(input, "!"))
			if command == "!" || command == "" {
				command = strings.TrimSpace(m.LastShellCommand)
			}
			if command == "" {
				return m, func() tea.Msg { return app.StatusMsg{Text: "No previous shell command"} }
			}
			if len(m.ShellHistory) == 0 || m.ShellHistory[len(m.ShellHistory)-1] != command {
				m.ShellHistory = append(m.ShellHistory, command)
			}
			m.ShellHistoryIdx = len(m.ShellHistory)
			m.LastShellCommand = command
			m.PaletteOverlayOpen = true
			m.PaletteOverlayTitle = "Running: " + command
			m.PaletteOverlayLines = []string{"running..."}
			m.PaletteOverlayOffset = 0
			return m, runShellCmd(command)
		}

		if len(commands) > 0 {
			selected := commands[m.PaletteCursor]
			switch selected.Kind {
			case "tab":
				m.ActiveTab = selected.Index
				return m, m.tabEnterCmd()
			case "quit":
				return m, tea.Quit
			case "reload_config":
				m.PaletteOpen = false
				m.PaletteInput.Blur()
				return m, config.LoadConfig()
			}
		}
		m.PaletteOpen = false
		m.PaletteInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.PaletteInput, cmd = m.PaletteInput.Update(msg)
	m.PaletteCursor = 0
	return m, cmd
}

func runShellCmd(command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()
		return shellResultMsg{Command: command, Output: string(output), Err: err}
	}
}

func (m Model) allPaletteCommands() []paletteCommand {
	commands := make([]paletteCommand, 0, len(m.Tabs)+3)
	for i, t := range m.Tabs {
		commands = append(commands, paletteCommand{Label: "Switch tab: " + t, Kind: "tab", Index: i})
	}
	commands = append(commands, paletteCommand{Label: "Reload config", Kind: "reload_config"})
	commands = append(commands, paletteCommand{Label: "Shell command (!cmd)", Kind: "help_shell"})
	commands = append(commands, paletteCommand{Label: "Quit Phantom", Kind: "quit"})
	return commands
}

func (m Model) filteredPaletteCommands() []paletteCommand {
	q := strings.ToLower(strings.TrimSpace(m.PaletteInput.Value()))
	all := m.allPaletteCommands()
	if q == "" || strings.HasPrefix(q, "!") {
		return all
	}
	out := make([]paletteCommand, 0, len(all))
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Label), q) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) tabIndex(name string) int {
	for i, t := range m.Tabs {
		if t == name {
			return i
		}
	}
	return -1
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
	case "Logs":
		tabContent = m.LogsModel.View()
	case "Explorer":
		tabContent = m.ExplorerModel.View()
	case "Processes":
		tabContent = m.ProcessesModel.View()
	case "Ports":
		tabContent = m.PortsModel.View()
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

	statusText := fmt.Sprintf("Phantom | Tab/Shift+Tab switch | 1-9 jump | : palette | q quit | %s", time.Now().Format("15:04:05"))
	if m.ConfigPath != "" {
		statusText = fmt.Sprintf("Phantom | Tab Shift+Tab 1-9 : q | cfg: %s | %s", m.ConfigPath, time.Now().Format("15:04:05"))
	}
	if m.WorkDir != "" {
		statusText += " | wd: " + m.WorkDir
	}
	if m.StatusText != "" && time.Now().Before(m.StatusUntil) {
		statusText += " | " + m.StatusText
	}
	statusBar := styles.StatusBarStyle.Width(m.Width).Render(statusText)

	contentHeight := max(1, m.Height-2)
	contentArea := styles.DocStyle.Width(m.Width).Height(contentHeight).MaxHeight(contentHeight).Render(tabContent)
	main := lipgloss.JoinVertical(lipgloss.Left, tabHeader, contentArea, statusBar)
	if !m.PaletteOpen {
		return main
	}

	commands := m.filteredPaletteCommands()
	rows := make([]string, 0, len(commands)+2)
	rows = append(rows, styles.ListHeaderStyle.Render("Command Palette"))
	rows = append(rows, m.PaletteInput.View())
	for i, c := range commands {
		row := c.Label
		if i == m.PaletteCursor {
			row = styles.ActiveTabStyle.Render(row)
		}
		rows = append(rows, row)
	}
	if len(commands) == 0 {
		rows = append(rows, styles.HelpStyle.Render("No matching commands."))
	}

	paletteBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1).
		Width(max(40, m.Width-20)).
		Render(strings.Join(rows, "\n"))

	if !m.PaletteOverlayOpen {
		return lipgloss.JoinVertical(lipgloss.Left, main, paletteBox)
	}

	start := min(m.PaletteOverlayOffset, max(0, len(m.PaletteOverlayLines)-1))
	end := min(len(m.PaletteOverlayLines), start+m.overlayHeight())
	overlayContent := strings.Join(m.PaletteOverlayLines[start:end], "\n")
	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("220")).
		Padding(1).
		Width(max(40, m.Width-20)).
		Render(lipgloss.NewStyle().Bold(true).Render(m.PaletteOverlayTitle) + "\n" + overlayContent + "\n" + styles.HelpStyle.Render("j/k scroll | y yank | Esc close"))

	return lipgloss.JoinVertical(lipgloss.Left, main, paletteBox, overlay)
}

func splitLines(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if len(out) > 1500 {
		return out[len(out)-1500:]
	}
	return out
}

func rootTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg { return rootTickMsg(t) })
}

func (m Model) overlayHeight() int {
	h := m.Height / 3
	if h < 8 {
		return 8
	}
	if h > 24 {
		return 24
	}
	return h
}

func (m Model) tabEnterCmd() tea.Cmd {
	switch m.Tabs[m.ActiveTab] {
	case "Git":
		return m.GitModel.LaunchCmd()
	case "Docker":
		return m.DockerModel.LaunchCmd()
	case "Nvim":
		return m.NvimModel.LaunchCmd()
	default:
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
