package explorer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"phantom/internal/app"
	"phantom/internal/ui/components/styles"
	"phantom/internal/utils"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

type mode int

const (
	modeBrowse mode = iota
	modePaste
	modeFile
	modeSearch
	modeFilter
)

type nodeRow struct {
	Path        string
	Depth       int
	Key         string
	Node        any
	IsContainer bool
}

type parseMsg struct {
	Raw    string
	Root   any
	Format string
	Err    error
}

type editorDoneMsg struct {
	Path string
	Err  error
}

type Model struct {
	Width, Height int

	CurrentMode mode
	RawInput    string
	Root        any
	Format      string
	ErrText     string

	rows     []nodeRow
	offset   int
	selected int
	expanded map[string]bool

	pasteInput  textarea.Model
	fileInput   textinput.Model
	searchInput textinput.Model
	filterInput textinput.Model

	SearchTerm      string
	FilterExpr      string
	ConvertedFormat string
	ConvertedOutput string
	editorPath      string
}

func New() Model {
	paste := textarea.New()
	paste.Placeholder = "Paste JSON/YAML/TOML here"
	paste.SetHeight(14)

	fileIn := textinput.New()
	fileIn.Prompt = "file> "
	fileIn.Placeholder = "./path/to/file.json"

	searchIn := textinput.New()
	searchIn.Prompt = "/ "
	searchIn.Placeholder = "search keys/values"

	filterIn := textinput.New()
	filterIn.Prompt = "jq> "
	filterIn.Placeholder = ".data.users[] | .email"

	return Model{
		CurrentMode: modeBrowse,
		expanded:    map[string]bool{"$": true},
		pasteInput:  paste,
		fileInput:   fileIn,
		searchInput: searchIn,
		filterInput: filterIn,
	}
}

func (m Model) Init() tea.Cmd { return nil }

// LoadRaw parses and sets a document immediately (used by CLI preloading).
func (m Model) LoadRaw(raw string) Model {
	root, format, err := parseInput(raw)
	if err != nil {
		m.ErrText = err.Error()
		return m
	}
	m.ErrText = ""
	m.RawInput = raw
	m.Root = root
	m.Format = format
	m.SearchTerm = ""
	m.FilterExpr = ""
	m.offset = 0
	m.selected = 0
	m.expanded = map[string]bool{"$": true}
	m.rebuildRows()
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.pasteInput.SetWidth(max(30, m.Width-10))
		m.fileInput.Width = max(20, m.Width-16)
		m.searchInput.Width = max(20, m.Width-16)
		m.filterInput.Width = max(20, m.Width-16)

	case parseMsg:
		if msg.Err != nil {
			m.ErrText = msg.Err.Error()
			return m, func() tea.Msg { return app.StatusMsg{Text: "Parse failed"} }
		}
		m.ErrText = ""
		m.RawInput = msg.Raw
		m.Root = msg.Root
		m.Format = msg.Format
		m.SearchTerm = ""
		m.FilterExpr = ""
		m.offset = 0
		m.selected = 0
		m.expanded = map[string]bool{"$": true}
		m.rebuildRows()
		return m, func() tea.Msg { return app.StatusMsg{Text: "Loaded " + msg.Format} }

	case editorDoneMsg:
		if msg.Err != nil {
			return m, func() tea.Msg { return app.StatusMsg{Text: "Editor failed: " + msg.Err.Error()} }
		}
		data, err := os.ReadFile(msg.Path)
		if err != nil {
			return m, func() tea.Msg { return app.StatusMsg{Text: "Read failed: " + err.Error()} }
		}
		return m, parseInputCmd(string(data))

	case tea.KeyMsg:
		switch m.CurrentMode {
		case modePaste:
			return m.updatePaste(msg)
		case modeFile:
			return m.updateFile(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeFilter:
			return m.updateFilter(msg)
		default:
			return m.updateBrowse(msg)
		}
	}
	return m, nil
}

