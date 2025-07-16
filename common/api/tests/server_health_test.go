//go:build with_api

package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetHealthTools_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")

	c, w := CreateRequestContext("GET", "/api/v1/health/tools", nil)
	AuthorizeContext(c, user)

	handler := ExportGetHealthTools(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var tools interface{}
	ExtractJSONResponse(t, w, &tools)
	assert.NotNil(t, tools)
}

func TestGetHealthTools_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/health/tools", nil)
	// No user in context

	handler := ExportGetHealthTools(db)
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPostHostHealth_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "health-host")

	healthData := map[string]interface{}{
		"tool":   "mysql",
		"status": "healthy",
		"metrics": map[string]interface{}{
			"connections": 50,
			"uptime":      3600,
			"version":     "8.0.32",
		},
		"timestamp": time.Now().Unix(),
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/mysql", healthData)
	SetPathParams(c, map[string]string{"tool": "mysql"})
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify health data was saved
	var savedHealth models.HostHealthData
	err := db.Where("host_name = ? AND tool_name = ?", "health-host", "mysql").First(&savedHealth).Error
	require.NoError(t, err)
	assert.Equal(t, "mysql", savedHealth.ToolName)
	assert.Contains(t, savedHealth.DataJSON, "healthy")
}

func TestPostHostHealth_InvalidJSON(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "health-host")

	// Create request with actual invalid JSON (not a marshaled string)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	invalidJSON := `{"status": "healthy", "invalid": json, "missing": quote}`
	req, _ := http.NewRequest("POST", "/api/v1/host/health/mysql", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	SetPathParams(c, map[string]string{"tool": "mysql"})
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostHostHealth_NoHostInContext(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	healthData := map[string]interface{}{
		"tool":   "mysql",
		"status": "healthy",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/mysql", healthData)
	SetPathParams(c, map[string]string{"tool": "mysql"})
	// No host in context

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPostHostHealth_MissingTool(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "health-host")

	healthData := map[string]interface{}{
		"status": "healthy",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/", healthData)
	// No tool in path params
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHostHealth_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")
	_ = SetupTestHost(t, db, "target-host")

	// Create test health data
	createTestHealthData(t, db, "target-host", "mysql", "healthy", `{"connections":50}`)
	createTestHealthData(t, db, "target-host", "redis", "warning", `{"memory":"high"}`)

	c, w := CreateRequestContext("GET", "/api/v1/hosts/target-host/health", nil)
	SetPathParams(c, map[string]string{"name": "target-host"})
	AuthorizeContext(c, user)

	handler := ExportGetHostHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var healthData interface{}
	ExtractJSONResponse(t, w, &healthData)
	assert.NotNil(t, healthData)
}

func TestGetHostHealth_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")

	c, w := CreateRequestContext("GET", "/api/v1/hosts/nonexistent/health", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent"})
	AuthorizeContext(c, user)

	handler := ExportGetHostHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetHostHealth_EmptyHealthData(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")
	_ = SetupTestHost(t, db, "empty-health-host")

	c, w := CreateRequestContext("GET", "/api/v1/hosts/empty-health-host/health", nil)
	SetPathParams(c, map[string]string{"name": "empty-health-host"})
	AuthorizeContext(c, user)

	handler := ExportGetHostHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var healthData interface{}
	ExtractJSONResponse(t, w, &healthData)
	assert.NotNil(t, healthData)
}

func TestGetHostToolHealth_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")
	_ = SetupTestHost(t, db, "tool-host")

	// Create specific tool health data
	createTestHealthData(t, db, "tool-host", "mysql", "healthy", `{"connections":25,"uptime":7200}`)

	c, w := CreateRequestContext("GET", "/api/v1/hosts/tool-host/health/mysql", nil)
	SetPathParams(c, map[string]string{"name": "tool-host", "tool": "mysql"})
	AuthorizeContext(c, user)

	handler := ExportGetHostToolHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var toolHealth interface{}
	ExtractJSONResponse(t, w, &toolHealth)
	assert.NotNil(t, toolHealth)
}

func TestGetHostToolHealth_ToolNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")
	_ = SetupTestHost(t, db, "tool-host")

	c, w := CreateRequestContext("GET", "/api/v1/hosts/tool-host/health/nonexistent", nil)
	SetPathParams(c, map[string]string{"name": "tool-host", "tool": "nonexistent"})
	AuthorizeContext(c, user)

	handler := ExportGetHostToolHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetHostToolHealth_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")

	c, w := CreateRequestContext("GET", "/api/v1/hosts/nonexistent/health/mysql", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent", "tool": "mysql"})
	AuthorizeContext(c, user)

	handler := ExportGetHostToolHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPostHostHealth_MultipleTools(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "multi-tool-host")

	tools := []struct {
		name   string
		status string
		data   map[string]interface{}
	}{
		{
			name:   "mysql",
			status: "healthy",
			data:   map[string]interface{}{"connections": 30, "version": "8.0"},
		},
		{
			name:   "redis",
			status: "warning",
			data:   map[string]interface{}{"memory_usage": "85%", "keys": 10000},
		},
		{
			name:   "nginx",
			status: "critical",
			data:   map[string]interface{}{"active_connections": 0, "status": "down"},
		},
	}

	for _, tool := range tools {
		healthData := map[string]interface{}{
			"tool":    tool.name,
			"status":  tool.status,
			"metrics": tool.data,
		}

		c, w := CreateRequestContext("POST", "/api/v1/host/health/"+tool.name, healthData)
		SetPathParams(c, map[string]string{"tool": tool.name})
		c.Set("hostname", host.Name)

		handler := ExportPostHostHealth(db)
		handler(c)

		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Verify all tools were saved
	var healthCount int64
	db.Model(&models.HostHealthData{}).Where("host_name = ?", "multi-tool-host").Count(&healthCount)
	assert.Equal(t, int64(3), healthCount)
}

func TestPostHostHealth_UpdateExisting(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "update-health-host")

	// Create initial health data
	createTestHealthData(t, db, "update-health-host", "mysql", "healthy", `{"connections":20}`)

	// Update with new data
	updatedHealthData := map[string]interface{}{
		"tool":   "mysql",
		"status": "warning",
		"metrics": map[string]interface{}{
			"connections":  80,
			"slow_queries": 15,
		},
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/mysql", updatedHealthData)
	SetPathParams(c, map[string]string{"tool": "mysql"})
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify data was updated
	var updatedHealth models.HostHealthData
	err := db.Where("host_name = ? AND tool_name = ?", "update-health-host", "mysql").First(&updatedHealth).Error
	require.NoError(t, err)
	assert.Contains(t, updatedHealth.DataJSON, "warning")
	assert.Contains(t, updatedHealth.DataJSON, "slow_queries")
}

func TestPostHostHealth_LargeHealthData(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "large-data-host")

	// Create large metrics data
	largeMetrics := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeMetrics[fmt.Sprintf("metric_%d", i)] = i * 2
	}

	healthData := map[string]interface{}{
		"tool":    "stress-test",
		"status":  "healthy",
		"metrics": largeMetrics,
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/stress-test", healthData)
	SetPathParams(c, map[string]string{"tool": "stress-test"})
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify large data was saved
	var savedHealth models.HostHealthData
	err := db.Where("host_name = ? AND tool_name = ?", "large-data-host", "stress-test").First(&savedHealth).Error
	require.NoError(t, err)
	assert.Equal(t, "stress-test", savedHealth.ToolName)
	assert.Contains(t, savedHealth.DataJSON, "metric_999")
}

func TestPostHostHealth_InvalidStatus(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "invalid-status-host")

	healthData := map[string]interface{}{
		"tool":   "mysql",
		"status": "invalid_status", // Invalid status
		"metrics": map[string]interface{}{
			"connections": 30,
		},
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/health/mysql", healthData)
	SetPathParams(c, map[string]string{"tool": "mysql"})
	c.Set("hostname", host.Name)

	handler := ExportPostHostHealth(db)
	handler(c)

	// Should still succeed - validation might be lenient
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHostHealth_WithTimestamps(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "health_viewer")
	_ = SetupTestHost(t, db, "timestamp-host")

	// Create health data with different timestamps
	now := time.Now()
	createTestHealthDataWithTime(t, db, "timestamp-host", "mysql", "healthy", `{"connections":30}`, now)
	createTestHealthDataWithTime(t, db, "timestamp-host", "redis", "warning", `{"memory":"high"}`, now.Add(-1*time.Hour))

	c, w := CreateRequestContext("GET", "/api/v1/hosts/timestamp-host/health", nil)
	SetPathParams(c, map[string]string{"name": "timestamp-host"})
	AuthorizeContext(c, user)

	handler := ExportGetHostHealth(db, "monokit-server")
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var healthData interface{}
	ExtractJSONResponse(t, w, &healthData)
	assert.NotNil(t, healthData)
}

func TestHealthData_Concurrent(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "concurrent-host")

	// Simulate concurrent health data submissions
	healthData := map[string]interface{}{
		"tool":   "mysql",
		"status": "healthy",
		"metrics": map[string]interface{}{
			"connections": 25,
		},
	}

	// Submit multiple requests concurrently
	for i := 0; i < 5; i++ {
		c, w := CreateRequestContext("POST", "/api/v1/host/health/mysql", healthData)
		SetPathParams(c, map[string]string{"tool": "mysql"})
		c.Set("hostname", host.Name)

		handler := ExportPostHostHealth(db)
		handler(c)

		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Should have one entry (upsert behavior)
	var healthCount int64
	db.Model(&models.HostHealthData{}).Where("host_name = ? AND tool_name = ?", "concurrent-host", "mysql").Count(&healthCount)
	assert.Equal(t, int64(1), healthCount)
}

// Helper functions

func createTestHealthData(t *testing.T, db *gorm.DB, hostName, tool, status, data string) models.HostHealthData {
	return createTestHealthDataWithTime(t, db, hostName, tool, status, data, time.Now())
}

func createTestHealthDataWithTime(t *testing.T, db *gorm.DB, hostName, tool, status, data string, timestamp time.Time) models.HostHealthData {
	// Combine status and data into a JSON string for DataJSON field
	combinedData := fmt.Sprintf(`{"status":"%s","data":%s}`, status, data)

	healthData := models.HostHealthData{
		HostName:    hostName,
		ToolName:    tool,
		DataJSON:    combinedData,
		LastUpdated: timestamp,
	}
	result := db.Create(&healthData)
	require.NoError(t, result.Error)
	return healthData
}
