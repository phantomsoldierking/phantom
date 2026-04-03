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

type (
	clustersMsg []list.Item
	opDoneMsg   struct{ err error }
	describeMsg struct{ text string }
	errMsg      struct{ err error }
)

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
	clusters      list.Model
	descView      viewport.Model
	sp            spinner.Model
	textPrompt    textinput.Model
	state         tabState
	jobTitle      string
	jobRunning    bool
	isInstalled   bool
	lastErr       error
	promptAction  string
	pendingName   string
	pendingConfig string
}

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
	return tea.Batch(m.sp.Tick, delayedListClustersCmd(0))
}

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
		if m.promptAction != "" {
			var cmd tea.Cmd
			m.textPrompt, cmd = m.textPrompt.Update(msg)
			switch msg.Type {
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textPrompt.Value())
				if m.promptAction == "create-name" {
					if val == "" {
						val = fmt.Sprintf("dev-%d", time.Now().Unix()%10000)
					}
					m.pendingName = val
					m.textPrompt.SetValue("")
					m.textPrompt.Placeholder = "Path to config YAML (optional)"
					m.promptAction = "create-config"
					return m, nil
				} else if m.promptAction == "create-config" {
					m.pendingConfig = val
					m.promptAction = ""
					m.jobRunning, m.jobTitle = true, "creating "+m.pendingName
					m.textPrompt.SetValue("")
					m.textPrompt.Placeholder = ""
					return m, tea.Batch(createClusterCmd(m.pendingName, m.pendingConfig), delayedListClustersCmd(2*time.Second))
				}
			case tea.KeyEsc:
				m.promptAction = ""
				m.textPrompt.SetValue("")
				m.pendingName, m.pendingConfig = "", ""
				return m, nil
			default:
				return m, cmd
			}
		} else {
			sel, _ := m.clusters.SelectedItem().(clusterItem)
			name := string(sel)
			switch msg.String() {
			case "r":
				m.jobRunning, m.jobTitle = true, "refreshing"
				return m, delayedListClustersCmd(1 * time.Second)
			case "n":
				m.textPrompt.Placeholder = "Cluster name (Enter for default)"
				m.textPrompt.SetValue("")
				m.promptAction = "create-name"
				return m, nil
			case "d":
				if name != "" {
					m.jobRunning, m.jobTitle = true, "deleting "+name
					return m, tea.Batch(deleteClusterCmd(name), delayedListClustersCmd(2*time.Second))
				}
			case "x":
				var names []string
				for _, i := range m.clusters.Items() {
					if c, ok := i.(clusterItem); ok {
						names = append(names, string(c))
					}
				}
				if len(names) > 0 {
					m.jobRunning, m.jobTitle = true, "deleting all clusters"
					return m, tea.Batch(deleteAllClustersCmd(names), delayedListClustersCmd(2*time.Second))
				}
			case "v":
				if name != "" {
					m.state = stateDescribe
					m.descView.SetContent("")
					m.descView.YOffset = 0
					m.jobRunning, m.jobTitle = true, "viewing "+name
					return m, describeClusterCmd(name)
				}
			case "l":
				if name != "" {
					m.state = stateDescribe
					m.descView.SetContent("")
					m.jobRunning, m.jobTitle = true, "listing nodes for "+name
					return m, listNodesCmd(name)
				}
			case "s":
				if name != "" {
					m.jobRunning, m.jobTitle = true, "switching context to "+name
					return m, switchContextCmd(name)
				}
			case "i":
				if name != "" {
					m.textPrompt.Placeholder = "Docker image (e.g. my-app:latest)"
					m.promptAction = "load-image"
					m.pendingName = name
					m.textPrompt.SetValue("")
					return m, nil
				}
			case "e":
				if name != "" {
					m.textPrompt.Placeholder = "Export path (dir)"
					m.promptAction = "export-logs"
					m.pendingName = name
					m.textPrompt.SetValue("")
					return m, nil
				}
			case "k":
				if name != "" {
					m.jobRunning, m.jobTitle = true, "exporting kubeconfig for "+name
					return m, exportKubeconfigCmd(name)
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
		if msg.err == nil {
			m.descView.SetContent(styles.SuccessStyle.Render(m.jobTitle + " completed successfully!"))
		} else {
			m.descView.SetContent(styles.ErrorStyle.Render("Error: " + msg.err.Error()))
		}
		return m, delayedListClustersCmd(1 * time.Second)

	case describeMsg:
		m.jobRunning = false
		m.descView.SetContent(styles.DocStyle.Render(msg.text))
		m.descView.YOffset = 0

	case errMsg:
		m.jobRunning, m.lastErr = false, msg.err
	}

	if m.promptAction != "" && m.promptAction != "create-name" && m.promptAction != "create-config" {
		keyMsg, ok := msg.(tea.KeyMsg)
		if !ok {
			return m, nil
		}
		var cmd tea.Cmd
		m.textPrompt, cmd = m.textPrompt.Update(keyMsg)
		switch keyMsg.Type {
		case tea.KeyEnter:
			val := strings.TrimSpace(m.textPrompt.Value())
			name := m.pendingName
			action := m.promptAction
			m.promptAction = ""
			m.textPrompt.SetValue("")
			m.textPrompt.Placeholder = ""
			switch action {
			case "load-image":
				m.jobRunning, m.jobTitle = true, "loading image into "+name
				return m, tea.Batch(loadImageCmd(name, val), delayedListClustersCmd(2*time.Second))
			case "export-logs":
				m.jobRunning, m.jobTitle = true, "exporting logs for "+name
				return m, tea.Batch(exportLogsCmd(name, val), delayedListClustersCmd(2*time.Second))
			}
		case tea.KeyEsc:
			m.promptAction = ""
			m.textPrompt.SetValue("")
			return m, nil
		default:
			return m, cmd
		}
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

func (m Model) View() string {
	if !m.isInstalled {
		return styles.ErrorStyle.Render("kind binary not found in $PATH")
	}

	var body string
	if m.promptAction != "" {
		prompt := ""
		switch m.promptAction {
		case "create-name":
			prompt = "Enter cluster name (default if blank):"
		case "create-config":
			prompt = "Enter path to config YAML (optional):"
		case "load-image":
			prompt = "Enter Docker image to load:"
		case "export-logs":
			prompt = "Enter directory to export logs:"
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render(prompt),
			m.textPrompt.View(),
			styles.HelpStyle.Render("esc:cancel  enter:confirm"),
		)
	} else {
		if m.state == stateList {
			if len(m.clusters.Items()) == 0 {
				body = styles.HelpStyle.Render("No kind clusters found. Press 'n' to create one.")
			} else {
				var rows []string
				for i, item := range m.clusters.Items() {
					c := item.(clusterItem)
					selected := "  "
					if i == m.clusters.Index() {
						selected = styles.ActiveTabStyle.Render("→ ")
					}
					row := lipgloss.JoinHorizontal(lipgloss.Top,
						selected,
						styles.ActiveTabStyle.Render(c.Title()),
						lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  (kind cluster)"),
					)
					rows = append(rows, row)
				}
				body = lipgloss.JoinVertical(lipgloss.Left, rows...)
			}
		} else {
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

	help := styles.HelpStyle.Render(
		"n:new  d:delete  x:delete-all  v:view  l:list-nodes  i:load-image  s:switch-context  k:export-kubeconfig  e:export-logs  r:refresh  esc:back",
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func delayedListClustersCmd(d time.Duration) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		time.Sleep(d)
		return listClustersCmd()()
	})
}

func listClustersCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("kind", "get", "clusters").Output()
		if err != nil {
			return errMsg{err}
		}
		items := []list.Item{}
		for _, l := range strings.Fields(string(out)) {
			items = append(items, clusterItem(l))
		}
		return clustersMsg(items)
	}
}

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

func deleteAllClustersCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		var lastErr error
		for _, n := range names {
			if err := exec.Command("kind", "delete", "cluster", "--name", n).Run(); err != nil {
				lastErr = err
			}
		}
		return opDoneMsg{lastErr}
	}
}

