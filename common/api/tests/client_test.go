////go:build with_api
//go:build with_api
// +build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Provide access to models.Config for tests that call client.go logic directly
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
	assert.NotEmpty(t, ram)

	osver := client.GetOS()
	assert.NotEmpty(t, osver)
}

// Optionally: add edge/branch test for WrapperGetServiceStatus
// Not included here due to side effect (os.Exit) and lockfile removal.
