package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list             list.Model
	spinner          spinner.Model
	textInput        textinput.Model
	loading          bool
	err              error
	inventories      []InventoryResponse
	currentScreen    string
	menus            map[string][]list.Item
	parentScreen     string
	userForm         []textinput.Model
	formFields       []string
	activeInput      int
	selectedHost     *Host
	hosts            []Host
	showHostDetails  bool
	loginForm        []textinput.Model
	loginFields      []string
	isLoggedIn       bool
	loggedInUser     string
	loggedInUserRole string
	deleteMode       bool
}

func initialModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "comma-separated list of inventories"
	ti.Focus()

	// Get terminal dimensions
	w, h := 80, 20 // default values

	// Initialize user form inputs
	fields := []string{"username", "password", "email", "role", "groups", "inventories"}
	userForm := make([]textinput.Model, len(fields))
	for i := range fields {
		ti := textinput.New()
		ti.Placeholder = fields[i]
		if fields[i] == "password" {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		userForm[i] = ti
	}

	// Initialize login form
	loginFields := []string{"username", "password"}
	loginForm := make([]textinput.Model, len(loginFields))
	for i := range loginFields {
		ti := textinput.New()
		ti.Placeholder = loginFields[i]
		if loginFields[i] == "password" {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		loginForm[i] = ti
	}
	loginForm[0].Focus()

	// Initialize menus
	menus := make(map[string][]list.Item)

	// Main menu
	menus["main"] = []list.Item{
		item{title: "Inventories", desc: "Manage inventories"},
		item{title: "Hosts", desc: "Manage hosts"},
		item{title: "Users", desc: "Manage users"},
		item{title: "Change Password", desc: "Update your password"},
	}

	// Inventory menu
	menus["inventories"] = []list.Item{
		item{title: "List Inventories", desc: "Show all inventories"},
		item{title: "Create Inventory", desc: "Create a new inventory"},
		item{title: "Delete Inventory", desc: "Delete an inventory"},
	}

	// Hosts menu
	menus["hosts"] = []list.Item{
		item{title: "List Hosts", desc: "Show all hosts"},
		item{title: "Delete Host", desc: "Delete a host"},
	}

	// Users menu
	menus["users"] = []list.Item{
		item{title: "List Users", desc: "Show all users"},
		item{title: "Create User", desc: "Create a new user"},
		item{title: "Delete User", desc: "Delete a user"},
	}

	// Set initial list items to main menu
	list := list.New(menus["main"], list.NewDefaultDelegate(), w, h)
	list.Title = "Monokit Client"

	return model{
		spinner:       sp,
		textInput:     ti,
		currentScreen: "main",
		list:          list,
		menus:         menus, // Add menus to model
		userForm:      userForm,
		formFields:    fields,
		activeInput:   0,
		loginForm:     loginForm,
		loginFields:   loginFields,
		isLoggedIn:    false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if !m.isLoggedIn {
				m.loading = true
				return m, m.attemptLogin()
			}
			if m.currentScreen == "create-user" {
				m.loading = true
				return m, m.createUser()
			}
			if m.currentScreen == "create" {
				m.loading = true
				name := strings.TrimSpace(m.textInput.Value())
				if name != "" {
					// Create inventory request
					data := map[string]string{"name": name}
					jsonData, err := json.Marshal(data)
					if err != nil {
						return m, nil
					}
					return m, func() tea.Msg {
						resp, err := SendGenericRequest("POST", "/api/v1/inventory", jsonData)
						if err != nil {
							return errorMsg(err)
						}
						defer resp.Body.Close()

						if resp.StatusCode != http.StatusCreated {
							var errorResp map[string]string
							json.NewDecoder(resp.Body).Decode(&errorResp)
							return errorMsg(fmt.Errorf("%s", errorResp["error"]))
						}

						// After successful creation, return to inventory list
						m.currentScreen = "inventories"
						return m.fetchInventories()
					}
				}
			}
			if m.currentScreen == "change-password" {
				m.loading = true
				return m, m.changePassword()
			}
			if i, ok := m.list.SelectedItem().(item); ok {
				switch m.currentScreen {
				case "main":
					switch i.title {
					case "Inventories":
						m.parentScreen = "main"
						m.currentScreen = "inventories"
						m.list.SetItems(m.menus["inventories"])
						m.list.Title = "Inventory Management"
					case "Hosts":
						m.parentScreen = "main"
						m.currentScreen = "hosts"
						m.list.SetItems(m.menus["hosts"])
						m.list.Title = "Host Management"
						m.showHostDetails = false
						m.selectedHost = nil
					case "Users":
						m.parentScreen = "main"
						m.currentScreen = "users"
						m.list.SetItems(m.menus["users"])
						m.list.Title = "User Management"
					case "Change Password":
						m.currentScreen = "change-password"
						m.textInput.SetValue("")
						m.textInput.Focus()
						m.textInput.Placeholder = "Enter new password"
					}
					return m, nil
				case "hosts":
					if i.title == "List Hosts" {
						m.loading = true
						m.deleteMode = false
						return m, m.fetchHosts
					}
					if i.title == "Delete Host" {
						m.loading = true
						m.deleteMode = true
						m.list.Title = "Hosts"
						return m, m.fetchHosts
					}
					if m.deleteMode && i.title != "List Hosts" && i.title != "Delete Host" {
						m.loading = true
						return m, m.deleteHost(i.title)
					}
					if !m.deleteMode {
						if m.showHostDetails {
							m.showHostDetails = false
						} else {
							for _, host := range m.hosts {
								if host.Name == i.title {
									m.selectedHost = &host
									m.showHostDetails = true
									break
								}
							}
						}
					}
					return m, nil
				case "inventories":
					if i.title == "Delete Inventory" {
						m.loading = true
						m.deleteMode = true
						m.list.Title = "Inventories"
						return m, m.fetchInventories
					}
					if i.title == "List Inventories" {
						m.loading = true
						m.deleteMode = false
						return m, m.fetchInventories
					}
					// Only attempt deletion if we're in delete mode and not clicking a menu item
					if m.deleteMode && i.title != "List Inventories" &&
						i.title != "Create Inventory" &&
						i.title != "Delete Inventory" {
						m.loading = true
						return m, m.deleteInventory(i.title)
					}
					return m.handleSubMenu(i.title)
				case "users":
					if i.title == "Delete User" {
						m.loading = true
						m.deleteMode = true
						m.list.Title = "Users"
						return m, m.fetchUsers()
					}
					if i.title == "List Users" {
						m.loading = true
						m.deleteMode = false
						return m, m.fetchUsers()
					}
					// Only attempt deletion if we're in delete mode and not clicking a menu item
					if m.deleteMode && i.title != "List Users" &&
						i.title != "Create User" &&
						i.title != "Delete User" {
						m.loading = true
						return m, m.deleteUser(i.title)
					}
					return m.handleSubMenu(i.title)
				}
			}
		case "esc":
			if m.currentScreen != "main" {
				m.currentScreen = m.parentScreen
				m.list.SetItems(m.menus[m.currentScreen])
				m.deleteMode = false // Reset delete mode when going back
				if m.currentScreen == "main" {
					m.list.Title = "Monokit Client"
				}
				// Reset host details when leaving hosts view
				if m.currentScreen != "hosts" {
					m.showHostDetails = false
					m.selectedHost = nil
				}
				return m, nil
			}
		case "tab":
			if !m.isLoggedIn {
				m.activeInput = (m.activeInput + 1) % len(m.loginForm)
				for i := range m.loginForm {
					if i == m.activeInput {
						m.loginForm[i].Focus()
					} else {
						m.loginForm[i].Blur()
					}
				}
				return m, nil
			} else if m.currentScreen == "create-user" {
				m.activeInput = (m.activeInput + 1) % len(m.userForm)
				for i := range m.userForm {
					if i == m.activeInput {
						m.userForm[i].Focus()
					} else {
						m.userForm[i].Blur()
					}
				}
				return m, nil
			}
		case "right", "l":
			if i, ok := m.list.SelectedItem().(item); ok {
				if m.currentScreen == "hosts" && !m.showHostDetails {
					// Show host details when pressing right in hosts view
					for _, host := range m.hosts {
						if host.Name == i.title {
							m.selectedHost = &host
							m.showHostDetails = true
							return m, nil
						}
					}
				}
				// Otherwise handle submenu navigation
				return m.handleSubMenu(i.title)
			}
		case "left", "h":
			if m.showHostDetails {
				// Hide host details when pressing left in details view
				m.showHostDetails = false
				return m, nil
			}
			if m.currentScreen != "main" {
				m.currentScreen = m.parentScreen
				m.list.SetItems(m.menus[m.currentScreen])
				m.deleteMode = false // Reset delete mode when going back
				if m.currentScreen == "main" {
					m.list.Title = "Monokit Client"
				}
				// Reset host details when leaving hosts view
				if m.currentScreen != "hosts" {
					m.showHostDetails = false
					m.selectedHost = nil
				}
			}
		}
	case inventoriesMsg:
		m.loading = false
		m.inventories = msg
		items := make([]list.Item, len(msg))
		for i, inv := range msg {
			items[i] = item{
				title: inv.Name,
				desc:  fmt.Sprintf("Hosts: %d", inv.Hosts),
			}
		}

		w, h := 80, 20 // Use same dimensions as initial model
		m.list = list.New(items, list.NewDefaultDelegate(), w, h)
		m.list.Title = "Inventories"
		return m, nil
	case errorMsg:
		m.loading = false
		m.err = msg
		// If the error is about authentication, reset login state
		if strings.Contains(msg.Error(), "unauthorized") ||
			strings.Contains(msg.Error(), "forbidden") ||
			strings.Contains(msg.Error(), "login failed") {
			m.isLoggedIn = false
			// Clear password field
			m.loginForm[1].SetValue("")
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case hostsMsg:
		m.loading = false
		m.hosts = msg
		items := make([]list.Item, len(msg))
		for i, host := range msg {
			status := host.Status
			if host.UpForDeletion {
				status = "Scheduled for deletion"
			}
			items[i] = item{
				title: host.Name,
				desc:  fmt.Sprintf("Inventory: %s, Status: %s", host.Inventory, status),
			}
		}

		w, h := 80, 20
		m.list = list.New(items, list.NewDefaultDelegate(), w, h)
		m.list.Title = "Hosts"
		return m, nil
	case usersMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, user := range msg {
			items[i] = item{
				title: user.Username,
				desc:  fmt.Sprintf("Role: %s, Email: %s", user.Role, user.Email),
			}
		}

		w, h := 80, 20
		m.list = list.New(items, list.NewDefaultDelegate(), w, h)
		m.list.Title = "Users"
		return m, nil
	case deleteMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Show success message
		m.err = nil
		// Reset deleteMode after successful deletion
		m.deleteMode = false
		// Return to main menu after password change
		if m.currentScreen == "change-password" {
			m.currentScreen = "main"
			m.list.SetItems(m.menus["main"])
			m.list.Title = "Monokit Client"
			return m, nil
		}
		// After successful deletion, refresh the list
		switch m.currentScreen {
		case "inventories":
			return m, m.fetchInventories
		case "hosts":
			return m, m.fetchHosts
		case "users":
			return m, m.fetchUsers()
		}
		return m, nil
	case loginSuccessMsg:
		m.loading = false
		m.isLoggedIn = true
		m.loggedInUser = msg.username
		m.loggedInUserRole = msg.role
		return m, nil
	}

	// Handle login form updates
	if !m.isLoggedIn {
		var cmd tea.Cmd
		for i := range m.loginForm {
			m.loginForm[i], cmd = m.loginForm[i].Update(msg)
		}
		return m, cmd
	}

	var cmd tea.Cmd
	switch m.currentScreen {
	case "main", "inventories", "hosts", "users":
		m.list, cmd = m.list.Update(msg)
		// Update selected host when list selection changes
		if m.currentScreen == "hosts" {
			if i, ok := m.list.SelectedItem().(item); ok {
				for _, host := range m.hosts {
					if host.Name == i.title {
						m.selectedHost = &host
						break
					}
				}
			}
		}
	case "create":
		m.textInput, cmd = m.textInput.Update(msg)
	case "create-user":
		var cmd tea.Cmd
		for i := range m.userForm {
			m.userForm[i], cmd = m.userForm[i].Update(msg)
		}
		return m, cmd
	case "change-password":
		m.textInput, cmd = m.textInput.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress any key to continue", m.err)
	}

	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading...", m.spinner.View())
	}

	var view string
	if !m.isLoggedIn {
		var b strings.Builder
		b.WriteString("\n\n   Login to Monokit\n\n")
		for i, input := range m.loginForm {
			b.WriteString(fmt.Sprintf("   %s: %s\n", m.loginFields[i], input.View()))
		}
		b.WriteString("\n   (tab to switch fields, enter to login)\n")
		return b.String()
	} else if m.currentScreen == "create" {
		view = fmt.Sprintf(
			"\n\n   Create New Inventory\n\n   %s\n\n   (press enter to submit, esc to go back)\n",
			m.textInput.View(),
		)
	} else if m.currentScreen == "change-password" {
		view = fmt.Sprintf(
			"\n\n   Change Password\n\n   %s\n\n   (press enter to submit, esc to go back)\n",
			m.textInput.View(),
		)
	} else if m.currentScreen == "create-user" {
		var b strings.Builder
		b.WriteString("\n\n   Create New User\n\n")
		for i, input := range m.userForm {
			b.WriteString(fmt.Sprintf("   %s: %s\n", m.formFields[i], input.View()))
		}
		b.WriteString("\n   (tab to switch fields, enter to submit, esc to go back)\n")
		view = b.String()
	} else {
		baseView := m.list.View()
		if m.deleteMode {
			baseView += "\n\nPress enter to confirm deletion"
		}
		if m.showHostDetails && m.selectedHost != nil {
			status := m.selectedHost.Status
			if m.selectedHost.UpForDeletion {
				status = "Scheduled for deletion"
			}
			view = docStyle.Render(fmt.Sprintf(`
   Host Details:
   Name: %s
   Inventory: %s
   Status: %s
   CPU Cores: %d
   RAM: %s
   OS: %s
   IP Address: %s
   Monokit Version: %s
   Groups: %s
   Disabled Components: %s
   Installed Components: %s
   Created: %s
   Last Updated: %s

   Press enter to return to list
`,
				m.selectedHost.Name,
				m.selectedHost.Inventory,
				status,
				m.selectedHost.CpuCores,
				m.selectedHost.Ram,
				m.selectedHost.Os,
				m.selectedHost.IpAddress,
				m.selectedHost.MonokitVersion,
				m.selectedHost.Groups,
				m.selectedHost.DisabledComponents,
				m.selectedHost.InstalledComponents,
				m.selectedHost.CreatedAt.Format("2006-01-02 15:04:05"),
				m.selectedHost.UpdatedAt.Format("2006-01-02 15:04:05")))
		} else {
			view = docStyle.Render(baseView)
		}
	}

	// Add user info at the bottom for all views when logged in
	if m.isLoggedIn {
		view += fmt.Sprintf("\n\n   Logged in as %s (%s)", m.loggedInUser, m.loggedInUserRole)
	}

	return view
}

