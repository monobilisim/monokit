////go:build with_api
//go:build with_api
// +build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Provide access to common.Config for tests that call client.go logic directly
	commonmain "github.com/monobilisim/monokit/common"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	common.ClientConf.URL = srv.URL
	// Needed for identifier reference
	commonmain.Config.Identifier = "testhost"

	t.Run("status enabled", func(t *testing.T) {
		enabled, wants := common.GetServiceStatus("serviceA")
		assert.True(t, enabled)
		assert.Equal(t, "", wants)
	})

	t.Run("disabled with wantsUpdate", func(t *testing.T) {
		enabled, wants := common.GetServiceStatus("serviceB")
		assert.False(t, enabled)
		assert.Equal(t, "2.1.0", wants)
	})

	t.Run("http error", func(t *testing.T) {
		enabled, wants := common.GetServiceStatus("serviceC")
		assert.True(t, enabled) // fallback is true on error
		assert.Empty(t, wants)
	})
}

func TestGetHosts(t *testing.T) {
	host := common.Host{Name: "hostA", CpuCores: 4, Ram: "8GB", MonokitVersion: "1.0", Os: "TestOS", DisabledComponents: "nil", InstalledComponents: "test", IpAddress: "1.2.3.4", Status: "Online", Groups: "grp", Inventory: "default"}
	list := []common.Host{host}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/hosts":
			_ = json.NewEncoder(w).Encode(list)
		case "/api/v1/hosts/hostA":
			_ = json.NewEncoder(w).Encode(host)
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	common.ClientConf.URL = srv.URL

	t.Run("list hosts", func(t *testing.T) {
		res := common.GetHosts("1", "")
		require.Len(t, res, 1)
		assert.Equal(t, "hostA", res[0].Name)
	})

	t.Run("single host", func(t *testing.T) {
		res := common.GetHosts("1", "hostA")
		require.Len(t, res, 1)
		assert.Equal(t, "hostA", res[0].Name)
	})
}

func TestSendUpdateTo_Disable_Enable(t *testing.T) {
	var called struct {
		path, method string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.path, called.method = r.URL.Path, r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()
	common.ClientConf.URL = srv.URL

	t.Run("SendUpdateTo", func(t *testing.T) {
		common.SendUpdateTo("1", "hostA", "v2")
		assert.Equal(t, "/api/v1/hosts/hostA/updateTo/v2", called.path)
		assert.Equal(t, "POST", called.method)
	})
	t.Run("SendDisable", func(t *testing.T) {
		common.SendDisable("1", "hostA", "compA")
		assert.Equal(t, "/api/v1/hosts/hostA/disable/compA", called.path)
		assert.Equal(t, "POST", called.method)
	})
	t.Run("SendEnable", func(t *testing.T) {
		common.SendEnable("1", "hostA", "compA")
		assert.Equal(t, "/api/v1/hosts/hostA/enable/compA", called.path)
		assert.Equal(t, "POST", called.method)
	})
}

func TestGetCPUCores_GetRAM_GetOS(t *testing.T) {
	cores := common.GetCPUCores()
	assert.GreaterOrEqual(t, cores, 0)

	ram := common.GetRAM()
	assert.NotEmpty(t, ram)

	osver := common.GetOS()
	assert.NotEmpty(t, osver)
}

// Optionally: add edge/branch test for WrapperGetServiceStatus
// Not included here due to side effect (os.Exit) and lockfile removal.
