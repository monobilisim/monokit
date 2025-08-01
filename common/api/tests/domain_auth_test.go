//go:build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/auth"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestRequireDomainAccess_NoUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Authentication required", response["error"])
}

func TestRequireDomainAccess_GlobalAdmin_NoDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a global admin user
	user := models.User{
		Username: "globaladmin",
		Role:     "global_admin",
		Email:    "admin@test.com",
	}
	db.Create(&user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test", func(c *gin.Context) {
		domainContext, exists := c.Get("domain_context")
		assert.True(t, exists)
		context := domainContext.(auth.DomainContext)
		assert.True(t, context.IsGlobalAdmin)
		assert.Nil(t, context.RequestedDomainID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAccess_GlobalAdmin_WithValidDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a domain
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Active:      true,
	}
	db.Create(&domain)

	// Create a global admin user
	user := models.User{
		Username: "globaladmin",
		Role:     "global_admin",
		Email:    "admin@test.com",
	}
	db.Create(&user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		domainContext, exists := c.Get("domain_context")
		assert.True(t, exists)
		context := domainContext.(auth.DomainContext)
		assert.True(t, context.IsGlobalAdmin)
		assert.NotNil(t, context.RequestedDomainID)
		assert.Equal(t, domain.ID, *context.RequestedDomainID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAccess_GlobalAdmin_WithInvalidDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a global admin user
	user := models.User{
		Username: "globaladmin",
		Role:     "global_admin",
		Email:    "admin@test.com",
	}
	db.Create(&user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test with invalid domain ID format
	req, _ := http.NewRequest("GET", "/test/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

func TestRequireDomainAccess_GlobalAdmin_WithNonExistentDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a global admin user
	user := models.User{
		Username: "globaladmin",
		Role:     "global_admin",
		Email:    "admin@test.com",
	}
	db.Create(&user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test with non-existent domain ID
	req, _ := http.NewRequest("GET", "/test/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Domain not found", response["error"])
}

func TestRequireDomainAccess_RegularUser_DatabaseError(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a regular user
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	// Close the database to simulate an error
	sqlDB, _ := db.DB()
	sqlDB.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Failed to load user domains", response["error"])
}

func TestRequireDomainAccess_RegularUser_NoDomains(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a regular user with no domain associations
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "User has no domain access", response["error"])
}

func TestRequireDomainAccess_RegularUser_WithDomains_NoDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a domain
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Active:      true,
	}
	db.Create(&domain)

	// Create a regular user
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	// Create domain user association
	domainUser := models.DomainUser{
		DomainID: domain.ID,
		UserID:   user.ID,
		Role:     "domain_user",
	}
	db.Create(&domainUser)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test", func(c *gin.Context) {
		domainContext, exists := c.Get("domain_context")
		assert.True(t, exists)
		context := domainContext.(auth.DomainContext)
		assert.False(t, context.IsGlobalAdmin)
		assert.Len(t, context.UserDomains, 1)
		assert.Equal(t, domain.ID, context.UserDomains[0].DomainID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAccess_RegularUser_WithValidDomainAccess(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a domain
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Active:      true,
	}
	db.Create(&domain)

	// Create a regular user
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	// Create domain user association
	domainUser := models.DomainUser{
		DomainID: domain.ID,
		UserID:   user.ID,
		Role:     "domain_user",
	}
	db.Create(&domainUser)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		domainContext, exists := c.Get("domain_context")
		assert.True(t, exists)
		context := domainContext.(auth.DomainContext)
		assert.False(t, context.IsGlobalAdmin)
		assert.NotNil(t, context.RequestedDomainID)
		assert.Equal(t, domain.ID, *context.RequestedDomainID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test/"+strconv.Itoa(int(domain.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAccess_RegularUser_WithInvalidDomainAccess(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create two domains
	domain1 := models.Domain{
		Name:        "test-domain-1",
		Description: "Test domain 1",
		Active:      true,
	}
	db.Create(&domain1)

	domain2 := models.Domain{
		Name:        "test-domain-2",
		Description: "Test domain 2",
		Active:      true,
	}
	db.Create(&domain2)

	// Create a regular user
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	// Create domain user association only for domain1
	domainUser := models.DomainUser{
		DomainID: domain1.ID,
		UserID:   user.ID,
		Role:     "domain_user",
	}
	db.Create(&domainUser)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Try to access domain2 which user doesn't have access to
	req, _ := http.NewRequest("GET", "/test/"+strconv.Itoa(int(domain2.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Access denied to this domain", response["error"])
}

func TestRequireDomainAccess_RegularUser_InvalidDomainIDFormat(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	// Create a domain
	domain := models.Domain{
		Name:        "test-domain",
		Description: "Test domain",
		Active:      true,
	}
	db.Create(&domain)

	// Create a regular user
	user := models.User{
		Username: "regularuser",
		Role:     "user",
		Email:    "user@test.com",
	}
	db.Create(&user)

	// Create domain user association
	domainUser := models.DomainUser{
		DomainID: domain.ID,
		UserID:   user.ID,
		Role:     "domain_user",
	}
	db.Create(&domainUser)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.Use(auth.RequireDomainAccess(db))
	router.GET("/test/:domain_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test with invalid domain ID format
	req, _ := http.NewRequest("GET", "/test/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Invalid domain ID", response["error"])
}

// Tests for RequireDomainAdmin middleware
func TestRequireDomainAdmin_NoDomainContext(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Domain context not found", response["error"])
}

func TestRequireDomainAdmin_GlobalAdmin(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		domainContext := auth.DomainContext{
			IsGlobalAdmin: true,
		}
		c.Set("domain_context", domainContext)
		c.Next()
	})
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAdmin_RegularUser_NoDomainID(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		domainContext := auth.DomainContext{
			IsGlobalAdmin: false,
			UserDomains:   []models.DomainUser{},
		}
		c.Set("domain_context", domainContext)
		c.Next()
	})
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Domain ID required for this operation", response["error"])
}

func TestRequireDomainAdmin_RegularUser_WithDomainAdmin(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	domainID := uint(1)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		domainContext := auth.DomainContext{
			IsGlobalAdmin:     false,
			RequestedDomainID: &domainID,
			UserDomains: []models.DomainUser{
				{
					DomainID: domainID,
					UserID:   1,
					Role:     "domain_admin",
				},
			},
		}
		c.Set("domain_context", domainContext)
		c.Next()
	})
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireDomainAdmin_RegularUser_WithoutDomainAdmin(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	domainID := uint(1)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		domainContext := auth.DomainContext{
			IsGlobalAdmin:     false,
			RequestedDomainID: &domainID,
			UserDomains: []models.DomainUser{
				{
					DomainID: domainID,
					UserID:   1,
					Role:     "domain_user", // Not admin
				},
			},
		}
		c.Set("domain_context", domainContext)
		c.Next()
	})
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Domain admin access required", response["error"])
}

func TestRequireDomainAdmin_RegularUser_WrongDomain(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	requestedDomainID := uint(1)
	userDomainID := uint(2)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		domainContext := auth.DomainContext{
			IsGlobalAdmin:     false,
			RequestedDomainID: &requestedDomainID,
			UserDomains: []models.DomainUser{
				{
					DomainID: userDomainID, // Different domain
					UserID:   1,
					Role:     "domain_admin",
				},
			},
		}
		c.Set("domain_context", domainContext)
		c.Next()
	})
	router.Use(auth.RequireDomainAdmin(db))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Domain admin access required", response["error"])
}