type inventoriesMsg []InventoryResponse
type errorMsg error
type hostsMsg []Host
type usersMsg []UserResponse
type deleteMsg struct {
	message string
	err     error
}
type successMsg string

func (m model) fetchInventories() tea.Msg {
	resp, err := SendGenericRequest("GET", "/api/v1/inventory", nil)
	if err != nil {
		return errorMsg(err)
	}
	defer resp.Body.Close()

	var inventories []InventoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&inventories); err != nil {
		return errorMsg(err)
	}

	return inventoriesMsg(inventories)
}

func (m model) handleSubMenu(title string) (tea.Model, tea.Cmd) {
	switch title {
	case "Inventories", "Hosts", "Users":
		// Main menu items
		m.parentScreen = "main"
		m.currentScreen = strings.ToLower(title)
		m.list.SetItems(m.menus[m.currentScreen])
		m.list.Title = title + " Management"
		return m, nil

	case "Create Inventory":
		m.currentScreen = "create"
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, nil

	case "List Inventories":
		m.loading = true
		m.deleteMode = false
		return m, m.fetchInventories

	case "Delete Inventory":
		m.loading = true
		m.deleteMode = true
		m.list.Title = "Inventories"
		return m, m.fetchInventories

	case "List Hosts":
		m.loading = true
		m.deleteMode = false
		return m, m.fetchHosts
	case "Delete Host":
		m.loading = true
		m.deleteMode = true
		m.list.Title = "Hosts"
		return m, m.fetchHosts

	case "List Users":
		m.loading = true
		m.deleteMode = false
		return m, m.fetchUsers()
	case "Create User":
		m.currentScreen = "create-user"
		return m, nil
	case "Delete User":
		m.loading = true
		m.deleteMode = true
		m.list.Title = "Users"
		return m, m.fetchUsers()
	case "Change Password":
		m.currentScreen = "change-password"
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.textInput.Placeholder = "Enter new password"
		return m, nil
	}
	return m, nil
}

