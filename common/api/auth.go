package common

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username       string `json:"username"`
	HashedPassword string `json:"hashedPassword"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	Groups         string `json:"groups"`
	Inventories    string `json:"inventories"`
}

type Session struct {
	gorm.Model
	Token   string    `json:"token"`
	Timeout time.Time `json:"timeout"`
	UserID  uint      `json:"userId"`
	User    User      `json:"user"`
}

type Group struct {
	gorm.Model
	Name  string `json:"name"`
	Hosts []Host `json:"hosts" gorm:"many2many:group_hosts;"`
	Users []User `json:"users" gorm:"many2many:group_users;"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CreateUser(username, password, email, role, groups, inventory string, db *gorm.DB) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	user := User{
		Username:       username,
		HashedPassword: hashedPassword,
		Email:          email,
		Role:           role,
		Groups:         groups,
		Inventories:    inventory,
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
		b[i] = charset[rand.Intn(len(charset))]
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
		// Check for admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "admin" {
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
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Find user
		var user User
		if result := db.Where("username = ?", req.Username).First(&user); result.Error != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Verify password
		if !VerifyPassword(req.Password, user.HashedPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate session token
		token := GenerateRandomString(32)
		timeout := time.Now().Add(24 * time.Hour)

		// Create session
		CreateSession(token, timeout, user, db)

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
		return fmt.Errorf("failed to count users: %v", err)
	}

	// If users exist, don't create initial admin
	if count > 0 {
		return nil
	}

	// Create initial admin user
	initialAdmin := User{
		Username: "admin",
		Email:    "admin@localhost",
		Role:     "admin",
		Groups:   "admins",
	}

	// Hash the default password "admin"
	hashedPassword, err := HashPassword("admin")
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}
	initialAdmin.HashedPassword = hashedPassword

	// Create the user
	if err := db.Create(&initialAdmin).Error; err != nil {
		return fmt.Errorf("failed to create initial admin: %v", err)
	}

	fmt.Println("Created initial admin user:")
	fmt.Println("  Username: admin")
	fmt.Println("  Password: admin")
	fmt.Println("Please change the password immediately!")

	return nil
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
// @Router /auth/me/update [put]
func updateMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		var req UpdateMeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get current user from database to ensure we have latest data
		var user User
		if err := db.First(&user, currentUser.(User).ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user data"})
			return
		}

		// Check if username is being changed and is available
		if req.Username != "" && req.Username != user.Username {
			var existingUser User
			if result := db.Where("username = ?", req.Username).First(&existingUser); result.Error == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
			user.Username = req.Username
		}

		// Update password if provided
		if req.Password != "" {
			hashedPassword, err := HashPassword(req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			user.HashedPassword = hashedPassword
		}

		// Update email if provided
		if req.Email != "" {
			user.Email = req.Email
		}

		// Save changes
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User details updated successfully"})
	}
}

// @Summary Delete own account
// @Description Delete your own account (not allowed if last admin)
// @Tags auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Router /auth/me [delete]
func deleteMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Authentication required"})
			return
		}

		// If user is admin, check if they're the last admin
		if currentUser.(User).Role == "admin" {
			var adminCount int64
			if err := db.Model(&User{}).Where("role = ?", "admin").Count(&adminCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin count"})
				return
			}

			if adminCount <= 1 {
				c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete the last admin account"})
				return
			}
		}

		// Delete the user
		user := currentUser.(User)
		if err := db.Delete(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Account deleted successfully"})
	}
}

// @Summary Get current user info
// @Description Get information about the currently logged in user
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		currentUser := user.(User)
		c.JSON(http.StatusOK, gin.H{
			"username": currentUser.Username,
			"role":     currentUser.Role,
		})
	}
}

func SetupAuthRoutes(r *gin.Engine, db *gorm.DB) {
	// Migrate auth-related schemas
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Session{})
	db.AutoMigrate(&Group{})
	db.AutoMigrate(&HostKey{})

	// Create initial admin user if no users exist
	if err := createInitialAdmin(db); err != nil {
		panic(err)
	}

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", loginUser(db))
		auth.POST("/logout", logoutUser(db))
		auth.PUT("/me/update", AuthMiddleware(db), updateMe(db))
		auth.POST("/register", AuthMiddleware(db), registerUser(db))
		auth.DELETE("/me", AuthMiddleware(db), deleteMe(db))
		auth.GET("/me", AuthMiddleware(db), getCurrentUser())
	}

	// Setup admin routes
	SetupAdminRoutes(r, db)
}

