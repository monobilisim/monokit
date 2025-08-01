//go:build with_api

package tests

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/logbuffer"
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
	_ = SetupTestHostKey(t, db, host, "host_log_key")

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
	c.Set("hostname", host.Name)

	// Set up log buffer with shorter intervals for faster tests
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()
	defer buf.Close()
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify log was saved (eventually)
	time.Sleep(200 * time.Millisecond) // Wait for flush
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

	logData := server.APILogRequest{
		Level:     "invalid_level",
		Component: "test",
		Message:   "Test message",
		Type:      "monokit",
	}

	c, w := CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("hostname", host.Name)

	// Set up log buffer with shorter intervals for faster tests
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()
	defer buf.Close()
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)
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
			c.Set("hostname", host.Name)

			// Set up log buffer with shorter intervals for faster tests
			cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
			buf := logbuffer.NewBuffer(db, cfg)
			buf.Start()
			defer buf.Close()
			c.Set("logBuffer", buf)

			handler := ExportSubmitHostLog(db)
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

	// Set up log buffer with shorter intervals for faster tests
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()
	defer buf.Close()
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)
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

	handler := ExportGetAllLogs(db)
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

	handler := ExportGetAllLogs(db)
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

	handler := ExportGetHostLogs(db)
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

	handler := ExportGetHostLogs(db)
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

	handler := ExportSearchLogs(db)
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

	handler := ExportSearchLogs(db)
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

	logIDStr := strconv.Itoa(int(logEntry.ID))
	c, w := CreateRequestContext("DELETE", "/api/v1/logs/"+logIDStr, nil)
	SetPathParams(c, map[string]string{"id": logIDStr})
	AuthorizeContext(c, user)

	handler := ExportDeleteLog(db)
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

	handler := ExportDeleteLog(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteLog_NonAdmin(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "regular_user") // Non-admin user
	logEntry := createTestLog(t, db, "host1", "info", "test", "Log to delete")

	logIDStr := strconv.Itoa(int(logEntry.ID))
	c, w := CreateRequestContext("DELETE", "/api/v1/logs/"+logIDStr, nil)
	SetPathParams(c, map[string]string{"id": logIDStr})
	AuthorizeContext(c, user)

	handler := ExportDeleteLog(db)
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

	handler := ExportGetHourlyLogStats(db)
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
	c.Set("hostname", host.Name)

	// Set up log buffer with shorter intervals for faster tests
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()
	defer buf.Close()
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify large message was saved
	time.Sleep(200 * time.Millisecond)
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
	c.Set("hostname", host.Name)

	// Set up log buffer with shorter intervals for faster tests
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 100 * time.Millisecond}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()
	defer buf.Close()
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)
	handler(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify special characters were preserved
	time.Sleep(200 * time.Millisecond)
	var savedLog models.HostLog
	err := db.Where("host_name = ?", "log-host").First(&savedLog).Error
	require.NoError(t, err)
	assert.Equal(t, specialMessage, savedLog.Message)
}

// Helper functions

func setupHostLogContext(t *testing.T, db *gorm.DB, host models.Host) *gin.Context {
	_ = SetupTestHostKey(t, db, host, "host_log_key")
	c, _ := CreateRequestContext("POST", "/api/v1/host/logs", nil)
	c.Request.Header.Set("Authorization", "host_log_key")
	c.Set("hostname", host.Name)
	return c
}

func createTestLog(t *testing.T, db *gorm.DB, hostName, level, component, message string) models.HostLog {
	return createTestLogWithTime(t, db, hostName, level, component, message, time.Now())
}

func TestLogBuffer_Batching(t *testing.T) {
	db := SetupTestDB(t)

	host := SetupTestHost(t, db, "batch-host")
	_ = SetupTestHostKey(t, db, host, "batch_key")

	// Configure buffer to flush on 10 items or every 5 seconds
	cfg := logbuffer.Config{BatchSize: 10, FlushInterval: 5 * time.Second}
	buf := logbuffer.NewBuffer(db, cfg)
	buf.Start()

	// Ensure buffer is closed before database
	defer func() {
		buf.Close()
		CleanupTestDB(db)
	}()

	c, _ := CreateRequestContext("POST", "/api/v1/host/logs", nil)
	c.Request.Header.Set("Authorization", "batch_key")
	c.Set("hostname", host.Name)
	c.Set("logBuffer", buf)

	handler := ExportSubmitHostLog(db)

	// --- Test 1: Trigger flush by size ---
	for i := 0; i < 9; i++ {
		logData := server.APILogRequest{Level: "info", Component: "batch", Message: "Message " + strconv.Itoa(i)}
		c, _ := CreateRequestContext("POST", "/api/v1/host/logs", logData)
		c.Set("hostname", host.Name)
		c.Set("logBuffer", buf)
		handler(c)
	}

	// 9 logs submitted, should not be in DB yet
	var count int64
	db.Model(&models.HostLog{}).Where("host_name = ?", host.Name).Count(&count)
	assert.Equal(t, int64(0), count, "Logs should not be flushed yet")

	// Submit 10th log to trigger flush
	logData := server.APILogRequest{Level: "info", Component: "batch", Message: "Message 9"}
	c, _ = CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("hostname", host.Name)
	c.Set("logBuffer", buf)
	handler(c)

	// Wait a moment for the flush to complete
	time.Sleep(200 * time.Millisecond)

	db.Model(&models.HostLog{}).Where("host_name = ?", host.Name).Count(&count)
	assert.Equal(t, int64(10), count, "Logs should be flushed after batch size reached")

	// --- Test 2: Trigger flush by time ---
	// Close the previous buffer first
	buf.Close()

	// Create a new buffer with shorter flush interval for timing test
	cfg.FlushInterval = 200 * time.Millisecond
	timeBuf := logbuffer.NewBuffer(db, cfg)
	timeBuf.Start()

	logData = server.APILogRequest{Level: "info", Component: "timed", Message: "Timed message"}
	c, _ = CreateRequestContext("POST", "/api/v1/host/logs", logData)
	c.Set("hostname", host.Name)
	c.Set("logBuffer", timeBuf)
	handler(c)

	// Should not be in DB yet
	db.Model(&models.HostLog{}).Where("component = ?", "timed").Count(&count)
	assert.Equal(t, int64(0), count)

	// Wait for time-based flush (wait a bit longer to be safe)
	time.Sleep(400 * time.Millisecond)
	db.Model(&models.HostLog{}).Where("component = ?", "timed").Count(&count)
	assert.Equal(t, int64(1), count, "Log should be flushed after interval")

	// Close the time buffer
	timeBuf.Close()
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
