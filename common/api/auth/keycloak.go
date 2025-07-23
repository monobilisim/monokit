//go:build with_api

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Export variables and functions for testing
var (
	ExportGenerateRandomState    = generateRandomState
	ExportHandleSSOLogin         = handleSSOLogin
	ExportHandleSSOCallback      = handleSSOCallback
	ExportExchangeCodeForToken   = exchangeCodeForToken
	ExportSyncKeycloakUser       = SyncKeycloakUser
	ExportKeycloakAuthMiddleware = KeycloakAuthMiddleware
	ExportJWKS                   = (*keyfunc.JWKS)(nil)
	ExportKeyFunc                = func(token *jwt.Token) (interface{}, error) {
		if jwks == nil {
			return nil, fmt.Errorf("JWKS is not initialized")
		}
		return jwks.Keyfunc(token)
	}
)

// SetTestJWKS allows test code to set the jwks variable directly
func SetTestJWKS(testJWKS *keyfunc.JWKS) {
	jwks = testJWKS
}

/*
// KeycloakConfig holds the settings for Keycloak integration
type KeycloakConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	URL              string `mapstructure:"url"`
	Realm            string `mapstructure:"realm"`
	ClientID         string `mapstructure:"clientId"`
	ClientSecret     string `mapstructure:"clientSecret"`
	DisableLocalAuth bool   `mapstructure:"disableLocalAuth"`
}
*/

// Define claims structure to match Keycloak JWT
type KeycloakClaims struct {
	jwt.RegisteredClaims
	Name              string                 `json:"name,omitempty"`
	PreferredUsername string                 `json:"preferred_username,omitempty"`
	GivenName         string                 `json:"given_name,omitempty"`
	FamilyName        string                 `json:"family_name,omitempty"`
	Email             string                 `json:"email,omitempty"`
	EmailVerified     bool                   `json:"email_verified,omitempty"`
	RealmAccess       map[string]interface{} `json:"realm_access,omitempty"`
	ResourceAccess    map[string]interface{} `json:"resource_access,omitempty"`
}

var jwksURL string
var jwks *keyfunc.JWKS

// SetupKeycloakRoutes configures the Keycloak SSO endpoints
func SetupKeycloakRoutes(r *gin.Engine, db *gorm.DB) {
	if !ServerConfig.Keycloak.Enabled {
		return
	}

	// Set the JWKS URL for token validation
	jwksURL = fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs",
		ServerConfig.Keycloak.URL,
		ServerConfig.Keycloak.Realm)

	// Initialize JWKS
	initJWKS()

	auth := r.Group("/api/v1/auth")
	{
		// Endpoint to initiate SSO login
		auth.GET("/sso/login", handleSSOLogin())

		// Callback endpoint for Keycloak to return after authentication
		auth.GET("/sso/callback", handleSSOCallback(db))
	}
}

// initJWKS initializes the JSON Web Key Set for validating Keycloak tokens
func initJWKS() {
	// Create the JWKS from the remote URL
	options := keyfunc.Options{
		RefreshInterval: time.Hour, // Refresh JWKS every hour
		RefreshErrorHandler: func(err error) {
			log.Error().Err(err).Msg("Error refreshing JWKS")
		},
	}

	var err error
	jwks, err = keyfunc.Get(jwksURL, options)
	if err != nil {
		log.Error().Str("jwksURL", jwksURL).Err(err).Msg("Failed to get JWKS")
	}

	// Assign to exported variable for testing
	ExportJWKS = jwks
}

// generateRandomState creates a random state parameter for OAuth flow
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// handleSSOLogin redirects the user to Keycloak login page
func handleSSOLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate a random state parameter for security
		state, err := generateRandomState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state parameter"})
			return
		}

		// Create the OAuth 2.0 authorization endpoint URL
		authURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth",
			ServerConfig.Keycloak.URL,
			ServerConfig.Keycloak.Realm)

		// Build the query parameters
		params := url.Values{}
		params.Add("client_id", ServerConfig.Keycloak.ClientID)
		// Always generate an absolute redirect URI using the Origin header (or fallback to host)
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "https://" + c.Request.Host
		}
		redirectURI := origin + "/api/v1/auth/sso/callback"
		// Save the absolute redirect URI in a cookie for later retrieval in the callback
		c.SetCookie("sso_redirect_uri", redirectURI, 3600, "/", "", false, true)
		params.Add("redirect_uri", redirectURI)
		params.Add("response_type", "code")
		params.Add("scope", "openid profile email")
		params.Add("state", state)

		// Set a cookie with the state parameter to verify later
		c.SetCookie("sso_state", state, 3600, "/", "", false, true)

		// Redirect to Keycloak login
		c.Redirect(http.StatusTemporaryRedirect, authURL+"?"+params.Encode())
	}
}

