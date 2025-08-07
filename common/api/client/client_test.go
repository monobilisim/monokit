package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/spf13/cobra"
)

func restoreSendReqShims(t *testing.T) (origSync func(cmd *cobra.Command, args []string), origGetReq func(string) (map[string]interface{}, error), origGetInstalled func() string) {
	origSyncFn := syncConfigFn
	origGetReqFn := getReqFn
	t.Cleanup(func() {
		syncConfigFn = origSyncFn
		getReqFn = origGetReqFn
		getInstalledComponents = origGetInstalled
	})
	return origSyncFn, origGetReqFn, origGetInstalled
}

func TestSendReq_Baseline(t *testing.T) {
	restoreSendReqShims(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/hosts/"):
			w.Write([]byte(`{"disabledComponents":"test-disable","groups":"test-group"}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/hosts":
			w.Write([]byte(`{"host":{},"apiKey":""}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	syncConfigFn = func(cmd *cobra.Command, args []string) {}
	getReqFn = func(_ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"disabledComponents": "test-disable",
			"groups":             "test-group",
		}, nil
	}
	getInstalledComponents = func() string { return "osHealth::demoComp" }

	os.Setenv("MONOKIT_TEST_IDENTIFIER", "testhost-abc")

	// Also set Config.Identifier to match fallback paths
}

func TestGetCPUCores(t *testing.T) {
	n := GetCPUCores()
	if n <= 0 {
		t.Errorf("CPUCores should be >0, got %d", n)
	}
}

func TestGetRAM(t *testing.T) {
	ram := GetRAM()
	if ram == "" || len(ram) < 2 {
		t.Errorf("GetRAM returned empty or too short string: %q", ram)
	}
}

/* Duplicate TestGetIP removed by Kilo Code */

func TestGetOS(t *testing.T) {
	osname := GetOS()
	if osname == "" || len(osname) < 2 {
		t.Errorf("GetOS empty or too short: %q", osname)
	}
}

// Test GetServiceStatus with various scenarios
func TestGetServiceStatus_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"enabled","wantsUpdateTo":"v2.0.0"}`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	status, updateVersion := GetServiceStatus("testservice")
	if !status {
		t.Errorf("Expected status to be true, got false")
	}
	if updateVersion != "v2.0.0" {
		t.Errorf("Expected updateVersion to be 'v2.0.0', got %q", updateVersion)
	}
}

func TestGetServiceStatus_Disabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"disabled":true}`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	status, _ := GetServiceStatus("testservice")
	if status {
		t.Errorf("Expected status to be false, got true")
	}
}

func TestGetServiceStatus_NetworkError(t *testing.T) {
	ClientConf = Client{URL: "http://nonexistent.example.com", HTTPClient: &http.Client{}}

	status, updateVersion := GetServiceStatus("testservice")
	if !status {
		t.Errorf("Expected status to be true (default on error), got false")
	}
	if updateVersion != "" {
		t.Errorf("Expected empty updateVersion on error, got %q", updateVersion)
	}
}

func TestGetServiceStatus_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid json`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	status, _ := GetServiceStatus("testservice")
	if !status {
		t.Errorf("Expected status to be true (default), got false")
	}
}

func TestGetServiceStatus_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	status, _ := GetServiceStatus("testservice")
	if !status {
		t.Errorf("Expected status to be true (default), got false")
	}
}

func TestGetServiceStatus_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	status, _ := GetServiceStatus("testservice")
	if !status {
		t.Errorf("Expected status to be true (default on error), got false")
	}
}

// Test GetReq function
func TestGetReq_SuccessCase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"testhost","status":"online"}`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	result, err := GetReq("1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result["name"] != "testhost" {
		t.Errorf("Expected name to be 'testhost', got %v", result["name"])
	}
}

func TestGetReq_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	_, err := GetReq("1")
	if err == nil {
		t.Errorf("Expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to contain '404', got %v", err)
	}
}

func TestGetReq_NetworkError(t *testing.T) {
	ClientConf = Client{URL: "http://nonexistent.example.com", HTTPClient: &http.Client{}}

	_, err := GetReq("1")
	if err == nil {
		t.Errorf("Expected network error, got nil")
	}
}