func (m model) fetchHosts() tea.Msg {
	resp, err := SendGenericRequest("GET", "/api/v1/hosts", nil)
	if err != nil {
		return errorMsg(err)
	}
	defer resp.Body.Close()

	var apiHosts []APIHost
	if err := json.NewDecoder(resp.Body).Decode(&apiHosts); err != nil {
		return errorMsg(err)
	}

	// Convert API responses to Host objects
	hosts := make([]Host, len(apiHosts))
	for i, ah := range apiHosts {
		hosts[i] = Host{
			Name:                ah.Name,
			CpuCores:            ah.CpuCores,
			Ram:                 ah.Ram,
			MonokitVersion:      ah.MonokitVersion,
			Os:                  ah.Os,
			DisabledComponents:  ah.DisabledComponents,
			InstalledComponents: ah.InstalledComponents,
			IpAddress:           ah.IpAddress,
			Status:              ah.Status,
			UpdatedAt:           ah.UpdatedAt,
			CreatedAt:           ah.CreatedAt,
			WantsUpdateTo:       ah.WantsUpdateTo,
			Groups:              ah.Groups,
			UpForDeletion:       ah.UpForDeletion,
			Inventory:           ah.Inventory,
		}
	}

	return hostsMsg(hosts)
}

func (m model) fetchUsers() tea.Cmd {
	return func() tea.Msg {
		resp, err := SendGenericRequest("GET", "/api/v1/admin/users", nil)
		if err != nil {
			return errorMsg(err)
		}
		defer resp.Body.Close()

		var users []UserResponse
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			return errorMsg(err)
		}

		return usersMsg(users)
	}
}

