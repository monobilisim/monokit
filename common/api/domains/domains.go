//go:build with_api

package domains

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
    "github.com/monobilisim/monokit/common/api/auth"
	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"gorm.io/gorm"
)

// Type aliases for commonly used types from models package
type (
	Domain                      = models.Domain
	DomainUser                  = models.DomainUser
	User                        = models.User
	CreateDomainRequest         = models.CreateDomainRequest
	UpdateDomainRequest         = models.UpdateDomainRequest
	DomainResponse              = models.DomainResponse
	AssignUserToDomainRequest   = models.AssignUserToDomainRequest
	UpdateDomainUserRoleRequest = models.UpdateDomainUserRoleRequest
	DomainUserResponse          = models.DomainUserResponse
	UserResponse                = models.UserResponse
)

// @Summary Create new domain
// @Description Create a new domain (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain body CreateDomainRequest true "Domain information"
// @Success 201 {object} DomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /domains [post]
func CreateDomain(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin for creating domains
        user, exists := c.Get("user")
        if !exists || user.(User).Role != "global_admin" {
            c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
            return
        }

		var req CreateDomainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if domain already exists
		var existingDomain Domain
		if result := db.Where("name = ?", req.Name).First(&existingDomain); result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Domain name already exists"})
			return
		}

		// Create new domain
		domain := Domain{
			Name:             req.Name,
			Description:      req.Description,
			Settings:         req.Settings,
			Active:           true,
			RedmineProjectID: req.RedmineProjectID,
		}

		if err := db.Create(&domain).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
			return
		}

		// Create Cloudflare domains if provided and Cloudflare is enabled
		var cfDomains []models.CloudflareDomain
		if len(req.CloudflareDomains) > 0 {
			// Check if Cloudflare is enabled in server config
			serverConfig := models.ServerConfig
			if serverConfig.Cloudflare.Enabled {
				cfService := cloudflare.NewService(db, serverConfig.Cloudflare)

				for _, cfReq := range req.CloudflareDomains {
					cfDomain, err := cfService.CreateCloudflareDomain(domain.ID, cfReq)
					if err != nil {
						// Log error but don't fail domain creation
						// The domain was already created successfully
						c.JSON(http.StatusPartialContent, gin.H{
							"message":   "Domain created but some Cloudflare domains failed",
							"domain_id": domain.ID,
							"error":     err.Error(),
						})
						return
					}
					cfDomains = append(cfDomains, *cfDomain)
				}
			}
		}

		response := DomainResponse{
			ID:               domain.ID,
			Name:             domain.Name,
			Description:      domain.Description,
			Settings:         domain.Settings,
			Active:           domain.Active,
			RedmineProjectID: domain.RedmineProjectID,
			CreatedAt:        domain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:        domain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusCreated, response)
	}
}