// handleSSOCallback processes the callback from Keycloak
func handleSSOCallback(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the state and code from the request
		state := c.Query("state")
		code := c.Query("code")
		errParam := c.Query("error")
		errorDesc := c.Query("error_description")

		// Check if there was an error during authentication
		if errParam != "" {
			log.Error().Str("errParam", errParam).Str("errorDesc", errorDesc).Msg("Keycloak authentication error")
			c.Redirect(http.StatusTemporaryRedirect, "/login?error="+url.QueryEscape(errorDesc))
			return
		}

		// Verify the state parameter to prevent CSRF
		storedState, err := c.Cookie("sso_state")
		if err != nil || state != storedState {
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=Invalid+state+parameter")
			return
		}

		// Retrieve the originally used redirect URI from the cookie
		redirectCookie, errCookie := c.Cookie("sso_redirect_uri")
		if errCookie != nil || redirectCookie == "" {
			// Dynamically generate the absolute redirect URI using Origin or X-Forwarded-Proto if missing
			origin := c.Request.Header.Get("Origin")
			if origin == "" {
				proto := c.GetHeader("X-Forwarded-Proto")
				if proto == "" {
					if c.Request.TLS != nil {
						proto = "https"
					} else {
						proto = "http"
					}
				}
				origin = proto + "://" + c.Request.Host
			}
			redirectCookie = origin + "/api/v1/auth/sso/callback"
		}
		// Exchange the authorization code for an access token using the original redirect URI
		tokenData, err := exchangeCodeForToken(code, redirectCookie)
		if err != nil {
			log.Error().Err(err).Msg("Error exchanging code for token")
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=Failed+to+authenticate")
			return
		}

		// Parse the access token
		token, err := jwt.ParseWithClaims(tokenData["access_token"].(string), &KeycloakClaims{}, ExportKeyFunc)

		if err != nil {
			log.Error().Err(err).Msg("Error validating token")
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=Invalid+token")
			return
		}

		// Extract the claims
		if claims, ok := token.Claims.(*KeycloakClaims); ok && token.Valid {
			// Create or update user in database
			user, err := SyncKeycloakUser(db, claims)
			if err != nil {
				log.Error().Err(err).Msg("Error syncing user from Keycloak")
				c.Redirect(http.StatusTemporaryRedirect, "/login?error=Failed+to+create+user")
				return
			}

			// Create a session for the user
			sessionToken := GenerateRandomString(32)
			timeout := time.Now().Add(24 * time.Hour)

			if err := CreateSession(sessionToken, timeout, user, db); err != nil {
				log.Error().Err(err).Msg("Error creating session")
				c.Redirect(http.StatusTemporaryRedirect, "/login?error=Failed+to+create+session")
				return
			}

			// Redirect to the frontend with the token
			// In a real app, you'd use a more secure way to pass the token
			c.SetCookie("Authorization", sessionToken, 3600*24, "/", "", false, true)
			c.Redirect(http.StatusTemporaryRedirect, "/?Authorization="+sessionToken)
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/login?error=Invalid+token+claims")
		}
	}
}

// exchangeCodeForToken exchanges the authorization code for an access token
func exchangeCodeForToken(code, redirectURI string) (map[string]interface{}, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		ServerConfig.Keycloak.URL,
		ServerConfig.Keycloak.Realm)

	// Create form data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", ServerConfig.Keycloak.ClientID)
	data.Set("client_secret", ServerConfig.Keycloak.ClientSecret)
	data.Set("redirect_uri", redirectURI)

	// Make the HTTP request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// SyncKeycloakUser creates or updates a user in the local database from Keycloak claims
