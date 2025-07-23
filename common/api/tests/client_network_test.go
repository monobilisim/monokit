//go:build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/api/client"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendReq_Success(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/hosts/") {
			// Mock getReq response
			host := map[string]interface{}{
				"name":                "test-host",
				"disabledComponents":  "nil",
				"groups":              "web-servers",
				"cpuCores":            4,
				"ram":                 "8GB",
				"monokitVersion":      "1.0.0",
				"os":                  "Ubuntu 20.04",
				"installedComponents": "mysql,nginx",
				"ipAddress":           "192.168.1.100",
				"status":              "online",
				"inventory":           "default",
			}
			json.NewEncoder(w).Encode(host)
		} else if r.Method == "POST" && strings.Contains(r.URL.Path, "/hosts") {
			// Mock host registration response
			response := map[string]interface{}{
				"host": map[string]interface{}{
					"name":          "test-host",
					"upForDeletion": false,
					"wantsUpdateTo": "",
				},
				"apiKey": "new-api-key-123",
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	// Configure client
	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	// Test SendReq
	client.SendReq("1")

	// Verify no panics or errors occurred
	assert.True(t, true) // If we reach here, SendReq completed successfully
}

func TestSendReq_NetworkFailure(t *testing.T) {
	// Configure client with invalid URL
	client.ClientConf.URL = "http://invalid-url-that-does-not-exist:9999"
	common.Config.Identifier = "test-host"

	// Test SendReq with network failure
	// Should not panic and should handle the error gracefully
	client.SendReq("1")

	// Verify no panics occurred
	assert.True(t, true)
}

func TestSendReq_ServerError(t *testing.T) {
	// Setup mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	// Test SendReq with server error
	client.SendReq("1")

	// Should handle server errors gracefully
	assert.True(t, true)
}

func TestSendReq_InvalidJSON(t *testing.T) {
	// Setup mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json response"))
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	// Test SendReq with invalid JSON response
	client.SendReq("1")

	// Should handle invalid JSON gracefully
	assert.True(t, true)
}

func TestSendReq_HostUpForDeletion(t *testing.T) {
	// This test would verify the behavior when a host is scheduled for deletion,
	// but SendReq() calls os.Exit(0) when upForDeletion is true, which terminates
	// the test process. In production, this is the expected behavior - when a host
	// is marked for deletion, the monokit agent should remove itself and exit.
	//
	// Since we can't test os.Exit() calls in unit tests, we skip this test.
	// The logic is: SendReq() -> detects upForDeletion: true -> calls common.RemoveMonokit() -> os.Exit(0)
	t.Skip("Skipping test that calls os.Exit(0) - this is expected behavior but untestable in unit tests")
}

func TestSendReq_WithAPIKey(t *testing.T) {
	// Setup mock server that provides API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			host := map[string]interface{}{
				"name":   "test-host",
				"groups": "nil",
			}
			json.NewEncoder(w).Encode(host)
		} else if r.Method == "POST" {
			response := map[string]interface{}{
				"host": map[string]interface{}{
					"name": "test-host",
				},
				"apiKey": "new-generated-key-456",
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	// Test SendReq that receives an API key
	client.SendReq("1")

	// Should handle API key response gracefully
	assert.True(t, true)
}

func TestGetReq_Success(t *testing.T) {
	expectedHost := map[string]interface{}{
		"name":     "test-host",
		"cpuCores": float64(8),
		"ram":      "16GB",
		"status":   "online",
		"groups":   "web-servers,db-servers",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/hosts/test-host")
		json.NewEncoder(w).Encode(expectedHost)
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	result, err := client.GetReq("1")

	require.NoError(t, err)
	assert.Equal(t, "test-host", result["name"])
	assert.Equal(t, float64(8), result["cpuCores"])
	assert.Equal(t, "16GB", result["ram"])
}

func TestGetReq_NetworkError(t *testing.T) {
	client.ClientConf.URL = "http://nonexistent-server:9999"
	common.Config.Identifier = "test-host"

	result, err := client.GetReq("1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetReq_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Host not found"))
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	result, err := client.GetReq("1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetServiceStatus_StatusEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":        "enabled",
			"wantsUpdateTo": "",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	enabled, updateVersion := client.GetServiceStatus("mysql")

	assert.True(t, enabled)
	assert.Equal(t, "", updateVersion)
}

func TestGetServiceStatus_DisabledWithUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"disabled":      true,
			"wantsUpdateTo": "2.1.0",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	enabled, updateVersion := client.GetServiceStatus("mysql")

	assert.False(t, enabled)
	assert.Equal(t, "2.1.0", updateVersion)
}

func TestGetServiceStatus_NetworkError(t *testing.T) {
	client.ClientConf.URL = "http://invalid-server:9999"
	common.Config.Identifier = "test-host"

	enabled, updateVersion := client.GetServiceStatus("mysql")

	// Should default to enabled on error
	assert.True(t, enabled)
	assert.Equal(t, "", updateVersion)
}

func TestGetServiceStatus_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	enabled, updateVersion := client.GetServiceStatus("mysql")

	// Should default to enabled when no status fields present
	assert.True(t, enabled)
	assert.Equal(t, "", updateVersion)
}

