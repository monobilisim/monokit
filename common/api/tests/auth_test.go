//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)

	// Test: Successful registration
	registerReq := common.RegisterRequest{
		Username:  "newuser",
		Password:  "password123",
		Email:     "new@example.com",
		Role:      "user",
		Groups:    "nil",
		Inventory: "default",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, admin)

	handler := common.ExportRegisterUser(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created
	var user common.User
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
	hashedPassword, err := common.HashPassword("testpass")
	require.NoError(t, err)
	db.Create(&common.User{
		Username:    "testuser",
		Password:    hashedPassword,
		Email:       "test@example.com",
		Role:        "user",
		Groups:      "nil",
		Inventories: "default",
		AuthMethod:  "local",
	})

	// Test: Successful login
	loginReq := common.LoginRequest{
		Username: "testuser",
		Password: "testpass",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", loginReq)

	handler := common.ExportLoginUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp common.LoginResponse
	ExtractJSONResponse(t, w, &resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "testuser", resp.User.Username)
	assert.Equal(t, "test@example.com", resp.User.Email)
	assert.Equal(t, "user", resp.User.Role)

	// Test: Wrong password
	wrongPassReq := common.LoginRequest{
		Username: "testuser",
		Password: "wrongpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", wrongPassReq)

	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test: Non-existent user
	nonExistentReq := common.LoginRequest{
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

	handler := common.ExportLogoutUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify session was deleted
	var count int64
	db.Model(&common.Session{}).Where("token = ?", session.Token).Count(&count)
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
	updateReq := common.UpdateMeRequest{
		Password: "newpassword",
	}
	c, w := CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler := common.ExportUpdateMe(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify password was changed
	var updatedUser common.User
	db.Where("username = ?", "testuser").First(&updatedUser)
	assert.True(t, common.VerifyPassword("newpassword", updatedUser.Password))
	assert.False(t, common.VerifyPassword("userpass", updatedUser.Password))

	// Test: Update email
	updateEmailReq := common.UpdateMeRequest{
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

	handler := common.ExportDeleteMe(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was deleted
	var count int64
	db.Model(&common.User{}).Where("username = ?", "usertodelete").Count(&count)
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

	handler := common.ExportGetCurrentUser()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response common.UserResponse
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
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	session := SetupSession(t, db, user)

	// Test: Valid token
	c, w := CreateRequestContext("GET", "/api/v1/protected", nil)
	c.Request.Header.Set("Authorization", session.Token)

	var handlerCalled bool
	middleware := common.ExportAuthMiddleware(db)

	// Create a test handler that will be called after middleware
	testHandler := func(c *gin.Context) {
		userObj, exists := c.Get("user")
		if exists {
			handlerCalled = true
			assert.Equal(t, user.ID, userObj.(common.User).ID)
		}
		c.Status(http.StatusOK)
	}

	// Call middleware then the test handler
	middleware(c)
	if c.Writer.Status() == http.StatusOK {
		testHandler(c)
	}

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Invalid token
	c, w = CreateRequestContext("GET", "/api/v1/protected", nil)
	c.Request.Header.Set("Authorization", "invalid-token")

	middleware(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test: Missing token
	c, w = CreateRequestContext("GET", "/api/v1/protected", nil)
	middleware(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