func SyncKeycloakUser(db *gorm.DB, claims *KeycloakClaims) (User, error) {
	var user User
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	// Split username at '@' to get the local part of the email address
	if strings.Contains(username, "@") {
		username = strings.Split(username, "@")[0]
	}

	// Check if user already exists
	result := db.Where("username = ?", username).First(&user)

	// Determine role from Keycloak claims
	role := "user" // Default role
	if realmRoles, ok := claims.RealmAccess["roles"].([]interface{}); ok {
		for _, r := range realmRoles {
			if roleStr, ok := r.(string); ok && roleStr == "admin" {
				role = "admin"
				break
			}
		}
	}

	if result.Error != nil {
		// User doesn't exist, create new user as Keycloak user
		user = User{
			Username:   username,
			Email:      claims.Email,
			Role:       role,
			Groups:     "nil",
			AuthMethod: "keycloak",
		}
		if err := db.Create(&user).Error; err != nil {
			return User{}, err
		}
	} else {
		// User exists, update fields that might have changed and mark as Keycloak user
		updates := map[string]interface{}{
			"email":       claims.Email,
			"role":        role,
			"auth_method": "keycloak",
		}
		if err := db.Model(&user).Updates(updates).Error; err != nil {
			return User{}, err
		}
	}

	return user, nil
}

// createOrGetDefaultAdminUser creates or retrieves a default admin user
func createOrGetDefaultAdminUser(db *gorm.DB) (User, error) {
	var user User
	result := db.Where("username = ?", "admin").First(&user)

	if result.Error != nil {
		// Create default admin user
		user = User{
			Username:   "admin",
			Email:      "admin@example.com",
			Role:       "admin",
			Groups:     "nil",
			AuthMethod: "local",
		}
		if err := db.Create(&user).Error; err != nil {
			return User{}, err
		}
	}

	return user, nil
}

// KeycloakAuthMiddleware is middleware that handles authentication with Keycloak
func KeycloakAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if not enabled
		if !ServerConfig.Keycloak.Enabled {
			c.Next()
			return
		}

		// Check for Authorization header; if missing, try retrieving token from cookie or query parameter
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			tokenCookie, err := c.Cookie("token")
			if err == nil && tokenCookie != "" {
				authHeader = "Bearer " + tokenCookie
			} else {
				tokenQuery := c.Query("token")
				if tokenQuery != "" {
					authHeader = "Bearer " + tokenQuery
				} else {
					if ServerConfig.Keycloak.DisableLocalAuth {
						if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
							c.Set("user", defaultUser)
						}
					}
					c.Next()
					return
				}
			}
		}

		// Extract token value and determine if it has Bearer prefix
		var tokenString string
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// If it doesn't have Bearer prefix, check if it might be a JWT token anyway
			tokenString = authHeader
			isDotFormat := strings.Count(tokenString, ".") == 2
			isLongEnough := len(tokenString) > 100
			if !(isDotFormat && isLongEnough) {
				if ServerConfig.Keycloak.DisableLocalAuth {
					if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
						c.Set("user", defaultUser)
					}
				}
				c.Next()
				return
			}
		}

		// Try parsing the token to check if it's actually a JWT
		_, parseErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return nil, fmt.Errorf("just checking format")
		})
		if parseErr != nil && !strings.Contains(parseErr.Error(), "just checking format") {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
				}
			}
			c.Next()
			return
		}

		// Parse the token with proper validation
		token, err := jwt.ParseWithClaims(tokenString, &KeycloakClaims{}, ExportKeyFunc)
		if err != nil {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
					c.Next()
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Keycloak token and could not create default user"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if !token.Valid {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
					c.Next()
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature and could not create default user"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(*KeycloakClaims)
		if !ok {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
					c.Next()
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format and could not create default user"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Ensure issuer matches our Keycloak
		expectedIssuer := fmt.Sprintf("%s/realms/%s", ServerConfig.Keycloak.URL, ServerConfig.Keycloak.Realm)
		issuer := strings.TrimRight(claims.Issuer, "/")
		expectedIssuer = strings.TrimRight(expectedIssuer, "/")
		if issuer != expectedIssuer {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
					c.Next()
					return
				}
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token issuer and could not create default user"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Sync the user
		user, err := SyncKeycloakUser(db, claims)
		if err != nil {
			if ServerConfig.Keycloak.DisableLocalAuth {
				if defaultUser, err := createOrGetDefaultAdminUser(db); err == nil {
					c.Set("user", defaultUser)
					c.Next()
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync user from Keycloak and could not create default user"})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync user from Keycloak"})
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func contains(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}
