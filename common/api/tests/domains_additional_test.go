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

func TestGetAllDomains_MissingUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/domains", nil)

	handler := domains.GetAllDomains(db)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Global admin access required", response["error"])
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

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Global admin access required", response["error"])
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
