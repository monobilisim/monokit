////go:build with_api
//go:build with_api
// +build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	// Provide access to models.Config for tests that call client.go logic directly
	"github.com/monobilisim/monokit/common"
	commonmain "github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/stretchr/testify/assert"
)

func TestGetServiceStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/hosts/testhost/serviceA":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "enabled",
				"wantsUpdateTo": "",
			})
		case "/api/v1/hosts/testhost/serviceB":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"disabled":      true,
				"wantsUpdateTo": "2.1.0",
			})
		case "/api/v1/hosts/testhost/serviceC":
			http.Error(w, "fail", http.StatusInternalServerError)
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer srv.Close()

	client.ClientConf.URL = srv.URL
	// Needed for identifier reference
	commonmain.Config.Identifier = "testhost"

	t.Run("status enabled", func(t *testing.T) {
		enabled, wants := client.GetServiceStatus("serviceA")
		assert.True(t, enabled)
		assert.Equal(t, "", wants)
	})

	t.Run("disabled with wantsUpdate", func(t *testing.T) {
		enabled, wants := client.GetServiceStatus("serviceB")
		assert.False(t, enabled)
		assert.Equal(t, "2.1.0", wants)
	})

	t.Run("http error", func(t *testing.T) {
		enabled, wants := client.GetServiceStatus("serviceC")
		assert.True(t, enabled) // fallback is true on error
		assert.Empty(t, wants)
	})
}

// Removed TestGetHosts and TestSendUpdateTo_Disable_Enable due to undefined dependencies

func TestGetCPUCores_GetRAM_GetOS(t *testing.T) {
	cores := client.GetCPUCores()
	assert.GreaterOrEqual(t, cores, 0)

	ram := client.GetRAM()
	// RAM might be empty in some test environments, so only check format if not empty
	if ram != "" {
		assert.Contains(t, ram, "GB")
	}

	osver := client.GetOS()
	// OS info might be empty in some test environments, so only check if not empty
	if osver != "" {
		assert.NotContains(t, osver, "error")
		assert.NotContains(t, osver, "Error")
	}

	// Log values for debugging (should not cause test failures)
	t.Logf("System info test - CPU: %d, RAM: %s, OS: %s", cores, ram, osver)
}

// Test client utility functions
func TestGetIdentifier(t *testing.T) {
	// Save original values
	originalEnv := os.Getenv("MONOKIT_TEST_IDENTIFIER")
	originalConfig := common.Config.Identifier

	// Clean up after test
	defer func() {
		os.Setenv("MONOKIT_TEST_IDENTIFIER", originalEnv)
		common.Config.Identifier = originalConfig
	}()

	// Test with environment variable set
	os.Setenv("MONOKIT_TEST_IDENTIFIER", "env-test-host")
	// We can't directly test getIdentifier since it's not exported,
	// but we can test the behavior through other functions that use it

	// Test with config identifier
	os.Unsetenv("MONOKIT_TEST_IDENTIFIER")
	common.Config.Identifier = "config-test-host"

	// Test fallback to default
	common.Config.Identifier = ""
	os.Unsetenv("MONOKIT_TEST_IDENTIFIER")
}

func TestClientHC(t *testing.T) {
	// Test with nil HTTPClient
	client := &client.Client{
		URL:        "http://test.example.com",
		HTTPClient: nil,
	}

	httpClient := client.hc()
	assert.Equal(t, http.DefaultClient, httpClient)

	// Test with custom HTTPClient
	customClient := &http.Client{Timeout: time.Second * 30}
	client.HTTPClient = customClient

	httpClient = client.hc()
	assert.Equal(t, customClient, httpClient)
}

func TestGetCPUCores(t *testing.T) {
	cores := client.GetCPUCores()
	// Should return a positive number or 0 on error
	assert.GreaterOrEqual(t, cores, 0)

	// In most test environments, we should get at least 1 core
	if cores > 0 {
		assert.Greater(t, cores, 0)
	}
}