// Tests for utility functions
func TestGetUserDomainIDs_NoDomainContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainIDs := auth.GetUserDomainIDs(c)
	assert.Empty(t, domainIDs)
}

func TestGetUserDomainIDs_GlobalAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: true,
	}
	c.Set("domain_context", domainContext)

	domainIDs := auth.GetUserDomainIDs(c)
	assert.Nil(t, domainIDs) // nil indicates access to all domains
}

func TestGetUserDomainIDs_RegularUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: false,
		UserDomains: []models.DomainUser{
			{DomainID: 1, UserID: 1, Role: "domain_user"},
			{DomainID: 2, UserID: 1, Role: "domain_admin"},
		},
	}
	c.Set("domain_context", domainContext)

	domainIDs := auth.GetUserDomainIDs(c)
	assert.Len(t, domainIDs, 2)
	assert.Contains(t, domainIDs, uint(1))
	assert.Contains(t, domainIDs, uint(2))
}

func TestHasDomainAdminAccess_NoDomainContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	hasAccess := auth.HasDomainAdminAccess(c, 1)
	assert.False(t, hasAccess)
}

func TestHasDomainAdminAccess_GlobalAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: true,
	}
	c.Set("domain_context", domainContext)

	hasAccess := auth.HasDomainAdminAccess(c, 1)
	assert.True(t, hasAccess)
}

