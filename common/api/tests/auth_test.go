//go:build with_api

package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

	// Test very long password
	longPassword := string(make([]byte, 1000))
	for i := range longPassword {
		longPassword = longPassword[:i] + "a" + longPassword[i+1:]
	}
	longHash, err := auth.HashPassword(longPassword)
	assert.NoError(t, err)
	assert.NotEmpty(t, longHash)
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

	// Test when regular user exists but no admin
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

	// Verify admin was created
	var newAdmin models.User
	result = db.Where("username = ? AND role = ?", "admin", "admin").First(&newAdmin)
	require.NoError(t, result.Error)
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

	// Test login with empty request
	c, w := CreateRequestContext("POST", "/api/v1/auth/login", models.LoginRequest{})
	handler := auth.ExportLoginUser(db)
	handler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

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
