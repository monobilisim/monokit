package common

import (
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Groups   string `json:"groups"`
}

type User struct {
	gorm.Model
	Username       string `json:"username"`
	HashedPassword string `json:"hashedPassword"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	Groups         string `json:"groups"`
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

type UpdateUserGroupsRequest struct {
	Groups string `json:"groups" binding:"required"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CreateUser(username string, password string, email string, role string, groups string, db *gorm.DB) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}
	user := User{Username: username, HashedPassword: hashedPassword, Email: email, Role: role, Groups: groups}
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

func SetupAuthRoutes(r *gin.Engine, db *gorm.DB) {
	// Migrate auth-related schemas
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Session{})
	db.AutoMigrate(&Group{})

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", func(c *gin.Context) {
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
			err := CreateUser(req.Username, req.Password, req.Email, req.Role, req.Groups, db)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
		})

		auth.POST("/login", func(c *gin.Context) {
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
		})

		auth.POST("/logout", func(c *gin.Context) {
			token := c.GetHeader("Authorization")
			if token == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
				return
			}

			// Delete session
			db.Where("token = ?", token).Delete(&Session{})

			c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
		})

		auth.PUT("/users/:username/groups", func(c *gin.Context) {
			// Check if requester is admin
			token := c.GetHeader("Authorization")
			var session Session
			if result := db.Preload("User").Where("token = ?", token).First(&session); result.Error != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				return
			}

			if session.User.Role != "admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can update user groups"})
				return
			}

			username := c.Param("username")
			var req UpdateUserGroupsRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Update user groups
			result := db.Model(&User{}).Where("username = ?", username).Update("groups", req.Groups)
			if result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user groups"})
				return
			}
			if result.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "User groups updated successfully"})
		})
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
			// For non-admin users, check if the requested host is in their groups
			hostName := c.Param("name")

			// For GET /hostsList, filter the response
			if c.FullPath() == "/api/v1/hostsList" {
				var filteredHosts []Host
				db.Find(&hostsList)
				userGroups := strings.Split(session.User.Groups, ",")

				for _, host := range hostsList {
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
			} else if hostName != "" {
				// For endpoints with specific host
				var host Host
				if result := db.Where("name = ?", hostName).First(&host); result.Error == nil {
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
