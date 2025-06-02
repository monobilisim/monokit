package common

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Mocks / stubs ---
type mockFS struct {
	files map[string][]byte
}

func (m *mockFS) ReadFile(path string) ([]byte, error) { return m.files[path], nil }

func (m *mockFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *mockFS) MkdirAll(path string, perm fs.FileMode) error { return nil }

type mockExiter struct {
	exited bool
	code   int
}

func (s *mockExiter) Exit(code int) { s.exited = true; s.code = code }

type stubSys struct{}

func (stubSys) CPUCores() int      { return 8 }
func (stubSys) RAM() string        { return "32GB" }
func (stubSys) PrimaryIP() string  { return "1.2.3.4" }
func (stubSys) OSPlatform() string { return "testOS 1.0" }

func TestHostService_SendHostReport_Basic(t *testing.T) {
	// ARRANGE: Setup fake API server and dependencies
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.Copy(io.Discard, r.Body)
		io.WriteString(w, `{"host":{}, "apiKey":"XYZ", "error":""}`)
	}))
	defer srv.Close()
	fs := &mockFS{files: map[string][]byte{"fake/dir/id": []byte("tok")}}
	exit := &mockExiter{}
	hs := &HostService{
		HTTP: srv.Client(),
		FS:   fs,
		Info: stubSys{},
		Exit: exit,
		Conf: &Config{URL: srv.URL, Identifier: "id", Version: "v", APIKeyDir: "fake/dir"},
	}

	// ACT
	err := hs.SendHostReport()

	// ASSERT
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "tok" {
		t.Errorf("Authorization header not preserved")
	}
	if string(fs.files["fake/dir/id"]) != "XYZ" {
		t.Errorf("API key not saved: got %s", string(fs.files["fake/dir/id"]))
	}
	if exit.exited {
		t.Errorf("should not exit in normal flow")
	}
}

// More tests can be added for: UpForDeletion, error API, hostkey not found, etc.

func TestHostService_GetServiceStatus_Variants(t *testing.T) {
	// arrange
	withResp := func(json string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(json))
		}))
	}
	hs := &HostService{
		FS:   &mockFS{files: map[string][]byte{}},
		Info: stubSys{},
		Exit: &mockExiter{},
		Conf: &Config{URL: "", Identifier: "foo", APIKeyDir: "/tmp"},
	}

	// "enabled" status
	srv := withResp(`{"status":"enabled"}`)
	defer srv.Close()
	hs.HTTP = srv.Client()
	hs.Conf.URL = srv.URL

	enabled, update, err := hs.GetServiceStatus("svc")
	if err != nil || !enabled || update != "" {
		t.Errorf("enabled branch failed: %v, %v, %v", enabled, update, err)
	}

	// "disabled" + wantsUpdateTo
	srv2 := withResp(`{"disabled":true,"wantsUpdateTo":"1.9"}`)
	defer srv2.Close()
	hs.HTTP = srv2.Client()
	hs.Conf.URL = srv2.URL
	enabled, update, err = hs.GetServiceStatus("svc")
	if err != nil || enabled || update != "1.9" {
		t.Errorf("disabled with update: %v %v %v", enabled, update, err)
	}
}

func TestHostService_SendHostReport_ApiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":"kaboom"}`)
	}))
	defer srv.Close()
	fs := &mockFS{files: map[string][]byte{"dir/id": []byte("tok")}}
	exit := &mockExiter{}
	hs := &HostService{
		HTTP: srv.Client(),
		FS:   fs,
		Info: stubSys{},
		Exit: exit,
		Conf: &Config{URL: srv.URL, Identifier: "id", Version: "v", APIKeyDir: "dir"},
	}

	err := hs.SendHostReport()
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestHostService_SendHostReport_UpForDeletion_CallsExit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"host":{"upForDeletion":true}}`)
	}))
	defer srv.Close()
	fs := &mockFS{files: map[string][]byte{"d/i": []byte("k")}}
	exit := &mockExiter{}
	hs := &HostService{
		HTTP: srv.Client(),
		FS:   fs,
		Info: stubSys{},
		Exit: exit,
		Conf: &Config{URL: srv.URL, Identifier: "i", Version: "v", APIKeyDir: "d"},
	}
	_ = hs.SendHostReport()
	if !exit.exited {
		t.Errorf("should have called Exit")
	}
}

func TestHostService_SendHostReport_WriteFileFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"apiKey":"x"}`)
	}))
	defer srv.Close()
	fs := &mockFS{files: map[string][]byte{"dd/ii": []byte("k")}}
	exit := &mockExiter{}
	hs := &HostService{
		HTTP: srv.Client(),
		FS:   &failingFS{mockFS: *fs},
		Info: stubSys{},
		Exit: exit,
		Conf: &Config{URL: srv.URL, Identifier: "ii", Version: "v", APIKeyDir: "dd"},
	}
	err := hs.SendHostReport()
	if err == nil || !strings.Contains(err.Error(), "failWrite") {
		t.Fatalf("want failWrite error, got %v", err)
	}
}

// failingFS always fails on WriteFile
type failingFS struct{ mockFS }

func (f *failingFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return fmt.Errorf("failWrite")
}
func (f *failingFS) MkdirAll(path string, perm fs.FileMode) error {
	return nil
}

func TestHostService_GetServiceStatus_DefaultEnabled(t *testing.T) {
	withResp := func(json string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(json))
		}))
	}
	hs := &HostService{
		FS:   &mockFS{files: map[string][]byte{}},
		Info: stubSys{},
		Exit: &mockExiter{},
		Conf: &Config{URL: "", Identifier: "foo", APIKeyDir: "/tmp"},
	}

	srv := withResp(`{}`)
	defer srv.Close()
	hs.HTTP = srv.Client()
	hs.Conf.URL = srv.URL
	enabled, _, err := hs.GetServiceStatus("svc")
	if err != nil || !enabled {
		t.Errorf("default enabled: %v %v", enabled, err)
	}
}

func TestHostService_GetHosts_Basic(t *testing.T) {
	host := Host{
		Name: "h1", CpuCores: 2, MonokitVersion: "v", Status: "Online", Groups: "g",
	}
	j := `[{"name":"h1","cpuCores":2,"monokitVersion":"v","status":"Online","groups":"g"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(j))
	}))
	defer srv.Close()
	hs := &HostService{
		HTTP: srv.Client(),
		FS:   &mockFS{files: map[string][]byte{}},
		Info: stubSys{},
		Exit: &mockExiter{},
		Conf: &Config{URL: srv.URL, Identifier: "foo", APIKeyDir: ""},
	}

	hosts, err := hs.GetHosts("1", "")
	if err != nil {
		t.Fatalf("gethosts all: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != host.Name {
		t.Errorf("got %+v", hosts)
	}
}
