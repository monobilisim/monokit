//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/auth"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)

	// Test: Successful registration
	registerReq := models.RegisterRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
		Role:     "user",
		Groups:   "nil",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler := auth.ExportRegisterUser(db)
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
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestLoginUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a user with known password
	hashedPassword, err := auth.HashPassword("testpass")
	require.NoError(t, err)
	db.Create(&models.User{
		Username:   "testuser",
		Password:   hashedPassword,
		Email:      "test@example.com",
		Role:       "user",
		Groups:     "nil",
		AuthMethod: "local",
	})

	// Test: Successful login
	loginReq := models.LoginRequest{
		Username: "testuser",
		Password: "testpass",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", loginReq)

	handler := auth.ExportLoginUser(db)
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

	handler := auth.ExportLogoutUser(db)
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

	handler := auth.ExportUpdateMe(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify password was changed
	var updatedUser models.User
	db.Where("username = ?", "testuser").First(&updatedUser)
	assert.True(t, auth.VerifyPassword("newpassword", updatedUser.Password))
	assert.False(t, auth.VerifyPassword("userpass", updatedUser.Password))

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

	handler := auth.ExportDeleteMe(db)
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

	handler := auth.ExportGetCurrentUser()
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
	router.Use(auth.ExportAuthMiddleware(db))
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

// Test additional auth functions and edge cases
func TestHashPassword(t *testing.T) {
	// Test normal password hashing
	password := "testpassword123"
	hash, err := auth.HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Test empty password
	emptyHash, err := auth.HashPassword("")
	assert.NoError(t, err)
	assert.NotEmpty(t, emptyHash)

	// Test very long password (bcrypt has 72-byte limit, so this should fail)
	longPassword := strings.Repeat("a", 100)
	longHash, err := auth.HashPassword(longPassword)
	assert.Error(t, err)
	assert.Empty(t, longHash)
	assert.Contains(t, err.Error(), "password length exceeds 72 bytes")

	// Test with password at the bcrypt limit (72 bytes - should work)
	limitPassword := strings.Repeat("a", 72)
	limitHash, err := auth.HashPassword(limitPassword)
	assert.NoError(t, err)
	assert.NotEmpty(t, limitHash)
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := auth.HashPassword(password)
	require.NoError(t, err)

	// Test correct password
	assert.True(t, auth.VerifyPassword(password, hash))

	// Test wrong password
	assert.False(t, auth.VerifyPassword("wrongpassword", hash))

	// Test empty password against hash
	assert.False(t, auth.VerifyPassword("", hash))

	// Test password against empty hash
	assert.False(t, auth.VerifyPassword(password, ""))

	// Test invalid hash format
	assert.False(t, auth.VerifyPassword(password, "invalid-hash"))
}

func TestGenerateRandomString(t *testing.T) {
	// Test different lengths
	lengths := []int{0, 1, 10, 32, 64, 100}

	for _, length := range lengths {
		result := auth.GenerateRandomString(length)
		assert.Len(t, result, length)

		if length > 0 {
			// Test that result contains only valid characters
			charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			for _, char := range result {
				assert.Contains(t, charset, string(char))
			}
		}
	}

	// Test uniqueness - generate multiple strings and ensure they're different
	strings := make(map[string]bool)
	for i := 0; i < 100; i++ {
		str := auth.GenerateRandomString(32)
		assert.False(t, strings[str], "Generated duplicate string: %s", str)
		strings[str] = true
	}
}

func TestCreateUserAuth(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test successful user creation
	err := auth.CreateUser("testuser", "password123", "test@example.com", "user", "group1", db)
	assert.NoError(t, err)

	// Verify user was created
	var user models.User
	result := db.Where("username = ?", "testuser").First(&user)
	require.NoError(t, result.Error)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "user", user.Role)
	assert.Equal(t, "group1", user.Groups)
	assert.Equal(t, "local", user.AuthMethod)
	assert.True(t, auth.VerifyPassword("password123", user.Password))

	// Test duplicate username
	err = auth.CreateUser("testuser", "password456", "test2@example.com", "admin", "group2", db)
	assert.Error(t, err)

	// Test with empty fields
	err = auth.CreateUser("", "", "", "", "", db)
	assert.NoError(t, err) // Should succeed but create user with empty fields

	// Test with special characters
	err = auth.CreateUser("user@domain.com", "p@ssw0rd!", "email@test.com", "admin", "group1,group2", db)
	assert.NoError(t, err)
}

func TestCreateInitialAdmin(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test creating initial admin when no users exist
	err := auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify admin user was created
	var user models.User
	result := db.Where("username = ?", "admin").First(&user)
	require.NoError(t, result.Error)
	assert.Equal(t, "admin", user.Username)
	assert.Equal(t, "admin", user.Role)
	assert.Equal(t, "local", user.AuthMethod)
	assert.NotEmpty(t, user.Password)

	// Test that it doesn't create another admin if one already exists
	initialPassword := user.Password
	err = auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify password didn't change
	var updatedUser models.User
	db.Where("username = ?", "admin").First(&updatedUser)
	assert.Equal(t, initialPassword, updatedUser.Password)

	// Test when regular user exists but no admin - should NOT create admin
	// because CreateInitialAdmin only works when database is completely empty
	db.Delete(&user)
	regularUser := models.User{
		Username:   "regular",
		Password:   "hashedpass",
		Email:      "regular@example.com",
		Role:       "user",
		AuthMethod: "local",
	}
	db.Create(&regularUser)

	err = auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify admin was NOT created (because regular user exists)
	var newAdmin models.User
	result = db.Where("username = ? AND role = ?", "admin", "admin").First(&newAdmin)
	assert.Error(t, result.Error) // Should be "record not found"
}

func TestSessionManagement(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")

	// Test creating session
	token := auth.GenerateRandomString(32)
	session := models.Session{
		Token:  token,
		UserID: user.ID,
	}
	result := db.Create(&session)
	require.NoError(t, result.Error)

	// Test finding session
	var foundSession models.Session
	result = db.Where("token = ?", token).First(&foundSession)
	require.NoError(t, result.Error)
	assert.Equal(t, user.ID, foundSession.UserID)

	// Test session cleanup (delete)
	result = db.Delete(&foundSession)
	require.NoError(t, result.Error)

	// Verify session was deleted
	var count int64
	db.Model(&models.Session{}).Where("token = ?", token).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAuthenticationEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test login with empty request (should return 400 Bad Request for validation error)
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", models.LoginRequest{})
	handler := auth.ExportLoginUser(db)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test register with invalid JSON
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", "invalid json")
	adminUser := SetupTestAdmin(t, db)
	AuthorizeContext(c, adminUser)
	registerHandler := auth.ExportRegisterUser(db)
	registerHandler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test update me with invalid JSON
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", "invalid json")
	user := SetupTestUser(t, db, "testuser")
	AuthorizeContext(c, user)
	updateHandler := auth.ExportUpdateMe(db)
	updateHandler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPasswordValidation(t *testing.T) {
	// Test various password scenarios
	testCases := []struct {
		password string
		valid    bool
	}{
		{"", true},                // Empty password should hash successfully
		{"short", true},           // Short password
		{"averagepassword", true}, // Average length
		{"verylongpasswordwithmanycharacters", true}, // Long password
		{"password with spaces", true},               // Password with spaces
		{"пароль", true},                             // Unicode password
		{"🔒🔑", true},                                 // Emoji password
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("password_%s", tc.password), func(t *testing.T) {
			hash, err := auth.HashPassword(tc.password)
			if tc.valid {
				assert.NoError(t, err)
				assert.NotEmpty(t, hash)
				assert.True(t, auth.VerifyPassword(tc.password, hash))
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCreateSession(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")

	// Test successful session creation
	token := "test-token-123"
	timeout := time.Now().Add(1 * time.Hour)

	err := auth.CreateSession(token, timeout, user, db)
	assert.NoError(t, err)

	// Verify session was created
	var session models.Session
	result := db.Where("token = ?", token).First(&session)
	require.NoError(t, result.Error)
	assert.Equal(t, user.ID, session.UserID)
	assert.Equal(t, token, session.Token)

	// Test duplicate token (should succeed - tokens can be reused)
	err = auth.CreateSession(token, timeout, user, db)
	assert.NoError(t, err)

	// Test with different user but same token (should succeed)
	user2 := SetupTestUser(t, db, "testuser2")
	err = auth.CreateSession(token, timeout, user2, db)
	assert.NoError(t, err)

	// Test with empty token
	err = auth.CreateSession("", timeout, user, db)
	assert.NoError(t, err) // Empty token should be allowed

	// Test with past timeout
	pastTimeout := time.Now().Add(-1 * time.Hour)
	err = auth.CreateSession("past-token", pastTimeout, user, db)
	assert.NoError(t, err) // Past timeout should be allowed (will be expired immediately)
}

func TestGenerateRandomStringEdgeCases(t *testing.T) {
	// Test zero length (should return empty string)
	result := auth.GenerateRandomString(0)
	assert.Empty(t, result)

	// Test very large length
	result = auth.GenerateRandomString(1000)
	assert.Len(t, result, 1000)

	// Test that multiple calls with same length produce different results
	results := make(map[string]bool)
	for i := 0; i < 50; i++ {
		str := auth.GenerateRandomString(16)
		assert.False(t, results[str], "Generated duplicate string: %s", str)
		results[str] = true
	}

	// Test character distribution (should only contain valid charset characters)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 10; i++ {
		str := auth.GenerateRandomString(100)
		for _, char := range str {
			assert.Contains(t, charset, string(char))
		}
	}
}

func TestCreateUserEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test with very long password (should trigger bcrypt error)
	longPassword := strings.Repeat("a", 100)
	err := auth.CreateUser("longpassuser", longPassword, "test@example.com", "user", "group1", db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password length exceeds 72 bytes")

	// Test with special characters in all fields
	err = auth.CreateUser("user@domain.com", "p@ssw0rd!", "email+test@domain.co.uk", "admin", "group1,group2,group3", db)
	assert.NoError(t, err)

	// Verify user was created with special characters
	var user models.User
	result := db.Where("username = ?", "user@domain.com").First(&user)
	require.NoError(t, result.Error)
	assert.Equal(t, "email+test@domain.co.uk", user.Email)
	assert.Equal(t, "group1,group2,group3", user.Groups)
	assert.Equal(t, "local", user.AuthMethod)

	// Test with nil/empty groups
	err = auth.CreateUser("emptyuser", "password", "empty@example.com", "user", "", db)
	assert.NoError(t, err)

	// Test with SQL injection attempts (should be safely handled by GORM)
	err = auth.CreateUser("'; DROP TABLE users; --", "password", "hack@example.com", "user", "group1", db)
	assert.NoError(t, err)

	// Verify the malicious username was stored safely
	var maliciousUser models.User
	result = db.Where("username = ?", "'; DROP TABLE users; --").First(&maliciousUser)
	require.NoError(t, result.Error)
	assert.Equal(t, "'; DROP TABLE users; --", maliciousUser.Username)
}

func TestSessionExpiration(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")

	// Create an expired session
	expiredToken := "expired-token"
	expiredTimeout := time.Now().Add(-1 * time.Hour)
	err := auth.CreateSession(expiredToken, expiredTimeout, user, db)
	require.NoError(t, err)

	// Create a valid session
	validToken := "valid-token"
	validTimeout := time.Now().Add(1 * time.Hour)
	err = auth.CreateSession(validToken, validTimeout, user, db)
	require.NoError(t, err)

	// Test middleware with expired token
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(auth.ExportAuthMiddleware(db))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test expired token - should return 401 and delete the session
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", expiredToken)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Verify expired session was deleted
	var count int64
	db.Model(&models.Session{}).Where("token = ?", expiredToken).Count(&count)
	assert.Equal(t, int64(0), count)

	// Test valid token - should work
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", validToken)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify valid session still exists
	db.Model(&models.Session{}).Where("token = ?", validToken).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestAuthMiddlewareEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	session := SetupSession(t, db, user)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(auth.ExportAuthMiddleware(db))
	router.GET("/protected", func(c *gin.Context) {
		userObj, exists := c.Get("user")
		if exists {
			assert.Equal(t, user.ID, userObj.(models.User).ID)
		}
		c.Status(http.StatusOK)
	})

	// Test with Bearer token prefix (should fall through to session token check)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code) // Bearer tokens without Keycloak should fail

	// Test with malformed Bearer token
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test with user already set in context (simulating Keycloak middleware)
	router2 := gin.New()
	router2.Use(func(c *gin.Context) {
		c.Set("user", user) // Simulate user already authenticated by Keycloak
		c.Next()
	})
	router2.Use(auth.ExportAuthMiddleware(db))
	router2.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/protected", nil)
	router2.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test with non-existent session token
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "non-existent-token")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test with empty authorization header
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegisterUserKeycloakScenarios(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)

	// Save original config
	originalKeycloakEnabled := models.ServerConfig.Keycloak.Enabled
	originalDisableLocalAuth := models.ServerConfig.Keycloak.DisableLocalAuth
	defer func() {
		models.ServerConfig.Keycloak.Enabled = originalKeycloakEnabled
		models.ServerConfig.Keycloak.DisableLocalAuth = originalDisableLocalAuth
	}()

	// Test: Registration when Keycloak is enabled and local auth is disabled
	models.ServerConfig.Keycloak.Enabled = true
	models.ServerConfig.Keycloak.DisableLocalAuth = true

	registerReq := models.RegisterRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
		Role:     "user",
		Groups:   "nil",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler := auth.ExportRegisterUser(db)
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Local registration is disabled")

	// Test: Registration when Keycloak is enabled but local auth is allowed
	models.ServerConfig.Keycloak.Enabled = true
	models.ServerConfig.Keycloak.DisableLocalAuth = false

	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Test: Registration when Keycloak is disabled
	models.ServerConfig.Keycloak.Enabled = false
	models.ServerConfig.Keycloak.DisableLocalAuth = false

	registerReq.Username = "anotheruser"
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRegisterUserPermissions(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	regularUser := SetupTestUser(t, db, "regularuser")
	globalAdminUser := SetupTestUser(t, db, "globaladmin")
	globalAdminUser.Role = "global_admin"
	db.Save(&globalAdminUser)

	registerReq := models.RegisterRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
		Role:     "user",
		Groups:   "nil",
	}

	// Test: Regular user cannot register others
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, regularUser)

	handler := auth.ExportRegisterUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Test: Global admin can register users
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, globalAdminUser)

	handler(c)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Test: No authentication
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRegisterUserValidation(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)

	// Test: Empty username (should fail validation)
	registerReq := models.RegisterRequest{
		Username: "",
		Password: "password123",
		Email:    "new@example.com",
		Role:     "user",
		Groups:   "nil",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler := auth.ExportRegisterUser(db)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code) // Empty username should fail validation

	// Test: Very long password (should fail due to bcrypt limit)
	registerReq = models.RegisterRequest{
		Username: "longpassuser",
		Password: strings.Repeat("a", 100),
		Email:    "long@example.com",
		Role:     "user",
		Groups:   "nil",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Test: Special characters in fields
	registerReq = models.RegisterRequest{
		Username: "user@domain.com",
		Password: "p@ssw0rd!",
		Email:    "email+test@domain.co.uk",
		Role:     "admin",
		Groups:   "group1,group2,group3",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/register", registerReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestLoginUserKeycloakScenarios(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Save original config
	originalKeycloakEnabled := models.ServerConfig.Keycloak.Enabled
	originalDisableLocalAuth := models.ServerConfig.Keycloak.DisableLocalAuth
	defer func() {
		models.ServerConfig.Keycloak.Enabled = originalKeycloakEnabled
		models.ServerConfig.Keycloak.DisableLocalAuth = originalDisableLocalAuth
	}()

	// Create a local user
	hashedPassword, err := auth.HashPassword("testpass")
	require.NoError(t, err)
	localUser := models.User{
		Username:   "localuser",
		Password:   hashedPassword,
		Email:      "local@example.com",
		Role:       "user",
		Groups:     "nil",
		AuthMethod: "local",
	}
	db.Create(&localUser)

	// Create a Keycloak user
	keycloakUser := models.User{
		Username:   "keycloakuser",
		Password:   hashedPassword,
		Email:      "keycloak@example.com",
		Role:       "user",
		Groups:     "nil",
		AuthMethod: "keycloak",
	}
	db.Create(&keycloakUser)

	// Test: Local user login when Keycloak is enabled but local auth is allowed
	models.ServerConfig.Keycloak.Enabled = true
	models.ServerConfig.Keycloak.DisableLocalAuth = false

	loginReq := models.LoginRequest{
		Username: "localuser",
		Password: "testpass",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", loginReq)

	handler := auth.ExportLoginUser(db)
	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Local user login when Keycloak is enabled and local auth is disabled
	models.ServerConfig.Keycloak.Enabled = true
	models.ServerConfig.Keycloak.DisableLocalAuth = true

	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Keycloak authentication required")

	// Test: Keycloak user login when local auth is disabled (should work)
	keycloakLoginReq := models.LoginRequest{
		Username: "keycloakuser",
		Password: "testpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", keycloakLoginReq)
	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Login when Keycloak is disabled
	models.ServerConfig.Keycloak.Enabled = false
	models.ServerConfig.Keycloak.DisableLocalAuth = false

	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoginUserValidation(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test: Empty username and password
	loginReq := models.LoginRequest{
		Username: "",
		Password: "",
	}
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", loginReq)

	handler := auth.ExportLoginUser(db)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Only username provided
	loginReq = models.LoginRequest{
		Username: "testuser",
		Password: "",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Only password provided
	loginReq = models.LoginRequest{
		Username: "",
		Password: "testpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test: SQL injection attempts
	loginReq = models.LoginRequest{
		Username: "'; DROP TABLE users; --",
		Password: "testpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code) // Should be safely handled

	// Test: Very long username
	loginReq = models.LoginRequest{
		Username: strings.Repeat("a", 1000),
		Password: "testpass",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test: Unicode characters
	loginReq = models.LoginRequest{
		Username: "пользователь",
		Password: "пароль",
	}
	c, w = CreateRequestContext("POST", "/api/v1/auth/login", loginReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateMeEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	user2 := SetupTestUser(t, db, "testuser2")

	// Test: Update username to existing username (should fail)
	updateReq := models.UpdateMeRequest{
		Username: user2.Username, // Try to use existing username
	}
	c, w := CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler := auth.ExportUpdateMe(db)
	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)

	// Test: Update username to same username (should work)
	updateReq = models.UpdateMeRequest{
		Username: "testuser",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: Update with very long password (should fail due to bcrypt limit)
	updateReq = models.UpdateMeRequest{
		Password: strings.Repeat("a", 100),
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Test: Update with empty fields (should not change anything)
	updateReq = models.UpdateMeRequest{
		Username: "",
		Password: "",
		Email:    "",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user data unchanged
	var updatedUser models.User
	db.Where("id = ?", user.ID).First(&updatedUser)
	assert.Equal(t, user.Username, updatedUser.Username)
	assert.Equal(t, user.Email, updatedUser.Email)

	// Test: Update all fields at once
	updateReq = models.UpdateMeRequest{
		Username: "newusername",
		Password: "newpassword",
		Email:    "new@example.com",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all fields were updated
	db.Where("id = ?", user.ID).First(&updatedUser)
	assert.Equal(t, "newusername", updatedUser.Username)
	assert.Equal(t, "new@example.com", updatedUser.Email)
	assert.True(t, auth.VerifyPassword("newpassword", updatedUser.Password))

	// Test: Update with special characters
	updateReq = models.UpdateMeRequest{
		Username: "user@domain.com",
		Email:    "email+test@domain.co.uk",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test: No authentication
	c, w = CreateRequestContext("PUT", "/api/v1/auth/me", updateReq)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteMeEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test: Delete last admin (should succeed - the logic doesn't prevent this)
	adminUser := SetupTestAdmin(t, db)

	c, w := CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	AuthorizeContext(c, adminUser)

	handler := auth.ExportDeleteMe(db)
	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify admin was deleted
	var count int64
	db.Model(&models.User{}).Where("id = ?", adminUser.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: Delete admin when multiple admins exist (should work)
	admin2 := SetupTestUser(t, db, "admin2")
	admin2.Role = "admin"
	db.Save(&admin2)

	c, w = CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify admin was deleted
	db.Model(&models.User{}).Where("id = ?", adminUser.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Verify sessions were also deleted
	db.Model(&models.Session{}).Where("user_id = ?", adminUser.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: Delete regular user (should work)
	regularUser := SetupTestUser(t, db, "regularuser")
	session := SetupSession(t, db, regularUser)

	c, w = CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	AuthorizeContext(c, regularUser)

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was deleted
	db.Model(&models.User{}).Where("id = ?", regularUser.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Verify sessions were also deleted
	db.Model(&models.Session{}).Where("token = ?", session.Token).Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: No authentication
	c, w = CreateRequestContext("DELETE", "/api/v1/auth/me", nil)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAttemptKeycloakAuth(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Save original config
	originalKeycloakEnabled := models.ServerConfig.Keycloak.Enabled
	defer func() {
		models.ServerConfig.Keycloak.Enabled = originalKeycloakEnabled
	}()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// Test: Keycloak disabled
	models.ServerConfig.Keycloak.Enabled = false
	result := auth.AttemptKeycloakAuth("some-token", db, c)
	assert.False(t, result)

	// Test: Keycloak enabled but invalid token
	models.ServerConfig.Keycloak.Enabled = true
	result = auth.AttemptKeycloakAuth("invalid-token", db, c)
	assert.False(t, result)

	// Test: Empty token
	result = auth.AttemptKeycloakAuth("", db, c)
	assert.False(t, result)

	// Test: Malformed JWT token
	result = auth.AttemptKeycloakAuth("not.a.jwt", db, c)
	assert.False(t, result)
}

func TestCreateInitialAdminEdgeCases(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test: Create initial admin when database is empty
	err := auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify admin was created
	var admin models.User
	result := db.Where("username = ? AND role = ?", "admin", "admin").First(&admin)
	require.NoError(t, result.Error)
	assert.Equal(t, "admin", admin.Username)
	assert.Equal(t, "admin", admin.Role)
	assert.Equal(t, "local", admin.AuthMethod)
	assert.True(t, auth.VerifyPassword("admin", admin.Password))

	// Test: Try to create initial admin again (should not create duplicate)
	originalPassword := admin.Password
	err = auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify no duplicate was created and password didn't change
	var count int64
	db.Model(&models.User{}).Where("username = ? AND role = ?", "admin", "admin").Count(&count)
	assert.Equal(t, int64(1), count)

	var updatedAdmin models.User
	db.Where("username = ? AND role = ?", "admin", "admin").First(&updatedAdmin)
	assert.Equal(t, originalPassword, updatedAdmin.Password)

	// Test: Create initial admin when regular users exist (should not create admin)
	db.Delete(&admin) // Remove admin
	regularUser := SetupTestUser(t, db, "regularuser")

	err = auth.CreateInitialAdmin(db)
	assert.NoError(t, err)

	// Verify admin was NOT created
	db.Model(&models.User{}).Where("username = ? AND role = ?", "admin", "admin").Count(&count)
	assert.Equal(t, int64(0), count)

	// Verify regular user still exists
	db.Model(&models.User{}).Where("id = ?", regularUser.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestSetupAuthRoutes(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Test that SetupAuthRoutes doesn't panic
	assert.NotPanics(t, func() {
		auth.SetupAuthRoutes(router, db)
	})

	// Test that routes are properly registered by making requests
	adminUser := SetupTestAdmin(t, db)

	// Test register route
	registerReq := models.RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Role:     "user",
		Groups:   "nil",
	}

	w := httptest.NewRecorder()
	reqBody, _ := json.Marshal(registerReq)
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "admin-token") // This would need to be a real session token

	// Create a session for the admin user to test protected routes
	session := SetupSession(t, db, adminUser)
	req.Header.Set("Authorization", session.Token)

	router.ServeHTTP(w, req)
	// Should get a response (not 404), indicating route is registered
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}
