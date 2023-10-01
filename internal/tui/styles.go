package tui

import "github.com/charmbracelet/lipgloss"

var (
	appStyle = lipgloss.NewStyle().
			PaddingTop(1).
			PaddingRight(2).
			PaddingBottom(1).
			PaddingLeft(2)

	podStyle = lipgloss.NewStyle().PaddingLeft(2).Render

	// https://coolors.co/palette/264653-2a9d8f-e9c46a-f4a261-e76f51
	//
	logStyle = lipgloss.NewStyle().PaddingLeft(1).Border(lipgloss.NormalBorder(), false, false, false, true) /*lipgloss.Border{
		TopLeft:    "| ",
		BottomLeft: "| ",
		Left:       "| ",
	})*/

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).
			MarginTop(1).
			MarginBottom(1).
			Render

	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51"))

	activeSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9C46A"))
	checkMark          = lipgloss.NewStyle().Foreground(lipgloss.Color("#2a9d8f")).SetString("✓")
	// TODO: Better X mark?
	xMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51")).SetString("x")
)