func (m model) deleteInventory(name string) tea.Cmd {
	return func() tea.Msg {
		resp, err := SendGenericRequest("DELETE", "/api/v1/inventory/"+name, nil)
		if err != nil {
			return deleteMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			return deleteMsg{err: fmt.Errorf("%s", errorResp["error"])}
		}

		return deleteMsg{message: fmt.Sprintf("Inventory %s deleted successfully", name)}
	}
}

func (m model) deleteHost(hostname string) tea.Cmd {
	return func() tea.Msg {
		resp, err := SendGenericRequest("DELETE", "/api/v1/hosts/"+hostname, nil)
		if err != nil {
			return errorMsg(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			return deleteMsg{err: fmt.Errorf("%s", errorResp["error"])}
		}

		return m.fetchHosts()
	}
}

func (m model) deleteUser(username string) tea.Cmd {
	return func() tea.Msg {
		// Only try to delete if we have a valid username (not a menu item)
		if username == "List Users" || username == "Create User" || username == "Delete User" {
			return nil
		}

		resp, err := SendGenericRequest("DELETE", "/api/v1/admin/users/"+username, nil)
		if err != nil {
			return deleteMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorResp map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
				return deleteMsg{err: fmt.Errorf("failed to delete user")}
			}
			return deleteMsg{err: fmt.Errorf("%s", errorResp["error"])}
		}

		// Return a command instead of directly calling fetchUsers
		return m.fetchUsers()() // Add () to execute the command
	}
}

