// internal/ui/tabs/kind/kind.go
package kind

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"phantom/internal/ui/components/styles"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

/*───────────────────────────
   Internal message types
───────────────────────────*/

type (
	clustersMsg []list.Item
	opDoneMsg   struct{ err error }   // create/delete finished
	describeMsg struct{ text string } // describe output
	errMsg      struct{ err error }
	logLineMsg  struct{ line string } // optional future use
)

/*───────────────────────────
   Model definition
───────────────────────────*/

type tabState int

const (
	stateList tabState = iota
	stateDescribe
)

type clusterItem string

func (c clusterItem) Title() string       { return string(c) }
func (c clusterItem) Description() string { return "kind cluster" }
func (c clusterItem) FilterValue() string { return string(c) }

type Model struct {
	Width, Height int

	// UI widgets
	clusters   list.Model
	descView   viewport.Model
	sp         spinner.Model
	textPrompt textinput.Model

	// State
	state         tabState
	jobTitle      string
	jobRunning    bool
	isInstalled   bool
	lastErr       error
	promptAction  string // "create" or "config"
	pendingName   string // for create
	pendingConfig string // for create
}

/*───────────────────────────
   Public API
───────────────────────────*/

func New() Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "kind clusters"
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 128
	ti.Prompt = "> "
	return Model{
		clusters:   l,
		descView:   viewport.New(0, 0),
		sp:         spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.SpinnerStyle)),
		textPrompt: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, listClustersCmd())
}

/*───────────────────────────
   Update – main event loop
───────────────────────────*/

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.clusters.SetSize(msg.Width-4, msg.Height-6)
		m.descView.Width = msg.Width - 4
		m.descView.Height = msg.Height - 6
		m.textPrompt.Width = msg.Width - 8

	case spinner.TickMsg:
		if m.jobRunning {
			var cmd tea.Cmd
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.jobRunning {
			break
		}
		// Prompt for cluster name or config path
		if m.promptAction != "" {
			switch msg.Type {
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textPrompt.Value())
				if m.promptAction == "create-name" {
					if val == "" {
						val = fmt.Sprintf("dev-%d", time.Now().Unix()%10000)
					}
					m.pendingName = val
					m.textPrompt.SetValue("")
					m.textPrompt.Placeholder = "Path to config YAML (optional, Enter for default)"
					m.promptAction = "create-config"
					return m, nil
				} else if m.promptAction == "create-config" {
					m.pendingConfig = val
					m.promptAction = ""
					m.jobRunning, m.jobTitle = true, "creating "+m.pendingName
					return m, createClusterCmd(m.pendingName, m.pendingConfig)
				}
			case tea.KeyEsc:
				m.promptAction = ""
				m.textPrompt.SetValue("")
				m.pendingName = ""
				m.pendingConfig = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.textPrompt, cmd = m.textPrompt.Update(msg)
				return m, cmd
			}
		} else {
			switch msg.String() {
			case "r": // refresh
				m.jobRunning, m.jobTitle = true, "refreshing"
				return m, listClustersCmd()
			case "n": // create
				m.textPrompt.Placeholder = "Cluster name (Enter for default)"
				m.textPrompt.SetValue("")
				m.promptAction = "create-name"
				return m, nil
			case "d": // delete
				if c, ok := m.clusters.SelectedItem().(clusterItem); ok {
					m.jobRunning, m.jobTitle = true, "deleting "+string(c)
					return m, deleteClusterCmd(string(c))
				}
			case "v": // view/describe selected
				if c, ok := m.clusters.SelectedItem().(clusterItem); ok {
					m.state = stateDescribe
					m.descView.SetContent("")
					m.descView.YOffset = 0
					m.descView.Width = m.Width - 4
					m.descView.Height = m.Height - 6
					m.jobRunning, m.jobTitle = true, "viewing "+string(c)
					return m, describeClusterCmd(string(c))
				}
			case "esc":
				if m.state == stateDescribe {
					m.state = stateList
					m.descView.SetContent("")
					return m, nil
				}
			}
		}

	case clustersMsg:
		m.jobRunning = false
		m.clusters.SetItems(msg)

	case opDoneMsg:
		m.jobRunning = false
		m.lastErr = msg.err
		return m, listClustersCmd()

	case describeMsg:
		m.jobRunning = false
		m.descView.SetContent(styles.DocStyle.Render(msg.text))
		m.descView.YOffset = 0

	case errMsg:
		m.jobRunning, m.lastErr = false, msg.err
	}

	// Delegate to list/viewport/textinput when appropriate
	if m.promptAction != "" {
		var cmd tea.Cmd
		m.textPrompt, cmd = m.textPrompt.Update(msg)
		return m, cmd
	}
	switch m.state {
	case stateList:
		var cmd tea.Cmd
		m.clusters, cmd = m.clusters.Update(msg)
		return m, cmd
	case stateDescribe:
		var cmd tea.Cmd
		m.descView, cmd = m.descView.Update(msg)
		return m, cmd
	}
	return m, nil
}