// Add this middleware function for protected routes
func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
			c.Abort()
			return
		}

		// First check if it's a host key
		var hostKey HostKey
		if result := db.Where("token = ?", token).First(&hostKey); result.Error == nil {
			// Host key is valid, check if the request is for this host
			hostName := c.Param("name")
			if hostName != hostKey.HostName {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid host access"})
				c.Abort()
				return
			}
			// Allow only PUT/POST operations on the host's own endpoint
			if c.Request.Method != "PUT" && c.Request.Method != "POST" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Operation not allowed"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		var session Session
		if result := db.Preload("User").Where("token = ?", token).First(&session); result.Error != nil {
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

		// Update session timeout
		session.Timeout = time.Now().Add(20 * time.Minute)
		db.Save(&session)

		// Check role-based access for hosts
		if session.User.Role != "admin" {
			// For non-admin users, check inventory and group access
			hostName := c.Param("name")

			// For GET hosts and GET /inventory, filter the response
			if c.FullPath() == "/api/v1/hosts" {
				var filteredHosts []Host
				db.Find(&hostsList)
				userGroups := strings.Split(session.User.Groups, ",")
				userInventories := strings.Split(session.User.Inventories, ",")

				for _, host := range hostsList {
					// Check if user has access to host's inventory
					hasInventoryAccess := false
					for _, inv := range userInventories {
						inv = strings.TrimSpace(inv)
						if inv == host.Inventory {
							hasInventoryAccess = true
							break
						}
					}
					if !hasInventoryAccess {
						continue
					}

					// Then check group access
					hostGroups := strings.Split(host.Groups, ",")
					for _, ug := range userGroups {
						ug = strings.TrimSpace(ug)
						for _, hg := range hostGroups {
							hg = strings.TrimSpace(hg)
							if ug == hg || (ug == "nil" && hg == "") {
								filteredHosts = append(filteredHosts, host)
								break
							}
						}
					}
				}

				c.Set("filteredHosts", filteredHosts)
			} else if c.FullPath() == "/api/v1/inventory" {
				// If user has specific inventory, only return that one
				if session.User.Inventories != "" {
					var results []struct {
						Inventory string
						Count     int
					}
					if err := db.Model(&Host{}).
						Select("inventory, count(*) as count").
						Where("inventory = ?", session.User.Inventories).
						Group("inventory").
						Find(&results).Error; err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventories"})
						c.Abort()
						return
					}

					inventories := make([]InventoryResponse, 0)
					if len(results) > 0 {
						inventories = append(inventories, InventoryResponse{
							Name:  results[0].Inventory,
							Hosts: results[0].Count,
						})
					}
					c.Set("filteredInventories", inventories)
				}
			} else if hostName != "" {
				// For endpoints with specific host
				var host Host
				if result := db.Where("name = ?", hostName).First(&host); result.Error == nil {
					// Check inventory access first
					if session.User.Inventories != "" && host.Inventory != session.User.Inventories {
						c.JSON(http.StatusForbidden, gin.H{"error": "No access to this host"})
						c.Abort()
						return
					}

					// Then check group access
					userGroups := strings.Split(session.User.Groups, ",")
					hostGroups := strings.Split(host.Groups, ",")

					hasAccess := false
					for _, ug := range userGroups {
						ug = strings.TrimSpace(ug)
						for _, hg := range hostGroups {
							hg = strings.TrimSpace(hg)
							if ug == hg || (ug == "nil" && hg == "") {
								hasAccess = true
								break
							}
						}
					}

					if !hasAccess {
						c.JSON(http.StatusForbidden, gin.H{"error": "No access to this host"})
						c.Abort()
						return
					}
				}
			}
		}

		// Add user to context
		c.Set("user", session.User)
		c.Next()
	}
}
