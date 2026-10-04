package lbPolicy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// End-to-end: gerçek SwitchMain akışı, yerel mock Caddy admin API'si (ağ/prod yok).
func TestSwitchMainEndToEnd(t *testing.T) {
	vhosts := []string{
		"api.optymus.tech", "api-private.optymus.tech", "app.optymus.tech",
		"connect.optymus.tech", "connect-api.optymus.tech", "connect-api-private.optymus.tech",
		"connect-api-tf.optymus.tech", "purchase.optymus.tech", "auth.optymus.tech",
		"auth-admin.optymus.tech", "kyc-api.optymus.tech", "kyc-app.optymus.tech",
		"api.nexus.optymus.tech", "app.nexus.optymus.tech", "vault.optymus.tech",
	}

	routes := make([]interface{}, 0, len(vhosts))
	for i, vh := range vhosts {
		routes = append(routes, map[string]interface{}{
			"match": []interface{}{map[string]interface{}{"host": []interface{}{vh}}},
			"handle": []interface{}{map[string]interface{}{
				"handler": "reverse_proxy",
				"upstreams": []interface{}{
					map[string]interface{}{"dial": "10.9.9." + strconv.Itoa(i+1) + ":8080"},
					map[string]interface{}{"dial": "dc1-node" + strconv.Itoa(i+1) + ":8080"},
				},
				"load_balancing": map[string]interface{}{
					"selection_policy": map[string]interface{}{"policy": "round_robin"},
				},
			}},
		})
	}
	body, _ := json.Marshal(map[string]interface{}{
		"srv0": map[string]interface{}{"routes": routes},
	})

	var mu sync.Mutex
	var patches []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/config/apps/http/servers":
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		case r.Method == "PATCH":
			var m map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&m)
			mu.Lock()
			patches = append(patches, m)
			mu.Unlock()
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	ConfReset()
	Config.Caddy.Api_Urls = []string{"u:p@" + srv.URL + ";mock"}
	Config.Caddy.Servers = vhosts
	Config.Caddy.Dynamic_Api_Urls = false
	Config.Caddy.Override_Config = true
	Config.Caddy.Loop_Order = "SERVERS"
	Config.Caddy.Lb_Policy_Change_Sleep = 0

	SwitchMain("first_dc1")

	if len(patches) != len(vhosts) {
		t.Fatalf("expected %d PATCHes, got %d", len(vhosts), len(patches))
	}
	for _, p := range patches {
		handle := p["handle"].([]interface{})[0].(map[string]interface{})
		pol := handle["load_balancing"].(map[string]interface{})["selection_policy"].(map[string]interface{})["policy"]
		if pol != "first" {
			t.Fatalf("policy not set to first: %v", pol)
		}
		ups := handle["upstreams"].([]interface{})
		if !strings.Contains(ups[0].(map[string]interface{})["dial"].(string), "dc1") {
			t.Fatalf("dc1 upstream not first: %v", ups)
		}
	}
}
