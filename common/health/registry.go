package health

import (
	"fmt"
	"sync"
)

// Provider defines the interface for a health check provider.
// Each health tool (osHealth, zimbraHealth, etc.) should implement this.
type Provider interface {
	// Name returns the unique name of the health provider (e.g., "osHealth").
	Name() string
	// Collect gathers health data for the specified host.
	// It should return a struct or map that can be marshalled to JSON,
	// compatible with the <health-card> web component.
	Collect(hostname string) (interface{}, error)
}

var (
	registry = make(map[string]Provider)
	mu       sync.RWMutex
)

// Register adds a health provider to the central registry.
// This should be called by each health module during its init() phase.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	if p == nil {
		// Or panic, depending on desired strictness
		fmt.Println("Cannot register a nil health provider")
		return
	}
	name := p.Name()
	if _, exists := registry[name]; exists {
		fmt.Printf("Health provider already registered: %s\n", name) // Or log an error
		return
	}
	registry[name] = p
}

// Get retrieves a registered health provider by its name.
// Returns nil if the provider is not found.
func Get(name string) Provider {
	mu.RLock()
	defer mu.RUnlock()
	provider, ok := registry[name]
	if !ok {
		return nil
	}
	return provider
}

// List returns the names of all registered health providers.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// GetAllProviders returns a map of all registered health providers.
// This is useful for iterating over all providers.
func GetAllProviders() map[string]Provider {
	mu.RLock()
	defer mu.RUnlock()
	// Return a copy to prevent external modification
	providersCopy := make(map[string]Provider)
	for name, p := range registry {
		providersCopy[name] = p
	}
	return providersCopy
}