func (m Model) updatePaste(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.CurrentMode = modeBrowse
		m.pasteInput.Blur()
		return m, nil
	case "ctrl+s":
		m.CurrentMode = modeBrowse
		m.pasteInput.Blur()
		return m, parseInputCmd(m.pasteInput.Value())
	}
	var cmd tea.Cmd
	m.pasteInput, cmd = m.pasteInput.Update(msg)
	return m, cmd
}

func (m Model) updateFile(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.CurrentMode = modeBrowse
		m.fileInput.Blur()
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.fileInput.Value())
		m.CurrentMode = modeBrowse
		m.fileInput.Blur()
		if path == "" {
			return m, func() tea.Msg { return app.StatusMsg{Text: "Path is empty"} }
		}
		return m, loadFileCmd(path)
	}
	var cmd tea.Cmd
	m.fileInput, cmd = m.fileInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.CurrentMode = modeBrowse
		m.searchInput.Blur()
		return m, nil
	case "enter":
		m.SearchTerm = strings.TrimSpace(m.searchInput.Value())
		m.CurrentMode = modeBrowse
		m.searchInput.Blur()
		m.offset = 0
		m.selected = 0
		m.rebuildRows()
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.CurrentMode = modeBrowse
		m.filterInput.Blur()
		return m, nil
	case "enter":
		expr := strings.TrimSpace(m.filterInput.Value())
		m.CurrentMode = modeBrowse
		m.filterInput.Blur()
		if expr == "" {
			return m, nil
		}
		return m, applyFilterCmd(m.Root, expr)
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m Model) updateBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "p":
		m.CurrentMode = modePaste
		m.pasteInput.SetValue(m.RawInput)
		m.pasteInput.Focus()
		return m, nil
	case "f":
		m.CurrentMode = modeFile
		m.fileInput.SetValue("")
		m.fileInput.Focus()
		return m, nil
	case "e":
		if strings.TrimSpace(m.RawInput) == "" {
			return m, func() tea.Msg { return app.StatusMsg{Text: "Nothing loaded"} }
		}
		path := filepath.Join(os.TempDir(), "phantom_explorer_input.tmp")
		if err := os.WriteFile(path, []byte(m.RawInput), 0o600); err != nil {
			return m, func() tea.Msg { return app.StatusMsg{Text: "Temp write failed: " + err.Error()} }
		}
		m.editorPath = path
		editor := strings.TrimSpace(os.Getenv("EDITOR"))
		if editor == "" {
			editor = "vi"
		}
		cmd := exec.Command(editor, path)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{Path: path, Err: err} })
	case "/":
		m.CurrentMode = modeSearch
		m.searchInput.SetValue(m.SearchTerm)
		m.searchInput.Focus()
		return m, nil
	case "F":
		m.CurrentMode = modeFilter
		m.filterInput.SetValue(m.FilterExpr)
		m.filterInput.Focus()
		return m, nil
	case "R":
		if strings.TrimSpace(m.RawInput) != "" {
			return m, parseInputCmd(m.RawInput)
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		m.selected = 0
		m.offset = 0
	case "G":
		if len(m.rows) > 0 {
			m.selected = len(m.rows) - 1
			m.offset = max(0, len(m.rows)-m.contentHeight())
		}
	case "enter":
		if row, ok := m.current(); ok && row.IsContainer {
			m.expanded[row.Path] = !m.expanded[row.Path]
			m.rebuildRows()
		}
	case "y":
		if row, ok := m.current(); ok {
			b, _ := json.MarshalIndent(row.Node, "", "  ")
			_, detail := utils.Yank(string(b))
			return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked node: " + detail} }
		}
	case "P":
		if row, ok := m.current(); ok {
			_, detail := utils.Yank(row.Path)
			return m, func() tea.Msg { return app.StatusMsg{Text: "Yanked path: " + detail} }
		}
	case "c":
		if row, ok := m.current(); ok {
			fmtOrder := []string{"json", "yaml", "toml"}
			next := 0
			for i, f := range fmtOrder {
				if f == m.ConvertedFormat {
					next = (i + 1) % len(fmtOrder)
					break
				}
			}
			m.ConvertedFormat = fmtOrder[next]
			out, err := convertNode(row.Node, m.ConvertedFormat)
			if err != nil {
				m.ConvertedOutput = "convert error: " + err.Error()
			} else {
				m.ConvertedOutput = out
			}
		}
	}
	return m, nil
}

