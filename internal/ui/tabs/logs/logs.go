package logs

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"phantom/internal/app"
	"phantom/internal/ui/components/styles"
	"phantom/internal/utils"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SourceType string

const (
	SourceFile         SourceType = "file"
	SourceJournaldUnit SourceType = "journald_unit"
	SourceCommand      SourceType = "command"
)

type SourceConfig struct {
	Name    string
	Type    SourceType
	Path    string
	Unit    string
	Cmd     string
	Color   string
	Enabled bool
}

type LogEntry struct {
	Source string
	Color  string
	Line   string
	At     time.Time
}

type sourceState struct {
	LastFileLineCount int
	LastSnapshot      []string
}

type tickMsg time.Time

type pollMsg struct {
	Entries []LogEntry
	States  map[string]sourceState
	Err     string
}

type Model struct {
	Width, Height int

	FilePath string
	Sources  []SourceConfig

	stateBySource map[string]sourceState
	entries       []LogEntry
	offset        int
	selected      int
	lastErr       string
	filtering     bool
	filter        textinput.Model
	follow        bool
	onlyErrors    bool

	sourcePickerOpen  bool
	sourcePickerIndex int
}

func New(filePath string) Model {
	in := textinput.New()
	in.Placeholder = "filter text"
	in.Prompt = "/ "
	m := Model{
		FilePath:         filePath,
		filter:           in,
		follow:           true,
		stateBySource:    make(map[string]sourceState),
		sourcePickerOpen: false,
	}
	m.ensureDefaultSource()
	return m
}

func (m *Model) ApplyConfig(filePath string, sources []SourceConfig) {
	if filePath != "" {
		m.FilePath = filePath
	}
	if len(sources) > 0 {
		m.Sources = normalizeSources(sources)
	} else {
		m.Sources = nil
		m.ensureDefaultSource()
	}
	if m.stateBySource == nil {
		m.stateBySource = make(map[string]sourceState)
	}
}

func (m *Model) ensureDefaultSource() {
	if len(m.Sources) > 0 {
		return
	}
	path := m.FilePath
	if strings.TrimSpace(path) == "" {
		path = "debug.log"
	}
	m.Sources = []SourceConfig{{
		Name:    "main",
		Type:    SourceFile,
		Path:    path,
		Color:   "cyan",
		Enabled: true,
	}}
}

