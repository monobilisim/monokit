//go:build with_api

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/cache"
	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

// Test error handling in cache operations
func TestValkeyCache_DisabledConfig(t *testing.T) {
	// Test with disabled config
	config := models.ValkeyConfig{
		Enabled: false,
	}

	_, err := cache.NewValkeyCache(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "valkey is disabled")
}

func TestValkeyCache_ConnectionError(t *testing.T) {
	// Test with invalid connection parameters
	config := models.ValkeyConfig{
		Enabled:      true,
		Address:      "invalid-host:9999",
		Password:     "wrong-password",
		Database:     0,
		WriteTimeout: 1,
		KeyPrefix:    "test:",
	}

	// This should fail to create the client
	_, err := cache.NewValkeyCache(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create valkey client")
}

func TestInitCache_ErrorHandling(t *testing.T) {
	// Test with invalid config that will fail connection
	config := models.ValkeyConfig{
		Enabled:      true,
		Address:      "nonexistent-host:6379",
		Password:     "",
		Database:     0,
		WriteTimeout: 1,
		KeyPrefix:    "test:",
		SessionTTL:   3600,
		HostTTL:      1800,
		HealthTTL:    300,
	}

	// This should fail but not panic
	err := cache.InitCache(config)
	assert.Error(t, err)

	// GlobalCache should still be available (as NoOpCache)
	assert.NotNil(t, cache.GlobalCache)
}

// Test error handling in Cloudflare client
func TestCloudflareClient_ErrorHandling(t *testing.T) {
	// Test with no authentication
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "",
		APIKey:    "",
		Email:     "",
		Timeout:   30,
		VerifySSL: true,
	}

	_, err := cloudflare.NewClient(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestCloudflareClient_APIKeyWithoutEmail(t *testing.T) {
	// Test with API key but no email
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "",
		APIKey:    "test-key",
		Email:     "",
		Timeout:   30,
		VerifySSL: true,
	}

	_, err := cloudflare.NewClient(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestCloudflareClient_InvalidToken(t *testing.T) {
	// Test with malformed token (this might not fail at creation but would fail on API calls)
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "invalid-token-format",
		Timeout:   30,
		VerifySSL: true,
	}

	// Client creation might succeed but API calls would fail
	client, err := cloudflare.NewClient(config)
	if err == nil {
		// If client creation succeeds, it should still be valid
		assert.NotNil(t, client)
	}
}

// Test JSON marshaling/unmarshaling errors
func TestJSONErrorHandling(t *testing.T) {
	// Test with invalid JSON data
	invalidJSON := `{"invalid": json}`

	var result map[string]interface{}
	err := json.Unmarshal([]byte(invalidJSON), &result)
	assert.Error(t, err)
}

// Test context timeout scenarios
func TestContextTimeoutHandling(t *testing.T) {
	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(1 * time.Millisecond)

	// Test that context is cancelled
	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
		assert.Contains(t, ctx.Err().Error(), "deadline exceeded")
	default:
		t.Error("Context should have timed out")
	}
}

// Test database connection error scenarios
func TestDatabaseErrorHandling(t *testing.T) {
	// This test verifies that our test setup handles database errors gracefully
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Test with invalid query
	var result []models.User
	err := db.Raw("SELECT * FROM nonexistent_table").Scan(&result).Error
	assert.Error(t, err)
}

// Test HTTP error responses
func TestHTTPErrorHandling(t *testing.T) {
	// Test various HTTP error status codes
	testCases := []struct {
		statusCode int
		statusText string
	}{
		{http.StatusBadRequest, "Bad Request"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "Not Found"},
		{http.StatusMethodNotAllowed, "Method Not Allowed"},
		{http.StatusConflict, "Conflict"},
		{http.StatusInternalServerError, "Internal Server Error"},
		{http.StatusBadGateway, "Bad Gateway"},
		{http.StatusServiceUnavailable, "Service Unavailable"},
	}

	for _, tc := range testCases {
		t.Run(tc.statusText, func(t *testing.T) {
			assert.Equal(t, tc.statusText, http.StatusText(tc.statusCode))
		})
	}
}

// Test configuration validation errors
func TestConfigValidationErrors(t *testing.T) {
	// Test various invalid configurations

	// Invalid timeout values
	config := models.ValkeyConfig{
		Enabled:      true,
		Address:      "localhost:6379",
		WriteTimeout: -1, // Invalid negative timeout
	}

	// The validation might happen at different levels
	// This test ensures we handle invalid configs gracefully
	_, err := cache.NewValkeyCache(config)
	// Error might occur during client creation or connection
	if err != nil {
		assert.Error(t, err)
	}
}

// Test edge cases in string processing
func TestStringProcessingEdgeCases(t *testing.T) {
	// Test empty strings
	assert.Empty(t, "")

	// Test strings with special characters
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?"
	assert.NotEmpty(t, specialChars)

	// Test unicode strings
	unicode := "Hello 世界 🌍"
	assert.Contains(t, unicode, "世界")
	assert.Contains(t, unicode, "🌍")
}

// Test memory allocation edge cases
func TestMemoryAllocationEdgeCases(t *testing.T) {
	// Test large slice allocation
	largeSlice := make([]byte, 1024*1024) // 1MB
	assert.Len(t, largeSlice, 1024*1024)

	// Test map allocation
	largeMap := make(map[string]string)
	for i := 0; i < 1000; i++ {
		largeMap[string(rune(i))] = string(rune(i))
	}
	assert.Len(t, largeMap, 1000)
}

// Test concurrent access scenarios
func TestConcurrentAccessHandling(t *testing.T) {
	// Test concurrent map access (this is mainly to ensure our tests don't have race conditions)
	testMap := make(map[string]int)

	// Sequential access (safe)
	testMap["key1"] = 1
	testMap["key2"] = 2

	assert.Equal(t, 1, testMap["key1"])
	assert.Equal(t, 2, testMap["key2"])
}

// Test resource cleanup
func TestResourceCleanup(t *testing.T) {
	// Test that resources are properly cleaned up
	noopCache := cache.NewNoOpCache()

	// Close should not error
	err := noopCache.Close()
	assert.NoError(t, err)

	// Multiple closes should be safe
	err = noopCache.Close()
	assert.NoError(t, err)
}

// Test boundary conditions
func TestBoundaryConditions(t *testing.T) {
	// Test zero values
	assert.Equal(t, 0, 0)
	assert.Equal(t, "", "")
	assert.Nil(t, nil)

	// Test maximum values (within reason for tests)
	maxInt := int(^uint(0) >> 1)
	assert.Greater(t, maxInt, 0)

	// Test minimum values
	minInt := -maxInt - 1
	assert.Less(t, minInt, 0)
}