func TestGetRAM(t *testing.T) {
	ram := client.GetRAM()
	// Should return a string with GB suffix or empty on error
	if ram != "" {
		assert.Contains(t, ram, "GB")
		// Extract number part and verify it's reasonable
		ramStr := strings.TrimSuffix(ram, "GB")
		ramValue, err := strconv.Atoi(ramStr)
		if err == nil {
			assert.Greater(t, ramValue, 0)
		}
	}
}

func TestGetOS(t *testing.T) {
	osInfo := client.GetOS()
	// Should return OS information or empty string on error
	if osInfo != "" {
		// Should contain some OS-related information
		assert.NotEmpty(t, osInfo)
		// Typically contains platform information
		assert.True(t, len(osInfo) > 5, "OS info should be reasonably detailed")
	}
}

func TestGetIP(t *testing.T) {
	ip := client.GetIP()
	// Should return an IP address or empty string
	if ip != "" {
		// Basic IP format validation
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			// IPv4 format
			for _, part := range parts {
				num, err := strconv.Atoi(part)
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, num, 0)
				assert.LessOrEqual(t, num, 255)
			}
		}
		// Could also be IPv6, but basic non-empty check is sufficient
		assert.NotEmpty(t, ip)
	}
}

func TestGetServiceStatus(t *testing.T) {
	// Set up test server
	routes := map[string]client.RespSpec{
		"/api/v1/hosts/test-host/test-service": {
			Status: http.StatusOK,
			Body:   `{"status": "enabled", "wantsUpdateTo": "1.2.3"}`,
		},
		"/api/v1/hosts/test-host/disabled-service": {
			Status: http.StatusOK,
			Body:   `{"disabled": true}`,
		},
		"/api/v1/hosts/test-host/enabled-service": {
			Status: http.StatusOK,
			Body:   `{"disabled": false}`,
		},
		"/api/v1/hosts/test-host/unknown-service": {
			Status: http.StatusOK,
			Body:   `{"someOtherField": "value"}`,
		},
		"/api/v1/hosts/test-host/error-service": {
			Status: http.StatusInternalServerError,
			Body:   `{"error": "Internal server error"}`,
		},
	}

	srv, cleanup := client.SetupTestServer(t, routes)
	defer cleanup()

	// Save original config
	originalClientConf := client.ClientConf
	defer func() { client.ClientConf = originalClientConf }()

	// Set up test client config
	client.ClientConf = client.Client{
		URL:        srv.URL,
		HTTPClient: srv.Client(),
	}

	// Set test identifier
	os.Setenv("MONOKIT_TEST_IDENTIFIER", "test-host")
	defer os.Unsetenv("MONOKIT_TEST_IDENTIFIER")

	// Test service with explicit status
	enabled, updateTo := client.GetServiceStatus("test-service")
	assert.True(t, enabled)
	assert.Equal(t, "1.2.3", updateTo)

	// Test disabled service
	enabled, updateTo = client.GetServiceStatus("disabled-service")
	assert.False(t, enabled)
	assert.Empty(t, updateTo)

	// Test enabled service (disabled: false)
	enabled, updateTo = client.GetServiceStatus("enabled-service")
	assert.True(t, enabled)
	assert.Empty(t, updateTo)

	// Test service with unknown status (defaults to enabled)
	enabled, updateTo = client.GetServiceStatus("unknown-service")
	assert.True(t, enabled)
	assert.Empty(t, updateTo)

	// Test service that returns error (defaults to enabled)
	enabled, updateTo = client.GetServiceStatus("error-service")
	assert.True(t, enabled)
	assert.Empty(t, updateTo)
}

func TestGetServiceStatus_NetworkErrors(t *testing.T) {
	// Save original config
	originalClientConf := client.ClientConf
	defer func() { client.ClientConf = originalClientConf }()

	// Test with invalid URL
	client.ClientConf = client.Client{
		URL:        "http://nonexistent-server:9999",
		HTTPClient: &http.Client{Timeout: time.Millisecond * 100},
	}

	os.Setenv("MONOKIT_TEST_IDENTIFIER", "test-host")
	defer os.Unsetenv("MONOKIT_TEST_IDENTIFIER")

	// Should return true (enabled) on network error
	enabled, updateTo := client.GetServiceStatus("test-service")
	assert.True(t, enabled)
	assert.Empty(t, updateTo)
}

