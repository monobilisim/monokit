//go:build with_api

package tests

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RSA private key for testing
const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEApu6QCnJzJdsi44a0xv8FGIz8zOCnqVmIcmALHzTAy2xmsQbQ
hUpfa8UqELdGgJmBjkxDQ+4QL/MvCPoRbqjEz9HoMJxvuP7UVqvkjVV2hgVG4jS0
SYhzltQqtHgM0Ma5CgPzLn8FBZogONjCnANGBHdZvCvgUCPJQGBJEEzI2lkNIJ5K
eVGvYLzMRX5RQNqVsJnWfEzsG5QgHuQBfkuouG0L95PXnbwe8w8USjDDn5kXb3Jl
zFyuqDYnXPdELL7IYdEcGUvCDovp2uKzBNxTGnTL3q9jfQ+7XgJ1n4qS0xrRZH3F
ixcQ98CzB/8R+wIdIpb/jlUZKOnRVbwf/IpCdQIDAQABAoIBAQCYqScDunCKrjgu
J51KXHRNWb9Chnj8N0q4b98Bft07ZbRLzNJJIEGW0ZrQpUjZBYXJFGBBtgLY5Kon
zbkGkJahORpRi74aGriR3DwyQYGNpLHDfV2WBYw5FpXfXok+QsjoX+FApn9H2QiE
dKuyz0CviNYvSbYlPLbDKkVcHI3GYmk9Yt3SGR1YxqcUkzFhwTZRh9SbEwnz3XY+
H5NrZzA+Uq+/8dPOF9aqCCvCCGM4cq8i+kS4odAYwWiYhJVj+saNH7QkznJR2yQJ
ZAYxveLRShBB9JXbx3bGPzeMHDJftX9YQukPggZJ4JK4OfqYdHKLKAHFUFCGnLER
Kr3EIqwNAoGBANUPVIK3BMyVdtn3jaOKZ1uOYfbMhmet8Z7NI8mfoKNFn8hZGRKb
5xPOjCluUYsKhLAp8B7/a7Jr1K2zJUvPZB/qTHfGpQESQ5xZQCBe+leU3W9/lz8a
ZA7rgJQmcgIXmEGejwgHqvy2D0GStxoXgOVMrGCFwwU0wlEGo0Z6KbO3AoGBAMjm
kqpHAIffk5jMuA0CpJUUgwT4sTWnKmIrPkHPkJWvtvCHsqjK3ZBCcbLUXQCf2NRk
BR8UkKZh1qz3sCGt3ufLFaaIk2/iwCkZAQdP8r0UEGmJ/rYZK7G/g3ZzDvqmu52b
GNpCNdZ2ktcF9/q4n5QdWvlZafL4Ids4qGTGSLTDAoGAffnNLiTnxXgow/5jYuLp
ERNBzVbx0DPrHHJzYLMJqROgHmCG7hcXZMXHOQVxIkYmkQlBWHvDyQR3WtXyuNwc
GoxyzXRrHycY1/45rm1HIvd9Vj7i6J+eAldvFe1ZwXGkDsVsjH7cGgsoOs5SU+z0
yZntjx0DYTtWIvsHK6rLPYMCgYAvLzXHff2JOMpZMTR53+XNwA0jgIJbcs2FUDsb
7X6qxAv5/r1xOH+qOUFnwTWogFZQ9lWOyXJm/LqV8AKjqYpbRBLVw8PZpIrEu3bN
QtjfkABbCUsGD+LKzCZbLKyZZb7VuH4iJRHevPD9pSS7R/sKz3KLc56T+Zbi13A3
JMLh2wKBgQCR9TlKI7fgZB7OuIGPmzGAOEEFvLXhJDpXAzlJf0VOAEaXBuCj4xXq
V9xKgVEKlMZ71+18siYSm+cBv7QxQ3a4aFnqYfr/3AnhUgZkh5FOzBISUXYgk8jZ
s4wJ5iHsQ/39ej1Omv8rG2Nd+Qntx5JwbPwPPGvx6BgahJlZzQjVWg==
-----END RSA PRIVATE KEY-----`

// Mock Keycloak config used for testing
func setupKeycloakConfig() {
	common.ServerConfig.Keycloak = common.KeycloakConfig{
		Enabled:          true,
		URL:              "https://keycloak.example.com",
		Realm:            "monokit",
		ClientID:         "monokit-client",
		ClientSecret:     "test-secret",
		DisableLocalAuth: false,
	}
}

// Helper to create a mock JWT token for testing
func createMockToken(t *testing.T, username string, role string, issuer string) string {
	// Create claims with standard fields
	claims := &common.KeycloakClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   "test-subject",
			ID:        "test-id",
			Audience:  []string{"test-audience"},
		},
		PreferredUsername: username,
		Email:             username + "@example.com",
		EmailVerified:     true,
		Name:              "Test User",
		RealmAccess: map[string]interface{}{
			"roles": []interface{}{role},
		},
	}

	// For testing purposes, we use a simple signing key
	// In production, this would be handled by Keycloak's JWKS
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-signing-key"))
	require.NoError(t, err)
	return tokenString
}

// Setup a mock JWKS for token validation
func setupMockJWKS(t *testing.T) {
	// Initialize a mock JWKS that will always return our test key
	jwkJSON := `{
		"keys": [{
			"kid": "test-key-id",
			"kty": "oct",
			"k": "dGVzdC1zaWduaW5nLWtleQ"
		}]
	}`

	var err error
	mockJWKS, err := keyfunc.NewJSON([]byte(jwkJSON))
	require.NoError(t, err)

	// Set both the export variable and the internal jwks variable
	common.ExportJWKS = mockJWKS
	common.SetTestJWKS(mockJWKS) // This will set the internal jwks variable
}

// TestTokenValidation tests that our mock token validation mechanism works
func TestTokenValidation(t *testing.T) {
	// Setup
	setupKeycloakConfig()

	// Create a mock key function that always succeeds
	originalKeyFunc := common.ExportKeyFunc
	defer func() { common.ExportKeyFunc = originalKeyFunc }()

	common.ExportKeyFunc = func(token *jwt.Token) (interface{}, error) {
		// Always return the same test key for any token
		return []byte("test-signing-key"), nil
	}

	// Create a token
	expectedIssuer := common.ServerConfig.Keycloak.URL + "/realms/" + common.ServerConfig.Keycloak.Realm
	tokenString := createMockToken(t, "test-user", "user", expectedIssuer)

	// Validate the token using our mock key function
	token, err := jwt.ParseWithClaims(tokenString, &common.KeycloakClaims{}, common.ExportKeyFunc)

	// Check if validation works
	assert.NoError(t, err, "Token validation should succeed with mocked key function")
	assert.True(t, token.Valid, "Token should be valid")

	// Extract and verify claims
	if claims, ok := token.Claims.(*common.KeycloakClaims); ok {
		assert.Equal(t, "test-user", claims.PreferredUsername)
		assert.Equal(t, expectedIssuer, claims.Issuer)
	} else {
		t.Fatal("Failed to extract claims")
	}
}

func TestGenerateRandomState(t *testing.T) {
	// Test that random state is generated with correct length
	state, err := common.ExportGenerateRandomState()
	assert.NoError(t, err)
	assert.NotEmpty(t, state)

	// Decode and check length (should be 32 bytes)
	decoded, err := base64.URLEncoding.DecodeString(state)
	assert.NoError(t, err)
	assert.Equal(t, 32, len(decoded))

	// Ensure multiple calls generate different values
	state2, err := common.ExportGenerateRandomState()
	assert.NoError(t, err)
	assert.NotEqual(t, state, state2)
}

func TestHandleSSOLogin(t *testing.T) {
	// Setup
	setupKeycloakConfig()

	// Test: Successful login redirect
	c, w := CreateRequestContext("GET", "/api/v1/auth/sso/login", nil)

	// Set a mock origin header
	c.Request.Header.Set("Origin", "https://app.example.com")

	handler := common.ExportHandleSSOLogin()
	handler(c)

	// Assert redirect status
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)

	// Assert redirect location contains expected parameters
	location := w.Header().Get("Location")
	assert.Contains(t, location, common.ServerConfig.Keycloak.URL)
	assert.Contains(t, location, common.ServerConfig.Keycloak.Realm)
	assert.Contains(t, location, common.ServerConfig.Keycloak.ClientID)
	assert.Contains(t, location, "response_type=code")
	assert.Contains(t, location, "scope=openid+profile+email")
	assert.Contains(t, location, "redirect_uri=")
	assert.Contains(t, location, "state=")

	// Check cookies were set
	cookies := w.Result().Cookies()
	var foundStateCookie, foundRedirectCookie bool
	for _, cookie := range cookies {
		if cookie.Name == "sso_state" {
			foundStateCookie = true
		}
		if cookie.Name == "sso_redirect_uri" {
			foundRedirectCookie = true
			// Regardless of encoding, check that the cookie contains elements we expect
			assert.Contains(t, cookie.Value, "app.example.com")
			assert.Contains(t, cookie.Value, "sso")
			assert.Contains(t, cookie.Value, "callback")
		}
	}
	assert.True(t, foundStateCookie, "State cookie not found")
	assert.True(t, foundRedirectCookie, "Redirect URI cookie not found")
}

func TestHandleSSOCallback(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupKeycloakConfig()

	// Create a mocked key function for token validation
	originalKeyFunc := common.ExportKeyFunc
	defer func() { common.ExportKeyFunc = originalKeyFunc }()

	common.ExportKeyFunc = func(token *jwt.Token) (interface{}, error) {
		// Always return the same test key for any token
		return []byte("test-signing-key"), nil
	}

	// Mock exchange token code - this will be replaced in the test with a mock
	originalExchangeCodeForToken := common.ExportExchangeCodeForToken
	defer func() {
		common.ExportExchangeCodeForToken = originalExchangeCodeForToken
	}()

	expectedIssuer := common.ServerConfig.Keycloak.URL + "/realms/" + common.ServerConfig.Keycloak.Realm

	// Test 1: Error in callback
	c, w := CreateRequestContext("GET", "/api/v1/auth/sso/callback?error=access_denied&error_description=User+cancelled", nil)
	handler := common.ExportHandleSSOCallback(db)
	handler(c)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=User+cancelled")

	// Test 2: Invalid state parameter
	c, w = CreateRequestContext("GET", "/api/v1/auth/sso/callback?state=invalid-state&code=test-code", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sso_state", Value: "correct-state"})
	handler(c)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?error=Invalid+state+parameter")

	// Bypass the HTTP request to exchangeCodeForToken by directly testing syncKeycloakUser
	// which is what we really want to test in this callback

	// Test 3: Create a new user
	claims := &common.KeycloakClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    expectedIssuer,
		},
		PreferredUsername: "new-keycloak-user",
		Email:             "new-keycloak-user@example.com",
		RealmAccess: map[string]interface{}{
			"roles": []interface{}{"admin"},
		},
	}

	user, err := common.ExportSyncKeycloakUser(db, claims)
	assert.NoError(t, err)
	assert.Equal(t, "new-keycloak-user", user.Username)
	assert.Equal(t, "new-keycloak-user@example.com", user.Email)
	assert.Equal(t, "admin", user.Role)
	assert.Equal(t, "keycloak", user.AuthMethod)

	// Test 4: Update existing user
	claims.RealmAccess = map[string]interface{}{
		"roles": []interface{}{"user"},
	}

	user, err = common.ExportSyncKeycloakUser(db, claims)
	assert.NoError(t, err)
	assert.Equal(t, "user", user.Role) // Role should be downgraded from admin to user
}

// TestKeycloakAuthMiddleware tests the authentication middleware
func TestKeycloakAuthMiddleware(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupKeycloakConfig()

	// Create an admin user for fallback
	admin := SetupTestAdmin(t, db)
	_ = admin // avoid unused variable warning

	expectedIssuer := common.ServerConfig.Keycloak.URL + "/realms/" + common.ServerConfig.Keycloak.Realm

	// Create a test handler that will be called after the middleware
	testHandler := func(c *gin.Context) {
		user, exists := c.Get("user")
		if exists {
			c.JSON(http.StatusOK, gin.H{"username": user.(common.User).Username})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		}
	}

	// Create mocked verification function
	// This is an internal function that should return test keys for our tokens
	originalKeyFunc := common.ExportKeyFunc
	defer func() { common.ExportKeyFunc = originalKeyFunc }()

	common.ExportKeyFunc = func(token *jwt.Token) (interface{}, error) {
		// Always return the same test key for any token
		return []byte("test-signing-key"), nil
	}

	// Test 1: Valid token
	validToken := createMockToken(t, "keycloak-user", "user", expectedIssuer)
	c, w := CreateRequestContext("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+validToken)

	// Apply middleware
	middleware := common.ExportKeycloakAuthMiddleware(db)
	middleware(c)

	// Check if user was set in context
	testHandler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user exists in database
	var user common.User
	result := db.Where("username = ?", "keycloak-user").First(&user)
	assert.NoError(t, result.Error)
	assert.Equal(t, "keycloak", user.AuthMethod)

	// Test 2: Invalid token (wrong issuer)
	invalidToken := createMockToken(t, "invalid-user", "user", "https://wrong-issuer.com")
	c, w = CreateRequestContext("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+invalidToken)

	// Apply middleware
	middleware(c)

	// Check middleware behavior
	testHandler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test 3: No token but Keycloak enabled (should pass through)
	c, w = CreateRequestContext("GET", "/test", nil)
	middleware(c)
	testHandler(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test 4: Disable local auth and no token (should create default admin)
	common.ServerConfig.Keycloak.DisableLocalAuth = true
	c, w = CreateRequestContext("GET", "/test", nil)
	middleware(c)
	testHandler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test 5: Token in cookie
	common.ServerConfig.Keycloak.DisableLocalAuth = false
	c, w = CreateRequestContext("GET", "/test", nil)
	c.Request.AddCookie(&http.Cookie{Name: "token", Value: validToken})
	middleware(c)
	testHandler(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSyncKeycloakUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test 1: Create new user
	claims := &common.KeycloakClaims{
		PreferredUsername: "new-sync-user",
		Email:             "new-sync@example.com",
		RealmAccess: map[string]interface{}{
			"roles": []interface{}{"user"},
		},
	}

	user, err := common.ExportSyncKeycloakUser(db, claims)
	assert.NoError(t, err)
	assert.Equal(t, "new-sync-user", user.Username)
	assert.Equal(t, "new-sync@example.com", user.Email)
	assert.Equal(t, "user", user.Role)
	assert.Equal(t, "keycloak", user.AuthMethod)

	// Test 2: Update existing user
	claims.RealmAccess = map[string]interface{}{
		"roles": []interface{}{"admin"},
	}
	claims.Email = "updated-email@example.com"

	user, err = common.ExportSyncKeycloakUser(db, claims)
	assert.NoError(t, err)
	assert.Equal(t, "new-sync-user", user.Username)
	assert.Equal(t, "updated-email@example.com", user.Email)
	assert.Equal(t, "admin", user.Role)

	// Test 3: Handle email as username
	claims = &common.KeycloakClaims{
		PreferredUsername: "user.with@example.com",
		Email:             "user.with@example.com",
	}

	user, err = common.ExportSyncKeycloakUser(db, claims)
	assert.NoError(t, err)
	assert.Equal(t, "user.with", user.Username) // Username should be local part of email
}
