package common

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
