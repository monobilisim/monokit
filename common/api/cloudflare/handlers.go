//go:build with_api

package cloudflare

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"gorm.io/gorm"
)

// Type aliases for commonly used types from models package
type (
	User                          = models.User
	CloudflareDomain              = models.CloudflareDomain
	CreateCloudflareDomainRequest = models.CreateCloudflareDomainRequest
	UpdateCloudflareDomainRequest = models.UpdateCloudflareDomainRequest
	CloudflareDomainResponse      = models.CloudflareDomainResponse
)

// @Summary Create Cloudflare domain configuration
// @Description Create a new Cloudflare domain configuration for a domain (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param cloudflare_domain body CreateCloudflareDomainRequest true "Cloudflare domain configuration"
// @Success 201 {object} CloudflareDomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare [post]
func CreateCloudflareDomain(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		var req CreateCloudflareDomainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Create Cloudflare domain configuration
		cfDomain, err := cfService.CreateCloudflareDomain(uint(domainID), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		response := CloudflareDomainResponse{
			ID:           cfDomain.ID,
			DomainID:     cfDomain.DomainID,
			ZoneName:     cfDomain.ZoneName,
			ZoneID:       cfDomain.ZoneID,
			ProxyEnabled: cfDomain.ProxyEnabled,
			Active:       cfDomain.Active,
			CreatedAt:    cfDomain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    cfDomain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusCreated, response)
	}
}

// @Summary Get Cloudflare domain configurations
// @Description Get all Cloudflare domain configurations for a domain (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Success 200 {array} CloudflareDomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare [get]
func GetCloudflareDomains(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		cfDomains, err := cfService.GetCloudflareDomains(uint(domainID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var responses []CloudflareDomainResponse
		for _, cfDomain := range cfDomains {
			responses = append(responses, CloudflareDomainResponse{
				ID:           cfDomain.ID,
				DomainID:     cfDomain.DomainID,
				ZoneName:     cfDomain.ZoneName,
				ZoneID:       cfDomain.ZoneID,
				ProxyEnabled: cfDomain.ProxyEnabled,
				Active:       cfDomain.Active,
				CreatedAt:    cfDomain.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:    cfDomain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		c.JSON(http.StatusOK, responses)
	}
}

// @Summary Get Cloudflare domain configuration by ID
// @Description Get specific Cloudflare domain configuration (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param cf_domain_id path int true "Cloudflare Domain ID"
// @Success 200 {object} CloudflareDomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare/{cf_domain_id} [get]
func GetCloudflareDomain(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		cfDomainIDStr := c.Param("cf_domain_id")
		cfDomainID, err := strconv.ParseUint(cfDomainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Cloudflare domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		cfDomain, err := cfService.GetCloudflareDomain(uint(cfDomainID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found"})
			return
		}

		// Verify the Cloudflare domain belongs to the specified domain
		if cfDomain.DomainID != uint(domainID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found in specified domain"})
			return
		}

		response := CloudflareDomainResponse{
			ID:           cfDomain.ID,
			DomainID:     cfDomain.DomainID,
			ZoneName:     cfDomain.ZoneName,
			ZoneID:       cfDomain.ZoneID,
			ProxyEnabled: cfDomain.ProxyEnabled,
			Active:       cfDomain.Active,
			CreatedAt:    cfDomain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    cfDomain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Update Cloudflare domain configuration
// @Description Update Cloudflare domain configuration (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param cf_domain_id path int true "Cloudflare Domain ID"
// @Param cloudflare_domain body UpdateCloudflareDomainRequest true "Cloudflare domain update information"
// @Success 200 {object} CloudflareDomainResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare/{cf_domain_id} [put]
func UpdateCloudflareDomain(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		cfDomainIDStr := c.Param("cf_domain_id")
		cfDomainID, err := strconv.ParseUint(cfDomainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Cloudflare domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Verify Cloudflare domain exists and belongs to the domain
		existingCfDomain, err := cfService.GetCloudflareDomain(uint(cfDomainID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found"})
			return
		}

		if existingCfDomain.DomainID != uint(domainID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found in specified domain"})
			return
		}

		var req UpdateCloudflareDomainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cfDomain, err := cfService.UpdateCloudflareDomain(uint(cfDomainID), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		response := CloudflareDomainResponse{
			ID:           cfDomain.ID,
			DomainID:     cfDomain.DomainID,
			ZoneName:     cfDomain.ZoneName,
			ZoneID:       cfDomain.ZoneID,
			ProxyEnabled: cfDomain.ProxyEnabled,
			Active:       cfDomain.Active,
			CreatedAt:    cfDomain.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    cfDomain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Delete Cloudflare domain configuration
// @Description Delete Cloudflare domain configuration (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param cf_domain_id path int true "Cloudflare Domain ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare/{cf_domain_id} [delete]
func DeleteCloudflareDomain(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		cfDomainIDStr := c.Param("cf_domain_id")
		cfDomainID, err := strconv.ParseUint(cfDomainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Cloudflare domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Verify Cloudflare domain exists and belongs to the domain
		existingCfDomain, err := cfService.GetCloudflareDomain(uint(cfDomainID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found"})
			return
		}

		if existingCfDomain.DomainID != uint(domainID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found in specified domain"})
			return
		}

		if err := cfService.DeleteCloudflareDomain(uint(cfDomainID)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// @Summary Test Cloudflare connection
// @Description Test connection to Cloudflare API for a specific domain configuration (global admin only)
// @Tags cloudflare
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param cf_domain_id path int true "Cloudflare Domain ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /domains/{domain_id}/cloudflare/{cf_domain_id}/test [post]
func TestCloudflareConnection(db *gorm.DB, cfService *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for global admin access
		user, exists := c.Get("user")
		if !exists || user.(User).Role != "global_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Global admin access required"})
			return
		}

		domainIDStr := c.Param("domain_id")
		domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		cfDomainIDStr := c.Param("cf_domain_id")
		cfDomainID, err := strconv.ParseUint(cfDomainIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Cloudflare domain ID"})
			return
		}

		// Verify domain exists
		var domain models.Domain
		if err := db.First(&domain, uint(domainID)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain"})
			return
		}

		// Verify Cloudflare domain exists and belongs to the domain
		existingCfDomain, err := cfService.GetCloudflareDomain(uint(cfDomainID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found"})
			return
		}

		if existingCfDomain.DomainID != uint(domainID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cloudflare domain not found in specified domain"})
			return
		}

		if err := cfService.TestConnection(uint(cfDomainID)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Connection test failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Connection test successful"})
	}
}