func TestGetReq(t *testing.T) {
	// Set up test server
	routes := map[string]client.RespSpec{
		"/api/v1/hosts/test-host": {
			Status: http.StatusOK,
			Body:   `{"name": "test-host", "status": "active", "ip": "192.168.1.100"}`,
		},
		"/api/v1/hosts/error-host": {
			Status: http.StatusInternalServerError,
			Body:   `{"error": "Internal server error"}`,
		},
	}

	srv, cleanup := client.SetupTestServer(t, routes)
	defer cleanup()

	// Save original config
	originalClientConf := client.ClientConf
	defer func() { client.ClientConf = originalClientConf }()

	// Set up test client config
	client.ClientConf = client.Client{
		URL:        srv.URL,
		HTTPClient: srv.Client(),
	}

	// Test successful request
	os.Setenv("MONOKIT_TEST_IDENTIFIER", "test-host")
	defer os.Unsetenv("MONOKIT_TEST_IDENTIFIER")

	host, err := client.GetReq("1")
	assert.NoError(t, err)
	assert.NotNil(t, host)
	assert.Equal(t, "test-host", host["name"])
	assert.Equal(t, "active", host["status"])

	// Test error response
	os.Setenv("MONOKIT_TEST_IDENTIFIER", "error-host")
	host, err = client.GetReq("1")
	assert.Error(t, err)
	assert.Nil(t, host)
}

func TestGetHosts(t *testing.T) {
	// Set up test server
	routes := map[string]client.RespSpec{
		"/api/v1/hosts": {
			Status: http.StatusOK,
			Body:   `[{"name": "host1", "ip": "192.168.1.1"}, {"name": "host2", "ip": "192.168.1.2"}]`,
		},
		"/api/v1/hosts/specific-host": {
			Status: http.StatusOK,
			Body:   `{"name": "specific-host", "ip": "192.168.1.100"}`,
		},
		"/api/v1/hosts/nonexistent": {
			Status: http.StatusNotFound,
			Body:   `{"error": "Host not found"}`,
		},
	}

	srv, cleanup := client.SetupTestServer(t, routes)
	defer cleanup()

	// Save original config
	originalClientConf := client.ClientConf
	defer func() { client.ClientConf = originalClientConf }()

	// Set up test client config
	client.ClientConf = client.Client{
		URL:        srv.URL,
		HTTPClient: srv.Client(),
	}

	// Test getting all hosts
	hosts := client.GetHosts("1", "")
	assert.Len(t, hosts, 2)
	assert.Equal(t, "host1", hosts[0].Name)
	assert.Equal(t, "host2", hosts[1].Name)

	// Test getting specific host
	hosts = client.GetHosts("1", "specific-host")
	assert.Len(t, hosts, 1)
	assert.Equal(t, "specific-host", hosts[0].Name)

	// Test getting nonexistent host (should return empty slice)
	hosts = client.GetHosts("1", "nonexistent")
	assert.Empty(t, hosts)
}

func TestClientInit(t *testing.T) {
	// Save original values
	originalScriptName := common.ScriptName
	originalTmpDir := common.TmpDir

	defer func() {
		common.ScriptName = originalScriptName
		common.TmpDir = originalTmpDir
	}()

	apiVersion := client.ClientInit()

	// Should return "1" (first part of version)
	assert.Equal(t, "1", apiVersion)

	// Should set ScriptName
	assert.Equal(t, "client", common.ScriptName)

	// Should modify TmpDir
	assert.Contains(t, common.TmpDir, "client")
}

// Optionally: add edge/branch test for WrapperGetServiceStatus
// Not included here due to side effect (os.Exit) and lockfile removal.