func TestHasDomainAdminAccess_RegularUser_WithAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: false,
		UserDomains: []models.DomainUser{
			{DomainID: 1, UserID: 1, Role: "domain_admin"},
			{DomainID: 2, UserID: 1, Role: "domain_user"},
		},
	}
	c.Set("domain_context", domainContext)

	hasAccess := auth.HasDomainAdminAccess(c, 1)
	assert.True(t, hasAccess)

	hasAccess = auth.HasDomainAdminAccess(c, 2)
	assert.False(t, hasAccess) // domain_user, not domain_admin

	hasAccess = auth.HasDomainAdminAccess(c, 3)
	assert.False(t, hasAccess) // no access to domain 3
}

func TestFilterByDomainAccess_GlobalAdmin(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: true,
	}
	c.Set("domain_context", domainContext)

	query := db.Model(&models.Host{})
	filteredQuery := auth.FilterByDomainAccess(c, query, "domain_id")

	// For global admin, query should be unchanged
	assert.Equal(t, query, filteredQuery)
}

func TestFilterByDomainAccess_RegularUser_WithDomains(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: false,
		UserDomains: []models.DomainUser{
			{DomainID: 1, UserID: 1, Role: "domain_user"},
			{DomainID: 2, UserID: 1, Role: "domain_admin"},
		},
	}
	c.Set("domain_context", domainContext)

	query := db.Model(&models.Host{})
	filteredQuery := auth.FilterByDomainAccess(c, query, "domain_id")

	// Query should be filtered to include only domains 1 and 2
	// We can't easily test the exact SQL, but we can verify it's different from the original
	assert.NotEqual(t, query, filteredQuery)
}

func TestFilterByDomainAccess_RegularUser_NoDomains(t *testing.T) {
	db := setupTestDB()
	defer cleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	domainContext := auth.DomainContext{
		IsGlobalAdmin: false,
		UserDomains:   []models.DomainUser{}, // No domains
	}
	c.Set("domain_context", domainContext)

	query := db.Model(&models.Host{})
	filteredQuery := auth.FilterByDomainAccess(c, query, "domain_id")

	// Query should be filtered to never match (1 = 0)
	assert.NotEqual(t, query, filteredQuery)
}

func TestExtractDomainFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected *uint
	}{
		{"/domains/123", uintPtr(123)},
		{"/api/v1/domains/456", uintPtr(456)},
		{"/api/v1/domains/456/hosts", uintPtr(456)},
		{"domains/789/users/1", uintPtr(789)},
		{"/domains/invalid", nil},
		{"/users/123", nil},
		{"/domains", nil},
		{"", nil},
	}

	for _, test := range tests {
		result := auth.ExtractDomainFromPath(test.path)
		if test.expected == nil {
			assert.Nil(t, result, "Path: %s", test.path)
		} else {
			assert.NotNil(t, result, "Path: %s", test.path)
			assert.Equal(t, *test.expected, *result, "Path: %s", test.path)
		}
	}
}

// Helper function to create a uint pointer
func uintPtr(u uint) *uint {
	return &u
}
