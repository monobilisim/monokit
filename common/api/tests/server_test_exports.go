//go:build with_api

package tests

import (
	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/server"
	"gorm.io/gorm"
)

// This file contains export functions for testing purposes only.
// These functions expose internal handlers for unit testing.
// Located in tests folder and using tests package for proper organization.

// ============ HOST MANAGEMENT EXPORTS ============

func ExportRegisterHost(db *gorm.DB) gin.HandlerFunc {
	return server.RegisterHost(db)
}

func ExportGetAllHosts(db *gorm.DB) gin.HandlerFunc {
	return server.GetAllHosts(db)
}

func ExportGetHostByName() gin.HandlerFunc {
	return server.GetHostByName()
}

func ExportDeleteHost(db *gorm.DB) gin.HandlerFunc {
	return server.DeleteHost(db)
}

func ExportForceDeleteHost(db *gorm.DB) gin.HandlerFunc {
	return server.ForceDeleteHost(db)
}

func ExportUpdateHost(db *gorm.DB) gin.HandlerFunc {
	return server.UpdateHost(db)
}

func ExportGetAssignedHosts(db *gorm.DB) gin.HandlerFunc {
	return server.GetAssignedHosts(db)
}

// ============ GROUP MANAGEMENT EXPORTS ============

func ExportGetAllGroups(db *gorm.DB) gin.HandlerFunc {
	return server.GetAllGroups(db)
}

// ============ COMPONENT MANAGEMENT EXPORTS ============

func ExportEnableComponent(db *gorm.DB) gin.HandlerFunc {
	return server.EnableComponent(db)
}

func ExportDisableComponent(db *gorm.DB) gin.HandlerFunc {
	return server.DisableComponent(db)
}

func ExportGetComponentStatus() gin.HandlerFunc {
	return server.GetComponentStatus()
}

// ============ HOST VERSION MANAGEMENT EXPORTS ============

func ExportUpdateHostVersion(db *gorm.DB) gin.HandlerFunc {
	return server.UpdateHostVersion(db)
}

// ============ MIDDLEWARE EXPORTS ============

func ExportHostAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return server.HostAuthMiddleware(db)
}

func ExportAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return server.AuthMiddleware(db)
}

// ============ UTILITY EXPORTS ============

func ExportGenerateToken() string {
	return server.GenerateToken()
}

// ============ AWX MANAGEMENT EXPORTS ============

func ExportCreateAwxHost(db *gorm.DB) gin.HandlerFunc {
	return server.CreateAwxHost(db)
}

func ExportDeleteAwxHost(db *gorm.DB) gin.HandlerFunc {
	return server.DeleteAwxHost(db)
}

func ExportGetAwxTemplatesGlobal(db *gorm.DB) gin.HandlerFunc {
	return server.GetAwxTemplatesGlobal(db)
}

func ExportExecuteAwxWorkflowJob(db *gorm.DB) gin.HandlerFunc {
	return server.ExecuteAwxWorkflowJob(db)
}

func ExportGetAwxJobStatus(db *gorm.DB) gin.HandlerFunc {
	return server.GetAwxJobStatus(db)
}

func ExportExecuteAwxJob(db *gorm.DB) gin.HandlerFunc {
	return server.ExecuteAwxJob(db)
}

func ExportGetHostAwxJobs(db *gorm.DB) gin.HandlerFunc {
	return server.GetHostAwxJobs(db)
}

func ExportGetHostAwxJobLogs(db *gorm.DB) gin.HandlerFunc {
	return server.GetHostAwxJobLogs(db)
}

func ExportGetAwxJobTemplateDetails(db *gorm.DB) gin.HandlerFunc {
	return server.GetAwxJobTemplateDetails(db)
}

func ExportGetAwxJobTemplates(db *gorm.DB) gin.HandlerFunc {
	return server.GetAwxJobTemplates(db)
}

func ExportGetAwxWorkflowTemplatesGlobal(db *gorm.DB) gin.HandlerFunc {
	return server.GetAwxWorkflowTemplatesGlobal(db)
}

func ExportEnsureHostInAwx(db *gorm.DB, host models.Host) (string, error) {
	return server.EnsureHostInAwx(db, host)
}

// ============ HEALTH MANAGEMENT EXPORTS ============

func ExportGetHealthTools(db *gorm.DB) gin.HandlerFunc {
	return server.GetHealthTools(db)
}

func ExportGetHostHealth(db *gorm.DB, monokitHostname string) gin.HandlerFunc {
	return server.GetHostHealth(db, monokitHostname)
}

func ExportPostHostHealth(db *gorm.DB) gin.HandlerFunc {
	return server.PostHostHealth(db)
}

func ExportGetHostToolHealth(db *gorm.DB, monokitHostname string) gin.HandlerFunc {
	return server.GetHostToolHealth(db, monokitHostname)
}

// ============ LOGGING EXPORTS ============

func ExportSubmitHostLog(db *gorm.DB) gin.HandlerFunc {
	return server.SubmitHostLog(db)
}

func ExportGetAllLogs(db *gorm.DB) gin.HandlerFunc {
	return server.GetAllLogs(db)
}

func ExportGetHostLogs(db *gorm.DB) gin.HandlerFunc {
	return server.GetHostLogs(db)
}

func ExportSearchLogs(db *gorm.DB) gin.HandlerFunc {
	return server.SearchLogs(db)
}

func ExportDeleteLog(db *gorm.DB) gin.HandlerFunc {
	return server.DeleteLog(db)
}

func ExportGetHourlyLogStats(db *gorm.DB) gin.HandlerFunc {
	return server.GetHourlyLogStats(db)
}
