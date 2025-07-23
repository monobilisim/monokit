//go:build with_api

package domains

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/redmine"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Note: Type aliases are already defined in domains.go

// RedmineProjectResponse represents a Redmine project response
type RedmineProjectResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

// RedmineIssueResponse represents a Redmine issue response
type RedmineIssueResponse struct {
	ID          int                    `json:"id"`
	Project     RedmineProjectResponse `json:"project"`
	Tracker     TrackerResponse        `json:"tracker"`
	Status      StatusResponse         `json:"status"`
	Priority    PriorityResponse       `json:"priority"`
	Author      RedmineUserResponse    `json:"author"`
	AssignedTo  *RedmineUserResponse   `json:"assigned_to,omitempty"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	StartDate   *string                `json:"start_date,omitempty"`
	DueDate     *string                `json:"due_date,omitempty"`
	DoneRatio   int                    `json:"done_ratio"`
	CreatedOn   string                 `json:"created_on"`
	UpdatedOn   string                 `json:"updated_on"`
}

// TrackerResponse represents a Redmine tracker response
type TrackerResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// StatusResponse represents a Redmine status response
type StatusResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PriorityResponse represents a Redmine priority response
type PriorityResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RedmineUserResponse represents a Redmine user response
type RedmineUserResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RedmineIssuesResponse represents the paginated issues response
type RedmineIssuesResponse struct {
	Issues     []RedmineIssueResponse `json:"issues"`
	TotalCount int                    `json:"total_count"`
	Offset     int                    `json:"offset"`
	Limit      int                    `json:"limit"`
}

// getRedmineClient creates a Redmine client from server configuration
func getRedmineClient() (*redmine.Client, error) {
	config := models.ServerConfig.Redmine
	if !config.Enabled {
		return nil, gin.Error{Err: gin.Error{}, Type: gin.ErrorTypePublic, Meta: "Redmine integration is disabled"}
	}
	return redmine.NewClient(config), nil
}

// getDomainRedmineProjectID returns the Redmine project identifier for a domain
func getDomainRedmineProjectID(domain *models.Domain) string {
	if domain.RedmineProjectID != "" {
		return domain.RedmineProjectID
	}
	// Default to domain name if no explicit project ID is set
	return domain.Name
}

// checkDomainAccess checks if the user has access to the specified domain
func checkDomainAccess(c *gin.Context, db *gorm.DB, domainID uint) (*models.Domain, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	currentUser := user.(models.User)

	// Global admins can access all domains
	if currentUser.Role == "global_admin" {
		var domain models.Domain
		if err := db.First(&domain, domainID).Error; err != nil {
			return nil, false
		}
		return &domain, true
	}

	// Regular users can only access domains they're assigned to
	var domainUser models.DomainUser
	if err := db.Preload("Domain").Where("user_id = ? AND domain_id = ?", currentUser.ID, domainID).First(&domainUser).Error; err != nil {
		return nil, false
	}

	return &domainUser.Domain, true
}

// @Summary Get Redmine project for domain
// @Description Get Redmine project information for a domain
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Success 200 {object} RedmineProjectResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /domains/{id}/redmine/project [get]
func GetDomainRedmineProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		domainID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		domain, hasAccess := checkDomainAccess(c, db, uint(domainID))
		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this domain"})
			return
		}

		client, err := getRedmineClient()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Redmine integration is not available"})
			return
		}

		projectID := getDomainRedmineProjectID(domain)
		project, err := client.GetProject(projectID)
		if err != nil {
			log.Error().
				Err(err).
				Str("domain", domain.Name).
				Str("project_id", projectID).
				Msg("Failed to get Redmine project")
			c.JSON(http.StatusNotFound, gin.H{"error": "Redmine project not found"})
			return
		}

		response := RedmineProjectResponse{
			ID:          project.ID,
			Name:        project.Name,
			Identifier:  project.Identifier,
			Description: project.Description,
			Status:      project.Status,
			CreatedOn:   project.CreatedOn,
			UpdatedOn:   project.UpdatedOn,
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Get Redmine issues for domain
// @Description Get Redmine issues for a domain's associated project
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Param limit query int false "Number of issues to return (default: 25, max: 100)"
// @Param offset query int false "Number of issues to skip (default: 0)"
// @Success 200 {object} RedmineIssuesResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /domains/{id}/redmine/issues [get]
func GetDomainRedmineIssues(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		domainID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		domain, hasAccess := checkDomainAccess(c, db, uint(domainID))
		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this domain"})
			return
		}

		// Parse query parameters
		limit := 25
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
				limit = parsedLimit
			}
		}

		offset := 0
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
				offset = parsedOffset
			}
		}

		client, err := getRedmineClient()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Redmine integration is not available"})
			return
		}

		projectID := getDomainRedmineProjectID(domain)
		issuesResp, err := client.GetProjectIssues(projectID, limit, offset)
		if err != nil {
			log.Error().
				Err(err).
				Str("domain", domain.Name).
				Str("project_id", projectID).
				Msg("Failed to get Redmine issues")

			// Check if it's a project not found error
			if strings.Contains(err.Error(), "404") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Redmine project not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Redmine issues"})
			}
			return
		}

		// Convert to response format
		issues := make([]RedmineIssueResponse, len(issuesResp.Issues))
		for i, issue := range issuesResp.Issues {
			var assignedTo *RedmineUserResponse
			if issue.AssignedTo != nil {
				assignedTo = &RedmineUserResponse{
					ID:   issue.AssignedTo.ID,
					Name: issue.AssignedTo.Name,
				}
			}

			issues[i] = RedmineIssueResponse{
				ID: issue.ID,
				Project: RedmineProjectResponse{
					ID:         issue.Project.ID,
					Name:       issue.Project.Name,
					Identifier: issue.Project.Identifier,
				},
				Tracker: TrackerResponse{
					ID:   issue.Tracker.ID,
					Name: issue.Tracker.Name,
				},
				Status: StatusResponse{
					ID:   issue.Status.ID,
					Name: issue.Status.Name,
				},
				Priority: PriorityResponse{
					ID:   issue.Priority.ID,
					Name: issue.Priority.Name,
				},
				Author: RedmineUserResponse{
					ID:   issue.Author.ID,
					Name: issue.Author.Name,
				},
				AssignedTo:  assignedTo,
				Subject:     issue.Subject,
				Description: issue.Description,
				StartDate:   issue.StartDate,
				DueDate:     issue.DueDate,
				DoneRatio:   issue.DoneRatio,
				CreatedOn:   issue.CreatedOn.Format("2006-01-02T15:04:05Z"),
				UpdatedOn:   issue.UpdatedOn.Format("2006-01-02T15:04:05Z"),
			}
		}

		response := RedmineIssuesResponse{
			Issues:     issues,
			TotalCount: issuesResp.TotalCount,
			Offset:     issuesResp.Offset,
			Limit:      issuesResp.Limit,
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Get specific Redmine issue for domain
// @Description Get a specific Redmine issue by ID for a domain's associated project
// @Tags domains
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Domain ID"
// @Param issue_id path int true "Issue ID"
// @Success 200 {object} RedmineIssueResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /domains/{id}/redmine/issues/{issue_id} [get]
func GetDomainRedmineIssue(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		domainID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
			return
		}

		issueID, err := strconv.Atoi(c.Param("issue_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid issue ID"})
			return
		}

		domain, hasAccess := checkDomainAccess(c, db, uint(domainID))
		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this domain"})
			return
		}

		client, err := getRedmineClient()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Redmine integration is not available"})
			return
		}

		issue, err := client.GetIssue(issueID)
		if err != nil {
			log.Error().
				Err(err).
				Str("domain", domain.Name).
				Int("issue_id", issueID).
				Msg("Failed to get Redmine issue")

			if strings.Contains(err.Error(), "404") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Redmine issue not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Redmine issue"})
			}
			return
		}

		// Verify the issue belongs to the domain's project
		projectID := getDomainRedmineProjectID(domain)
		if issue.Project.Identifier != projectID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Issue does not belong to this domain's project"})
			return
		}

		var assignedTo *RedmineUserResponse
		if issue.AssignedTo != nil {
			assignedTo = &RedmineUserResponse{
				ID:   issue.AssignedTo.ID,
				Name: issue.AssignedTo.Name,
			}
		}

		response := RedmineIssueResponse{
			ID: issue.ID,
			Project: RedmineProjectResponse{
				ID:         issue.Project.ID,
				Name:       issue.Project.Name,
				Identifier: issue.Project.Identifier,
			},
			Tracker: TrackerResponse{
				ID:   issue.Tracker.ID,
				Name: issue.Tracker.Name,
			},
			Status: StatusResponse{
				ID:   issue.Status.ID,
				Name: issue.Status.Name,
			},
			Priority: PriorityResponse{
				ID:   issue.Priority.ID,
				Name: issue.Priority.Name,
			},
			Author: RedmineUserResponse{
				ID:   issue.Author.ID,
				Name: issue.Author.Name,
			},
			AssignedTo:  assignedTo,
			Subject:     issue.Subject,
			Description: issue.Description,
			StartDate:   issue.StartDate,
			DueDate:     issue.DueDate,
			DoneRatio:   issue.DoneRatio,
			CreatedOn:   issue.CreatedOn.Format("2006-01-02T15:04:05Z"),
			UpdatedOn:   issue.UpdatedOn.Format("2006-01-02T15:04:05Z"),
		}

		c.JSON(http.StatusOK, response)
	}
}