func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	m.selected = max(0, min(len(m.rows)-1, m.selected+delta))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.contentHeight() {
		m.offset = m.selected - m.contentHeight() + 1
	}
}

func (m *Model) rebuildRows() {
	m.rows = flattenRows(m.Root, "$", "$", 0, m.expanded)
	if m.SearchTerm != "" {
		q := strings.ToLower(m.SearchTerm)
		filtered := make([]nodeRow, 0, len(m.rows))
		for _, r := range m.rows {
			label := strings.ToLower(r.Key + " " + r.Path + " " + previewValue(r.Node))
			if strings.Contains(label, q) {
				filtered = append(filtered, r)
			}
		}
		m.rows = filtered
	}
	if m.selected >= len(m.rows) {
		m.selected = max(0, len(m.rows)-1)
	}
}

func flattenRows(node any, key, path string, depth int, expanded map[string]bool) []nodeRow {
	if node == nil {
		return []nodeRow{{Path: path, Key: key, Node: nil, Depth: depth, IsContainer: false}}
	}
	rows := []nodeRow{}
	isContainer := false
	switch node.(type) {
	case map[string]any, []any:
		isContainer = true
	}
	rows = append(rows, nodeRow{Path: path, Key: key, Node: node, Depth: depth, IsContainer: isContainer})
	if !isContainer || !expanded[path] {
		return rows
	}
	switch val := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			childPath := path + "." + k
			rows = append(rows, flattenRows(val[k], k, childPath, depth+1, expanded)...)
		}
	case []any:
		for i, item := range val {
			childKey := "[" + strconv.Itoa(i) + "]"
			childPath := path + childKey
			rows = append(rows, flattenRows(item, childKey, childPath, depth+1, expanded)...)
		}
	}
	return rows
}

func (m Model) current() (nodeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return nodeRow{}, false
	}
	return m.rows[m.selected], true
}

func (m Model) View() string {
	header := styles.ListHeaderStyle.Render("Explorer")
	if m.ErrText != "" {
		header += "\n" + styles.ErrorStyle.Render(m.ErrText)
	}

	help := styles.HelpStyle.Render("p:paste f:file e:editor Enter:expand /:search F:filter P:path-yank y:node-yank c:convert R:reset")

	if m.CurrentMode == modePaste {
		body := lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render("Paste mode (Ctrl+S to parse, Esc to cancel)"),
			m.pasteInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
	}
	if m.CurrentMode == modeFile {
		body := lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render("File mode (Enter to load, Esc to cancel)"),
			m.fileInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
	}
	if m.CurrentMode == modeSearch {
		body := lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render("Search mode (Enter to apply, Esc to cancel)"),
			m.searchInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
	}
	if m.CurrentMode == modeFilter {
		body := lipgloss.JoinVertical(lipgloss.Left,
			styles.ListHeaderStyle.Render("Filter mode (gojq expression; Enter to apply, Esc to cancel)"),
			m.filterInput.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
	}

	left := m.renderTree()
	right := m.renderDetail()
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.Width/2-2).Render(left),
		lipgloss.NewStyle().Width(m.Width-m.Width/2-4).Render(right),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, help)
}

