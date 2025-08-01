//go:build with_api

package tests

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monobilisim/monokit/common/api/clientport"
	"github.com/monobilisim/monokit/common/api/host"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostService_GetHosts_SingleHost(t *testing.T) {
	// Create a test server that returns a single host
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/hosts/test-host", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		host := models.Host{
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(host)
	}))
	defer server.Close()

	// Create host service
	hostService := &host.HostService{
		HTTP: &http.Client{},
		Conf: &host.Config{
			URL: server.URL,
		},
	}

	// Test getting single host
	hosts, err := hostService.GetHosts("1", "test-host")
	require.NoError(t, err)
	require.Len(t, hosts, 1)

	assert.Equal(t, "test-host", hosts[0].Name)
	assert.Equal(t, 8, hosts[0].CpuCores)
	assert.Equal(t, "16GB", hosts[0].Ram)
	assert.Equal(t, "Ubuntu 22.04", hosts[0].Os)
}

func TestHostService_GetHosts_AllHosts(t *testing.T) {
	// Create a test server that returns multiple hosts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/hosts", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		hosts := []models.Host{
			{
				Name:      "host1",
				CpuCores:  4,
				Ram:       "8GB",
				Os:        "Ubuntu 20.04",
				Status:    "online",
				IpAddress: "192.168.1.10",
			},
			{
				Name:      "host2",
				CpuCores:  8,
				Ram:       "16GB",
				Os:        "Ubuntu 22.04",
				Status:    "offline",
				IpAddress: "192.168.1.20",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hosts)
	}))
	defer server.Close()

	// Create host service
	hostService := &host.HostService{
		HTTP: &http.Client{},
		Conf: &host.Config{
			URL: server.URL,
		},
	}

	// Test getting all hosts
	hosts, err := hostService.GetHosts("1", "")
	require.NoError(t, err)
	require.Len(t, hosts, 2)

	assert.Equal(t, "host1", hosts[0].Name)
	assert.Equal(t, 4, hosts[0].CpuCores)
	assert.Equal(t, "host2", hosts[1].Name)
	assert.Equal(t, 8, hosts[1].CpuCores)
}

func TestHostService_GetHosts_HTTPError(t *testing.T) {
	// Create host service with invalid URL
	hostService := &host.HostService{
		HTTP: &http.Client{},
		Conf: &host.Config{
			URL: "http://invalid-url-that-does-not-exist",
		},
	}

	// Test getting hosts with HTTP error
	hosts, err := hostService.GetHosts("1", "test-host")
	assert.Error(t, err)
	assert.Nil(t, hosts)
}

func TestHostService_GetServiceStatus_Enabled(t *testing.T) {
	// Create a test server that returns enabled service status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/hosts/test-host/test-service", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		response := map[string]interface{}{
			"status":        "enabled",
			"wantsUpdateTo": "2.1.0",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create mock filesystem that returns a host key
	mockFS := &MockFS{
		ReadFileFunc: func(path string) ([]byte, error) {
			if path == "/var/lib/mono/api/hostkey/test-host" {
				return []byte("test-api-key"), nil
			}
			return nil, assert.AnError
		},
	}

	// Create host service
	hostService := &host.HostService{
		HTTP: &http.Client{},
		FS:   mockFS,
		Conf: &host.Config{
			URL:        server.URL,
			Identifier: "test-host",
			APIKeyDir:  "/var/lib/mono/api/hostkey",
		},
	}

	// Test getting service status
	enabled, wantsUpdateTo, err := hostService.GetServiceStatus("test-service")
	require.NoError(t, err)
	assert.True(t, enabled)
	assert.Equal(t, "2.1.0", wantsUpdateTo)
}

func TestHostService_GetServiceStatus_Disabled(t *testing.T) {
	// Create a test server that returns disabled service status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/hosts/test-host/test-service", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		response := map[string]interface{}{
			"disabled": true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create mock filesystem that returns no host key
	mockFS := &MockFS{
		ReadFileFunc: func(path string) ([]byte, error) {
			return nil, assert.AnError // Simulate no key file
		},
	}

	// Create host service
	hostService := &host.HostService{
		HTTP: &http.Client{},
		FS:   mockFS,
		Conf: &host.Config{
			URL:        server.URL,
			Identifier: "test-host",
			APIKeyDir:  "/var/lib/mono/api/hostkey",
		},
	}

	// Test getting service status
	enabled, wantsUpdateTo, err := hostService.GetServiceStatus("test-service")
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.Empty(t, wantsUpdateTo)
}

func TestHostService_GetServiceStatus_HTTPError(t *testing.T) {
	// Create host service with invalid URL
	hostService := &host.HostService{
		HTTP: &http.Client{},
		FS:   clientport.OSFS{},
		Conf: &host.Config{
			URL:        "http://invalid-url-that-does-not-exist",
			Identifier: "test-host",
			APIKeyDir:  "/var/lib/mono/api/hostkey",
		},
	}

	// Test getting service status with HTTP error
	enabled, wantsUpdateTo, err := hostService.GetServiceStatus("test-service")
	assert.Error(t, err)
	assert.True(t, enabled) // Should default to true on error
	assert.Empty(t, wantsUpdateTo)
}

func TestHostService_GetServiceStatus_DefaultBehavior(t *testing.T) {
	// Create a test server that returns empty response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			// No status or disabled field
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create host service
	hostService := &host.HostService{
		HTTP: &http.Client{},
		FS:   clientport.OSFS{},
		Conf: &host.Config{
			URL:        server.URL,
			Identifier: "test-host",
			APIKeyDir:  "/var/lib/mono/api/hostkey",
		},
	}

	// Test getting service status with default behavior
	enabled, wantsUpdateTo, err := hostService.GetServiceStatus("test-service")
	require.NoError(t, err)
	assert.True(t, enabled) // Should default to true
	assert.Empty(t, wantsUpdateTo)
}

// MockFS for testing filesystem operations
type MockFS struct {
	ReadFileFunc  func(path string) ([]byte, error)
	WriteFileFunc func(path string, data []byte, perm fs.FileMode) error
	MkdirAllFunc  func(path string, perm fs.FileMode) error
}

func (m *MockFS) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, assert.AnError
}

func (m *MockFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(path, data, perm)
	}
	return nil
}

func (m *MockFS) MkdirAll(path string, perm fs.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return nil
}