func describeClusterCmd(name string) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		var configYaml string
		if err == nil {
			configFile := filepath.Join(home, ".kind", "clusters", name, "config")
			if data, err := os.ReadFile(configFile); err == nil {
				configYaml = string(data)
			}
		}
		if configYaml == "" {
			var b bytes.Buffer
			_ = exec.Command("kubectl", "cluster-info", "--context", "kind-"+name).Run()
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

func listNodesCmd(name string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("kubectl", "--context", "kind-"+name, "get", "nodes", "-o", "wide").CombinedOutput()
		if err != nil {
			return errMsg{err}
		}
		return describeMsg{string(out)}
	}
}

func switchContextCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("kubectl", "config", "use-context", "kind-"+name).Run()
		return opDoneMsg{err}
	}
}

func loadImageCmd(name, image string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("kind", "load", "docker-image", image, "--name", name).Run()
		return opDoneMsg{err}
	}
}

func exportLogsCmd(name, path string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("kind", "export", "logs", path, "--name", name).Run()
		return opDoneMsg{err}
	}
}

func exportKubeconfigCmd(name string) tea.Cmd {
	return func() tea.Msg {
		// uses 'kind export kubeconfig --name'
		err := exec.Command("kind", "export", "kubeconfig", "--name", name).Run()
		return opDoneMsg{err}
	}
}

func runKindCmd(done tea.Msg, args ...string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("kind", args...).Run()
		return opDoneMsg{err}
	}
}

func (m *Model) SetInstalled(installed bool) { m.isInstalled = installed }
