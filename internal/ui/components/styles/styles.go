package styles

import "github.com/charmbracelet/lipgloss"

// General styles
var (
	DocStyle  = lipgloss.NewStyle().Padding(0, 1)
	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// Tab styles
var (
	ActiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3E5C76")).
			Padding(0, 1)
	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8FA3B8")).
			Padding(0, 1)
)

// List styles
var (
	ListHeaderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("36")).
		MarginRight(2).
		Padding(0, 1)
)

// HTTP Panel styles
var (
	FocusedPaneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("69"))
	BlurredPaneStyle  = lipgloss.NewStyle().Border(lipgloss.HiddenBorder())
	FocusedInputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	BlurredInputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	SuccessStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))
	ErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	SpinnerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
)

// System Panel styles
var (
	BarStyle       = lipgloss.NewStyle().Background(lipgloss.Color("#575B7E")).Foreground(lipgloss.Color("#E5E5E5"))
	BarHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EAF2FF")).
			Background(lipgloss.Color("#1E3A5F")).
			Padding(0, 1)
)

// JSON Highlighting styles
var (
	JSONKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("153"))
	JSONStringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	JSONNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	JSONBoolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("202"))
	JSONNullStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)
