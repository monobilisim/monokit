//go:build with_api

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
	"github.com/valkey-io/valkey-go"
)

// CacheService defines the interface for cache operations
type CacheService interface {
	// Basic operations
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	FlushAll(ctx context.Context) error

	// Specialized operations for common use cases
	SetSession(ctx context.Context, token string, session *models.Session) error
	GetSession(ctx context.Context, token string) (*models.Session, error)
	DeleteSession(ctx context.Context, token string) error

	SetHostAuth(ctx context.Context, token string, hostName string) error
	GetHostAuth(ctx context.Context, token string) (string, error)
	DeleteHostAuth(ctx context.Context, token string) error

	SetHealthData(ctx context.Context, hostName, toolName string, data interface{}) error
	GetHealthData(ctx context.Context, hostName, toolName string, dest interface{}) error
	DeleteHealthData(ctx context.Context, hostName, toolName string) error

	SetHost(ctx context.Context, hostName string, host *models.Host) error
	GetHost(ctx context.Context, hostName string) (*models.Host, error)
	DeleteHost(ctx context.Context, hostName string) error

	// Utility methods
	Close() error
	Ping(ctx context.Context) error
}

// ValkeyCache implements CacheService using Valkey
type ValkeyCache struct {
	client valkey.Client
	config models.ValkeyConfig
}

// NewValkeyCache creates a new Valkey cache instance
func NewValkeyCache(config models.ValkeyConfig) (*ValkeyCache, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("valkey is disabled in configuration")
	}

	// Build connection options
	options := valkey.ClientOption{
		InitAddress:      []string{config.Address},
		Password:         config.Password,
		SelectDB:         config.Database,
		ConnWriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
	}

	// Create client
	client, err := valkey.NewClient(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	return &ValkeyCache{
		client: client,
		config: config,
	}, nil
}

// buildKey creates a prefixed key
func (v *ValkeyCache) buildKey(key string) string {
	return v.config.KeyPrefix + key
}

// Set stores a value with TTL
func (v *ValkeyCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if ttl > 0 {
		return v.client.Do(ctx, v.client.B().Set().Key(v.buildKey(key)).Value(string(data)).Ex(ttl).Build()).Error()
	} else {
		return v.client.Do(ctx, v.client.B().Set().Key(v.buildKey(key)).Value(string(data)).Build()).Error()
	}
}

// Get retrieves a value and unmarshals it
func (v *ValkeyCache) Get(ctx context.Context, key string, dest interface{}) error {
	result := v.client.Do(ctx, v.client.B().Get().Key(v.buildKey(key)).Build())
	if result.Error() != nil {
		return result.Error()
	}

	data, err := result.ToString()
	if err != nil {
		return fmt.Errorf("failed to get string value: %w", err)
	}

	return json.Unmarshal([]byte(data), dest)
}

// Delete removes a key
func (v *ValkeyCache) Delete(ctx context.Context, key string) error {
	return v.client.Do(ctx, v.client.B().Del().Key(v.buildKey(key)).Build()).Error()
}

// Exists checks if a key exists
func (v *ValkeyCache) Exists(ctx context.Context, key string) (bool, error) {
	result := v.client.Do(ctx, v.client.B().Exists().Key(v.buildKey(key)).Build())
	if result.Error() != nil {
		return false, result.Error()
	}

	count, err := result.ToInt64()
	return count > 0, err
}

// FlushAll clears all keys with the configured prefix
func (v *ValkeyCache) FlushAll(ctx context.Context) error {
	// Get all keys with our prefix
	pattern := v.config.KeyPrefix + "*"
	result := v.client.Do(ctx, v.client.B().Keys().Pattern(pattern).Build())
	if result.Error() != nil {
		return result.Error()
	}

	keys, err := result.AsStrSlice()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete all keys
	return v.client.Do(ctx, v.client.B().Del().Key(keys...).Build()).Error()
}

// SetSession stores a session with session-specific TTL
func (v *ValkeyCache) SetSession(ctx context.Context, token string, session *models.Session) error {
	ttl := time.Duration(v.config.SessionTTL) * time.Second
	return v.Set(ctx, "session:"+token, session, ttl)
}