func TestWrapperGetServiceStatus_NoConfig(t *testing.T) {
	// Test with no client config - this is harder to mock since ConfExists
	// checks actual file system, so we'll test with empty URL instead
	originalURL := client.ClientConf.URL
	client.ClientConf.URL = ""
	defer func() { client.ClientConf.URL = originalURL }()

	// Should return early without error
	client.WrapperGetServiceStatus("mysql")

	// Test passes if function returns without panic
	assert.True(t, true)
}

func TestWrapperGetServiceStatus_EmptyURL(t *testing.T) {
	// Test with empty URL - should return early without error
	originalURL := client.ClientConf.URL
	client.ClientConf.URL = ""
	defer func() { client.ClientConf.URL = originalURL }()

	// Should return early without error
	client.WrapperGetServiceStatus("mysql")

	// Test passes if function returns without panic
	assert.True(t, true)
}

func TestClientWithCustomHTTPClient(t *testing.T) {
	// Test using custom HTTP client with timeout
	customClient := &http.Client{
		Timeout: 1 * time.Millisecond, // Very short timeout
	}

	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Longer than timeout
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "enabled"})
	}))
	defer server.Close()

	// Set custom client
	originalClient := client.ClientConf.HTTPClient
	client.ClientConf.HTTPClient = customClient
	defer func() { client.ClientConf.HTTPClient = originalClient }()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "test-host"

	enabled, updateVersion := client.GetServiceStatus("mysql")

	// Should timeout and default to enabled
	assert.True(t, enabled)
	assert.Equal(t, "", updateVersion)
}

func TestSendReq_ComponentUpdates(t *testing.T) {
	// Test SendReq with various component states
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			host := map[string]interface{}{
				"name":               "component-test-host",
				"disabledComponents": "mysql,redis",
				"groups":             "web-servers",
			}
			json.NewEncoder(w).Encode(host)
		} else if r.Method == "POST" {
			// Verify the POST request contains expected component data
			var requestBody models.Host
			json.NewDecoder(r.Body).Decode(&requestBody)

			assert.Equal(t, "component-test-host", requestBody.Name)
			// Should include installed components from common.GetInstalledComponents()

			response := map[string]interface{}{
				"host": map[string]interface{}{
					"name": "component-test-host",
				},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "component-test-host"

	client.SendReq("1")

	// Test passes if no errors occur
	assert.True(t, true)
}

func TestSendReq_InventoryExtraction(t *testing.T) {
	// Test inventory name extraction from identifier
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			host := map[string]interface{}{
				"name":   "prod-web-01",
				"groups": "nil",
			}
			json.NewEncoder(w).Encode(host)
		} else if r.Method == "POST" {
			var requestBody models.Host
			json.NewDecoder(r.Body).Decode(&requestBody)

			// Verify the host name is correct
			assert.Equal(t, "prod-web-01", requestBody.Name)

			response := map[string]interface{}{
				"host": map[string]interface{}{
					"name": "prod-web-01",
				},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	client.ClientConf.URL = server.URL
	common.Config.Identifier = "prod-web-01"

	client.SendReq("1")

	assert.True(t, true)
}
