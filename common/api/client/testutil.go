package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// respSpec defines an HTTP status and body for the test server's response.
type respSpec struct {
	Status int
	Body   string
}

// setupTestServer creates an httptest.Server that responds to specified routes, and injects ClientConf for isolation.
func setupTestServer(t *testing.T, routes map[string]respSpec) (*httptest.Server, func()) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spec, ok := routes[r.URL.Path]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(spec.Status)
		w.Write([]byte(spec.Body))
	}))
	oldConf := ClientConf
	ClientConf = Client{URL: srv.URL, HTTPClient: srv.Client()}
	restore := func() {
		ClientConf = oldConf
		srv.Close()
	}
	t.Cleanup(restore)
	return srv, restore
}
