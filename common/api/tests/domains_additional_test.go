//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/domains"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDomain_MissingUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := domains.CreateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Global admin access required", response["error"])
}

func TestCreateDomain_NonAdminUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create regular user
	user := models.User{
		Username: "user",
		Email:    "user@example.com",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	reqBody := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := domains.CreateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Global admin access required", response["error"])
}

func TestCreateDomain_InvalidJSONRequest(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := domains.CreateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "invalid character")
}

// Test database error scenarios
func TestCreateDomain_DatabaseError(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	// Close the database to simulate error
	sqlDB, _ := db.DB()
	sqlDB.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	reqBody := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := domains.CreateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Test duplicate domain creation
func TestCreateDomain_DuplicateName_Additional(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create existing domain
	existingDomain := models.Domain{
		Name:        "existing-domain",
		Description: "Existing domain",
		Active:      true,
	}
	require.NoError(t, db.Create(&existingDomain).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	reqBody := models.CreateDomainRequest{
		Name:        "existing-domain", // Same name
		Description: "Duplicate domain",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := domains.CreateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Domain name already exists", response["error"])
}

// Test GetAllDomains with no user context
func TestGetAllDomains_NoUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler := domains.GetAllDomains(db)
	handler(c)

    assert.Equal(t, http.StatusForbidden, w.Code)

    var response map[string]string
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Equal(t, "Global admin access required", response["error"]) // unchanged for no-auth
}

// Test GetAllDomains with non-admin user
func TestGetAllDomains_NonAdmin(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create regular user
	user := models.User{
		Username: "user",
		Email:    "user@example.com",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

    handler := domains.GetAllDomains(db)
    handler(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response []models.DomainResponse
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Len(t, response, 0)
}

// Test GetAllDomains success
func TestGetAllDomains_Success_Additional(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test domains
	domain1 := models.Domain{
		Name:        "domain1",
		Description: "First domain",
		Active:      true,
	}
	domain2 := models.Domain{
		Name:        "domain2",
		Description: "Second domain",
		Active:      false,
	}
	require.NoError(t, db.Create(&domain1).Error)
	require.NoError(t, db.Create(&domain2).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	handler := domains.GetAllDomains(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.DomainResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Expect only the two test domains when ignoring the default one
	var filtered []models.DomainResponse
	for _, d := range response {
		if d.Name != "default" {
			filtered = append(filtered, d)
		}
	}
	assert.Len(t, filtered, 2)

	// Map by name to avoid relying on ordering
	domainsByName := map[string]models.DomainResponse{}
	for _, d := range filtered {
		domainsByName[d.Name] = d
	}

	d1 := domainsByName["domain1"]
	assert.Equal(t, "First domain", d1.Description)
	assert.True(t, d1.Active)

	d2 := domainsByName["domain2"]
	assert.Equal(t, "Second domain", d2.Description)
	assert.False(t, d2.Active)
}


func TestGetAllDomains_MissingUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/domains", nil)

	handler := domains.GetAllDomains(db)
	handler(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response []models.DomainResponse
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Len(t, response, 0)
}

func TestGetAllDomains_NonAdminUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create regular user
	user := models.User{
		Username: "user",
		Email:    "user@example.com",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	c.Request = httptest.NewRequest("GET", "/domains", nil)

    handler := domains.GetAllDomains(db)
    handler(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response []models.DomainResponse
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Len(t, response, 0)
}

func TestGetDomainByID_InvalidIDParam(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	c.Request = httptest.NewRequest("GET", "/domains/invalid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := domains.GetDomainByID(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestGetDomainByID_DomainNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	c.Request = httptest.NewRequest("GET", "/domains/999", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	handler := domains.GetDomainByID(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Domain not found", response["error"])
}

func TestUpdateDomain_InvalidID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	reqBody := models.UpdateDomainRequest{
		Name:        "updated-domain",
		Description: "Updated domain",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("PUT", "/domains/invalid", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := domains.UpdateDomain(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestDeleteDomain_InvalidID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	c.Request = httptest.NewRequest("DELETE", "/domains/invalid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := domains.DeleteDomain(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestAssignUserToDomain_InvalidDomainID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create admin user
	user := models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "global_admin",
	}
	require.NoError(t, db.Create(&user).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)

	reqBody := models.AssignUserToDomainRequest{
		UserID: 1,
		Role:   "domain_user",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/domains/invalid/users", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler := domains.AssignUserToDomain(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestTypeAliases(t *testing.T) {
	// Test that type aliases work correctly
	var domain domains.Domain
	var domainUser domains.DomainUser
	var user domains.User
	var createReq domains.CreateDomainRequest
	var updateReq domains.UpdateDomainRequest
	var domainResp domains.DomainResponse
	var assignReq domains.AssignUserToDomainRequest
	var updateRoleReq domains.UpdateDomainUserRoleRequest
	var domainUserResp domains.DomainUserResponse
	var userResp domains.UserResponse

	// Just verify the types exist and can be instantiated
	assert.NotNil(t, &domain)
	assert.NotNil(t, &domainUser)
	assert.NotNil(t, &user)
	assert.NotNil(t, &createReq)
	assert.NotNil(t, &updateReq)
	assert.NotNil(t, &domainResp)
	assert.NotNil(t, &assignReq)
	assert.NotNil(t, &updateRoleReq)
	assert.NotNil(t, &domainUserResp)
	assert.NotNil(t, &userResp)
}
