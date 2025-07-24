package http

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"phantom/internal/ui/components/styles" // Corrected import path
	"phantom/internal/utils"                // Corrected import path

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the HTTP client tab.
type Model struct {
	Width, Height int
	// Panes
	Collections list.Model
	History     list.Model
	// Inputs
	Methods        []string
	SelectedMethod int
	URL            textinput.Model
	Headers        textarea.Model
	Body           textarea.Model
	// Response
	Response        viewport.Model
	ResponseHeaders string
	ResponseBody    string
	ResponseCode    int
	ResponseViewTab int // 0: Pretty, 1: Raw, 2: Headers
	// State
	FocusedPane  int // 0: List, 1: Request, 2: Response
	FocusedInput int // 0: Method, 1: URL, 2: Headers, 3: Body
	Sending      bool
	Spinner      spinner.Model
	LastError    string
	// Config
	Environment map[string]string
}

// RequestItem represents an item in the collections/history list.
type RequestItem struct {
	Name, Method, URL, Headers, Body string
}

func (i RequestItem) Title() string       { return fmt.Sprintf("%s %s", i.Method, i.Name) }
func (i RequestItem) Description() string { return i.URL }
func (i RequestItem) FilterValue() string { return i.Name }

// HTTPResponseMsg is sent when an HTTP request completes.
type HTTPResponseMsg struct {
	Body, Headers string
	Code          int
	Err           error
}

// New creates a new HTTP model.
func New() Model {
	m := Model{
		FocusedPane:     1,
		FocusedInput:    0,
		ResponseViewTab: 0,
		Methods:         []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		SelectedMethod:  0,
	}

	m.URL = textinput.New()
	m.URL.Placeholder = "https://api.example.com"
	m.URL.Prompt = ""

	m.Headers = textarea.New()
	m.Headers.Placeholder = `"Content-Type": "application/json"`
	m.Headers.SetHeight(5)

	m.Body = textarea.New()
	m.Body.Placeholder = `{"key": "value"}`
	m.Body.SetHeight(10)

	m.Response = viewport.New(0, 0)
	m.Spinner = spinner.New()
	m.Spinner.Spinner = spinner.Dot
	m.Spinner.Style = styles.SpinnerStyle

	m.Collections = list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m.Collections.Title = "Collections"
	m.History = list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m.History.Title = "History"

	m.focus() // Set initial focus
	return m
}

// Init initializes the HTTP model.
func (m Model) Init() tea.Cmd {
	return m.Spinner.Tick
}

