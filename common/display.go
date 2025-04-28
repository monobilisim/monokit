// Package common provides common utilities for terminal display and formatting
package common

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Default colors for display styles
	PrimaryColor    = lipgloss.Color("#7D56F4") // Purple
	SecondaryColor  = lipgloss.Color("#6B97F7") // Light Blue
	SuccessColor    = lipgloss.Color("#00FF00") // Bright Green
	WarningColor    = lipgloss.Color("#F5B041") // Yellow
	ErrorColor      = lipgloss.Color("#FF0000") // Bright Red
	NormalTextColor = lipgloss.Color("#FFFFFF") // White
)

// DisplayBox creates a nice looking box around content
func DisplayBox(title string, content string) string {
	// Create box style with rounded borders
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0).
		Width(80)

	// Style the title
	titleStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		PaddingLeft(2)

	// Format and return the output
	output := titleStyle.Render(title) + "\n\n" + content

	return boxStyle.Render(output)
}

// SectionTitle formats a section title
func SectionTitle(title string) string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		PaddingLeft(2)

	return sectionStyle.Render(title)
}

// ListItem formats a list item with bullet point and proper spacing
func ListItem(label string, value string, isSuccess bool) string {
	statusStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	if !isSuccess {
		statusStyle = lipgloss.NewStyle().Foreground(ErrorColor)
	}

	contentStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		PaddingLeft(8)

	itemStyle := lipgloss.NewStyle().
		Foreground(NormalTextColor)

	line := fmt.Sprintf("•  %-12s  %s",
		label,
		statusStyle.Render(value))

	return contentStyle.Render(itemStyle.Render(line))
}

// StatusListItem formats a status list item with bullet point, status, and values
func StatusListItem(label string, statusPrefix string, limits string, current string, isSuccess bool) string {
	statusStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	statusText := "less than"

	if !isSuccess {
		statusStyle = lipgloss.NewStyle().Foreground(ErrorColor)
		statusText = "more than"
	}

	if statusPrefix != "" {
		statusText = statusPrefix
	}

	contentStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		PaddingLeft(8)

	itemStyle := lipgloss.NewStyle().
		Foreground(NormalTextColor)

	line := fmt.Sprintf("•  %-12s  %s %s (%s)",
		label,
		statusStyle.Render(statusText),
		limits,
		current)

	return contentStyle.Render(itemStyle.Render(line))
}

// SimpleStatusListItem formats a status list item with a simple "is" format.
// Example: "• ServiceName is Running"
func SimpleStatusListItem(label string, expectedState string, isSuccess bool) string {
	statusStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	if !isSuccess {
		statusStyle = lipgloss.NewStyle().Foreground(ErrorColor)
	}

	contentStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		PaddingLeft(8)

	itemStyle := lipgloss.NewStyle().
		Foreground(NormalTextColor)

	// Format: "• label                is ExpectedState" (Aligned)
	line := fmt.Sprintf("•  %-20s is %s", // Increased padding for label to 20 chars
		label,
		statusStyle.Render(expectedState)) // Render the expected state with color

	return contentStyle.Render(itemStyle.Render(line))
}

// NewTitleStyle returns a style for titles
func NewTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		PaddingLeft(2)
}