// @Summary Get all domains
// @Description Get list of all domains (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {array} DomainResponse
// @Failure 403 {object} map[string]string
// @Router /domains [get]
func GetAllDomains(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Only global admins are allowed to list all domains
        user, exists := c.Get("user")
        if !exists || user.(User).Role != "global_admin" {
            c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
            return
        }

        var domains []Domain
        if err := db.Find(&domains).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domains"})
			return
		}

		var responses []DomainResponse
		for _, domain := range domains {
			responses = append(responses, DomainResponse{
				ID:               domain.ID,
				Name:             domain.Name,
				Description:      domain.Description,
				Settings:         domain.Settings,
				Active:           domain.Active,
				RedmineProjectID: domain.RedmineProjectID,
				CreatedAt:        domain.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:        domain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		c.JSON(http.StatusOK, responses)
	}
}

// @Summary Get domain by ID
// @Description Get specific domain information (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Success 200 {object} DomainResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{id} [get]
func GetDomainByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin for reading domains in this handler
        user, exists := c.Get("user")
        if !exists || user.(User).Role != "global_admin" {
            c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
            return
        }

		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		var domain Domain
		if err := db.First(&domain, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		response := DomainResponse{
			ID:               domain.ID,
			Name:             domain.Name,
			Description:      domain.Description,
			Settings:         domain.Settings,
			Active:           domain.Active,
			RedmineProjectID: domain.RedmineProjectID,
			CreatedAt:        domain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:        domain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Update domain
// @Description Update domain information (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Param domain body UpdateDomainRequest true "Domain update information"
// @Success 200 {object} DomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{id} [put]
func UpdateDomain(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Allow global admin or domain admin for this domain
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            return
        }

		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

        if user.(User).Role != "global_admin" {
            if !auth.HasDomainAdminAccess(c, uint(id)) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
                return
            }
        }

		var req UpdateDomainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var domain Domain
		if err := db.First(&domain, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Update fields if provided
		if req.Name != "" {
			// Check if new name conflicts with existing domain
			var existingDomain Domain
			if result := db.Where("name = ? AND id != ?", req.Name, domain.ID).First(&existingDomain); result.Error == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Domain name already exists"})
				return
			}
			domain.Name = req.Name
		}
		if req.Description != "" {
			domain.Description = req.Description
		}
		if req.Settings != "" {
			domain.Settings = req.Settings
		}
		if req.Active != nil {
			domain.Active = *req.Active
		}
		if req.RedmineProjectID != "" {
			domain.RedmineProjectID = req.RedmineProjectID
		}

		if err := db.Save(&domain).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update domain"})
			return
		}

		response := DomainResponse{
			ID:               domain.ID,
			Name:             domain.Name,
			Description:      domain.Description,
			Settings:         domain.Settings,
			Active:           domain.Active,
			RedmineProjectID: domain.RedmineProjectID,
			CreatedAt:        domain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:        domain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Delete domain
// @Description Delete domain (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /domains/{id} [delete]
func DeleteDomain(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin to delete domain
        user, exists := c.Get("user")
        if !exists || user.(User).Role != "global_admin" {
            c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
            return
        }

		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		var domain Domain
		if err := db.First(&domain, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Check if domain has associated resources (users, hosts, etc.)
		var userCount int64
		db.Model(&DomainUser{}).Where("domain_id = ?", domain.ID).Count(&userCount)
		if userCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete domain with associated users"})
			return
		}

		// Delete the domain
		if err := db.Delete(&domain).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Domain deleted successfully"})
	}
}

// @Summary Assign user to domain
// @Description Assign a user to a domain with a specific role (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Param assignment body AssignUserToDomainRequest true "User assignment information"
// @Success 201 {object} DomainUserResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /domains/{id}/users [post]
func AssignUserToDomain(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin or domain admin for this domain
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            return
        }

		idStr := c.Param("id")
        domainID, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

        if user.(User).Role != "global_admin" {
            if !auth.HasDomainAdminAccess(c, uint(domainID)) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
                return
            }
        }

		var req AssignUserToDomainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate role
		if req.Role != "domain_admin" && req.Role != "domain_user" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be 'domain_admin' or 'domain_user'"})
			return
		}

		// Check if domain exists
		var domain Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Check if user exists
		var targetUser User
		if err := db.First(&targetUser, req.UserID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		// Check if user is already assigned to this domain
		var existingAssignment DomainUser
		if result := db.Where("domain_id = ? AND user_id = ?", domainID, req.UserID).First(&existingAssignment); result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already assigned to this domain"})
			return
		}

		// Create domain user assignment
		domainUser := DomainUser{
			DomainID: uint(domainID),
			UserID:   req.UserID,
			Role:     req.Role,
		}

		if err := db.Create(&domainUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign user to domain"})
			return
		}

		// Load the created assignment with user details
		if err := db.Preload("User").First(&domainUser, domainUser.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assignment details"})
			return
		}

		response := DomainUserResponse{
			ID:       domainUser.ID,
			DomainID: domainUser.DomainID,
			UserID:   domainUser.UserID,
			Role:     domainUser.Role,
			User: UserResponse{
				Username: domainUser.User.Username,
				Email:    domainUser.User.Email,
				Role:     domainUser.User.Role,
				Groups:   domainUser.User.Groups,
			},
		}

		c.JSON(http.StatusCreated, response)
	}
}

