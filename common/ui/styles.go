// Package ui provides common UI components for terminal-based interfaces.
// It uses the charmbracelet/lipgloss library for styling terminal output.
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/monobilisim/monokit/common"
)

var (
	// Colors - reuse existing color definitions from common package
	StatusActiveColor   = common.SuccessColor
	StatusInactiveColor = common.ErrorColor
	HeaderColor         = common.PrimaryColor
	SectionTitleColor   = common.SecondaryColor
	InfoColor           = lipgloss.Color("#FFB946") // Orange/Yellow
	SuccessColor        = common.SuccessColor
	WarningColor        = common.WarningColor
	ErrorColor          = common.ErrorColor
	NormalTextColor     = common.NormalTextColor

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(HeaderColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			PaddingLeft(2)

	SectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(SectionTitleColor).
			PaddingTop(1).
			PaddingBottom(1)

	StatusActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(StatusActiveColor)

	StatusInactiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(StatusInactiveColor)

	InfoStyle = lipgloss.NewStyle().
			Foreground(InfoColor)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor)

	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(NormalTextColor)

	ValueStyle = lipgloss.NewStyle().
			Foreground(NormalTextColor)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(HeaderColor).
			Padding(1).
			MarginBottom(1)
)

// FormatKeyValue formats a key-value pair with consistent styling
func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s: %s", 
		KeyStyle.Render(key), 
		ValueStyle.Render(value))
}

// FormatStatusText returns a colored status text based on active state
func FormatStatusText(text string, active bool) string {
	if active {
		return StatusActiveStyle.Render(text)
	}
	return StatusInactiveStyle.Render(text)
}

// RenderTitle renders a title with the TitleStyle
func RenderTitle(title string) string {
	return TitleStyle.Render(title)
}

// RenderSection renders a section header with the SectionStyle
func RenderSection(title string) string {
	return SectionStyle.Render(title)
}

// RenderBox renders content within a box with the BoxStyle
func RenderBox(content string) string {
	return BoxStyle.Render(content)
}
