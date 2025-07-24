package kind

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"phantom/internal/ui/components/styles"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type clusterItem string

func (c clusterItem) Title() string       { return string(c) }
func (c clusterItem) Description() string { return "Kind Kubernetes Cluster" }
func (c clusterItem) FilterValue() string { return string(c) }

type state int

const (
	listView state = iota
	creatingView
	deletingView
)

// Model represents the Kind cluster management tab.
type Model struct {
	Width, Height int
	List          list.Model
	Spinner       spinner.Model
	TextInput     textinput.Model
	State         state
	IsInstalled   bool
	Loading       bool
	Err           error
}

// Messages for the Kind tab
type clustersLoadedMsg []list.Item
type clusterCreatedMsg struct{ err error }
type clusterDeletedMsg struct{ err error }
type errMsg struct{ err error }

// New creates a new Kind model.
func New() Model {
	m := Model{
		List: list.New(nil, list.NewDefaultDelegate(), 0, 0),
		Spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(styles.SpinnerStyle),
		),
		TextInput: textinput.New(),
		State:     listView,
	}
	m.List.Title = "Kind Clusters"
	m.TextInput.Placeholder = "new-cluster-name"
	m.TextInput.Focus()
	return m
}

// Init initializes the Kind model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, getClustersCmd)
}

// Update handles messages for the Kind model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.State == creatingView || m.State == deletingView {
			return m, nil // Block input while busy
		}
		switch m.State {
		case listView:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
				m.Loading = true
				return m, getClustersCmd
			case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
				m.State = creatingView
				m.TextInput.SetValue("")
				m.TextInput.Focus()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
				if item, ok := m.List.SelectedItem().(clusterItem); ok {
					m.State = deletingView
					m.Loading = true
					return m, deleteClusterCmd(string(item))
				}
			}
		case creatingView:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.State = creatingView
				m.Loading = true
				return m, createClusterCmd(m.TextInput.Value())
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.State = listView
				return m, nil
			}
		}
	case clustersLoadedMsg:
		m.Loading = false
		m.List.SetItems(msg)
	case clusterCreatedMsg:
		m.Loading = false
		m.State = listView
		m.Err = msg.err
		return m, getClustersCmd
	case clusterDeletedMsg:
		m.Loading = false
		m.State = listView
		m.Err = msg.err
		return m, getClustersCmd
	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	case errMsg:
		m.Err = msg.err
	}

	if m.State == creatingView {
		m.TextInput, cmd = m.TextInput.Update(msg)
	} else {
		m.List, cmd = m.List.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the Kind model.
func (m Model) View() string {
	if !m.IsInstalled {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			styles.ErrorStyle.Render("Kind binary not found in PATH.\nPlease install it to use this feature."),
		)
	}

	if m.Loading {
		var action string
		switch m.State {
		case creatingView:
			action = "Creating cluster..."
		case deletingView:
			action = "Deleting cluster..."
		default:
			action = "Loading clusters..."
		}
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("%s %s", m.Spinner.View(), action),
		)
	}

	if m.State == creatingView {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center,
				"Enter new cluster name:",
				m.TextInput.View(),
				styles.HelpStyle.Render("Enter: create | Esc: cancel"),
			),
		)
	}

	help := styles.HelpStyle.Render("r: refresh | n: new | d: delete | q: quit")
	return lipgloss.JoinVertical(lipgloss.Left, m.List.View(), help)
}

// SetSize sets the size of the Kind model.
func (m *Model) SetSize(w, h int) {
	m.Width, m.Height = w, h
	m.List.SetSize(w, h-2)
}

// Commands for Kind operations
func getClustersCmd() tea.Msg {
	time.Sleep(500 * time.Millisecond) // Give kind a moment
	cmd := exec.Command("kind", "get", "clusters")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errMsg{fmt.Errorf("failed to get clusters: %s\n%s", err, string(out))}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	items := make([]list.Item, 0, len(lines))
	if len(lines) == 1 && lines[0] == "" {
		return clustersLoadedMsg(items)
	}
	for _, line := range lines {
		items = append(items, clusterItem(line))
	}
	return clustersLoadedMsg(items)
}

func createClusterCmd(name string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("kind", "create", "cluster", "--name", name)
		err := cmd.Run()
		return clusterCreatedMsg{err}
	}
}

func deleteClusterCmd(name string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("kind", "delete", "cluster", "--name", name)
		err := cmd.Run()
		return clusterDeletedMsg{err}
	}
}