// Update handles messages for the HTTP model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Sending {
			return m, nil
		}

		// Pane/Global controls
		switch msg.String() {
		case "ctrl+l": // Switch focused pane
			m.FocusedPane = (m.FocusedPane + 1) % 3
			m.focus()
			return m, nil
		case "ctrl+s": // Send request
			m.Sending = true
			m.LastError = ""
			m.ResponseBody = ""
			m.ResponseHeaders = ""
			cmds = append(cmds, m.Spinner.Tick, m.sendRequest())
			return m, tea.Batch(cmds...)
		}

		// Delegate to focused pane
		switch m.FocusedPane {
		case 0: // List Pane
			m.Collections, cmd = m.Collections.Update(msg)
			cmds = append(cmds, cmd)
			if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
				if item, ok := m.Collections.SelectedItem().(RequestItem); ok {
					m.loadRequest(item)
				}
			}
		case 1: // Request Pane
			cmds = append(cmds, m.updateRequestInputs(msg))
		case 2: // Response Pane
			switch msg.String() {
			case "h", "left":
				m.ResponseViewTab--
				if m.ResponseViewTab < 0 {
					m.ResponseViewTab = 2
				}
				m.updateResponseView()
			case "l", "right":
				m.ResponseViewTab = (m.ResponseViewTab + 1) % 3
				m.updateResponseView()
			default:
				m.Response, cmd = m.Response.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case HTTPResponseMsg:
		m.Sending = false
		if msg.Err != nil {
			m.LastError = msg.Err.Error()
			m.ResponseCode = 0
		} else {
			m.ResponseBody = msg.Body
			m.ResponseHeaders = msg.Headers
			m.ResponseCode = msg.Code
			m.updateResponseView()
			// Add to history
			historyItem := RequestItem{
				Name:    m.URL.Value(),
				Method:  m.Methods[m.SelectedMethod],
				URL:     m.URL.Value(),
				Headers: m.Headers.Value(),
				Body:    m.Body.Value(),
			}
			newHistory := append([]list.Item{historyItem}, m.History.Items()...)
			if len(newHistory) > 20 { // Limit history
				newHistory = newHistory[:20]
			}
			m.History.SetItems(newHistory)
		}

	case spinner.TickMsg:
		if m.Sending {
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// View renders the HTTP model.
func (m Model) View() string {
	listPane := lipgloss.JoinVertical(lipgloss.Left, m.Collections.View(), m.History.View())

	var requestBuilder strings.Builder
	requestBuilder.WriteString(m.renderMethodSelector())
	requestBuilder.WriteString(m.renderInput("URL", m.URL, 1))
	requestBuilder.WriteString(m.renderTextarea("Headers", m.Headers, 2))
	requestBuilder.WriteString(m.renderTextarea("Body", m.Body, 3))
	requestPane := requestBuilder.String()

	var responseBuilder strings.Builder
	statusStyle := styles.SuccessStyle
	if m.ResponseCode >= 400 {
		statusStyle = styles.ErrorStyle
	}
	status := statusStyle.Render(fmt.Sprintf("%d", m.ResponseCode))
	responseHeader := styles.ListHeaderStyle.Render(fmt.Sprintf("Response - Status: %s", status))

	tabs := []string{"Pretty", "Raw", "Headers"}
	var renderedTabs []string
	for i, t := range tabs {
		style := styles.InactiveTabStyle
		if i == m.ResponseViewTab {
			style = styles.ActiveTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(t))
	}
	responseTabs := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	responseBuilder.WriteString(responseHeader + "\n" + responseTabs + "\n")
	if m.Sending {
		responseBuilder.WriteString(fmt.Sprintf("\n%s Sending request...", m.Spinner.View()))
	} else if m.LastError != "" {
		responseBuilder.WriteString(styles.ErrorStyle.Render(m.LastError))
	} else {
		responseBuilder.WriteString(m.Response.View())
	}
	responsePane := responseBuilder.String()

	listStyle, reqStyle, respStyle := styles.BlurredPaneStyle, styles.BlurredPaneStyle, styles.BlurredPaneStyle
	switch m.FocusedPane {
	case 0:
		listStyle = styles.FocusedPaneStyle
	case 1:
		reqStyle = styles.FocusedPaneStyle
	case 2:
		respStyle = styles.FocusedPaneStyle
	}

	help := styles.HelpStyle.Render("Focus: Ctrl+L | Send: Ctrl+S | Navigate: Tab/Arrows | Resp View: H/L")

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			listStyle.Width(m.Width/4).Height(m.Height-2).Render(listPane),
			reqStyle.Width(m.Width/2).Height(m.Height-2).Render(requestPane),
			respStyle.Width(m.Width-m.Width/4-m.Width/2).Height(m.Height-2).Render(responsePane),
		),
		help,
	)
}

// SetSize sets the size of the HTTP model.
func (m *Model) SetSize(w, h int) {
	m.Width, m.Height = w, h
	listWidth := w / 4
	reqWidth := w / 2
	respWidth := w - listWidth - reqWidth - 6

	m.Collections.SetSize(listWidth, h/2-2)
	m.History.SetSize(listWidth, h/2-2)

	m.URL.Width = reqWidth - 4
	m.Headers.SetWidth(reqWidth - 4)
	m.Body.SetWidth(reqWidth - 4)

	m.Response.Width = respWidth
	m.Response.Height = h - 6
}

func (m *Model) updateRequestInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle up/down focus change
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, key.NewBinding(key.WithKeys("up", "shift+tab"))) {
		m.FocusedInput--
		if m.FocusedInput < 0 {
			m.FocusedInput = 3
		}
		m.focus()
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, key.NewBinding(key.WithKeys("down", "tab"))) {
		m.FocusedInput = (m.FocusedInput + 1) % 4
		m.focus()
		return nil
	}

	// Handle input for the focused element
	switch m.FocusedInput {
	case 0: // Method selector
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "l", "right":
				m.SelectedMethod = (m.SelectedMethod + 1) % len(m.Methods)
			case "h", "left":
				m.SelectedMethod--
				if m.SelectedMethod < 0 {
					m.SelectedMethod = len(m.Methods) - 1
				}
			}
		}
	case 1:
		m.URL, cmd = m.URL.Update(msg)
		cmds = append(cmds, cmd)
	case 2:
		m.Headers, cmd = m.Headers.Update(msg)
		cmds = append(cmds, cmd)
	case 3:
		m.Body, cmd = m.Body.Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) focus() {
	m.URL.Blur()
	m.Headers.Blur()
	m.Body.Blur()

	switch m.FocusedInput {
	case 0:
		// No text input to focus, the view will highlight it based on state.
	case 1:
		m.URL.Focus()
	case 2:
		m.Headers.Focus()
	case 3:
		m.Body.Focus()
	}
}

