package opnsenseHealth

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3B8CC"))
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Underline(true)
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
)

func RenderOpnsenseHealthCLI(data *OpnsenseHealthData) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("SSL Certificate Information") + "\n\n")

	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Subject:"), valueStyle.Render(data.Subject)))
	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Issuer:"), valueStyle.Render(data.Issuer)))
	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Expiry Date:"), valueStyle.Render(data.ExpiryDate)))

	daysRemainingStr := fmt.Sprintf("%d days", data.DaysRemaining)
	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Days Remaining:"), valueStyle.Render(daysRemainingStr)))

	statusStyle := successStyle
	switch data.Status {
	case "Expiring Soon":
		statusStyle = warningStyle
	case "Expired", "Connection Failed", "No Certificate Found":
		statusStyle = errorStyle
	}

	b.WriteString(fmt.Sprintf("%s %s\n", textStyle.Render("Status:"), statusStyle.Render(data.Status)))

	return b.String()
}
