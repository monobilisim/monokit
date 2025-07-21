//go:build with_api

package auth

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/monobilisim/monokit/common/api/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Type aliases for commonly used types from models package
type (
	User            = models.User
	LoginRequest    = models.LoginRequest
	LoginResponse   = models.LoginResponse
	RegisterRequest = models.RegisterRequest
	UserResponse    = models.UserResponse
	ErrorResponse   = models.ErrorResponse
	KeycloakConfig  = models.KeycloakConfig
	DBTX            = models.DBTX
	Session         = models.Session
	UpdateMeRequest = models.UpdateMeRequest
)

// Variable aliases
var (
	ServerConfig = &models.ServerConfig
)

// Export functions for testing
func ExportRegisterUser(db *gorm.DB) gin.HandlerFunc {
	return registerUser(db)
}

func ExportLoginUser(db *gorm.DB) gin.HandlerFunc {
	return loginUser(db)
}

func ExportLogoutUser(db *gorm.DB) gin.HandlerFunc {
	return logoutUser(db)
}

func ExportUpdateMe(db *gorm.DB) gin.HandlerFunc {
	return updateMe(db)
}

func ExportDeleteMe(db *gorm.DB) gin.HandlerFunc {
	return deleteMe(db)
}

func ExportGetCurrentUser() gin.HandlerFunc {
	return getCurrentUser()
}

func ExportAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return AuthMiddleware(db)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CreateUser(username, password, email, role, groups, inventory string, db DBTX) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	// Mark newly created users as local authentication users.
	user := User{
		Username:    username,
		Password:    hashedPassword,
		Email:       email,
		Role:        role,
		Groups:      groups,
		Inventories: inventory,
		AuthMethod:  "local",
	}

	return db.Create(&user).Error
}

func CreateSession(token string, timeout time.Time, user User, db *gorm.DB) error {
	session := Session{Token: token, Timeout: timeout, User: user}
	return db.Create(&session).Error
}

func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		// Use crypto/rand instead of math/rand
		randomBytes := make([]byte, 1)
		if _, err := rand.Read(randomBytes); err != nil {
			// Fallback to less secure method if crypto/rand fails
			randomBytes[0] = byte(time.Now().UnixNano() % int64(len(charset)))
		}
		b[i] = charset[int(randomBytes[0])%len(charset)]
	}
	return string(b)
}

// @Summary Register user
// @Description Register a new user (admin only)
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "User registration info"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /auth/register [post]
func registerUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If local authentication is disabled, reject registration.
		if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Local registration is disabled. Please use Keycloak SSO login."})
			return
		}

		// Check for admin access
		userObj, exists := c.Get("user")
		if !exists || (userObj.(User).Role != "admin" && userObj.(User).Role != "global_admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user already exists
		var existingUser User
		if result := db.Where("username = ?", req.Username).First(&existingUser); result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		// Create new user
		err := CreateUser(req.Username, req.Password, req.Email, req.Role, req.Groups, req.Inventory, db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
	}
}

// @Summary Login user
// @Description Authenticate user and get token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/login [post]
func loginUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fmt.Printf("Login error: Invalid request format - %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("Login attempt for user: %s\n", req.Username)

		var user User
		if result := db.Where("username = ?", req.Username).First(&user); result.Error != nil {
			fmt.Printf("Login error: User not found - %v\n", result.Error)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		// If local authentication is disabled, only Keycloak users can login
		if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth && user.AuthMethod != "keycloak" {
			fmt.Printf("Login error: Local authentication disabled, user %s is not a Keycloak user\n", user.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
			return
		}

		fmt.Printf("User found: %s (role: %s)\n", user.Username, user.Role)

		// Verify password
		if !VerifyPassword(req.Password, user.Password) {
			fmt.Printf("Login error: Password verification failed for user %s\n", user.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		fmt.Printf("Password verified for user: %s\n", user.Username)

		// Generate session token
		token := GenerateRandomString(32)
		timeout := time.Now().Add(24 * time.Hour)

		// Create session
		if err := CreateSession(token, timeout, user, db); err != nil {
			fmt.Printf("Login error: Failed to create session - %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return
		}

		fmt.Printf("Login successful for user: %s\n", user.Username)

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"username": user.Username,
				"email":    user.Email,
				"role":     user.Role,
				"groups":   user.Groups,
			},
		})
	}
}

// @Summary Logout user
// @Description Invalidate user token
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /auth/logout [post]
func logoutUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
			return
		}

		// Delete session
		db.Where("token = ?", token).Delete(&Session{})

		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
	}
}

