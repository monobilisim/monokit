//go:build with_api

package auth

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"gorm.io/gorm"
)

// Type aliases for domain-specific types from models package
type (
	Domain     = models.Domain
	DomainUser = models.DomainUser
)

// DomainContext holds domain-related information for the current request
type DomainContext struct {
	UserDomains       []DomainUser `json:"user_domains"`
	IsGlobalAdmin     bool         `json:"is_global_admin"`
	RequestedDomainID *uint        `json:"requested_domain_id,omitempty"`
}

// RequireDomainAccess middleware ensures user has access to the requested domain
// This middleware should be used after RequireAuth
func RequireDomainAccess(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		currentUser := user.(models.User)

        // Treat both global_admin and admin as having access to all domains for domain-scoped routes
        if currentUser.Role == "global_admin" || currentUser.Role == "admin" {
			domainContext := DomainContext{
				IsGlobalAdmin: true,
			}

            // If domain ID is specified in path, validate it exists
            domainIDStr := c.Param("domain_id")
            if domainIDStr == "" {
                // Also support routes that use ":id" for domain ID
                domainIDStr = c.Param("id")
            }
            if domainIDStr == "" {
                // Fallback: try to extract from full path
                if id := ExtractDomainFromPath(c.Request.URL.Path); id != nil {
                    domainIDStr = strconv.FormatUint(uint64(*id), 10)
                }
            }
            if domainIDStr != "" {
				domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
					c.Abort()
					return
				}

				var domain Domain
				if err := db.First(&domain, uint(domainID)).Error; err != nil {
					if err == gorm.ErrRecordNotFound {
						c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
						c.Abort()
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate domain"})
					c.Abort()
					return
				}

                domainIDUint := uint(domainID)
                domainContext.RequestedDomainID = &domainIDUint
			}

			c.Set("domain_context", domainContext)
			c.Next()
			return
		}

        // For non-global admins, load their domain associations
        var userDomains []DomainUser
        if err := db.Preload("Domain").Where("user_id = ?", currentUser.ID).Find(&userDomains).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user domains"})
            c.Abort()
            return
        }

		domainContext := DomainContext{
			UserDomains:   userDomains,
			IsGlobalAdmin: false,
		}

        // If domain ID is specified in path, ensure user has access to it
        domainIDStr := c.Param("domain_id")
        if domainIDStr == "" {
            // Also support routes that use ":id" for domain ID
            domainIDStr = c.Param("id")
        }
        if domainIDStr == "" {
            // Fallback: try to extract from full path
            if id := ExtractDomainFromPath(c.Request.URL.Path); id != nil {
                domainIDStr = strconv.FormatUint(uint64(*id), 10)
            }
        }
        if domainIDStr != "" {
			domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
				c.Abort()
				return
			}

			hasAccess := false
			for _, userDomain := range userDomains {
				if userDomain.DomainID == uint(domainID) {
					hasAccess = true
					break
				}
			}

			if !hasAccess {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this domain"})
				c.Abort()
				return
			}

			domainIDUint := uint(domainID)
			domainContext.RequestedDomainID = &domainIDUint
        }

		c.Set("domain_context", domainContext)
		c.Next()
	}
}

// RequireDomainAdmin middleware ensures user has admin access to the requested domain
// This middleware should be used after RequireDomainAccess
func RequireDomainAdmin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		domainContext, exists := c.Get("domain_context")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Domain context not found"})
			c.Abort()
			return
		}

		context := domainContext.(DomainContext)

		// Global admins always have admin access
		if context.IsGlobalAdmin {
			c.Next()
			return
		}

		// Check if user has domain admin role for the requested domain
        if context.RequestedDomainID == nil {
            // Try to infer from route params if not set yet (support both :domain_id and :id)
            domainIDStr := c.Param("domain_id")
            if domainIDStr == "" {
                domainIDStr = c.Param("id")
            }
            if domainIDStr == "" {
                if id := ExtractDomainFromPath(c.Request.URL.Path); id != nil {
                    domainID := *id
                    context.RequestedDomainID = &domainID
                    // Update context in request
                    c.Set("domain_context", context)
                }
            } else {
                if id64, err := strconv.ParseUint(domainIDStr, 10, 32); err == nil {
                    id := uint(id64)
                    context.RequestedDomainID = &id
                    c.Set("domain_context", context)
                }
            }
            if context.RequestedDomainID == nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Domain ID required for this operation"})
                c.Abort()
                return
            }
        }

		hasAdminAccess := false
		for _, userDomain := range context.UserDomains {
			if userDomain.DomainID == *context.RequestedDomainID && userDomain.Role == "domain_admin" {
				hasAdminAccess = true
				break
			}
		}

		if !hasAdminAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Domain admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserDomainIDs returns a list of domain IDs the current user has access to
func GetUserDomainIDs(c *gin.Context) []uint {
	domainContext, exists := c.Get("domain_context")
	if !exists {
		return []uint{}
	}

	context := domainContext.(DomainContext)

	// Global admins have access to all domains - return empty slice to indicate "all"
	if context.IsGlobalAdmin {
		return nil // nil indicates access to all domains
	}

	var domainIDs []uint
	for _, userDomain := range context.UserDomains {
		domainIDs = append(domainIDs, userDomain.DomainID)
	}

	return domainIDs
}

// HasDomainAdminAccess checks if the current user has admin access to a specific domain
func HasDomainAdminAccess(c *gin.Context, domainID uint) bool {
	domainContext, exists := c.Get("domain_context")
	if !exists {
		return false
	}

	context := domainContext.(DomainContext)

	// Global admins always have admin access
	if context.IsGlobalAdmin {
		return true
	}

	// Check if user has domain admin role for the specified domain
	for _, userDomain := range context.UserDomains {
		if userDomain.DomainID == domainID && userDomain.Role == "domain_admin" {
			return true
		}
	}

	return false
}

// FilterByDomainAccess adds domain filtering to a GORM query based on user's domain access
func FilterByDomainAccess(c *gin.Context, query *gorm.DB, domainField string) *gorm.DB {
	domainIDs := GetUserDomainIDs(c)

	// If domainIDs is nil, user is global admin with access to all domains
	if domainIDs == nil {
		return query
	}

	// If domainIDs is empty, user has no domain access
	if len(domainIDs) == 0 {
		// Return a query that will never match
		return query.Where("1 = 0")
	}

	// Filter by user's accessible domains
	return query.Where(domainField+" IN ?", domainIDs)
}

// ExtractDomainFromPath extracts domain ID from URL path parameters
// Supports paths like /domains/{id}/... or /api/v1/domains/{id}/...
func ExtractDomainFromPath(path string) *uint {
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for i, part := range parts {
		if part == "domains" && i+1 < len(parts) {
			if domainID, err := strconv.ParseUint(parts[i+1], 10, 32); err == nil {
				id := uint(domainID)
				return &id
			}
		}
	}

	return nil
}
