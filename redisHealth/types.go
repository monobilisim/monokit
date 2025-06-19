//go:build plugin && linux

package redisHealth

// RedisHealthProvider implements the health.Provider interface
type RedisHealthProvider struct{}

// Name returns the name of the provider
func (p *RedisHealthProvider) Name() string {
	return "redisHealth"
}

// Collect gathers Redis health data.
// The 'hostname' parameter is ignored for redisHealth as it collects local data.
func (p *RedisHealthProvider) Collect(_ string) (interface{}, error) {
	return collectRedisHealthData()
}

// Plugin-specific types and interfaces are now in shared_types.go