// createInitialAdmin creates an admin user if no users exist in the database
func createInitialAdmin(db *gorm.DB) error {
	// Check if any users exist
	var count int64
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		fmt.Printf("Error counting users: %v\n", err)
		return fmt.Errorf("failed to count users: %v", err)
	}

	fmt.Printf("Current user count: %d\n", count)

	// If users exist, don't create initial admin
	if count > 0 {
		fmt.Println("Users already exist, skipping initial admin creation")
		return nil
	}

	fmt.Println("No users found, creating initial admin user")

	// Create initial admin user
	initialAdmin := User{
		Username:    "admin",
		Email:       "admin@localhost",
		Role:        "admin",
		Groups:      "admins",
		Inventories: "default",
	}

	// Hash the default password "admin"
	hashedPassword, err := HashPassword("admin")
	if err != nil {
		fmt.Printf("Error hashing password: %v\n", err)
		return fmt.Errorf("failed to hash password: %v", err)
	}
	initialAdmin.Password = hashedPassword

	// Create the user
	if err := db.Create(&initialAdmin).Error; err != nil {
		fmt.Printf("Error creating initial admin: %v\n", err)
		return fmt.Errorf("failed to create initial admin: %v", err)
	}

	fmt.Println("Created initial admin user:")
	fmt.Println("  Username: admin")
	fmt.Println("  Password: admin")
	fmt.Println("  Role: admin")
	fmt.Println("  Email: admin@localhost")
	fmt.Println("Please change the password immediately!")

	return nil
}

// CreateInitialAdmin creates an admin user if no users exist in the database (exported for server package)
func CreateInitialAdmin(db *gorm.DB) error {
	return createInitialAdmin(db)
}

// @Summary Update own user details
// @Description Update your own username, password, or email
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param user body UpdateMeRequest true "User details to update"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /auth/me [put]
func updateMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current user
		currentUser, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		var req UpdateMeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user := currentUser.(User)

		// Update fields if provided
		if req.Username != "" {
			// Check if username is already taken
			var existingUser User
			if result := db.Where("username = ? AND id != ?", req.Username, user.ID).First(&existingUser); result.Error == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
			user.Username = req.Username
		}

		if req.Password != "" {
			hashedPassword, err := HashPassword(req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			user.Password = hashedPassword
		}

		if req.Email != "" {
			user.Email = req.Email
		}

		// Save changes
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
	}
}

// @Summary Delete own account
// @Description Delete your own user account
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} ErrorResponse
// @Router /auth/me [delete]
func deleteMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current user
		currentUser, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		user := currentUser.(User)

		// Don't allow deleting the last admin
		if user.Role == "admin" {
			var adminCount int64
			if err := db.Model(&User{}).Where("role = ?", "admin").Count(&adminCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin count"})
				return
			}

			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last admin account"})
				return
			}
		}

		// Delete user
		if err := db.Delete(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		// Delete associated sessions
		db.Where("user_id = ?", user.ID).Delete(&Session{})

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}

// @Summary Get current user
// @Description Get details of the currently authenticated user
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/me [get]
func getCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		currentUser := user.(User)
		c.JSON(http.StatusOK, gin.H{
			"username": currentUser.Username,
			"email":    currentUser.Email,
			"role":     currentUser.Role,
			"groups":   currentUser.Groups,
		})
	}
}

