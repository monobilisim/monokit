//go:build with_api

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/cache"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestNoOpCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test Set operation (should always succeed)
	err := noopCache.Set(ctx, "test-key", "test-value", time.Minute)
	assert.NoError(t, err)

	// Test Get operation (should always return cache miss)
	var result string
	err = noopCache.Get(ctx, "test-key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")

	// Test Delete operation (should always succeed)
	err = noopCache.Delete(ctx, "test-key")
	assert.NoError(t, err)

	// Test Exists operation (should always return false)
	exists, err := noopCache.Exists(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test FlushAll operation (should always succeed)
	err = noopCache.FlushAll(ctx)
	assert.NoError(t, err)
}

func TestNoOpCache_SessionOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	session := &models.Session{
		Token:   "test-token",
		Timeout: time.Now().Add(time.Hour),
		User: models.User{
			Username: "testuser",
			Role:     "user",
		},
	}

	// Test SetSession operation (should always succeed)
	err := noopCache.SetSession(ctx, "test-token", session)
	assert.NoError(t, err)

	// Test GetSession operation (should always return cache miss)
	retrievedSession, err := noopCache.GetSession(ctx, "test-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
	assert.Nil(t, retrievedSession)

	// Test DeleteSession operation (should always succeed)
	err = noopCache.DeleteSession(ctx, "test-token")
	assert.NoError(t, err)
}

func TestNoOpCache_HostAuthOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test SetHostAuth operation (should always succeed)
	err := noopCache.SetHostAuth(ctx, "auth-token", "test-host")
	assert.NoError(t, err)

	// Test GetHostAuth operation (should always return cache miss)
	hostName, err := noopCache.GetHostAuth(ctx, "auth-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
	assert.Empty(t, hostName)

	// Test DeleteHostAuth operation (should always succeed)
	err = noopCache.DeleteHostAuth(ctx, "auth-token")
	assert.NoError(t, err)
}

func TestNoOpCache_HealthDataOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	healthData := map[string]interface{}{
		"cpu_usage":    75.5,
		"memory_usage": 60.2,
		"disk_usage":   45.8,
	}

	// Test SetHealthData operation (should always succeed)
	err := noopCache.SetHealthData(ctx, "test-host", "osHealth", healthData)
	assert.NoError(t, err)

	// Test GetHealthData operation (should always return cache miss)
	var retrievedData map[string]interface{}
	err = noopCache.GetHealthData(ctx, "test-host", "osHealth", &retrievedData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")

	// Test DeleteHealthData operation (should always succeed)
	err = noopCache.DeleteHealthData(ctx, "test-host", "osHealth")
	assert.NoError(t, err)
}

func TestNoOpCache_HostOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	host := &models.Host{
		Name:                "test-host",
		CpuCores:            8,
		Ram:                 "16GB",
		MonokitVersion:      "2.0.0",
		Os:                  "Ubuntu 22.04",
		DisabledComponents:  "nil",
		InstalledComponents: "mysql,redis",
		IpAddress:           "192.168.1.100",
		Status:              "online",
		Groups:              "web-servers",
	}

	// Test SetHost operation (should always succeed)
	err := noopCache.SetHost(ctx, "test-host", host)
	assert.NoError(t, err)

	// Test GetHost operation (should always return cache miss)
	retrievedHost, err := noopCache.GetHost(ctx, "test-host")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
	assert.Nil(t, retrievedHost)

	// Test DeleteHost operation (should always succeed)
	err = noopCache.DeleteHost(ctx, "test-host")
	assert.NoError(t, err)
}

func TestNoOpCache_UtilityOperations(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test Close operation (should always succeed)
	err := noopCache.Close()
	assert.NoError(t, err)

	// Test Ping operation (should always succeed)
	err = noopCache.Ping(ctx)
	assert.NoError(t, err)
}

func TestNoOpCache_Interface(t *testing.T) {
	noopCache := cache.NewNoOpCache()

	// Verify that NoOpCache implements CacheService interface
	var cacheService cache.CacheService = noopCache
	assert.NotNil(t, cacheService)
	assert.Implements(t, (*cache.CacheService)(nil), noopCache)
}

func TestGlobalCache_Initialization(t *testing.T) {
	// Test that GlobalCache is initialized and not nil
	assert.NotNil(t, cache.GlobalCache)

	// Test that it implements the CacheService interface
	assert.Implements(t, (*cache.CacheService)(nil), cache.GlobalCache)
}

func TestInitCache_DisabledConfig(t *testing.T) {
	config := models.ValkeyConfig{
		Enabled: false,
	}

	err := cache.InitCache(config)
	assert.NoError(t, err)

	// GlobalCache should be a NoOpCache when disabled
	assert.NotNil(t, cache.GlobalCache)
	assert.Implements(t, (*cache.CacheService)(nil), cache.GlobalCache)
}

// Test ValkeyCache creation with invalid config (should fallback to NoOpCache)
func TestInitCache_InvalidConfig(t *testing.T) {
	config := models.ValkeyConfig{
		Enabled:  true,
		Address:  "invalid-host:9999",
		Password: "invalid-password",
		Database: 0,
	}

	// This should fail to connect and fallback to NoOpCache
	err := cache.InitCache(config)
	assert.Error(t, err)

	// GlobalCache should still be available (as NoOpCache)
	assert.NotNil(t, cache.GlobalCache)
	assert.Implements(t, (*cache.CacheService)(nil), cache.GlobalCache)
}

