package processes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"phantom/internal/app"
	"phantom/internal/ui/components/styles"
	"phantom/internal/utils"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/process"
)

type tickMsg time.Time

type procInfo struct {
	PID  int32
	Name string
	CPU  float64
	RSS  uint64
}

type procListMsg struct {
	items []procInfo
	err   error
}

type Model struct {
	Width, Height int
	items         []procInfo
	offset        int
	selected      int
	filtering     bool
	filter        textinput.Model
	lastErr       string
	limit         int

	modalOpen         bool
	modalTitle        string
	modalLines        []string
	modalOffset       int
	confirmForceKill  bool
	signalPicker      bool
	signalPickerIndex int
}

var signalOptions = []struct {
	label string
	sig   syscall.Signal
}{
	{label: "SIGHUP", sig: syscall.SIGHUP},
	{label: "SIGUSR1", sig: syscall.SIGUSR1},
	{label: "SIGUSR2", sig: syscall.SIGUSR2},
	{label: "SIGSTOP", sig: syscall.SIGSTOP},
	{label: "SIGCONT", sig: syscall.SIGCONT},
}

func New() Model {
	in := textinput.New()
	in.Placeholder = "filter process name"
	in.Prompt = "/ "
	return Model{limit: 120, filter: in}
}

func (m *Model) FocusPID(pid int32) {
	for i, p := range m.items {
		if p.PID == pid {
			m.selected = i
			if m.selected < m.offset {
				m.offset = m.selected
			}
			if m.selected >= m.offset+m.contentHeight() {
				m.offset = m.selected - m.contentHeight() + 1
			}
			break
		}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadProcessesCmd(m.limit), tickCmd())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(loadProcessesCmd(m.limit), tickCmd())
	case procListMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = ""
			m.items = msg.items
			if m.selected >= len(m.visibleItems()) {
				m.selected = max(0, len(m.visibleItems())-1)
			}
		}
	case tea.KeyMsg:
		if m.filtering {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			switch msg.Type {
			case tea.KeyEnter:
				m.filtering = false
				m.offset = 0
				m.selected = 0
				return m, nil
			case tea.KeyEsc:
				m.filtering = false
				m.filter.SetValue("")
				return m, nil
			default:
				return m, cmd
			}
		}

		if m.modalOpen {
			return m.handleModalKeys(msg)
		}

		switch msg.String() {
		case "r":
			return m, loadProcessesCmd(m.limit)
		case "/":
			m.filtering = true
			m.filter.Focus()
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "g":
			m.selected = 0
			m.offset = 0
		case "G":
			if n := len(m.visibleItems()); n > 0 {
				m.selected = n - 1
				m.offset = max(0, n-m.contentHeight())
			}
		case "enter":
			m.openActionsModal()
		case "e":
			return m.openEnvModal()
		case "f":
			return m.openFDModal()
		case "c", "y":
			if p, ok := m.current(); ok {
				_, detail := utils.Yank(fmt.Sprintf("%d", p.PID))
				return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked PID: " + detail} }
			}
		}
	}
	return m, nil
}

