//go:build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/redmine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedmineNewClient(t *testing.T) {
	config := models.RedmineConfig{
		Enabled:   true,
		URL:       "https://redmine.example.com",
		APIKey:    "test-api-key",
		Timeout:   60,
		VerifySSL: true,
	}

	client := redmine.NewClient(config)
	assert.NotNil(t, client)
}

func TestRedmineNewClient_WithDefaultTimeout(t *testing.T) {
	config := models.RedmineConfig{
		Enabled:   true,
		URL:       "https://redmine.example.com",
		APIKey:    "test-api-key",
		Timeout:   0, // Should use default
		VerifySSL: true,
	}

	client := redmine.NewClient(config)
	assert.NotNil(t, client)
}

func TestClient_GetProject_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/test-project.json", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Redmine-API-Key"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Monokit-API/1.0", r.Header.Get("User-Agent"))

		project := redmine.Project{
			ID:          1,
			Name:        "Test Project",
			Identifier:  "test-project",
			Description: "A test project",
			Status:      1,
			CreatedOn:   "2023-01-01T00:00:00Z",
		}

		response := redmine.ProjectResponse{
			Project: project,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "test-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	project, err := client.GetProject("test-project")

	require.NoError(t, err)
	assert.Equal(t, 1, project.ID)
	assert.Equal(t, "Test Project", project.Name)
	assert.Equal(t, "test-project", project.Identifier)
	assert.Equal(t, "A test project", project.Description)
}

func TestClient_GetProject_DisabledRedmine(t *testing.T) {
	config := models.RedmineConfig{
		Enabled: false,
		URL:     "https://redmine.example.com",
		APIKey:  "test-api-key",
	}

	client := redmine.NewClient(config)
	project, err := client.GetProject("test-project")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redmine integration is disabled")
	assert.Nil(t, project)
}

func TestClient_GetProject_NoAPIKey(t *testing.T) {
	config := models.RedmineConfig{
		Enabled: true,
		URL:     "https://redmine.example.com",
		APIKey:  "", // No API key
	}

	client := redmine.NewClient(config)
	project, err := client.GetProject("test-project")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redmine API key is not configured")
	assert.Nil(t, project)
}

func TestClient_GetProject_HTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Project not found"))
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "test-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	project, err := client.GetProject("nonexistent-project")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed with status 404")
	assert.Nil(t, project)
}

func TestClient_GetProjectIssues_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/issues.json", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-project", r.URL.Query().Get("project_id"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		assert.Equal(t, "*", r.URL.Query().Get("status_id"))

		issues := []redmine.Issue{
			{
				ID:        1,
				Subject:   "Test Issue 1",
				Project:   redmine.Project{ID: 1, Name: "Test Project"},
				Status:    redmine.Status{ID: 1, Name: "New"},
				CreatedOn: time.Now(),
				UpdatedOn: time.Now(),
			},
			{
				ID:        2,
				Subject:   "Test Issue 2",
				Project:   redmine.Project{ID: 1, Name: "Test Project"},
				Status:    redmine.Status{ID: 2, Name: "In Progress"},
				CreatedOn: time.Now(),
				UpdatedOn: time.Now(),
			},
		}

		response := redmine.IssuesResponse{
			Issues:     issues,
			TotalCount: 2,
			Offset:     0,
			Limit:      10,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "test-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	issuesResp, err := client.GetProjectIssues("test-project", 10, 0)

	require.NoError(t, err)
	assert.Equal(t, 2, issuesResp.TotalCount)
	assert.Len(t, issuesResp.Issues, 2)
	assert.Equal(t, "Test Issue 1", issuesResp.Issues[0].Subject)
	assert.Equal(t, "Test Issue 2", issuesResp.Issues[1].Subject)
}

func TestClient_GetIssue_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/issues/123.json", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		issue := redmine.Issue{
			ID:          123,
			Subject:     "Test Issue",
			Description: "This is a test issue",
			Project:     redmine.Project{ID: 1, Name: "Test Project"},
			Status:      redmine.Status{ID: 1, Name: "New"},
			Priority:    redmine.Priority{ID: 2, Name: "Normal"},
			Author:      redmine.User{ID: 1, Name: "Test User"},
			DoneRatio:   0,
			CreatedOn:   time.Now(),
			UpdatedOn:   time.Now(),
		}

		response := struct {
			Issue redmine.Issue `json:"issue"`
		}{
			Issue: issue,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "test-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	issue, err := client.GetIssue(123)

	require.NoError(t, err)
	assert.Equal(t, 123, issue.ID)
	assert.Equal(t, "Test Issue", issue.Subject)
	assert.Equal(t, "This is a test issue", issue.Description)
	assert.Equal(t, "Test Project", issue.Project.Name)
}

func TestRedmineClient_TestConnection_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects.json", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("limit"))

		response := redmine.ProjectsResponse{
			Projects:   []redmine.Project{{ID: 1, Name: "Test Project"}},
			TotalCount: 1,
			Offset:     0,
			Limit:      1,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "test-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	err := client.TestConnection()

	assert.NoError(t, err)
}

func TestRedmineClient_TestConnection_Failure(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	config := models.RedmineConfig{
		Enabled:   true,
		URL:       server.URL,
		APIKey:    "invalid-api-key",
		VerifySSL: false,
	}

	client := redmine.NewClient(config)
	err := client.TestConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed with status 401")
}
