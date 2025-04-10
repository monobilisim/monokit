package ui

import (
	"fmt"
	"strings"
)

// StatsPanel represents a box containing a collection of key-value statistics
type StatsPanel struct {
	Title    string
	KeyVals  map[string]string
	MinWidth int
}

// NewStatsPanel creates a new StatsPanel with the given title
func NewStatsPanel(title string) *StatsPanel {
	return &StatsPanel{
		Title:    title,
		KeyVals:  make(map[string]string),
		MinWidth: 40,
	}
}

// AddStat adds a statistic to the panel
func (p *StatsPanel) AddStat(key, value string) {
	p.KeyVals[key] = value
}

// Render renders the panel as a styled string
func (p *StatsPanel) Render() string {
	var sb strings.Builder

	// Render title
	sb.WriteString(RenderSection(p.Title))
	sb.WriteString("\n")

	// Render stats
	var keyValPairs []string
	for key, val := range p.KeyVals {
		keyValPairs = append(keyValPairs, FormatKeyValue(key, val))
	}

	content := strings.Join(keyValPairs, "\n")
	sb.WriteString(RenderBox(content))

	return sb.String()
}

// StatusPanel is for displaying the status of a service
type StatusPanel struct {
	Title       string
	Status      string
	IsActive    bool
	Description string
	Details     map[string]string
}

// NewStatusPanel creates a new StatusPanel
func NewStatusPanel(title string) *StatusPanel {
	return &StatusPanel{
		Title:   title,
		Details: make(map[string]string),
	}
}

// SetStatus sets the status of the panel
func (p *StatusPanel) SetStatus(status string, isActive bool) {
	p.Status = status
	p.IsActive = isActive
}

// SetDescription sets the description text
func (p *StatusPanel) SetDescription(desc string) {
	p.Description = desc
}

// AddDetail adds a detail to the panel
func (p *StatusPanel) AddDetail(key, value string) {
	p.Details[key] = value
}

// Render renders the panel as a styled string
func (p *StatusPanel) Render() string {
	var sb strings.Builder

	// Title and status
	titleStatus := fmt.Sprintf("%s: %s", 
		p.Title, 
		FormatStatusText(p.Status, p.IsActive))
	
	titleStyle := TitleStyle.Copy().
		PaddingRight(1)
	
	sb.WriteString(titleStyle.Render(titleStatus))
	sb.WriteString("\n")

	// Description if present
	if p.Description != "" {
		sb.WriteString(p.Description)
		sb.WriteString("\n\n")
	}

	// Details
	if len(p.Details) > 0 {
		var details []string
		for key, val := range p.Details {
			details = append(details, FormatKeyValue(key, val))
		}
		content := strings.Join(details, "\n")
		sb.WriteString(content)
	}

	return BoxStyle.Render(sb.String())
}

// TablePanel represents a simple table
type TablePanel struct {
	Title    string
	Headers  []string
	Rows     [][]string
	MinWidth int
}

// NewTablePanel creates a new TablePanel
func NewTablePanel(title string, headers []string) *TablePanel {
	return &TablePanel{
		Title:    title,
		Headers:  headers,
		Rows:     make([][]string, 0),
		MinWidth: 60,
	}
}

// AddRow adds a row to the table
func (p *TablePanel) AddRow(row []string) {
	p.Rows = append(p.Rows, row)
}

// Render renders the table as a styled string
func (p *TablePanel) Render() string {
	var sb strings.Builder

	// Render title
	sb.WriteString(RenderSection(p.Title))
	sb.WriteString("\n")

	// Calculate column widths
	colWidths := make([]int, len(p.Headers))
	for i, header := range p.Headers {
		colWidths[i] = len(header)
	}

	for _, row := range p.Rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Render headers
	var headerParts []string
	for i, header := range p.Headers {
		style := KeyStyle.Copy().Width(colWidths[i])
		headerParts = append(headerParts, style.Render(header))
	}
	sb.WriteString(strings.Join(headerParts, " | "))
	sb.WriteString("\n")

	// Separator
	var separator []string
	for _, width := range colWidths {
		separator = append(separator, strings.Repeat("─", width))
	}
	sb.WriteString(strings.Join(separator, "─┼─"))
	sb.WriteString("\n")

	// Render rows
	for _, row := range p.Rows {
		var rowParts []string
		for i, cell := range row {
			if i < len(colWidths) {
				style := ValueStyle.Copy().Width(colWidths[i])
				rowParts = append(rowParts, style.Render(cell))
			}
		}
		sb.WriteString(strings.Join(rowParts, " | "))
		sb.WriteString("\n")
	}

	return RenderBox(sb.String())
}