func (m Model) handleModalKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.signalPicker {
		switch msg.String() {
		case "left", "h", "k", "up":
			m.signalPickerIndex--
			if m.signalPickerIndex < 0 {
				m.signalPickerIndex = len(signalOptions) - 1
			}
			return m, nil
		case "right", "l", "j", "down":
			m.signalPickerIndex = (m.signalPickerIndex + 1) % len(signalOptions)
			return m, nil
		case "enter":
			if p, ok := m.current(); ok {
				err := sendSignal(p.PID, signalOptions[m.signalPickerIndex].sig)
				m.signalPicker = false
				if err != nil {
					return m, func() tea.Msg { return app.StatusMsg{Text: "Signal failed: " + err.Error()} }
				}
				return m, func() tea.Msg { return app.StatusMsg{Text: "Sent " + signalOptions[m.signalPickerIndex].label} }
			}
		case "esc":
			m.signalPicker = false
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.modalOpen = false
		m.modalTitle = ""
		m.modalLines = nil
		m.modalOffset = 0
		m.confirmForceKill = false
		return m, nil
	case "j", "down":
		m.modalOffset++
		maxOffset := max(0, len(m.modalLines)-m.modalHeight())
		m.modalOffset = min(m.modalOffset, maxOffset)
		return m, nil
	case "up":
		m.modalOffset = max(0, m.modalOffset-1)
		return m, nil
	case "c", "y":
		if p, ok := m.current(); ok {
			_, detail := utils.Yank(fmt.Sprintf("%d", p.PID))
			return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked PID: " + detail} }
		}
	case "k":
		if p, ok := m.current(); ok {
			err := sendSignal(p.PID, syscall.SIGTERM)
			if err != nil {
				return m, func() tea.Msg { return app.StatusMsg{Text: "SIGTERM failed: " + err.Error()} }
			}
			return m, func() tea.Msg { return app.StatusMsg{Text: "Sent SIGTERM"} }
		}
	case "K":
		if !m.confirmForceKill {
			m.confirmForceKill = true
			m.modalLines = append([]string{"Press K again to confirm SIGKILL."}, m.modalLines...)
			return m, nil
		}
		if p, ok := m.current(); ok {
			err := sendSignal(p.PID, syscall.SIGKILL)
			m.confirmForceKill = false
			if err != nil {
				return m, func() tea.Msg { return app.StatusMsg{Text: "SIGKILL failed: " + err.Error()} }
			}
			return m, func() tea.Msg { return app.StatusMsg{Text: "Sent SIGKILL"} }
		}
	case "s":
		m.signalPicker = true
		m.signalPickerIndex = 0
		return m, nil
	case "e":
		return m.openEnvModal()
	case "f":
		return m.openFDModal()
	}
	return m, nil
}

func (m *Model) openActionsModal() {
	p, ok := m.current()
	if !ok {
		return
	}
	m.modalOpen = true
	m.modalTitle = fmt.Sprintf("Process Actions (%s:%d)", p.Name, p.PID)
	m.modalLines = []string{
		"k: SIGTERM",
		"K: SIGKILL (confirm)",
		"s: custom signal picker",
		"e: environment variables",
		"f: open file descriptors",
		"c/y: copy PID",
		"Esc: close",
	}
	m.modalOffset = 0
	m.confirmForceKill = false
	m.signalPicker = false
}

func (m Model) openEnvModal() (Model, tea.Cmd) {
	p, ok := m.current()
	if !ok {
		return m, nil
	}
	lines, err := readProcEnv(p.PID)
	if err != nil {
		m.modalOpen = true
		m.modalTitle = fmt.Sprintf("Env (%d)", p.PID)
		m.modalLines = []string{"Error: " + err.Error()}
		m.modalOffset = 0
		return m, nil
	}
	m.modalOpen = true
	m.modalTitle = fmt.Sprintf("Env (%s:%d)", p.Name, p.PID)
	m.modalLines = lines
	m.modalOffset = 0
	return m, nil
}

func (m Model) openFDModal() (Model, tea.Cmd) {
	p, ok := m.current()
	if !ok {
		return m, nil
	}
	lines, err := readProcFDs(p.PID)
	if err != nil {
		m.modalOpen = true
		m.modalTitle = fmt.Sprintf("FDs (%d)", p.PID)
		m.modalLines = []string{"Error: " + err.Error()}
		m.modalOffset = 0
		return m, nil
	}
	m.modalOpen = true
	m.modalTitle = fmt.Sprintf("FDs (%s:%d)", p.Name, p.PID)
	m.modalLines = lines
	m.modalOffset = 0
	return m, nil
}

func (m Model) current() (procInfo, bool) {
	items := m.visibleItems()
	if len(items) == 0 || m.selected < 0 || m.selected >= len(items) {
		return procInfo{}, false
	}
	return items[m.selected], true
}

func sendSignal(pid int32, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func readProcEnv(pid int32) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}
	raw := strings.Split(string(data), "\x00")
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		out = append(out, entry)
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"(empty environ)"}
	}
	return out, nil
}