func (m *Model) renderInput(title string, input textinput.Model, index int) string {
	style := styles.BlurredInputStyle
	if m.FocusedPane == 1 && m.FocusedInput == index {
		style = styles.FocusedInputStyle
	}
	return fmt.Sprintf("%s\n%s\n", style.Render(title), input.View())
}

func (m *Model) renderTextarea(title string, ta textarea.Model, index int) string {
	style := styles.BlurredInputStyle
	if m.FocusedPane == 1 && m.FocusedInput == index {
		style = styles.FocusedInputStyle
	}
	return fmt.Sprintf("%s\n%s\n", style.Render(title), ta.View())
}

func (m *Model) renderMethodSelector() string {
	var renderedMethods []string
	for i, method := range m.Methods {
		style := styles.InactiveTabStyle
		if m.FocusedPane == 1 && m.FocusedInput == 0 && i == m.SelectedMethod {
			style = styles.ActiveTabStyle
		}
		renderedMethods = append(renderedMethods, style.Render(method))
	}

	titleStyle := styles.BlurredInputStyle
	if m.FocusedPane == 1 && m.FocusedInput == 0 {
		titleStyle = styles.FocusedInputStyle
	}

	selector := lipgloss.JoinHorizontal(lipgloss.Top, renderedMethods...)
	return fmt.Sprintf("%s\n%s\n", titleStyle.Render("Method (h/l or left/right to change)"), selector)
}

func (m *Model) loadRequest(item RequestItem) {
	m.URL.SetValue(item.URL)
	m.Headers.SetValue(item.Headers)
	m.Body.SetValue(item.Body)

	// Find the index of the method and set it
	m.SelectedMethod = 0 // default to GET
	for i, method := range m.Methods {
		if method == item.Method {
			m.SelectedMethod = i
			break
		}
	}
}

func (m *Model) updateResponseView() {
	switch m.ResponseViewTab {
	case 0: // Pretty
		m.Response.SetContent(utils.PrettyPrintJSON(m.ResponseBody))
	case 1: // Raw
		m.Response.SetContent(m.ResponseBody)
	case 2: // Headers
		m.Response.SetContent(m.ResponseHeaders)
	}
	m.Response.GotoTop()
}

func (m Model) sendRequest() tea.Cmd {
	return func() tea.Msg {
		url := m.substituteEnv(m.URL.Value())
		headers := m.substituteEnv(m.Headers.Value())
		body := m.substituteEnv(m.Body.Value())

		args := []string{"-i", "-s", "-S", "-L"}
		args = append(args, "-X", m.Methods[m.SelectedMethod])
		for _, h := range strings.Split(headers, "\n") {
			if h != "" {
				args = append(args, "-H", h)
			}
		}
		if body != "" {
			args = append(args, "-d", body)
		}
		args = append(args, url)

		cmd := exec.Command("curl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return HTTPResponseMsg{Err: fmt.Errorf("curl failed: %w\nOutput: %s", err, string(output))}
		}

		respStr := string(output)
		parts := strings.SplitN(respStr, "\r\n\r\n", 2)
		if len(parts) != 2 {
			if strings.Contains(respStr, "HTTP/1.1 100 Continue") {
				respStr = strings.SplitN(respStr, "\r\n\r\n", 2)[1]
			}
			lastHeaderIndex := strings.LastIndex(respStr, "HTTP/")
			if lastHeaderIndex > 0 {
				respStr = respStr[lastHeaderIndex:]
			}
			parts = strings.SplitN(respStr, "\r\n\r\n", 2)
			if len(parts) != 2 {
				return HTTPResponseMsg{Err: fmt.Errorf("failed to parse HTTP response: %s", respStr)}
			}
		}

		var statusCode int
		fmt.Sscanf(parts[0], "HTTP/1.1 %d", &statusCode)
		fmt.Sscanf(parts[0], "HTTP/2 %d", &statusCode)
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		return HTTPResponseMsg{Headers: parts[0], Body: parts[1], Code: statusCode}
	}
}

func (m Model) substituteEnv(input string) string {
	re := regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)
	return re.ReplaceAllStringFunc(input, func(s string) string {
		key := re.FindStringSubmatch(s)[1]
		if val, ok := m.Environment[key]; ok {
			return val
		}
		return s // Return original if not found
	})
}