func (m Model) renderTree() string {
	if len(m.rows) == 0 {
		return styles.HelpStyle.Render("No document loaded. Press p to paste or f to load a file.")
	}
	start := min(m.offset, max(0, len(m.rows)-1))
	end := min(len(m.rows), start+m.contentHeight())
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		r := m.rows[i]
		prefix := strings.Repeat("  ", r.Depth)
		nodeType := detectType(r.Node)
		symbol := "-"
		if r.IsContainer {
			if m.expanded[r.Path] {
				symbol = "v"
			} else {
				symbol = ">"
			}
		}
		line := fmt.Sprintf("%s%s %s (%s)", prefix, symbol, r.Key, nodeType)
		if i == m.selected {
			line = styles.ActiveTabStyle.Render(line)
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderDetail() string {
	if row, ok := m.current(); ok {
		pretty, _ := json.MarshalIndent(row.Node, "", "  ")
		base := []string{
			styles.ListHeaderStyle.Render("Path: " + row.Path),
			string(pretty),
		}
		if strings.TrimSpace(m.ConvertedOutput) != "" {
			base = append(base,
				styles.ListHeaderStyle.Render("Converted ("+m.ConvertedFormat+")"),
				m.ConvertedOutput,
			)
		}
		return strings.Join(base, "\n")
	}
	return ""
}

func parseInputCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		root, format, err := parseInput(raw)
		return parseMsg{Raw: raw, Root: root, Format: format, Err: err}
	}
}

func loadFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		b, err := os.ReadFile(path)
		if err != nil {
			return parseMsg{Err: err}
		}
		root, format, err := parseInput(string(b))
		return parseMsg{Raw: string(b), Root: root, Format: format, Err: err}
	}
}

func applyFilterCmd(root any, expr string) tea.Cmd {
	return func() tea.Msg {
		q, err := gojq.Parse(expr)
		if err != nil {
			return parseMsg{Err: err}
		}
		iter := q.Run(root)
		results := make([]any, 0)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				return parseMsg{Err: err}
			}
			results = append(results, v)
		}
		out := any(results)
		if len(results) == 1 {
			out = results[0]
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return parseMsg{Raw: string(b), Root: normalizeNode(out), Format: "json"}
	}
}

func parseInput(raw string) (any, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", errors.New("input is empty")
	}

	var j any
	if err := json.Unmarshal([]byte(raw), &j); err == nil {
		return normalizeNode(j), "json", nil
	}

	var y any
	if err := yaml.Unmarshal([]byte(raw), &y); err == nil {
		return normalizeNode(y), "yaml", nil
	}

	var t map[string]any
	if err := toml.Unmarshal([]byte(raw), &t); err == nil {
		return normalizeNode(t), "toml", nil
	}

	return nil, "", errors.New("unable to parse input as JSON, YAML, or TOML")
}

func normalizeNode(v any) any {
	switch val := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(val))
		for k, vv := range val {
			m[k] = normalizeNode(vv)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(val))
		for k, vv := range val {
			m[fmt.Sprintf("%v", k)] = normalizeNode(vv)
		}
		return m
	case []any:
		arr := make([]any, len(val))
		for i, item := range val {
			arr[i] = normalizeNode(item)
		}
		return arr
	default:
		return v
	}
}

func convertNode(node any, format string) (string, error) {
	node = normalizeNode(node)
	switch format {
	case "json":
		b, err := json.MarshalIndent(node, "", "  ")
		return string(b), err
	case "yaml":
		b, err := yaml.Marshal(node)
		return string(b), err
	case "toml":
		m, ok := node.(map[string]any)
		if !ok {
			return "", errors.New("TOML conversion requires object/map at current node")
		}
		var sb strings.Builder
		err := toml.NewEncoder(&sb).Encode(m)
		return sb.String(), err
	default:
		return "", errors.New("unknown format")
	}
}

func detectType(v any) string {
	switch vv := v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return fmt.Sprintf("object{%d}", len(vv))
	case []any:
		return fmt.Sprintf("array[%d]", len(vv))
	case string:
		return "string"
	case bool:
		return "bool"
	case float64, float32, int, int64, uint64, int32, uint32:
		return "number"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func previewValue(v any) string {
	if v == nil {
		return "null"
	}
	switch vv := v.(type) {
	case string:
		if len(vv) > 40 {
			return vv[:37] + "..."
		}
		return vv
	case bool, float64, float32, int, int64, uint64, int32, uint32:
		return fmt.Sprintf("%v", vv)
	default:
		return detectType(v)
	}
}

func (m Model) contentHeight() int {
	h := m.Height - 9
	if h < 6 {
		return 6
	}
	return h
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