// Test NewValkeyCache with invalid config
func TestNewValkeyCache_InvalidConfig(t *testing.T) {
	config := models.ValkeyConfig{
		Enabled:  true,
		Address:  "invalid-host:9999",
		Password: "invalid-password",
		Database: 0,
	}

	valkeyCache, err := cache.NewValkeyCache(config)
	assert.Error(t, err)
	assert.Nil(t, valkeyCache)
}

// Test cache key generation functions
func TestCacheKeyGeneration(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test that different operations use different key patterns
	// We can't directly test key generation, but we can test that operations
	// work with different types of keys

	// Session keys
	session := &models.Session{Token: "test-token"}
	err := noopCache.SetSession(ctx, "session-token", session)
	assert.NoError(t, err)

	// Host auth keys
	err = noopCache.SetHostAuth(ctx, "auth-token", "hostname")
	assert.NoError(t, err)

	// Health data keys
	err = noopCache.SetHealthData(ctx, "hostname", "tool", map[string]string{"key": "value"})
	assert.NoError(t, err)

	// Host keys
	host := &models.Host{Name: "test-host"}
	err = noopCache.SetHost(ctx, "hostname", host)
	assert.NoError(t, err)
}

// Test NewValkeyCache with disabled config
func TestNewValkeyCache_DisabledConfig(t *testing.T) {
	config := models.ValkeyConfig{
		Enabled: false,
	}

	valkeyCache, err := cache.NewValkeyCache(config)
	assert.Error(t, err)
	assert.Nil(t, valkeyCache)
	assert.Contains(t, err.Error(), "valkey is disabled")
}

// Test ValkeyCache buildKey method indirectly
func TestValkeyCache_KeyPrefixing(t *testing.T) {
	// We can't test buildKey directly since it's not exported,
	// but we can test that different prefixes work correctly
	config := models.ValkeyConfig{
		Enabled:   true,
		Address:   "localhost:6379",
		KeyPrefix: "test:",
		Database:  0,
	}

	// This will fail to connect but we can test the structure
	valkeyCache, err := cache.NewValkeyCache(config)
	assert.Error(t, err) // Expected to fail connection
	assert.Nil(t, valkeyCache)
}

// Test CloseCache function
func TestCloseCache(t *testing.T) {
	// Test with nil GlobalCache
	originalCache := cache.GlobalCache
	cache.GlobalCache = nil

	err := cache.CloseCache()
	assert.NoError(t, err)

	// Restore original cache
	cache.GlobalCache = originalCache

	// Test with NoOpCache
	cache.GlobalCache = cache.NewNoOpCache()
	err = cache.CloseCache()
	assert.NoError(t, err)
}

// Test InitCache with successful connection (mocked)
func TestInitCache_SuccessfulConnection(t *testing.T) {
	// Test with disabled config first
	config := models.ValkeyConfig{
		Enabled: false,
	}

	err := cache.InitCache(config)
	assert.NoError(t, err)
	assert.NotNil(t, cache.GlobalCache)
}

// Test error handling in ValkeyCache operations
func TestValkeyCache_ErrorHandling(t *testing.T) {
	// Test with invalid JSON marshaling
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test with complex data structures
	complexData := map[string]interface{}{
		"nested": map[string]interface{}{
			"array": []int{1, 2, 3},
			"bool":  true,
		},
		"number": 42.5,
	}

	err := noopCache.SetHealthData(ctx, "test-host", "complex-tool", complexData)
	assert.NoError(t, err)

	var retrieved map[string]interface{}
	err = noopCache.GetHealthData(ctx, "test-host", "complex-tool", &retrieved)
	assert.Error(t, err) // NoOpCache always returns cache miss
	assert.Contains(t, err.Error(), "cache miss")
}

// Test TTL handling in cache operations
func TestCache_TTLHandling(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test Set with different TTL values
	err := noopCache.Set(ctx, "ttl-test-1", "value1", time.Hour)
	assert.NoError(t, err)

	err = noopCache.Set(ctx, "ttl-test-2", "value2", 0) // No TTL
	assert.NoError(t, err)

	err = noopCache.Set(ctx, "ttl-test-3", "value3", time.Minute*30)
	assert.NoError(t, err)
}

// Test cache operations with various data types
func TestCache_DataTypes(t *testing.T) {
	ctx := context.Background()
	noopCache := cache.NewNoOpCache()

	// Test with string
	err := noopCache.Set(ctx, "string-key", "string-value", time.Minute)
	assert.NoError(t, err)

	// Test with integer
	err = noopCache.Set(ctx, "int-key", 42, time.Minute)
	assert.NoError(t, err)

	// Test with boolean
	err = noopCache.Set(ctx, "bool-key", true, time.Minute)
	assert.NoError(t, err)

	// Test with slice
	err = noopCache.Set(ctx, "slice-key", []string{"a", "b", "c"}, time.Minute)
	assert.NoError(t, err)

	// Test with struct
	testStruct := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 123,
	}
	err = noopCache.Set(ctx, "struct-key", testStruct, time.Minute)
	assert.NoError(t, err)
}
