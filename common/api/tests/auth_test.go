//go:build with_api

package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/admin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)

	// Test: Successful registration
	registerReq := models.RegisterRequest{
		Username:  "newuser",
		Password:  "password123",
		Email:     "new@example.com",
		Role:      "user",
		Groups:    "nil",
		Inventory: "default",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, admin)

	handler := admin.ExportRegisterUser(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created
	var user models.User
	result := db.Where("username = ?", "newuser").First(&user)
	require.NoError(t, result.Error)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, "user", user.Role)

	// Test: Non-admin registration attempt
	regularUser := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, regularUser)

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Test: Duplicate username
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, admin)

	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestLoginUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a user with known password
	hashedPassword, err := models.HashPassword("testpass")
	require.NoError(t, err)
	db.Create(&models.User{
		Username:    "testuser",
		Password:    hashedPassword,
		Email:       "test@example.com",
		Role:        "user",
		Groups:      "nil",
		Inventories: "default",
		AuthMethod:  "local",
	})

	// Test: Successful login
	loginReq := models.LoginRequest{
		Username: "testuser",
		Password: "testpass",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", loginReq)

	handler := admin.ExportLoginUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.LoginResponse
	ExtractJSONResponse(t, w, &resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "testuser", resp.User.Username)
	assert.Equal(t, "test@example.com", resp.User.Email)
	assert.Equal(t, "user", resp.User.Role)

	// Test: Wrong password
	wrongPassReq := models.LoginRequest{
		Username: "testuser",
		Password: "wrongpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", wrongPassReq)

	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test: Non-existent user
	nonExistentReq := models.LoginRequest{
		Username: "nonexistent",
		Password: "testpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", nonExistentReq)

	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogoutUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	session := SetupSession(t, db, user)

	// Test: Successful logout
	c, w := CreateRequestContext("POST", "/api/v1/auth/logout", nil)
	c.Request.Header.Set("Authorization", session.Token)

	handler := admin.ExportLogoutUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify session was deleted
	var count int64
	db.Model(&models.Session{}).Where("token = ?", session.Token).Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: Invalid token - still returns OK but doesn't find a session to delete
	c, w = CreateRequestContext("POST", "/api/v1/auth/logout", nil)
	c.Request.Header.Set("Authorization", "invalid-token")

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Missing token
	c, w = CreateRequestContext("POST", "/api/v1/auth/logout", nil)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateMe(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")

	// Test: Update password
	updateReq := models.UpdateMeRequest{
		Password: "newpassword",
	}
	c, w := CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler := admin.ExportUpdateMe(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify password was changed
	var updatedUser models.User
	db.Where("username = ?", "testuser").First(&updatedUser)
	assert.True(t, models.VerifyPassword("newpassword", updatedUser.Password))
	assert.False(t, models.VerifyPassword("userpass", updatedUser.Password))

	// Test: Update email
	updateEmailReq := models.UpdateMeRequest{
		Email: "new@example.com",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateEmailReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify email was updated
	db.Where("username = ?", "testuser").First(&updatedUser)
	assert.Equal(t, "new@example.com", updatedUser.Email)

	// Test: No authentication
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteMe(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "usertodelete")

	// Test: Successful delete
	c, w := CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	AuthorizeContext(c, user)

	handler := admin.ExportDeleteMe(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was deleted
	var count int64
	db.Model(&models.User{}).Where("username = ?", "usertodelete").Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: No authentication
	c, w = CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetCurrentUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	user.Groups = "group1,group2"
	db.Save(&user)

	// Test: Successful request
	c, w := CreateRequestContext("GET", "/api/v1/auth/me", nil)
	AuthorizeContext(c, user)

	handler := admin.ExportGetCurrentUser()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.UserResponse
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "testuser", response.Username)
	assert.Equal(t, "user", response.Role)
	assert.Equal(t, "group1,group2", response.Groups)

	// Test: No authentication
	c, w = CreateRequestContext("GET", "/api/v1/auth/me", nil)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	session := SetupSession(t, db, user)

	// Set up a gin router with the middleware and a test handler
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(admin.ExportAuthMiddleware(db))
	handlerCalled := false
	router.GET("/api/v1/protected", func(c *gin.Context) {
		userObj, exists := c.Get("user")
		if exists {
			handlerCalled = true
			assert.Equal(t, user.ID, userObj.(models.User).ID)
		}
		c.Status(http.StatusOK)
	})

	// Test: Valid token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected", nil)
	req.Header.Set("Authorization", session.Token)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Invalid token
	handlerCalled = false
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/protected", nil)
	req.Header.Set("Authorization", "invalid-token")
	router.ServeHTTP(w, req)
	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test: Missing token
	handlerCalled = false
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/protected", nil)
	router.ServeHTTP(w, req)
	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
