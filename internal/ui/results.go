package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ResultsModel is a static bubbletea model for displaying a titled list of results.
// It renders once and exits — no keyboard interaction.
type ResultsModel struct {
	Title  string
	Items  []string // pre-formatted strings (typically FormatPath output)
	Footer string
}

func (m ResultsModel) Init() tea.Cmd {
	return tea.Quit
}

func (m ResultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

// View builds the complete rendered output:
// header → divider → items → footer
func (m ResultsModel) View() string {
	var sb strings.Builder

	sb.WriteString(StyleHeader.Render(m.Title))
	sb.WriteString("\n")
	sb.WriteString(StyleMuted.Render(strings.Repeat("─", 40)))
	sb.WriteString("\n")

	for _, item := range m.Items {
		sb.WriteString(item)
		sb.WriteString("\n")
	}

	if m.Footer != "" {
		sb.WriteString(StyleMuted.Render(m.Footer))
		sb.WriteString("\n")
	}

	return sb.String()
}
