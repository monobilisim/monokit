//go:build with_api

package domains

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestGetRedmineClient_Disabled(t *testing.T) {
	// Save original config
	originalConfig := models.ServerConfig.Redmine
	defer func() {
		models.ServerConfig.Redmine = originalConfig
	}()

	// Disable Redmine
	models.ServerConfig.Redmine.Enabled = false

	client, err := getRedmineClient()
	assert.Nil(t, client)
	assert.Error(t, err)
}

func TestGetRedmineClient_Enabled(t *testing.T) {
	// Save original config
	originalConfig := models.ServerConfig.Redmine
	defer func() {
		models.ServerConfig.Redmine = originalConfig
	}()

	// Enable Redmine with test config
	models.ServerConfig.Redmine = models.RedmineConfig{
		Enabled: true,
		URL:     "http://test-redmine.com",
		APIKey:  "test-api-key",
	}

	client, err := getRedmineClient()
	assert.NotNil(t, client)
	assert.NoError(t, err)
}

func TestGetDomainRedmineProjectID_WithProjectID(t *testing.T) {
	domain := &models.Domain{
		Name:             "test-domain",
		RedmineProjectID: "custom-project-id",
	}

	projectID := getDomainRedmineProjectID(domain)
	assert.Equal(t, "custom-project-id", projectID)
}

func TestGetDomainRedmineProjectID_WithoutProjectID(t *testing.T) {
	domain := &models.Domain{
		Name:             "test-domain",
		RedmineProjectID: "",
	}

	projectID := getDomainRedmineProjectID(domain)
	assert.Equal(t, "test-domain", projectID)
}

func TestCheckDomainAccess_NoUser(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Create gin context without user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	domain, hasAccess := checkDomainAccess(c, db, 1)
	assert.Nil(t, domain)
	assert.False(t, hasAccess)
}

func TestCheckDomainAccess_GlobalAdmin(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Create global admin user
	user := models.User{
		Username: "admin",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create domain
	domain := models.Domain{
		Name: "test-domain",
	}
	require.NoError(t, db.Create(&domain).Error)

	// Create gin context with global admin
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	resultDomain, hasAccess := checkDomainAccess(c, db, domain.ID)
	assert.NotNil(t, resultDomain)
	assert.True(t, hasAccess)
	assert.Equal(t, domain.Name, resultDomain.Name)
}

func TestCheckDomainAccess_GlobalAdmin_DomainNotFound(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Create global admin user
	user := models.User{
		Username: "admin",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create gin context with global admin
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	resultDomain, hasAccess := checkDomainAccess(c, db, 999) // Non-existent domain
	assert.Nil(t, resultDomain)
	assert.False(t, hasAccess)
}

func TestCheckDomainAccess_RegularUser_WithAccess(t *testing.T) {
	db := setupRedmineTestDB(t)
	user, domain := setupRedmineTestData(t, db)

	// Create gin context with regular user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	resultDomain, hasAccess := checkDomainAccess(c, db, domain.ID)
	assert.NotNil(t, resultDomain)
	assert.True(t, hasAccess)
	assert.Equal(t, domain.Name, resultDomain.Name)
}

func TestCheckDomainAccess_RegularUser_NoAccess(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Create user without domain access
	user := models.User{
		Username: "testuser",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create domain without user association
	domain := models.Domain{
		Name: "test-domain",
	}
	require.NoError(t, db.Create(&domain).Error)

	// Create gin context with user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	resultDomain, hasAccess := checkDomainAccess(c, db, domain.ID)
	assert.Nil(t, resultDomain)
	assert.False(t, hasAccess)
}

func TestGetDomainRedmineProject_InvalidDomainID(t *testing.T) {
	db := setupRedmineTestDB(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := GetDomainRedmineProject(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestGetDomainRedmineProject_NoAccess(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Create user without domain access
	user := models.User{
		Username: "testuser",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("user", user)

	handler := GetDomainRedmineProject(db)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Access denied to this domain", response["error"])
}

func TestGetDomainRedmineProject_RedmineDisabled(t *testing.T) {
	db := setupRedmineTestDB(t)
	user, _ := setupRedmineTestData(t, db)

	// Save original config and disable Redmine
	originalConfig := models.ServerConfig.Redmine
	defer func() {
		models.ServerConfig.Redmine = originalConfig
	}()
	models.ServerConfig.Redmine.Enabled = false

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("user", user)

	handler := GetDomainRedmineProject(db)
	handler(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Redmine integration is not available", response["error"])
}

func TestGetDomainRedmineIssues_InvalidDomainID(t *testing.T) {
	db := setupRedmineTestDB(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := GetDomainRedmineIssues(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestGetDomainRedmineIssue_InvalidDomainID(t *testing.T) {
	db := setupRedmineTestDB(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: "invalid"},
		{Key: "issue_id", Value: "123"},
	}

	handler := GetDomainRedmineIssue(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestGetDomainRedmineIssue_InvalidIssueID(t *testing.T) {
	db := setupRedmineTestDB(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: "1"},
		{Key: "issue_id", Value: "invalid"},
	}

	handler := GetDomainRedmineIssue(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid issue ID", response["error"])
}
