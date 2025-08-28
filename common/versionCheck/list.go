package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type AppVersion struct {
	Name       string
	OldVersion string
	NewVersion string
}

var notInstalled []string
var notUpdated []AppVersion
var versionErrors []error
var updated []AppVersion

func addToNotInstalled(name string) {
	notInstalled = append(notInstalled, name)
}

func addToNotUpdated(app AppVersion) {
	notUpdated = append(notUpdated, app)
}

func addToVersionErrors(err error) {
	versionErrors = append(versionErrors, err)
}

func addToUpdated(app AppVersion) {
	updated = append(updated, app)
}

func PrintList() {
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1)

	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#F25D94")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1)

	notInstalledStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#FFA500")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1)

	notUpdatedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#04B575")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1)

	updatedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#00A86B")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#626262"))

	errorItemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#F25D94"))

	updatedItemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#00A86B"))

	// Create border style for the entire output
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		PaddingLeft(1).
		PaddingRight(1).
		MarginTop(1).
		MarginBottom(1)

	var content strings.Builder

	// Header
	content.WriteString(titleStyle.Render("📋 Version Check Summary"))
	content.WriteString("\n\n")

	// Errors section
	if len(versionErrors) > 0 {
		content.WriteString(errorStyle.Render("❌ Errors"))
		content.WriteString("\n")
		for _, err := range versionErrors {
			content.WriteString(errorItemStyle.Render(fmt.Sprintf("• %s", err.Error())))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Not installed section
	if len(notInstalled) > 0 {
		content.WriteString(notInstalledStyle.Render("⚠️  Not Installed"))
		content.WriteString("\n")
		for _, name := range notInstalled {
			content.WriteString(itemStyle.Render(fmt.Sprintf("• %s", name)))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Not updated section
	if len(notUpdated) > 0 {
		content.WriteString(notUpdatedStyle.Render("✅ Not Updated"))
		content.WriteString("\n")
		for _, app := range notUpdated {
			content.WriteString(itemStyle.Render(fmt.Sprintf("• %s - %s", app.Name, app.OldVersion)))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Updated section
	if len(updated) > 0 {
		content.WriteString(updatedStyle.Render("🚀 Updated"))
		content.WriteString("\n")
		for _, app := range updated {
			content.WriteString(updatedItemStyle.Render(fmt.Sprintf("• %s: %s → %s", app.Name, app.OldVersion, app.NewVersion)))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// If nothing to show
	if len(versionErrors) == 0 && len(notInstalled) == 0 && len(notUpdated) == 0 && len(updated) == 0 {
		content.WriteString(itemStyle.Render("🎉 No version information to display"))
		content.WriteString("\n")
	}

	// Print the bordered content
	fmt.Println(borderStyle.Render(strings.TrimRight(content.String(), "\n")))
}