// SetupAuthRoutes sets up all authentication-related routes
func SetupAuthRoutes(r *gin.Engine, db *gorm.DB) {
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", registerUser(db))
		auth.POST("/login", loginUser(db))
		auth.POST("/logout", logoutUser(db))

		// Protected routes
		me := auth.Group("/me")
		me.Use(AuthMiddleware(db))
		{
			me.GET("", getCurrentUser())
			me.PUT("", updateMe(db))
			me.DELETE("", deleteMe(db))
		}
	}
}

// AuthMiddleware handles authentication for protected routes
func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is already set in context by Keycloak middleware
		if _, exists := c.Get("user"); exists {
			// User is already authenticated by Keycloak
			c.Next()
			return
		}

		// Get authorization header
		token := c.GetHeader("Authorization")
		if token == "" {
			// If Keycloak is available but no token is provided
			if ServerConfig.Keycloak.Enabled {
				// Redirect to login (let frontend handle Keycloak button)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check if token is a Bearer token (potentially Keycloak)
		if strings.HasPrefix(token, "Bearer ") {
			tokenString := strings.TrimPrefix(token, "Bearer ")

			// Attempt Keycloak authentication
			if ServerConfig.Keycloak.Enabled {
				authAttempt := AttemptKeycloakAuth(tokenString, db, c)
				if authAttempt {
					// Successfully authenticated with Keycloak
					c.Next()
					return
				}

				// If Keycloak auth failed and local auth is disabled
				if ServerConfig.Keycloak.DisableLocalAuth {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
					c.Abort()
					return
				}
			}

			// If we reach here, either Keycloak is not enabled or the token is not a valid Keycloak token
			// Fall through to check if it's a local token
		}

		// Handle normal session token authentication
		var session Session
		if err := db.Preload("User").Where("token = ?", token).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if time.Now().After(session.Timeout) {
			db.Delete(&session)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			c.Abort()
			return
		}

		// If Keycloak is enabled and local auth is disabled, allow only sessions tied to Keycloak users.
		if ServerConfig.Keycloak.Enabled && ServerConfig.Keycloak.DisableLocalAuth && session.User.AuthMethod != "keycloak" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Keycloak authentication required"})
			c.Abort()
			return
		}

		c.Set("user", session.User)
		c.Next()
	}
}

// AttemptKeycloakAuth is a helper function to manually validate a Keycloak token
func AttemptKeycloakAuth(tokenString string, db *gorm.DB, c *gin.Context) bool {
	// If Keycloak is not enabled, authentication fails
	if !ServerConfig.Keycloak.Enabled {
		return false
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &KeycloakClaims{}, func(token *jwt.Token) (interface{}, error) {
		if jwks == nil {
			return nil, fmt.Errorf("JWKS is not initialized")
		}
		return jwks.Keyfunc(token)
	})

	if err != nil || !token.Valid {
		fmt.Printf("Keycloak token validation failed: %v\n", err)
		return false
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*KeycloakClaims)
	if !ok {
		fmt.Printf("Failed to extract KeycloakClaims from token\n")
		return false
	}

	// Ensure issuer matches our Keycloak
	expectedIssuer := fmt.Sprintf("%s/realms/%s", ServerConfig.Keycloak.URL, ServerConfig.Keycloak.Realm)
	issuer := strings.TrimRight(claims.Issuer, "/")
	expectedIssuer = strings.TrimRight(expectedIssuer, "/")

	if issuer != expectedIssuer {
		fmt.Printf("Token issuer does not match expected issuer\n")
		fmt.Printf("Expected: %s, Got: %s\n", expectedIssuer, issuer)
		return false
	}

	// Token is valid, sync the user
	user, err := SyncKeycloakUser(db, claims)
	if err != nil {
		fmt.Printf("Failed to sync Keycloak user: %v\n", err)
		return false
	}

	// Set the user in the context and continue
	c.Set("user", user)
	return true
}
