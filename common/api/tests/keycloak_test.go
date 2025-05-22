//go:build with_api

package tests

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
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

// generateTestRSAKey generates an RSA key for testing
func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, string) {
	// Generate a new RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "Failed to generate RSA key")

	// Encode the private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return privateKey, string(privateKeyPEM)
}

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

// SigningMethod and key used for tokens - can be switched between tests
var testSigningMethod jwt.SigningMethod = jwt.SigningMethodRS256
var testSigningKey interface{} = []byte("test-signing-key")
var testKeyID = "test-key-id"

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

	// Create a JWT token with current signing method
	token := jwt.NewWithClaims(testSigningMethod, claims)
	// Add a key ID
	token.Header["kid"] = testKeyID

	var tokenString string
	var err error

	if testSigningMethod == jwt.SigningMethodRS256 {
		// For RSA signing, use the private key from generateTestRSAKey
		privateKey, _ := generateTestRSAKey(t)
		tokenString, err = token.SignedString(privateKey)
	} else {
		// For other methods (like HS256), use the testSigningKey
		tokenString, err = token.SignedString(testSigningKey)
	}

	require.NoError(t, err)
	return tokenString
}

// Setup a mock JWKS for token validation
func setupMockJWKS(t *testing.T) {
	// Generate a test key
	privateKey, _ := generateTestRSAKey(t)

	// Create a JWK (JSON Web Key) from the public key
	publicKey := privateKey.Public()
	n := publicKey.(*rsa.PublicKey).N
	e := publicKey.(*rsa.PublicKey).E

	// Convert n and e to base64url encoded strings
	nBytes := n.Bytes()
	nBase64 := base64.RawURLEncoding.EncodeToString(nBytes)

	eBytes := make([]byte, 4)
	eBytes[0] = byte(e >> 24)
	eBytes[1] = byte(e >> 16)
	eBytes[2] = byte(e >> 8)
	eBytes[3] = byte(e)
	eBase64 := base64.RawURLEncoding.EncodeToString(eBytes[1:]) // Skip leading zero byte

	// Create a JSON string with our JWK
	jwkJSON := fmt.Sprintf(`{
		"keys": [{
			"kid": "test-key-id",
			"kty": "RSA",
			"n": "%s",
			"e": "%s",
			"use": "sig",
			"alg": "RS256"
		}]
	}`, nBase64, eBase64)

	var err error
	mockJWKS, err := keyfunc.NewJSON([]byte(jwkJSON))
	require.NoError(t, err)

	// Set both the export variable and the internal jwks variable
	common.ExportJWKS = mockJWKS
	common.SetTestJWKS(mockJWKS) // This will set the internal jwks variable
}

// TestTokenValidation tests our token validation approach
func TestTokenValidation(t *testing.T) {
	// Setup
	setupKeycloakConfig()

	// Save original settings
	origSigningMethod := testSigningMethod
	origSigningKey := testSigningKey
	defer func() {
		testSigningMethod = origSigningMethod
		testSigningKey = origSigningKey
	}()

	// Use HS256 for simpler testing
	testSigningMethod = jwt.SigningMethodHS256
	testSigningKey = []byte("test-signing-key")

	// Create a token
	expectedIssuer := common.ServerConfig.Keycloak.URL + "/realms/" + common.ServerConfig.Keycloak.Realm
	tokenString := createMockToken(t, "test-user", "user", expectedIssuer)

	// Validate the token using our test key directly
	token, err := jwt.ParseWithClaims(tokenString, &common.KeycloakClaims{}, func(token *jwt.Token) (interface{}, error) {
		return testSigningKey, nil
	})

	// Check if validation works
	assert.NoError(t, err, "Token validation should succeed")
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

	// Set up the mock JWKS for validation
	setupMockJWKS(t)

	// Save original settings
	origSigningMethod := testSigningMethod
	origSigningKey := testSigningKey
	defer func() {
		testSigningMethod = origSigningMethod
		testSigningKey = origSigningKey
	}()

	// Switch to HS256 for this test
	testSigningMethod = jwt.SigningMethodHS256
	testSigningKey = []byte("test-signing-key")

	// Override key function to use our simple key
	originalKeyFunc := common.ExportKeyFunc
	defer func() { common.ExportKeyFunc = originalKeyFunc }()

	common.ExportKeyFunc = func(token *jwt.Token) (interface{}, error) {
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

	// Set up the mock JWKS for validation
	setupMockJWKS(t)

	// Save original settings
	origSigningMethod := testSigningMethod
	origSigningKey := testSigningKey
	defer func() {
		testSigningMethod = origSigningMethod
		testSigningKey = origSigningKey
	}()

	// Switch to HS256 for this test
	testSigningMethod = jwt.SigningMethodHS256
	testSigningKey = []byte("test-signing-key")

	// Override key function to use our simple key
	originalKeyFunc := common.ExportKeyFunc
	defer func() { common.ExportKeyFunc = originalKeyFunc }()

	common.ExportKeyFunc = func(token *jwt.Token) (interface{}, error) {
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