func normalizeSources(in []SourceConfig) []SourceConfig {
	out := make([]SourceConfig, 0, len(in))
	for i, s := range in {
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" {
			s.Name = fmt.Sprintf("src%d", i+1)
		}
		if s.Type == "" {
			if s.Path != "" {
				s.Type = SourceFile
			} else if s.Unit != "" {
				s.Type = SourceJournaldUnit
			} else {
				s.Type = SourceCommand
			}
		}
		if !s.Enabled {
			s.Enabled = true
		}
		out = append(out, s)
	}
	return out
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.pollCmd(), tickCmd())
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(m.pollCmd(), tickCmd())
	case pollMsg:
		if msg.Err != "" {
			m.lastErr = msg.Err
		} else {
			m.lastErr = ""
		}
		if msg.States != nil {
			m.stateBySource = msg.States
		}
		if len(msg.Entries) > 0 {
			m.entries = append(m.entries, msg.Entries...)
			if len(m.entries) > 5000 {
				m.entries = m.entries[len(m.entries)-5000:]
			}
		}
		if m.follow {
			visible := m.visibleEntries()
			m.selected = max(0, len(visible)-1)
			m.offset = max(0, len(visible)-m.contentHeight())
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

		switch msg.String() {
		case "r":
			return m, m.pollCmd()
		case "f":
			m.follow = !m.follow
		case "l":
			m.onlyErrors = !m.onlyErrors
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
			if n := len(m.visibleEntries()); n > 0 {
				m.selected = n - 1
				m.offset = max(0, n-m.contentHeight())
			}
		case "S":
			m.sourcePickerOpen = !m.sourcePickerOpen
		case "[":
			if m.sourcePickerOpen && len(m.Sources) > 0 {
				m.sourcePickerIndex--
				if m.sourcePickerIndex < 0 {
					m.sourcePickerIndex = len(m.Sources) - 1
				}
				m.Sources[m.sourcePickerIndex].Enabled = !m.Sources[m.sourcePickerIndex].Enabled
			}
		case "]":
			if m.sourcePickerOpen && len(m.Sources) > 0 {
				m.sourcePickerIndex = (m.sourcePickerIndex + 1) % len(m.Sources)
				m.Sources[m.sourcePickerIndex].Enabled = !m.Sources[m.sourcePickerIndex].Enabled
			}
		case "y":
			visible := m.visibleEntries()
			if len(visible) > 0 && m.selected >= 0 && m.selected < len(visible) {
				_, detail := utils.Yank(visible[m.selected].Line)
				return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked: " + detail} }
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	header := styles.ListHeaderStyle.Render("Logs")
	status := styles.HelpStyle.Render(fmt.Sprintf("r:refresh f:follow(%v) l:error-only(%v) /:filter S:sources []:toggle y:yank", m.follow, m.onlyErrors))
	if m.lastErr != "" {
		status = styles.ErrorStyle.Render(m.lastErr)
	}
	if m.filtering {
		status = lipgloss.JoinVertical(lipgloss.Left, status, m.filter.View())
	}

	rows := []string{}
	visible := m.visibleEntries()
	if len(visible) == 0 {
		rows = append(rows, styles.HelpStyle.Render("No lines to display."))
	} else {
		start := min(m.offset, max(0, len(visible)-1))
		end := min(len(visible), start+m.contentHeight())
		for i := start; i < end; i++ {
			entry := visible[i]
			prefix := "  "
			lineStyle := lipgloss.NewStyle()
			if i == m.selected {
				prefix = "->"
				lineStyle = lineStyle.Foreground(lipgloss.Color("69"))
			}
			badge := sourceBadge(entry.Source, entry.Color)
			rows = append(rows, lineStyle.Render(fmt.Sprintf("%s %s %s", prefix, badge, entry.Line)))
		}
	}
	body := strings.Join(rows, "\n")

	if !m.sourcePickerOpen {
		return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
	}

	sourceRows := []string{styles.ListHeaderStyle.Render("Sources")}
	for i, s := range m.Sources {
		cursor := "  "
		if i == m.sourcePickerIndex {
			cursor = "->"
		}
		toggle := "on "
		if !s.Enabled {
			toggle = "off"
		}
		sourceRows = append(sourceRows, fmt.Sprintf("%s [%s] %s", cursor, toggle, sourceBadge(s.Name, s.Color)))
	}
	sourcePane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1).
		Width(min(40, max(28, m.Width/3))).
		Render(strings.Join(sourceRows, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, sourcePane, "  ", body), status)
}

func sourceBadge(name, color string) string {
	c := mapColor(color)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("[" + name + "]")
}

func mapColor(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "red":
		return "196"
	case "green":
		return "70"
	case "yellow":
		return "220"
	case "blue":
		return "39"
	case "magenta":
		return "201"
	case "cyan":
		return "44"
	case "white":
		return "255"
	default:
		return "250"
	}
}

func (m Model) pollCmd() tea.Cmd {
	snapshot := m
	return func() tea.Msg {
		if len(snapshot.Sources) == 0 {
			snapshot.ensureDefaultSource()
		}
		states := cloneStates(snapshot.stateBySource)
		entries := make([]LogEntry, 0, 128)
		var errs []string

		for _, src := range snapshot.Sources {
			if !src.Enabled {
				continue
			}
			state := states[src.Name]
			newLines, nextState, err := pollSource(src, state)
			states[src.Name] = nextState
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", src.Name, err))
				continue
			}
			for _, line := range newLines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				entries = append(entries, LogEntry{Source: src.Name, Color: src.Color, Line: line, At: time.Now()})
			}
		}

		errText := ""
		if len(errs) > 0 {
			errText = strings.Join(errs, " | ")
		}
		return pollMsg{Entries: entries, States: states, Err: errText}
	}
}