// @Summary Get domain users
// @Description Get all users assigned to a domain (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Success 200 {array} DomainUserResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{id}/users [get]
func GetDomainUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin or domain admin for this domain
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            return
        }

		idStr := c.Param("id")
        domainID, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

        if user.(User).Role != "global_admin" {
            if !auth.HasDomainAdminAccess(c, uint(domainID)) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
                return
            }
        }

		// Check if domain exists
		var domain Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		var domainUsers []DomainUser
		if err := db.Preload("User").Where("domain_id = ?", domainID).Find(&domainUsers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain users"})
			return
		}

		var responses []DomainUserResponse
		for _, domainUser := range domainUsers {
			responses = append(responses, DomainUserResponse{
				ID:       domainUser.ID,
				DomainID: domainUser.DomainID,
				UserID:   domainUser.UserID,
				Role:     domainUser.Role,
				User: UserResponse{
					Username: domainUser.User.Username,
					Email:    domainUser.User.Email,
					Role:     domainUser.User.Role,
					Groups:   domainUser.User.Groups,
				},
			})
		}

		c.JSON(http.StatusOK, responses)
	}
}

// @Summary Update domain user role
// @Description Update a user's role within a domain (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param user_id path int true "User ID"
// @Param role body UpdateDomainUserRoleRequest true "Role update information"
// @Success 200 {object} DomainUserResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/users/{user_id} [put]
func UpdateDomainUserRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin or domain admin for this domain
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            return
        }

        domainIDStr := c.Param("id")
        domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

        if user.(User).Role != "global_admin" {
            if !auth.HasDomainAdminAccess(c, uint(domainID)) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
                return
            }
        }

		userIDStr := c.Param("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var req UpdateDomainUserRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate role
		if req.Role != "domain_admin" && req.Role != "domain_user" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be 'domain_admin' or 'domain_user'"})
			return
		}

		// Find the domain user assignment
		var domainUser DomainUser
		if err := db.Where("domain_id = ? AND user_id = ?", domainID, userID).First(&domainUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User is not assigned to this domain"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find domain user assignment"})
			return
		}

		// Update the role
		domainUser.Role = req.Role
		if err := db.Save(&domainUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
			return
		}

		// Load the updated assignment with user details
		if err := db.Preload("User").First(&domainUser, domainUser.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated assignment details"})
			return
		}

		response := DomainUserResponse{
			ID:       domainUser.ID,
			DomainID: domainUser.DomainID,
			UserID:   domainUser.UserID,
			Role:     domainUser.Role,
			User: UserResponse{
				Username: domainUser.User.Username,
				Email:    domainUser.User.Email,
				Role:     domainUser.User.Role,
				Groups:   domainUser.User.Groups,
			},
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Remove user from domain
// @Description Remove a user from a domain (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/users/{user_id} [delete]
func RemoveUserFromDomain(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Require global admin or domain admin for this domain
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            return
        }

        domainIDStr := c.Param("id")
        domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

        if user.(User).Role != "global_admin" {
            if !auth.HasDomainAdminAccess(c, uint(domainID)) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
                return
            }
        }

		userIDStr := c.Param("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Find the domain user assignment
		var domainUser DomainUser
		if err := db.Where("domain_id = ? AND user_id = ?", domainID, userID).First(&domainUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User is not assigned to this domain"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find domain user assignment"})
			return
		}

		// Remove the assignment
		if err := db.Delete(&domainUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user from domain"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User removed from domain successfully"})
	}
}

// @Summary Get user's domains
// @Description Get all domains a user has access to (global admin only)
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {array} DomainUserResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/{user_id}/domains [get]
func GetUserDomains(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Keep as global admin only
        user, exists := c.Get("user")
        if !exists || user.(User).Role != "global_admin" {
            c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
            return
        }

		userIDStr := c.Param("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Check if user exists
		var targetUser User
		if err := db.First(&targetUser, uint(userID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}

		var domainUsers []DomainUser
		if err := db.Preload("Domain").Where("user_id = ?", userID).Find(&domainUsers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user domains"})
			return
		}

		var responses []DomainUserResponse
		for _, domainUser := range domainUsers {
			responses = append(responses, DomainUserResponse{
				ID:       domainUser.ID,
				DomainID: domainUser.DomainID,
				UserID:   domainUser.UserID,
				Role:     domainUser.Role,
				User: UserResponse{
					Username: targetUser.Username,
					Email:    targetUser.Email,
					Role:     targetUser.Role,
					Groups:   targetUser.Groups,
				},
			})
		}

		c.JSON(http.StatusOK, responses)
	}
}