func TestGetReq_InvalidURL(t *testing.T) {
	ClientConf = Client{URL: "://invalid-url", HTTPClient: &http.Client{}}

	_, err := GetReq("1")
	if err == nil {
		t.Errorf("Expected error for invalid URL, got nil")
	}
}

// Test GetHosts function
func TestGetHosts_SingleHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"testhost","status":"online"}`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}
	AuthConfig = ClientAuth{Token: "test-token"}

	hosts := GetHosts("1", "testhost")
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Name != "testhost" {
		t.Errorf("Expected host name 'testhost', got %q", hosts[0].Name)
	}
}

func TestGetHosts_MultipleHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name":"host1","status":"online"},{"name":"host2","status":"offline"}]`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	hosts := GetHosts("1", "")
	if len(hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetHosts_NetworkError(t *testing.T) {
	ClientConf = Client{URL: "http://nonexistent.example.com", HTTPClient: &http.Client{}}

	hosts := GetHosts("1", "")
	if hosts != nil {
		t.Errorf("Expected nil hosts on network error, got %v", hosts)
	}
}

func TestGetHosts_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	hosts := GetHosts("1", "")
	if hosts != nil {
		t.Errorf("Expected nil hosts on HTTP error, got %v", hosts)
	}
}

func TestGetHosts_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid json`))
	}))
	defer srv.Close()

	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}

	hosts := GetHosts("1", "")
	if hosts != nil {
		t.Errorf("Expected nil hosts on invalid JSON, got %v", hosts)
	}
}

// Test Client.hc() method
func TestClient_hc(t *testing.T) {
	// Test with custom HTTP client
	customClient := &http.Client{}
	c := &Client{HTTPClient: customClient}
	if c.hc() != customClient {
		t.Errorf("Expected custom client, got default client")
	}

	// Test with nil HTTP client (should return default)
	c = &Client{}
	if c.hc() != http.DefaultClient {
		t.Errorf("Expected default client, got custom client")
	}
}

// Test getIdentifier function
func TestGetIdentifier_Environment(t *testing.T) {
	// Save original value
	originalValue := os.Getenv("MONOKIT_TEST_IDENTIFIER")
	defer func() {
		if originalValue != "" {
			os.Setenv("MONOKIT_TEST_IDENTIFIER", originalValue)
		} else {
			os.Unsetenv("MONOKIT_TEST_IDENTIFIER")
		}
	}()

	// Test with environment variable set
	os.Setenv("MONOKIT_TEST_IDENTIFIER", "test-env-id")
	if getIdentifier() != "test-env-id" {
		t.Errorf("Expected 'test-env-id', got %q", getIdentifier())
	}

	// Test with environment variable unset (should use fallback)
	os.Unsetenv("MONOKIT_TEST_IDENTIFIER")
	result := getIdentifier()
	if result == "" {
		t.Errorf("Expected non-empty identifier, got empty string")
	}
}

// Test error handling in GetIP function
func TestGetIP_ErrorHandling(t *testing.T) {
	// Save original function
	originalNetInterfacesFn := netInterfacesFn
	defer func() { netInterfacesFn = originalNetInterfacesFn }()

	// Mock function to return error
	netInterfacesFn = func() (net.InterfaceStatList, error) {
		return nil, io.ErrUnexpectedEOF
	}

	ip := GetIP()
	if ip != "" {
		t.Errorf("Expected empty IP on error, got %q", ip)
	}
}

// Test GetIP with no valid interfaces
func TestGetIP_NoValidInterfaces(t *testing.T) {
	// Save original function
	originalNetInterfacesFn := netInterfacesFn
	defer func() { netInterfacesFn = originalNetInterfacesFn }()

	// Mock function to return only loopback interface
	netInterfacesFn = func() (net.InterfaceStatList, error) {
		return []net.InterfaceStat{
			{Name: "lo", Addrs: []net.InterfaceAddr{{Addr: "127.0.0.1/8"}}},
		}, nil
	}

	ip := GetIP()
	if ip != "" {
		t.Errorf("Expected empty IP with only loopback, got %q", ip)
	}
}