func (m model) createUser() tea.Cmd {
	return func() tea.Msg {
		data := map[string]string{
			"username":    m.userForm[0].Value(),
			"password":    m.userForm[1].Value(),
			"email":       m.userForm[2].Value(),
			"role":        m.userForm[3].Value(),
			"groups":      m.userForm[4].Value(),
			"inventories": m.userForm[5].Value(),
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			return errorMsg(err)
		}

		resp, err := SendGenericRequest("POST", "/api/v1/admin/users", jsonData)
		if err != nil {
			return errorMsg(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			return errorMsg(fmt.Errorf("%s", errorResp["error"]))
		}

		return m.fetchUsers()
	}
}

func (m model) attemptLogin() tea.Cmd {
	return func() tea.Msg {
		username := m.loginForm[0].Value()
		password := m.loginForm[1].Value()

		if err := Login(username, password); err != nil {
			return errorMsg(err)
		}

		// Get user info after login
		resp, err := SendGenericRequest("GET", "/api/v1/auth/me", nil)
		if err != nil {
			return errorMsg(err)
		}
		defer resp.Body.Close()

		var response struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return errorMsg(err)
		}

		return loginSuccessMsg{username: response.Username, role: response.Role}
	}
}

// Update loginSuccessMsg type
type loginSuccessMsg struct {
	username string
	role     string
}

