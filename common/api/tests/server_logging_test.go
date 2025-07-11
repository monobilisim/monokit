//go:build with_api

package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubmitHostLog_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "log-host")
	hostKey := SetupTestHostKey(t, db, host, "host_log_key")

	logData := server.APILogRequest{
		Level:     "info",
		Component: "mysql",
		Message:   "Database connection established",
		Timestamp: time.Now().Format(time.RFC3339),
		Metadata:  `{"version":"8.0","port":3306}`,
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Request.Header.Set("Authorization", "host_log_key")

	// Set up host auth middleware context
	c.Set("host", host)

	handler := server.ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify log was saved
	var savedLog models.HostLog
	err := db.Where("host_name = ? AND message = ?", "log-host", "Database connection established").First(&savedLog).Error
	require.NoError(t, err)
	assert.Equal(t, "info", savedLog.Level)
	assert.Equal(t, "mysql", savedLog.Component)
	assert.Equal(t, "monokit", savedLog.Type)
}

func TestSubmitHostLog_InvalidLevel(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "log-host")
	c := setupHostLogContext(t, db, host)

	logData := server.APILogRequest{
		Level:     "invalid_level",
		Component: "test",
		Message:   "Test message",
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("host", host)

	handler := server.ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubmitHostLog_MissingRequiredFields(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "log-host")

	testCases := []struct {
		name    string
		logData server.APILogRequest
	}{
		{
			name: "missing level",
			logData: server.APILogRequest{
				Component: "test",
				Message:   "Test message",
				Type:      "monokit",
			},
		},
		{
			name: "missing component",
			logData: server.APILogRequest{
				Level:   "info",
				Message: "Test message",
				Type:    "monokit",
			},
		},
		{
			name: "missing message",
			logData: server.APILogRequest{
				Level:     "info",
				Component: "test",
				Type:      "monokit",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := CreateRequestContext("POST", "/api/v1/host/logs", tc.logData)
			c.Set("host", host)

			handler := server.ExportSubmitHostLog(db)
			handler(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestSubmitHostLog_NoHostInContext(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	logData := server.APILogRequest{
		Level:     "info",
		Component: "test",
		Message:   "Test message",
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	// No host set in context

	handler := server.ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetAllLogs_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	// Create test logs
	createTestLog(t, db, "host1", "info", "component1", "Log message 1")
	createTestLog(t, db, "host2", "error", "component2", "Log message 2")
	createTestLog(t, db, "host3", "warning", "component1", "Log message 3")

	c, w := CreateRequestContext("GET", "/api/v1/logs", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetAllLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APILogsResponse
	ExtractJSONResponse(t, w, &response)

	assert.GreaterOrEqual(t, len(response.Logs), 3)
	assert.Greater(t, response.Pagination.Total, int64(0))
}

func TestGetAllLogs_WithPagination(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	// Create many test logs
	for i := 0; i < 150; i++ {
		createTestLog(t, db, "host1", "info", "component1", "Log message "+string(rune(i)))
	}

	c, w := CreateRequestContext("GET", "/api/v1/logs?page=2&page_size=50", nil)
	SetQueryParams(c, map[string]string{"page": "2", "page_size": "50"})
	AuthorizeContext(c, user)

	handler := server.ExportGetAllLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APILogsResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, 2, response.Pagination.Page)
	assert.Equal(t, 50, response.Pagination.PageSize)
	assert.LessOrEqual(t, len(response.Logs), 50)
}

func TestGetHostLogs_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	// Create logs for different hosts
	createTestLog(t, db, "target-host", "info", "component1", "Target log 1")
	createTestLog(t, db, "target-host", "error", "component2", "Target log 2")
	createTestLog(t, db, "other-host", "info", "component1", "Other log")

	c, w := CreateRequestContext("GET", "/api/v1/logs/target-host", nil)
	SetPathParams(c, map[string]string{"hostname": "target-host"})
	AuthorizeContext(c, user)

	handler := server.ExportGetHostLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APIHostLogsResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, "target-host", response.HostName)
	assert.GreaterOrEqual(t, len(response.Logs), 2)

	// Verify all logs are for the target host
	for _, log := range response.Logs {
		assert.Equal(t, "target-host", log.HostName)
	}
}

func TestGetHostLogs_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	c, w := CreateRequestContext("GET", "/api/v1/logs/nonexistent-host", nil)
	SetPathParams(c, map[string]string{"hostname": "nonexistent-host"})
	AuthorizeContext(c, user)

	handler := server.ExportGetHostLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APIHostLogsResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, "nonexistent-host", response.HostName)
	assert.Equal(t, 0, len(response.Logs))
}

func TestSearchLogs_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	// Create test logs with specific patterns
	createTestLog(t, db, "host1", "error", "mysql", "Database connection failed")
	createTestLog(t, db, "host2", "info", "mysql", "Database connection established")
	createTestLog(t, db, "host1", "error", "redis", "Cache connection failed")

	searchRequest := server.APILogSearchRequest{
		Level:       "error",
		Component:   "mysql",
		MessageText: "connection",
		Page:        1,
		PageSize:    100,
	}

	c, w := CreateRequestContext("POST", "/api/v1/logs/search", searchRequest)
	AuthorizeContext(c, user)

	handler := server.ExportSearchLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APILogsResponse
	ExtractJSONResponse(t, w, &response)

	assert.GreaterOrEqual(t, len(response.Logs), 1)

	// Verify search criteria are met
	for _, log := range response.Logs {
		assert.Equal(t, "error", log.Level)
		assert.Equal(t, "mysql", log.Component)
		assert.Contains(t, strings.ToLower(log.Message), "connection")
	}
}

func TestSearchLogs_WithTimeRange(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "log_viewer")

	// Create logs with specific timestamps
	now := time.Now()
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	createTestLogWithTime(t, db, "host1", "info", "test", "Past log", past)
	createTestLogWithTime(t, db, "host1", "info", "test", "Current log", now)
	createTestLogWithTime(t, db, "host1", "info", "test", "Future log", future)

	searchRequest := server.APILogSearchRequest{
		StartTime: past.Add(30 * time.Minute).Format(time.RFC3339),
		EndTime:   now.Add(30 * time.Minute).Format(time.RFC3339),
		Page:      1,
		PageSize:  100,
	}

	c, w := CreateRequestContext("POST", "/api/v1/logs/search", searchRequest)
	AuthorizeContext(c, user)

	handler := server.ExportSearchLogs(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response server.APILogsResponse
	ExtractJSONResponse(t, w, &response)

	// Should only return the current log
	assert.Equal(t, 1, len(response.Logs))
	assert.Contains(t, response.Logs[0].Message, "Current log")
}

func TestDeleteLog_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestAdmin(t, db) // Admin required for deletion

	// Create test log
	logEntry := createTestLog(t, db, "host1", "info", "test", "Log to delete")

	c, w := CreateRequestContext("DELETE", "/api/v1/logs/"+string(rune(logEntry.ID)), nil)
	SetPathParams(c, map[string]string{"id": string(rune(logEntry.ID))})
	AuthorizeContext(c, user)

	handler := server.ExportDeleteLog(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify log was deleted
	var logCount int64
	db.Model(&models.HostLog{}).Where("id = ?", logEntry.ID).Count(&logCount)
	assert.Equal(t, int64(0), logCount)
}

func TestDeleteLog_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestAdmin(t, db)

	c, w := CreateRequestContext("DELETE", "/api/v1/logs/99999", nil)
	SetPathParams(c, map[string]string{"id": "99999"})
	AuthorizeContext(c, user)

	handler := server.ExportDeleteLog(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteLog_NonAdmin(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "regular_user") // Non-admin user
	logEntry := createTestLog(t, db, "host1", "info", "test", "Log to delete")

	c, w := CreateRequestContext("DELETE", "/api/v1/logs/"+string(rune(logEntry.ID)), nil)
	SetPathParams(c, map[string]string{"id": string(rune(logEntry.ID))})
	AuthorizeContext(c, user)

	handler := server.ExportDeleteLog(db)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetHourlyLogStats_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "stats_viewer")

	// Create logs in different hours
	now := time.Now()
	hour1 := now.Truncate(time.Hour)
	hour2 := hour1.Add(-1 * time.Hour)

	// Create multiple logs in each hour
	for i := 0; i < 5; i++ {
		createTestLogWithTime(t, db, "host1", "info", "test", "Hour 1 log", hour1)
		createTestLogWithTime(t, db, "host1", "error", "test", "Hour 2 log", hour2)
	}

	c, w := CreateRequestContext("GET", "/api/v1/logs/hourly", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetHourlyLogStats(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats interface{}
	ExtractJSONResponse(t, w, &stats)
	assert.NotNil(t, stats)
}

func TestLogSubmission_LargeMessage(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "log-host")

	// Create a very large log message
	largeMessage := strings.Repeat("This is a very long log message. ", 1000)

	logData := server.APILogRequest{
		Level:     "info",
		Component: "test",
		Message:   largeMessage,
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("host", host)

	handler := server.ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify large message was saved
	var savedLog models.HostLog
	err := db.Where("host_name = ?", "log-host").First(&savedLog).Error
	require.NoError(t, err)
	assert.Equal(t, largeMessage, savedLog.Message)
}

func TestLogSubmission_SpecialCharacters(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	host := SetupTestHost(t, db, "log-host")

	specialMessage := "Message with special chars: \n\t\"quotes\" 'apostrophes' & <html> tags 🚀 unicode"

	logData := server.APILogRequest{
		Level:     "info",
		Component: "test",
		Message:   specialMessage,
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("host", host)

	handler := server.ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify special characters were preserved
	var savedLog models.HostLog
	err := db.Where("host_name = ?", "log-host").First(&savedLog).Error
	require.NoError(t, err)
	assert.Equal(t, specialMessage, savedLog.Message)
}

// Helper functions

func setupHostLogContext(t *testing.T, db *gorm.DB, host models.Host) *gin.Context {
	hostKey := SetupTestHostKey(t, db, host, "host_log_key")
	c, _ := CreateRequestContext("POST", "/api/v1/host/logs", nil)
	c.Request.Header.Set("Authorization", "host_log_key")
	c.Set("host", host)
	return c
}

func createTestLog(t *testing.T, db *gorm.DB, hostName, level, component, message string) models.HostLog {
	return createTestLogWithTime(t, db, hostName, level, component, message, time.Now())
}

func createTestLogWithTime(t *testing.T, db *gorm.DB, hostName, level, component, message string, timestamp time.Time) models.HostLog {
	logEntry := models.HostLog{
		HostName:  hostName,
		Level:     level,
		Component: component,
		Message:   message,
		Timestamp: timestamp,
		Type:      "monokit",
	}
	result := db.Create(&logEntry)
	require.NoError(t, result.Error)
	return logEntry
}
