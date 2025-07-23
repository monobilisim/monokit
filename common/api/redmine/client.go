//go:build with_api

package redmine

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
)

// Client wraps the Redmine API client functionality
type Client struct {
	config models.RedmineConfig
	client *http.Client
}

// Issue represents a Redmine issue
type Issue struct {
	ID          int       `json:"id"`
	Project     Project   `json:"project"`
	Tracker     Tracker   `json:"tracker"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Author      User      `json:"author"`
	AssignedTo  *User     `json:"assigned_to,omitempty"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	StartDate   *string   `json:"start_date,omitempty"`
	DueDate     *string   `json:"due_date,omitempty"`
	DoneRatio   int       `json:"done_ratio"`
	CreatedOn   time.Time `json:"created_on"`
	UpdatedOn   time.Time `json:"updated_on"`
}

// Project represents a Redmine project
type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

// Tracker represents a Redmine tracker
type Tracker struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Status represents a Redmine issue status
type Status struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Priority represents a Redmine issue priority
type Priority struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// User represents a Redmine user
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IssuesResponse represents the response from the issues API
type IssuesResponse struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
}

// ProjectResponse represents the response from the project API
type ProjectResponse struct {
	Project Project `json:"project"`
}

// ProjectsResponse represents the response from the projects API
type ProjectsResponse struct {
	Projects   []Project `json:"projects"`
	TotalCount int       `json:"total_count"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// NewClient creates a new Redmine API client
func NewClient(config models.RedmineConfig) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.VerifySSL,
		},
	}

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &Client{
		config: config,
		client: client,
	}
}

// makeRequest makes an authenticated request to the Redmine API
func (c *Client) makeRequest(method, endpoint string) (*http.Response, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("Redmine integration is disabled")
	}

	if c.config.APIKey == "" {
		return nil, fmt.Errorf("Redmine API key is not configured")
	}

	fullURL := c.config.URL + endpoint
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header
	req.Header.Set("X-Redmine-API-Key", c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Monokit-API/1.0")

	log.Debug().
		Str("component", "redmine").
		Str("method", method).
		Str("url", fullURL).
		Msg("Making Redmine API request")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// GetProject retrieves a project by identifier
func (c *Client) GetProject(identifier string) (*Project, error) {
	endpoint := fmt.Sprintf("/projects/%s.json", url.QueryEscape(identifier))
	resp, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		return nil, fmt.Errorf("failed to decode project response: %w", err)
	}

	return &projectResp.Project, nil
}

// GetProjectIssues retrieves issues for a project
func (c *Client) GetProjectIssues(projectIdentifier string, limit, offset int) (*IssuesResponse, error) {
	params := url.Values{}
	params.Set("project_id", projectIdentifier)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))
	params.Set("status_id", "*") // Include all statuses

	endpoint := fmt.Sprintf("/issues.json?%s", params.Encode())
	resp, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issuesResp IssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&issuesResp); err != nil {
		return nil, fmt.Errorf("failed to decode issues response: %w", err)
	}

	return &issuesResp, nil
}

// GetIssue retrieves a specific issue by ID
func (c *Client) GetIssue(issueID int) (*Issue, error) {
	endpoint := fmt.Sprintf("/issues/%d.json", issueID)
	resp, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issueResp struct {
		Issue Issue `json:"issue"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issueResp); err != nil {
		return nil, fmt.Errorf("failed to decode issue response: %w", err)
	}

	return &issueResp.Issue, nil
}

// TestConnection tests the connection to Redmine API
func (c *Client) TestConnection() error {
	endpoint := "/projects.json?limit=1"
	resp, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Info().
		Str("component", "redmine").
		Msg("Successfully connected to Redmine API")

	return nil
}
