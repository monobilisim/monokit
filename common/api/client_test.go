package common

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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

	var reqPath, reqMethod, reqBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath = r.URL.Path
		reqMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		reqBody = string(body)
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

	SendReq("1")

	assert.Equal(t, "/api/v1/hosts", reqPath)
	assert.Equal(t, "POST", reqMethod)
	var posted Host
	_ = json.Unmarshal([]byte(reqBody), &posted)
	assert.Equal(t, "testhost-abc", posted.Name)
	assert.Equal(t, "osHealth::demoComp", posted.InstalledComponents)
	assert.Equal(t, "test-group", posted.Groups)
	assert.Equal(t, "test-disable", posted.DisabledComponents)
}
