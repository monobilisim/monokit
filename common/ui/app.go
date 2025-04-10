package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// SingleBoxModel represents a simple non-scrollable box UI model
type SingleBoxModel struct {
	Title      string
	Content    string
	ready      bool
	termWidth  int
	termHeight int
}

// NewSingleBoxModel creates a new SingleBoxModel
func NewSingleBoxModel(title string) SingleBoxModel {
	return SingleBoxModel{
		Title:   title,
		Content: "",
	}
}

// Init initializes the SingleBoxModel
func (m SingleBoxModel) Init() tea.Cmd {
	return nil
}

// Update handles user input and updates the SingleBoxModel
func (m SingleBoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.ready = true
	}

	return m, nil
}

// View renders the SingleBoxModel
func (m SingleBoxModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Main box style with rounded borders - reuse existing BoxStyle
	boxStyle := BoxStyle.Copy().
		Width(m.termWidth - 4)
	
	// Format content with appropriate spacing
	formattedContent := m.Title + "\n\n" + m.Content
		
	// Return the boxed content
	return boxStyle.Render(formattedContent)
}

// SetContent sets the content of the model
func (m *SingleBoxModel) SetContent(content string) {
	m.Content = content
}

// RunApp initializes and runs a bubbletea application with the given model
func RunApp(initialModel tea.Model) error {
	p := tea.NewProgram(initialModel)
	_, err := p.Run()
	return err
}
