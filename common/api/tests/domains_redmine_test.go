//go:build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/domains"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRedmineTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&models.Domain{}, &models.User{}, &models.DomainUser{})
	require.NoError(t, err)

	return db
}

func setupRedmineTestData(t *testing.T, db *gorm.DB) (models.User, models.Domain) {
	// Create test user
	user := models.User{
		Username: "testuser",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test domain
	domain := models.Domain{
		Name:             "test-domain",
		RedmineProjectID: "test-project",
	}
	require.NoError(t, db.Create(&domain).Error)

	// Create domain user association
	domainUser := models.DomainUser{
		UserID:   user.ID,
		DomainID: domain.ID,
		Role:     "domain_user",
	}
	require.NoError(t, db.Create(&domainUser).Error)

	return user, domain
}

func setupRedmineConfig() {
	models.ServerConfig.Redmine = models.RedmineConfig{
		Enabled:   true,
		URL:       "http://redmine.example.com",
		APIKey:    "test-api-key",
		Timeout:   30,
		VerifySSL: false,
	}
}

func setupDisabledRedmineConfig() {
	models.ServerConfig.Redmine = models.RedmineConfig{
		Enabled: false,
	}
}

// Simple test to verify the handler exists and can be called
func TestGetDomainRedmineProject_HandlerExists(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Test that the handler function exists and can be created
	handler := domains.GetDomainRedmineProject(db)
	assert.NotNil(t, handler)
}

func TestGetDomainRedmineProject_InvalidDomainID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	setupRedmineConfig()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/domains/invalid/redmine/project", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := domains.GetDomainRedmineProject(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestGetDomainRedmineProject_RedmineDisabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	setupDisabledRedmineConfig()

	// Create test domain
	domain := models.Domain{
		Name: "test-domain",
	}
	require.NoError(t, db.Create(&domain).Error)

	// Create test user with global admin role
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/domains/1/redmine/project", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("user", user)

	handler := domains.GetDomainRedmineProject(db)
	handler(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Redmine integration is not available", response["error"])
}

func TestGetDomainRedmineIssues_HandlerExists(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Test that the handler function exists and can be created
	handler := domains.GetDomainRedmineIssues(db)
	assert.NotNil(t, handler)
}

func TestGetDomainRedmineIssue_HandlerExists(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Test that the handler function exists and can be created
	handler := domains.GetDomainRedmineIssue(db)
	assert.NotNil(t, handler)
}

func TestRedmineResponseStructures(t *testing.T) {
	// Test that the response structures can be created and marshaled
	project := domains.RedmineProjectResponse{
		ID:          1,
		Name:        "Test Project",
		Identifier:  "test-project",
		Description: "Test Description",
		Status:      1,
		CreatedOn:   "2023-01-01T00:00:00Z",
		UpdatedOn:   "2023-01-02T00:00:00Z",
	}

	data, err := json.Marshal(project)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test Project")

	// Test issue response structure
	issue := domains.RedmineIssueResponse{
		ID:       123,
		Subject:  "Test Issue",
		Project:  project,
		Tracker:  domains.TrackerResponse{ID: 1, Name: "Bug"},
		Status:   domains.StatusResponse{ID: 1, Name: "New"},
		Priority: domains.PriorityResponse{ID: 2, Name: "Normal"},
		Author:   domains.RedmineUserResponse{ID: 1, Name: "Test User"},
	}

	data, err = json.Marshal(issue)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test Issue")
}