func (m model) changePassword() tea.Cmd {
	return func() tea.Msg {
		if m.textInput.Value() == "" {
			return errorMsg(fmt.Errorf("password cannot be empty"))
		}

		data := map[string]string{
			"password": m.textInput.Value(),
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			return errorMsg(err)
		}

		resp, err := SendGenericRequest("PUT", "/api/v1/auth/me/update", jsonData)
		if err != nil {
			return errorMsg(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			return errorMsg(fmt.Errorf("%s", errorResp["error"]))
		}

		// Return to main menu with success message
		return deleteMsg{message: "Password updated successfully"}
	}
}

func StartTUI(cmd *cobra.Command, args []string) {
	ClientInit()

	// Test token validity and get user info
	resp, err := SendGenericRequest("GET", "/api/v1/auth/me", nil)
	isValidToken := true
	var username, role string
	if err != nil || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		isValidToken = false
	} else {
		var response struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err == nil {
			username = response.Username
			role = response.Role
		} else {
			isValidToken = false
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	mainMenuItems := []list.Item{
		item{title: "Inventories", desc: "Manage inventories"},
		item{title: "Hosts", desc: "Manage hosts"},
		item{title: "Users", desc: "Manage users"},
		item{title: "Change Password", desc: "Update your password"},
	}

	inventoryItems := []list.Item{
		item{title: "List Inventories", desc: "Show all inventories"},
		item{title: "Create Inventory", desc: "Create a new inventory"},
		item{title: "Delete Inventory", desc: "Delete an inventory"},
	}

	hostItems := []list.Item{
		item{title: "List Hosts", desc: "Show all hosts"},
		item{title: "Delete Host", desc: "Schedule a host for deletion"},
	}

	userItems := []list.Item{
		item{title: "List Users", desc: "Show all users"},
		item{title: "Create User", desc: "Create a new user"},
		item{title: "Delete User", desc: "Delete a user"},
	}

	m := initialModel()
	m.isLoggedIn = isValidToken
	if isValidToken {
		m.loggedInUser = username
		m.loggedInUserRole = role
	}
	m.list.SetItems(mainMenuItems)
	m.list.Title = "Monokit Client"
	m.menus = map[string][]list.Item{
		"main":        mainMenuItems,
		"inventories": inventoryItems,
		"hosts":       hostItems,
		"users":       userItems,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
