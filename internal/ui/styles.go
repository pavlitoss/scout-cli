package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	StyleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	StyleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	StyleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	StyleGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	StyleBold   = lipgloss.NewStyle().Bold(true)

	StyleTag    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	StylePath   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	StyleError  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	StyleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	StyleHeader = lipgloss.NewStyle().Bold(true)
)

// Error prints msg to stderr in red.
func Error(msg string) {
	fmt.Fprintln(os.Stderr, StyleError.Render(msg))
}

// PrintHeader prints a bold title followed by a muted divider line.
func PrintHeader(title string) {
	fmt.Println(StyleHeader.Render(title))
	fmt.Println(StyleMuted.Render(strings.Repeat("─", 40)))
}

// FormatTag returns the tag name styled green and bold.
func FormatTag(name string) string {
	return StyleTag.Render(name)
}

// FormatPath returns the path styled cyan, with the home directory replaced by ~.
func FormatPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	return StylePath.Render(path)
}
