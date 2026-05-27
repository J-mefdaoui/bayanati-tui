package main

import (
	"github.com/charmbracelet/lipgloss"
)

// Color scheme
var (
	greenDeep   = lipgloss.Color("#0f3622")
	greenMid    = lipgloss.Color("#1a5a3a")
	greenAccent = lipgloss.Color("#4ADE80")
	red         = lipgloss.Color("#ef4444")
	mutedColor  = lipgloss.Color("#6b8f71")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(greenAccent).
			Background(greenDeep)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenMid).
			Padding(1, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(greenAccent).
			Bold(true)

	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenAccent).
			Padding(1, 1)

	buttonStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Margin(0, 1).
			Background(greenMid).
			Foreground(lipgloss.Color("#e8f5e9"))

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	snackbarStyle = lipgloss.NewStyle().
			Background(greenDeep).
			Foreground(greenAccent).
			Padding(0, 2).
			Margin(1, 0)
)