func readProcFDs(pid int32) ([]string, error) {
	root := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		fdPath := filepath.Join(root, e.Name())
		resolved, err := os.Readlink(fdPath)
		if err != nil {
			resolved = "?"
		}
		out = append(out, fmt.Sprintf("fd %s -> %s", e.Name(), resolved))
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"(no file descriptors)"}
	}
	return out, nil
}

func (m Model) View() string {
	header := styles.ListHeaderStyle.Render("Processes")
	status := styles.HelpStyle.Render("r:refresh /:filter Enter:actions e:env f:fds c/y:copy-pid j/k:scroll")
	if m.lastErr != "" {
		status = styles.ErrorStyle.Render(m.lastErr)
	}
	if m.filtering {
		status = lipgloss.JoinVertical(lipgloss.Left, status, m.filter.View())
	}

	items := m.visibleItems()
	if len(items) == 0 {
		base := lipgloss.JoinVertical(lipgloss.Left, header, styles.HelpStyle.Render("No processes found."), status)
		if m.modalOpen {
			return m.renderModal(base)
		}
		return base
	}

	start := min(m.offset, max(0, len(items)-1))
	end := min(len(items), start+m.contentHeight())
	rows := []string{
		lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%-8s %-8s %-12s %s", "PID", "CPU%", "RSS", "NAME")),
	}
	for i := start; i < end; i++ {
		p := items[i]
		row := fmt.Sprintf("%-8d %-8.1f %-12s %s", p.PID, p.CPU, utils.FormatBytes(p.RSS), p.Name)
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
	lines := m.modalLines
	if len(lines) == 0 {
		lines = []string{"(no data)"}
	}
	start := min(m.modalOffset, max(0, len(lines)-1))
	end := min(len(lines), start+m.modalHeight())
	content := strings.Join(lines[start:end], "\n")

	if m.signalPicker {
		pickerRows := []string{"Select signal (Enter to send, Esc to cancel):"}
		for i, s := range signalOptions {
			row := "  " + s.label
			if i == m.signalPickerIndex {
				row = "-> " + s.label
			}
			pickerRows = append(pickerRows, row)
		}
		content = content + "\n\n" + strings.Join(pickerRows, "\n")
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1).
		Width(min(max(50, m.Width-12), m.Width-4)).
		Render(lipgloss.NewStyle().Bold(true).Render(m.modalTitle) + "\n" + content)

	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

func loadProcessesCmd(limit int) tea.Cmd {
	return func() tea.Msg {
		ps, err := process.Processes()
		if err != nil {
			return procListMsg{err: err}
		}
		items := make([]procInfo, 0, len(ps))
		for _, p := range ps {
			name, err := p.Name()
			if err != nil || name == "" {
				continue
			}
			memInfo, _ := p.MemoryInfo()
			cpuPercent, _ := p.CPUPercent()
			var rss uint64
			if memInfo != nil {
				rss = memInfo.RSS
			}
			items = append(items, procInfo{PID: p.Pid, Name: name, CPU: cpuPercent, RSS: rss})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].RSS == items[j].RSS {
				return items[i].CPU > items[j].CPU
			}
			return items[i].RSS > items[j].RSS
		})
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		return procListMsg{items: items}
	}
}

func (m Model) visibleItems() []procInfo {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if q == "" {
		return m.items
	}
	out := make([]procInfo, 0, len(m.items))
	for _, p := range m.items {
		if strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

func (m *Model) move(delta int) {
	items := m.visibleItems()
	if len(items) == 0 {
		m.selected, m.offset = 0, 0
		return
	}
	m.selected = max(0, min(len(items)-1, m.selected+delta))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.contentHeight() {
		m.offset = m.selected - m.contentHeight() + 1
	}
}

func (m Model) contentHeight() int {
	h := m.Height - 8
	if h < 5 {
		return 5
	}
	return h
}

func (m Model) modalHeight() int {
	h := m.Height / 3
	if h < 6 {
		return 6
	}
	if h > 20 {
		return 20
	}
	return h
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
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
