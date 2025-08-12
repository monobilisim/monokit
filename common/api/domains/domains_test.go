//go:build with_api

package domains

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&models.Domain{}, &models.DomainUser{}, &models.User{})
	require.NoError(t, err)

	return db
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func createTestUser(db *gorm.DB, role string) models.User {
	user := models.User{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     role,
	}
	db.Create(&user)
	return user
}

func createTestDomain(db *gorm.DB) models.Domain {
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Active:      true,
	}
	db.Create(&domain)
	return domain
}

// Test CreateDomain
func TestCreateDomain_Success(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	// Create global admin user
	user := createTestUser(db, "global_admin")

	router.POST("/domains", func(c *gin.Context) {
		c.Set("user", user)
		CreateDomain(db)(c)
	})

	reqBody := CreateDomainRequest{
		Name:        "new-domain",
		Description: "New test domain",
		Settings:    `{"key": "value"}`,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response DomainResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "new-domain", response.Name)
	assert.Equal(t, "New test domain", response.Description)
	assert.True(t, response.Active)
}

func TestCreateDomain_NonGlobalAdmin(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	// Create regular user
	user := createTestUser(db, "user")

	router.POST("/domains", func(c *gin.Context) {
		c.Set("user", user)
		CreateDomain(db)(c)
	})

	reqBody := CreateDomainRequest{Name: "new-domain"}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateDomain_NoUser(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	router.POST("/domains", CreateDomain(db))

	reqBody := CreateDomainRequest{Name: "new-domain"}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateDomain_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")

	router.POST("/domains", func(c *gin.Context) {
		c.Set("user", user)
		CreateDomain(db)(c)
	})

	req, _ := http.NewRequest("POST", "/domains", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateDomain_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	createTestDomain(db) // Creates "test-domain"

	router.POST("/domains", func(c *gin.Context) {
		c.Set("user", user)
		CreateDomain(db)(c)
	})

	reqBody := CreateDomainRequest{Name: "test-domain"} // Same name
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/domains", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// Test GetAllDomains
func TestGetAllDomains_Success(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain1 := createTestDomain(db)
	domain2 := models.Domain{Name: "domain2", Description: "Second domain", Active: true}
	db.Create(&domain2)

	router.GET("/domains", func(c *gin.Context) {
		c.Set("user", user)
		GetAllDomains(db)(c)
	})

	req, _ := http.NewRequest("GET", "/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []DomainResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, domain1.Name, response[0].Name)
	assert.Equal(t, domain2.Name, response[1].Name)
}

func TestGetAllDomains_NonGlobalAdmin(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "user")

    router.GET("/domains", func(c *gin.Context) {
        c.Set("user", user)
        GetAllDomains(db)(c)
    })

	req, _ := http.NewRequest("GET", "/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var response []DomainResponse
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Len(t, response, 0)
}

// Test GetDomainByID
func TestGetDomainByID_Success(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain := createTestDomain(db)

	router.GET("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		GetDomainByID(db)(c)
	})

	req, _ := http.NewRequest("GET", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DomainResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, domain.Name, response.Name)
	assert.Equal(t, domain.ID, response.ID)
}

func TestGetDomainByID_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")

	router.GET("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		GetDomainByID(db)(c)
	})

	req, _ := http.NewRequest("GET", "/domains/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetDomainByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")

	router.GET("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		GetDomainByID(db)(c)
	})

	req, _ := http.NewRequest("GET", "/domains/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetDomainByID_NonGlobalAdmin(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "user")
	domain := createTestDomain(db)

	router.GET("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		GetDomainByID(db)(c)
	})

	req, _ := http.NewRequest("GET", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Test UpdateDomain
func TestUpdateDomain_Success(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain := createTestDomain(db)

	router.PUT("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		UpdateDomain(db)(c)
	})

	reqBody := UpdateDomainRequest{
		Name:        "updated-domain",
		Description: "Updated description",
		Settings:    `{"updated": "value"}`,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/domains/"+strconv.Itoa(int(domain.ID)), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DomainResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "updated-domain", response.Name)
	assert.Equal(t, "Updated description", response.Description)
}

func TestUpdateDomain_NonGlobalAdmin(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "user")
	domain := createTestDomain(db)

	router.PUT("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		UpdateDomain(db)(c)
	})

	reqBody := UpdateDomainRequest{Name: "updated-domain"}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/domains/"+strconv.Itoa(int(domain.ID)), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateDomain_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")

	router.PUT("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		UpdateDomain(db)(c)
	})

	reqBody := UpdateDomainRequest{Name: "updated-domain"}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/domains/999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateDomain_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain1 := createTestDomain(db)
	domain2 := models.Domain{Name: "domain2", Description: "Second domain", Active: true}
	db.Create(&domain2)

	router.PUT("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		UpdateDomain(db)(c)
	})

	reqBody := UpdateDomainRequest{Name: "domain2"} // Try to use existing name
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/domains/"+strconv.Itoa(int(domain1.ID)), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// Test DeleteDomain
func TestDeleteDomain_Success(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain := createTestDomain(db)

	router.DELETE("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		DeleteDomain(db)(c)
	})

	req, _ := http.NewRequest("DELETE", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify domain was deleted
	var count int64
	db.Model(&models.Domain{}).Where("id = ?", domain.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteDomain_WithAssociatedUsers(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")
	domain := createTestDomain(db)

	// Create domain user association
	domainUser := models.DomainUser{
		DomainID: domain.ID,
		UserID:   user.ID,
		Role:     "domain_user",
	}
	db.Create(&domainUser)

	router.DELETE("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		DeleteDomain(db)(c)
	})

	req, _ := http.NewRequest("DELETE", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteDomain_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "global_admin")

	router.DELETE("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		DeleteDomain(db)(c)
	})

	req, _ := http.NewRequest("DELETE", "/domains/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteDomain_NonGlobalAdmin(t *testing.T) {
	db := setupTestDB(t)
	router := setupTestRouter()

	user := createTestUser(db, "user")
	domain := createTestDomain(db)

	router.DELETE("/domains/:id", func(c *gin.Context) {
		c.Set("user", user)
		DeleteDomain(db)(c)
	})

	req, _ := http.NewRequest("DELETE", "/domains/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
