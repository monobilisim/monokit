//go:build with_api

package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	_ = SetupTestSession(t, db, user, "valid_token_123")

	c, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", "valid_token_123")

	// Create middleware and test
	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	// Should not abort and should set user in context
	assert.False(t, c.IsAborted())
	contextUser, exists := c.Get("user")
	assert.True(t, exists)
	assert.Equal(t, user.Username, contextUser.(models.User).Username)
}

func TestAuthMiddleware_BearerToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	_ = SetupTestSession(t, db, user, "bearer_token_456")

	c, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", "Bearer bearer_token_456")

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.False(t, c.IsAborted())
	contextUser, exists := c.Get("user")
	assert.True(t, exists)
	assert.Equal(t, user.Username, contextUser.(models.User).Username)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", "invalid_token")

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/test", nil)
	// No Authorization header set

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ExpiredSession(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")

	// Create expired session
	expiredSession := models.Session{
		Token:   "expired_token",
		Timeout: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		UserID:  user.ID,
		User:    user,
	}
	db.Create(&expiredSession)

	c, w := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", "expired_token")

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Verify session was deleted
	var sessionCount int64
	db.Model(&models.Session{}).Where("token = ?", "expired_token").Count(&sessionCount)
	assert.Equal(t, int64(0), sessionCount)
}

func TestAuthMiddleware_UserAlreadyInContext(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "keycloak_user")

	c, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	// Simulate Keycloak middleware already setting user
	c.Set("user", user)

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	// Should not abort and should preserve existing user
	assert.False(t, c.IsAborted())
	contextUser, exists := c.Get("user")
	assert.True(t, exists)
	assert.Equal(t, user.Username, contextUser.(models.User).Username)
}

func TestAuthMiddleware_WithKeycloakEnabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Enable Keycloak in config for this test
	originalKeycloakEnabled := models.ServerConfig.Keycloak.Enabled
	models.ServerConfig.Keycloak.Enabled = true
	defer func() {
		models.ServerConfig.Keycloak.Enabled = originalKeycloakEnabled
	}()

	c, w := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid_jwt_token")

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	// Should attempt Keycloak auth and fallback to local auth
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHostAuthMiddleware_ValidHostKey(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "testhost")
	hostKey := SetupTestHostKey(t, db, host, "valid_host_key_123")

	// Verify the host key was actually created
	var foundKey models.HostKey
	err := db.Where("token = ?", "valid_host_key_123").First(&foundKey).Error
	require.NoError(t, err, "Host key should be found in database")
	assert.Equal(t, hostKey.Token, foundKey.Token)
	assert.Equal(t, hostKey.HostName, foundKey.HostName)

	c, _ := CreateRequestContext("GET", "/api/v1/host/config", nil)
	c.Request.Header.Set("Authorization", "valid_host_key_123")

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.False(t, c.IsAborted())
	contextHostname, exists := c.Get("hostname")
	assert.True(t, exists)
	assert.Equal(t, host.Name, contextHostname.(string))
}

func TestHostAuthMiddleware_InvalidHostKey(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/host/config", nil)
	c.Request.Header.Set("Authorization", "invalid_host_key")

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHostAuthMiddleware_MissingHostKey(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/host/config", nil)
	// No Authorization header

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHostAuthMiddleware_OrphanedHostKey(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a host key that references a non-existent host
	// This simulates a scenario where the host was deleted but the key wasn't cleaned up
	orphanedKey := models.HostKey{
		Token:    "orphaned_host_key",
		HostName: "non_existent_host", // This host doesn't exist
	}
	db.Create(&orphanedKey)

	c, w := CreateRequestContext("GET", "/api/v1/host/config", nil)
	c.Request.Header.Set("Authorization", "orphaned_host_key")

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHostAuthMiddleware_DeletedHost(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "deleted_host")
	_ = SetupTestHostKey(t, db, host, "valid_key_deleted_host")

	// Verify the host and key were created
	var foundHost models.Host
	err := db.Where("name = ?", "deleted_host").First(&foundHost).Error
	require.NoError(t, err, "Host should exist before deletion")

	var foundKey models.HostKey
	err = db.Where("token = ?", "valid_key_deleted_host").First(&foundKey).Error
	require.NoError(t, err, "Host key should exist")

	// Delete the host (soft delete) - this should make the host key invalid
	db.Delete(&host)

	// Verify the host is soft-deleted (should not be found in normal queries)
	err = db.Where("name = ?", "deleted_host").First(&foundHost).Error
	assert.Error(t, err, "Host should not be found after soft deletion")

	c, w := CreateRequestContext("GET", "/api/v1/host/config", nil)
	c.Request.Header.Set("Authorization", "valid_key_deleted_host")

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGenerateTokenMiddleware(t *testing.T) {
	token1 := server.ExportGenerateToken()
	token2 := server.ExportGenerateToken()

	// Tokens should be non-empty
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)

	// Tokens should be different
	assert.NotEqual(t, token1, token2)

	// Tokens should be hex encoded (64 characters for 32 bytes)
	assert.Len(t, token1, 64)
	assert.Len(t, token2, 64)

	// Tokens should only contain hex characters
	assert.Regexp(t, "^[0-9a-f]+$", token1)
	assert.Regexp(t, "^[0-9a-f]+$", token2)
}

func TestAuthMiddleware_ConcurrentSessions(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "concurrent_user")
	_ = SetupTestSession(t, db, user, "concurrent_token_1")
	_ = SetupTestSession(t, db, user, "concurrent_token_2")

	// Test first session
	c1, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c1.Request.Header.Set("Authorization", "concurrent_token_1")

	middleware := server.ExportAuthMiddleware(db)
	middleware(c1)

	assert.False(t, c1.IsAborted())

	// Test second session
	c2, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c2.Request.Header.Set("Authorization", "concurrent_token_2")

	middleware(c2)

	assert.False(t, c2.IsAborted())
}

func TestAuthMiddleware_LongToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	longToken := strings.Repeat("a", 1000) // Very long token
	_ = SetupTestSession(t, db, user, longToken)

	c, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", longToken)

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.False(t, c.IsAborted())
	contextUser, exists := c.Get("user")
	assert.True(t, exists)
	assert.Equal(t, user.Username, contextUser.(models.User).Username)
}

func TestAuthMiddleware_SpecialCharactersInToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "testuser")
	specialToken := "token-with-special-chars_123!@#$%"
	_ = SetupTestSession(t, db, user, specialToken)

	c, _ := CreateRequestContext("GET", "/api/v1/test", nil)
	c.Request.Header.Set("Authorization", specialToken)

	middleware := server.ExportAuthMiddleware(db)
	middleware(c)

	assert.False(t, c.IsAborted())
	contextUser, exists := c.Get("user")
	assert.True(t, exists)
	assert.Equal(t, user.Username, contextUser.(models.User).Username)
}

func TestHostAuthMiddleware_AutoDetectHostFromToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "auto-detect-host")
	_ = SetupTestHostKey(t, db, host, "auto_detect_key")

	c, _ := CreateRequestContext("GET", "/api/v1/host/config", nil)
	c.Request.Header.Set("Authorization", "auto_detect_key")

	middleware := server.ExportHostAuthMiddleware(db)
	middleware(c)

	assert.False(t, c.IsAborted())

	// Verify host was auto-detected and set in context
	contextHostname, exists := c.Get("hostname")
	assert.True(t, exists)
	assert.Equal(t, "auto-detect-host", contextHostname.(string))
}
