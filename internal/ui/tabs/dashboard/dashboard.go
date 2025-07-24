package dashboard

import (
	"fmt"
	"strings"
	"time"

	"phantom/internal/ui/components/styles"
	"phantom/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Model represents the dashboard tab.
type Model struct {
	Width, Height int
	SysInfo       SysInfoMsg
	Procs         []*process.Process
}

// Messages for the dashboard
type TickMsg time.Time
type SysInfoMsg struct{ CpuPercent, MemPercent, DiskPercent float64 }
type ProcListMsg []*process.Process

// Init initializes the dashboard model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(TickCmd(), GetSysInfo(), GetProcs())
}

// Update handles messages for the dashboard model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		return m, tea.Batch(GetSysInfo(), GetProcs())
	case SysInfoMsg:
		m.SysInfo = msg
	case ProcListMsg:
		m.Procs = msg
	}
	return m, nil
}

// View renders the dashboard model.
func (m Model) View() string {
	cpuBar := renderBar("CPU", m.SysInfo.CpuPercent)
	memBar := renderBar("Memory", m.SysInfo.MemPercent)
	diskBar := renderBar("Disk", m.SysInfo.DiskPercent)
	stats := lipgloss.JoinVertical(lipgloss.Left, cpuBar, memBar, diskBar)

	var procList strings.Builder
	procList.WriteString(styles.ListHeaderStyle.Render("Top Processes (by memory)"))
	procList.WriteString("\n")
	for i, p := range m.Procs {
		if i > (m.Height - 10) {
			break
		}
		name, _ := p.Name()
		memInfo, _ := p.MemoryInfo()
		procList.WriteString(fmt.Sprintf("%-30s %-10d %-10s", name, p.Pid, utils.FormatBytes(memInfo.RSS)))
		procList.WriteString("\n")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.Width/2-2).Render(stats),
		lipgloss.NewStyle().Width(m.Width/2-2).Render(procList.String()),
	)
}

// Helper functions for system info
func TickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg { return TickMsg(t) })
}

func GetSysInfo() tea.Cmd {
	return func() tea.Msg {
		cpuPs, _ := cpu.Percent(0, false)
		memPs, _ := mem.VirtualMemory()
		diskPs, _ := disk.Usage("/")
		return SysInfoMsg{CpuPercent: cpuPs[0], MemPercent: memPs.UsedPercent, DiskPercent: diskPs.UsedPercent}
	}
}

func GetProcs() tea.Cmd {
	return func() tea.Msg {
		ps, _ := process.Processes()
		// Sorting processes by memory usage would be a good enhancement here
		return ProcListMsg(ps)
	}
}

func renderBar(name string, percent float64) string {
	width := 50
	fillWidth := int(float64(width) * (percent / 100))
	bar := strings.Repeat("=", fillWidth) + strings.Repeat("-", width-fillWidth)
	return styles.BarHeaderStyle.Render(fmt.Sprintf("%-6s [%s] %.2f%%", name, bar, percent))
}
