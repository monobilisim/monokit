//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/domains"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.POST("/domains", domains.CreateDomain(db))

	// Test data
	domainRequest := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain for unit tests",
		Settings:    `{"theme": "dark"}`,
	}

	jsonData, err := json.Marshal(domainRequest)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.DomainResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-domain", response.Name)
	assert.Equal(t, "Test domain for unit tests", response.Description)
	assert.Equal(t, `{"theme": "dark"}`, response.Settings)
	assert.True(t, response.Active)
}

func TestCreateDomain_Forbidden(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a regular admin user (not global admin)
	admin := SetupTestUser(t, db, "admin")

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.POST("/domains", domains.CreateDomain(db))

	// Test data
	domainRequest := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain for unit tests",
	}

	jsonData, err := json.Marshal(domainRequest)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Global admin access required", response["error"])
}

func TestCreateDomain_DuplicateName(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Create an existing domain
	existingDomain := models.Domain{
		Name:        "existing-domain",
		Description: "Existing domain",
		Active:      true,
	}
	require.NoError(t, db.Create(&existingDomain).Error)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.POST("/domains", domains.CreateDomain(db))

	// Test data with same name
	domainRequest := models.CreateDomainRequest{
		Name:        "existing-domain",
		Description: "Duplicate domain",
	}

	jsonData, err := json.Marshal(domainRequest)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Domain name already exists", response["error"])
}

func TestCreateDomain_InvalidJSON(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.POST("/domains", domains.CreateDomain(db))

	// Invalid JSON
	invalidJSON := `{"name": "test", "description":}`

	// Make request
	req, err := http.NewRequest("POST", "/domains", bytes.NewBufferString(invalidJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAllDomains_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Create test domains
	domain1 := models.Domain{
		Name:        "domain1",
		Description: "First domain",
		Active:      true,
	}
	domain2 := models.Domain{
		Name:        "domain2",
		Description: "Second domain",
		Active:      true, // Will be set to false after creation
	}
	require.NoError(t, db.Create(&domain1).Error)
	require.NoError(t, db.Create(&domain2).Error)

	// Explicitly set domain2 to inactive after creation to override GORM default
	require.NoError(t, db.Model(&domain2).Update("active", false).Error)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.GET("/domains", domains.GetAllDomains(db))

	// Make request
	req, err := http.NewRequest("GET", "/domains", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.DomainResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should have 3 domains: default domain + 2 test domains
	assert.Len(t, response, 3)

	// Find domains by name (excluding default domain)
	var foundDomain1, foundDomain2 *models.DomainResponse
	var testDomains []models.DomainResponse
	for i := range response {
		if response[i].Name == "domain1" {
			foundDomain1 = &response[i]
			testDomains = append(testDomains, response[i])
		} else if response[i].Name == "domain2" {
			foundDomain2 = &response[i]
			testDomains = append(testDomains, response[i])
		}
	}

	// Verify we found exactly 2 test domains (excluding default)
	assert.Len(t, testDomains, 2)

	require.NotNil(t, foundDomain1)
	require.NotNil(t, foundDomain2)

	assert.Equal(t, "First domain", foundDomain1.Description)
	assert.True(t, foundDomain1.Active)

	assert.Equal(t, "Second domain", foundDomain2.Description)
	assert.False(t, foundDomain2.Active)
}

func TestGetAllDomains_Forbidden(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a regular user (not global admin)
	admin := SetupTestUser(t, db, "regularuser")

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

    router.GET("/domains", domains.GetAllDomains(db))

	// Make request
	req, err := http.NewRequest("GET", "/domains", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
    assert.Equal(t, http.StatusOK, w.Code)

    var response []models.DomainResponse
    err = json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Len(t, response, 0)
}

func TestGetDomainByID_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Create test domain
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Settings:    `{"key": "value"}`,
		Active:      true,
	}
	require.NoError(t, db.Create(&domain).Error)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.GET("/domains/:id", domains.GetDomainByID(db))

	// Make request
	req, err := http.NewRequest("GET", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DomainResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, domain.ID, response.ID)
	assert.Equal(t, "test-domain", response.Name)
	assert.Equal(t, "Test domain", response.Description)
	assert.Equal(t, `{"key": "value"}`, response.Settings)
	assert.True(t, response.Active)
}

func TestGetDomainByID_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.GET("/domains/:id", domains.GetDomainByID(db))

	// Make request with non-existent ID
	req, err := http.NewRequest("GET", "/domains/999", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Domain not found", response["error"])
}

func TestGetDomainByID_InvalidID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets the user
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.GET("/domains/:id", domains.GetDomainByID(db))

	// Make request with invalid ID
	req, err := http.NewRequest("GET", "/domains/invalid", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

// Test UpdateDomain functionality
func TestUpdateDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)

	// Create test domain
	domain := models.Domain{
		Name:        "original-domain",
		Description: "Original description",
		Settings:    `{"theme": "dark"}`,
		Active:      true,
	}
	require.NoError(t, db.Create(&domain).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.PUT("/domains/:id", domains.UpdateDomain(db))

	updateRequest := models.UpdateDomainRequest{
		Name:        "updated-domain",
		Description: "Updated description",
		Settings:    `{"theme": "light"}`,
	}

	jsonData, err := json.Marshal(updateRequest)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", "/domains/"+strconv.Itoa(int(domain.ID)), bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DomainResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "updated-domain", response.Name)
	assert.Equal(t, "Updated description", response.Description)
	assert.Equal(t, `{"theme": "light"}`, response.Settings)
}

// Test DeleteDomain functionality
func TestDeleteDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)

	// Create test domain
	domain := models.Domain{
		Name:        "domain-to-delete",
		Description: "Domain to be deleted",
		Active:      true,
	}
	require.NoError(t, db.Create(&domain).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})

	router.DELETE("/domains/:id", domains.DeleteDomain(db))

	req, err := http.NewRequest("DELETE", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify domain was deleted
	var deletedDomain models.Domain
	err = db.First(&deletedDomain, domain.ID).Error
	assert.Error(t, err) // Should be gorm.ErrRecordNotFound
}

// Test domain operations with no authentication
func TestDomainOperations_NoAuth(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// No auth middleware - user context will be missing
	router.POST("/domains", domains.CreateDomain(db))
	router.GET("/domains", domains.GetAllDomains(db))
	router.GET("/domains/:id", domains.GetDomainByID(db))

	testCases := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/domains", `{"name": "test"}`},
		{"GET", "/domains", ""},
		{"GET", "/domains/1", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			var req *http.Request
			var err error

			if tc.body != "" {
				req, err = http.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(tc.method, tc.path, nil)
			}
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)

			var response map[string]string
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "Global admin access required", response["error"])
		})
	}
}
