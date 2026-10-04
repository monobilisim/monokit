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
// Değişikliklerin LB başına TEK /load ile uygulandığını doğrular.
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

	fullConfig := map[string]interface{}{
		"admin": map[string]interface{}{"listen": "localhost:2019"},
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"srv0": map[string]interface{}{"listen": []interface{}{":443"}, "routes": routes},
				},
			},
			"tls": map[string]interface{}{},
		},
	}
	serversOnly := fullConfig["apps"].(map[string]interface{})["http"].(map[string]interface{})["servers"]
	serversBody, _ := json.Marshal(serversOnly)
	fullBody, _ := json.Marshal(fullConfig)

	var mu sync.Mutex
	var loads []map[string]interface{}
	gets := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/config/apps/http/servers":
			mu.Lock()
			gets++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(serversBody)
		case r.Method == "GET" && r.URL.Path == "/config/":
			mu.Lock()
			gets++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(fullBody)
		case r.Method == "POST" && r.URL.Path == "/load":
			var m map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			loads = append(loads, m)
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

	if len(loads) != 1 {
		t.Fatalf("expected exactly 1 config load, got %d (gets=%d)", len(loads), gets)
	}

	loaded := loads[0]["apps"].(map[string]interface{})["http"].(map[string]interface{})["servers"].(map[string]interface{})["srv0"].(map[string]interface{})["routes"].([]interface{})
	if len(loaded) != len(vhosts) {
		t.Fatalf("expected %d routes in loaded config, got %d", len(vhosts), len(loaded))
	}
	for _, r := range loaded {
		handle := r.(map[string]interface{})["handle"].([]interface{})[0].(map[string]interface{})
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
