package ports

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"phantom/internal/app"
	"phantom/internal/ui/components/styles"
	"phantom/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type tickMsg time.Time

type portItem struct {
	Proto   string
	Port    uint32
	Addr    string
	Status  string
	PID     int32
	Process string
}

type portMsg struct {
	items []portItem
	err   error
}

type Model struct {
	Width, Height int
	items         []portItem
	selected      int
	offset        int
	showAll       bool
	lastErr       string

	modalOpen        bool
	confirmForceKill bool
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadPortsCmd(m.showAll), tickCmd())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(loadPortsCmd(m.showAll), tickCmd())
	case portMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = ""
			m.items = msg.items
			if m.selected >= len(m.items) {
				m.selected = max(0, len(m.items)-1)
			}
		}
	case tea.KeyMsg:
		if m.modalOpen {
			return m.handleModal(msg)
		}

		switch msg.String() {
		case "r":
			return m, loadPortsCmd(m.showAll)
		case "a":
			m.showAll = !m.showAll
			return m, loadPortsCmd(m.showAll)
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "g":
			m.selected, m.offset = 0, 0
		case "G":
			if n := len(m.items); n > 0 {
				m.selected = n - 1
				m.offset = max(0, n-m.contentHeight())
			}
		case "enter":
			m.modalOpen = true
			m.confirmForceKill = false
		case "p":
			if it, ok := m.current(); ok && it.PID > 0 {
				return m, func() tea.Msg { return app.NavigateMsg{Tab: "Processes", PID: it.PID} }
			}
		case "c", "y":
			if it, ok := m.current(); ok {
				_, detail := utils.Yank(it.Addr)
				return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked: " + detail} }
			}
		}
	}
	return m, nil
}

func (m Model) handleModal(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modalOpen = false
		m.confirmForceKill = false
		return m, nil
	case "c", "y":
		if it, ok := m.current(); ok {
			_, detail := utils.Yank(it.Addr)
			return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked: " + detail} }
		}
	case "p":
		if it, ok := m.current(); ok && it.PID > 0 {
			m.modalOpen = false
			return m, func() tea.Msg { return app.NavigateMsg{Tab: "Processes", PID: it.PID} }
		}
	case "k":
		if it, ok := m.current(); ok && it.PID > 0 {
			if err := signalPID(it.PID, syscall.SIGTERM); err != nil {
				return m, func() tea.Msg { return app.StatusMsg{Text: "SIGTERM failed: " + err.Error()} }
			}
			return m, func() tea.Msg { return app.StatusMsg{Text: "Sent SIGTERM to owner"} }
		}
	case "K":
		if !m.confirmForceKill {
			m.confirmForceKill = true
			return m, func() tea.Msg { return app.StatusMsg{Text: "Press K again to confirm SIGKILL"} }
		}
		if it, ok := m.current(); ok && it.PID > 0 {
			m.confirmForceKill = false
			if err := signalPID(it.PID, syscall.SIGKILL); err != nil {
				return m, func() tea.Msg { return app.StatusMsg{Text: "SIGKILL failed: " + err.Error()} }
			}
			return m, func() tea.Msg { return app.StatusMsg{Text: "Sent SIGKILL to owner"} }
		}
	}
	return m, nil
}

func (m Model) current() (portItem, bool) {
	if len(m.items) == 0 || m.selected < 0 || m.selected >= len(m.items) {
		return portItem{}, false
	}
	return m.items[m.selected], true
}

func signalPID(pid int32, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (m Model) View() string {
	header := styles.ListHeaderStyle.Render("Ports")
	status := styles.HelpStyle.Render(fmt.Sprintf("r:refresh a:all(%v) Enter:actions p:jump c/y:copy j/k:scroll", m.showAll))
	if m.lastErr != "" {
		status = styles.ErrorStyle.Render(m.lastErr)
	}
	if len(m.items) == 0 {
		base := lipgloss.JoinVertical(lipgloss.Left, header, styles.HelpStyle.Render("No socket entries found."), status)
		if m.modalOpen {
			return m.renderModal(base)
		}
		return base
	}

	start := min(m.offset, max(0, len(m.items)-1))
	end := min(len(m.items), start+m.contentHeight())
	rows := []string{lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%-6s %-7s %-7s %-16s %-7s %s", "PROTO", "PORT", "PID", "PROCESS", "STATE", "ADDRESS"))}
	for i := start; i < end; i++ {
		it := m.items[i]
		row := fmt.Sprintf("%-6s %-7d %-7d %-16s %-7s %s", it.Proto, it.Port, it.PID, truncate(it.Process, 16), it.Status, it.Addr)
		if i == m.selected {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Render("-> " + row)
		} else {
			row = "   " + row
		}
		rows = append(rows, row)
	}
	base := lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(rows, "\n"), status)
	if m.modalOpen {
		return m.renderModal(base)
	}
	return base
}

func (m Model) renderModal(base string) string {
	it, _ := m.current()
	content := []string{
		fmt.Sprintf("Port %d (%s)", it.Port, it.Process),
		"k: SIGTERM owner process",
		"K: SIGKILL owner process (confirm)",
		"p: jump to process tab",
		"c/y: copy host:port",
		"Esc: close",
	}
	if it.PID <= 0 {
		content = append([]string{"No owning PID available."}, content...)
	}
	if m.confirmForceKill {
		content = append([]string{"Press K again to confirm SIGKILL."}, content...)
	}
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1).
		Width(min(max(42, m.Width/2), m.Width-4)).
		Render(strings.Join(content, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

func loadPortsCmd(showAll bool) tea.Cmd {
	return func() tea.Msg {
		conns, err := gnet.Connections("inet")
		if err != nil {
			return portMsg{err: err}
		}

		out := make([]portItem, 0, len(conns))
		procNameCache := map[int32]string{}
		for _, c := range conns {
			if !showAll && strings.ToUpper(c.Status) != "LISTEN" {
				continue
			}
			pid := c.Pid
			name := "-"
			if pid > 0 {
				if cached, ok := procNameCache[pid]; ok {
					name = cached
				} else if p, err := process.NewProcess(pid); err == nil {
					if n, err := p.Name(); err == nil && n != "" {
						name = n
					}
					procNameCache[pid] = name
				}
			}

			proto := fmt.Sprintf("%d", c.Type)
			if c.Type == 1 {
				proto = "tcp"
			} else if c.Type == 2 {
				proto = "udp"
			}

			addr := fmt.Sprintf("%s:%d", c.Laddr.IP, c.Laddr.Port)
			out = append(out, portItem{
				Proto:   strings.ToUpper(proto),
				Port:    c.Laddr.Port,
				Addr:    addr,
				Status:  strings.ToUpper(c.Status),
				PID:     pid,
				Process: name,
			})
		}

		sort.Slice(out, func(i, j int) bool {
			if out[i].Port == out[j].Port {
				return out[i].PID < out[j].PID
			}
			return out[i].Port < out[j].Port
		})
		return portMsg{items: out}
	}
}

func (m *Model) move(delta int) {
	if len(m.items) == 0 {
		m.selected, m.offset = 0, 0
		return
	}
	m.selected = max(0, min(len(m.items)-1, m.selected+delta))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.contentHeight() {
		m.offset = m.selected - m.contentHeight() + 1
	}
}

func (m Model) contentHeight() int {
	h := m.Height - 7
	if h < 5 {
		return 5
	}
	return h
}

func tickCmd() tea.Cmd {
	return tea.Tick(2500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
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