/*───────────────────────────
   View
───────────────────────────*/

func (m Model) View() string {
	if !m.isInstalled {
		return styles.ErrorStyle.Render("kind binary not found in $PATH")
	}

	var body string
	if m.promptAction != "" {
		prompt := ""
		switch m.promptAction {
		case "create-name":
			prompt = "Enter cluster name (Enter for default):"
		case "create-config":
			prompt = "Enter path to config YAML (optional, Enter for default):"
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render(prompt),
			m.textPrompt.View(),
			styles.HelpStyle.Render("esc:cancel  enter:confirm"),
		)
	} else {
		switch m.state {
		case stateList:
			if len(m.clusters.Items()) == 0 {
				body = styles.HelpStyle.Render("No kind clusters found. Press 'n' to create one.")
			} else {
				// Render clusters in a beautiful table
				var rows []string
				for i, item := range m.clusters.Items() {
					c, _ := item.(clusterItem)
					selected := ""
					if i == m.clusters.Index() {
						selected = styles.ActiveTabStyle.Render("→ ")
					} else {
						selected = "  "
					}
					// Show cluster name and a short description
					row := lipgloss.JoinHorizontal(lipgloss.Top,
						selected,
						styles.ActiveTabStyle.Render(c.Title()),
						lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  (kind cluster)"),
					)
					rows = append(rows, row)
				}
				body = lipgloss.JoinVertical(lipgloss.Left, rows...)
			}
		case stateDescribe:
			// Show a scrollable, styled viewport for YAML/describe output
			body = m.descView.View()
			body = lipgloss.JoinVertical(lipgloss.Left, body, styles.HelpStyle.Render("↑/↓: scroll  esc:back"))
		}
	}

	header := styles.ListHeaderStyle.Render("Kind Clusters")
	if m.jobRunning {
		header = m.sp.View() + " " + header + " – " + m.jobTitle
	}
	if m.lastErr != nil {
		body = styles.ErrorStyle.Render(m.lastErr.Error())
	}

	help := styles.HelpStyle.Render("n:new  d:delete  v:view  r:refresh  esc:back")
	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

/*───────────────────────────
   Commands
───────────────────────────*/

func listClustersCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("kind", "get", "clusters").Output()
		if err != nil {
			return errMsg{err}
		}

		items := make([]list.Item, 0)
		for _, l := range strings.Fields(string(out)) {
			items = append(items, clusterItem(l))
		}
		return clustersMsg(items)
	}
}

// createClusterCmd now takes optional configPath
func createClusterCmd(name, configPath string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"create", "cluster", "--name", name}
		if configPath != "" {
			args = append(args, "--config", configPath)
		}
		err := exec.Command("kind", args...).Run()
		return opDoneMsg{err}
	}
}

func deleteClusterCmd(name string) tea.Cmd {
	return runKindCmd(opDoneMsg{}, "delete", "cluster", "--name", name)
}

// describeClusterCmd now tries to show the kind cluster config YAML if available
func describeClusterCmd(name string) tea.Cmd {
	return func() tea.Msg {
		// Try to find the kind config YAML for this cluster
		home, err := os.UserHomeDir()
		var configYaml string
		if err == nil {
			// Kind stores config in ~/.kind/clusters/<name>
			kindDir := filepath.Join(home, ".kind", "clusters", name)
			data, err := os.ReadFile(kindDir)
			if err == nil {
				configYaml = string(data)
			}
		}
		if configYaml == "" {
			// fallback: show kubectl cluster-info and nodes
			var b bytes.Buffer
			cmd := exec.Command("kubectl", "cluster-info", "--context", "kind-"+name)
			cmd.Stdout = &b
			cmd.Stderr = &b
			_ = cmd.Run()
			nodes, _ := exec.Command("kubectl", "--context", "kind-"+name, "get", "nodes", "-o", "wide").CombinedOutput()
			info := strings.TrimSpace(b.String())
			nodesStr := strings.TrimSpace(string(nodes))
			if info == "" && nodesStr == "" {
				configYaml = "Unable to describe cluster (is kubectl installed & in PATH?)"
			} else {
				configYaml = info + "\n\n" + nodesStr
			}
		}
		return describeMsg{configYaml}
	}
}

func runKindCmd(done tea.Msg, args ...string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("kind", args...).Run()
		return opDoneMsg{err}
	}
}
func (m *Model) SetInstalled(installed bool) {
	m.isInstalled = installed
}