func cloneStates(in map[string]sourceState) map[string]sourceState {
	out := make(map[string]sourceState, len(in))
	for k, v := range in {
		cp := sourceState{LastFileLineCount: v.LastFileLineCount}
		if len(v.LastSnapshot) > 0 {
			cp.LastSnapshot = append([]string{}, v.LastSnapshot...)
		}
		out[k] = cp
	}
	return out
}

func pollSource(src SourceConfig, state sourceState) ([]string, sourceState, error) {
	switch src.Type {
	case SourceFile:
		path := strings.TrimSpace(src.Path)
		if path == "" {
			return nil, state, fmt.Errorf("missing file path")
		}
		lines, err := readLines(path)
		if err != nil {
			return nil, state, err
		}
		if state.LastFileLineCount > len(lines) {
			state.LastFileLineCount = 0
		}
		newLines := lines[state.LastFileLineCount:]
		state.LastFileLineCount = len(lines)
		if len(newLines) > 200 {
			newLines = newLines[len(newLines)-200:]
		}
		return newLines, state, nil
	case SourceJournaldUnit:
		unit := strings.TrimSpace(src.Unit)
		if unit == "" {
			return nil, state, fmt.Errorf("missing journald unit")
		}
		return pollCommandSnapshot("journalctl -u "+shellEscape(unit)+" -n 80 --no-pager --output=short-iso", state)
	case SourceCommand:
		cmd := strings.TrimSpace(src.Cmd)
		if cmd == "" {
			return nil, state, fmt.Errorf("missing command")
		}
		return pollCommandSnapshot(cmd, state)
	default:
		return nil, state, fmt.Errorf("unsupported source type: %s", src.Type)
	}
}

func pollCommandSnapshot(command string, state sourceState) ([]string, sourceState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, state, nil
		}
		return nil, state, err
	}
	lines := splitLines(string(out), 300)
	newLines := diffSnapshot(state.LastSnapshot, lines)
	state.LastSnapshot = lines
	return newLines, state, nil
}

func diffSnapshot(prev, curr []string) []string {
	if len(curr) == 0 {
		return nil
	}
	if len(prev) == 0 {
		if len(curr) > 50 {
			return curr[len(curr)-50:]
		}
		return curr
	}
	last := prev[len(prev)-1]
	for i := len(curr) - 1; i >= 0; i-- {
		if curr[i] == last {
			if i+1 >= len(curr) {
				return nil
			}
			return curr[i+1:]
		}
	}
	if len(curr) > 50 {
		return curr[len(curr)-50:]
	}
	return curr
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	lines := []string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func splitLines(input string, maxLines int) []string {
	raw := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if maxLines > 0 && len(out) > maxLines {
		return out[len(out)-maxLines:]
	}
	return out
}

func shellEscape(s string) string {
	s = strings.ReplaceAll(s, "'", "'\\''")
	return "'" + s + "'"
}

func (m *Model) move(delta int) {
	visible := m.visibleEntries()
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	m.selected = max(0, min(len(visible)-1, m.selected+delta))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.contentHeight() {
		m.offset = m.selected - m.contentHeight() + 1
	}
}

func (m Model) visibleEntries() []LogEntry {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	enabled := make(map[string]bool, len(m.Sources))
	for _, s := range m.Sources {
		enabled[s.Name] = s.Enabled
	}
	out := make([]LogEntry, 0, len(m.entries))
	for _, e := range m.entries {
		if !enabled[e.Source] {
			continue
		}
		lower := strings.ToLower(e.Line)
		if m.onlyErrors && !(strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic")) {
			continue
		}
		if q != "" && !strings.Contains(lower, q) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (m Model) contentHeight() int {
	h := m.Height - 6
	if h < 5 {
		return 5
	}
	return h
}

func tickCmd() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
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