// GetSession retrieves a session
func (v *ValkeyCache) GetSession(ctx context.Context, token string) (*models.Session, error) {
	var session models.Session
	err := v.Get(ctx, "session:"+token, &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession removes a session
func (v *ValkeyCache) DeleteSession(ctx context.Context, token string) error {
	return v.Delete(ctx, "session:"+token)
}

// SetHostAuth stores host authentication with host-specific TTL
func (v *ValkeyCache) SetHostAuth(ctx context.Context, token string, hostName string) error {
	ttl := time.Duration(v.config.HostTTL) * time.Second
	return v.Set(ctx, "hostauth:"+token, hostName, ttl)
}

// GetHostAuth retrieves host name by token
func (v *ValkeyCache) GetHostAuth(ctx context.Context, token string) (string, error) {
	var hostName string
	err := v.Get(ctx, "hostauth:"+token, &hostName)
	return hostName, err
}

// DeleteHostAuth removes host authentication
func (v *ValkeyCache) DeleteHostAuth(ctx context.Context, token string) error {
	return v.Delete(ctx, "hostauth:"+token)
}

// SetHealthData stores health data with health-specific TTL
func (v *ValkeyCache) SetHealthData(ctx context.Context, hostName, toolName string, data interface{}) error {
	ttl := time.Duration(v.config.HealthTTL) * time.Second
	return v.Set(ctx, fmt.Sprintf("health:%s:%s", hostName, toolName), data, ttl)
}

// GetHealthData retrieves health data
func (v *ValkeyCache) GetHealthData(ctx context.Context, hostName, toolName string, dest interface{}) error {
	return v.Get(ctx, fmt.Sprintf("health:%s:%s", hostName, toolName), dest)
}

// DeleteHealthData removes health data
func (v *ValkeyCache) DeleteHealthData(ctx context.Context, hostName, toolName string) error {
	return v.Delete(ctx, fmt.Sprintf("health:%s:%s", hostName, toolName))
}

// SetHost stores host information with host-specific TTL
func (v *ValkeyCache) SetHost(ctx context.Context, hostName string, host *models.Host) error {
	ttl := time.Duration(v.config.HostTTL) * time.Second
	return v.Set(ctx, "host:"+hostName, host, ttl)
}

// GetHost retrieves host information
func (v *ValkeyCache) GetHost(ctx context.Context, hostName string) (*models.Host, error) {
	var host models.Host
	err := v.Get(ctx, "host:"+hostName, &host)
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// DeleteHost removes host information
func (v *ValkeyCache) DeleteHost(ctx context.Context, hostName string) error {
	return v.Delete(ctx, "host:"+hostName)
}

// Close closes the Valkey client
func (v *ValkeyCache) Close() error {
	v.client.Close()
	return nil
}

// Ping checks if the Valkey server is reachable
func (v *ValkeyCache) Ping(ctx context.Context) error {
	return v.client.Do(ctx, v.client.B().Ping().Build()).Error()
}

// NoOpCache is a no-op implementation for when caching is disabled
type NoOpCache struct{}

// NewNoOpCache creates a no-op cache instance
func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

// No-op implementations
func (n *NoOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (n *NoOpCache) Get(ctx context.Context, key string, dest interface{}) error {
	return fmt.Errorf("cache miss: no-op cache")
}

func (n *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (n *NoOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (n *NoOpCache) FlushAll(ctx context.Context) error {
	return nil
}

func (n *NoOpCache) SetSession(ctx context.Context, token string, session *models.Session) error {
	return nil
}

func (n *NoOpCache) GetSession(ctx context.Context, token string) (*models.Session, error) {
	return nil, fmt.Errorf("cache miss: no-op cache")
}

func (n *NoOpCache) DeleteSession(ctx context.Context, token string) error {
	return nil
}

func (n *NoOpCache) SetHostAuth(ctx context.Context, token string, hostName string) error {
	return nil
}

func (n *NoOpCache) GetHostAuth(ctx context.Context, token string) (string, error) {
	return "", fmt.Errorf("cache miss: no-op cache")
}

func (n *NoOpCache) DeleteHostAuth(ctx context.Context, token string) error {
	return nil
}

func (n *NoOpCache) SetHealthData(ctx context.Context, hostName, toolName string, data interface{}) error {
	return nil
}

func (n *NoOpCache) GetHealthData(ctx context.Context, hostName, toolName string, dest interface{}) error {
	return fmt.Errorf("cache miss: no-op cache")
}

func (n *NoOpCache) DeleteHealthData(ctx context.Context, hostName, toolName string) error {
	return nil
}

func (n *NoOpCache) SetHost(ctx context.Context, hostName string, host *models.Host) error {
	return nil
}

func (n *NoOpCache) GetHost(ctx context.Context, hostName string) (*models.Host, error) {
	return nil, fmt.Errorf("cache miss: no-op cache")
}

func (n *NoOpCache) DeleteHost(ctx context.Context, hostName string) error {
	return nil
}

func (n *NoOpCache) Close() error {
	return nil
}

func (n *NoOpCache) Ping(ctx context.Context) error {
	return nil
}

// Global cache instance
var GlobalCache CacheService

// init ensures GlobalCache is never nil
func init() {
	GlobalCache = NewNoOpCache()
}

// InitCache initializes the global cache instance
func InitCache(config models.ValkeyConfig) error {
	if !config.Enabled {
		log.Info().Msg("Valkey cache is disabled, using no-op cache")
		GlobalCache = NewNoOpCache()
		return nil
	}

	cache, err := NewValkeyCache(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize Valkey cache, falling back to no-op cache")
		GlobalCache = NewNoOpCache()
		return err
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cache.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to ping Valkey server, falling back to no-op cache")
		GlobalCache = NewNoOpCache()
		return err
	}

	log.Info().
		Str("address", config.Address).
		Int("database", config.Database).
		Msg("Valkey cache initialized successfully")

	GlobalCache = cache
	return nil
}

// CloseCache closes the global cache instance
func CloseCache() error {
	if GlobalCache != nil {
		return GlobalCache.Close()
	}
	return nil
}
