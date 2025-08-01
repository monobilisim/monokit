//go:build with_api

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/cache"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Host:     "invalid-host",
		Port:     9999,
		Password: "invalid-password",
		DB:       0,
		Timeout:  1, // Very short timeout to ensure failure
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
		Host:     "invalid-host",
		Port:     9999,
		Password: "invalid-password",
		DB:       0,
		Timeout:  1,
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
