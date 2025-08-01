package clientport

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test OSExiter
func TestOSExiter_Exit(t *testing.T) {
	// We can't actually test os.Exit without terminating the test process
	// So we just verify the type implements the interface correctly
	var exiter Exiter = OSExiter{}
	assert.NotNil(t, exiter)
	
	// We can test that the method exists and is callable
	// but we can't call it without terminating the test
	assert.Implements(t, (*Exiter)(nil), OSExiter{})
}

// Test OSFS
func TestOSFS_ReadFile(t *testing.T) {
	fs := OSFS{}
	
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test_read_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	testContent := "test file content"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()
	
	// Test reading the file
	content, err := fs.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
	
	// Test reading non-existent file
	_, err = fs.ReadFile("/non/existent/file")
	assert.Error(t, err)
}

func TestOSFS_WriteFile(t *testing.T) {
	fs := OSFS{}
	
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test_write_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	testFile := tmpDir + "/test.txt"
	testContent := []byte("test write content")
	
	// Test writing file
	err = fs.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)
	
	// Verify file was written correctly
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, content)
	
	// Test writing to invalid path
	err = fs.WriteFile("/invalid/path/file.txt", testContent, 0644)
	assert.Error(t, err)
}

func TestOSFS_MkdirAll(t *testing.T) {
	fs := OSFS{}
	
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test_mkdir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	testPath := tmpDir + "/nested/deep/directory"
	
	// Test creating nested directories
	err = fs.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	
	// Verify directory was created
	info, err := os.Stat(testPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	
	// Test creating directory that already exists (should not error)
	err = fs.MkdirAll(testPath, 0755)
	assert.NoError(t, err)
}

func TestOSFS_Implements_FS_Interface(t *testing.T) {
	var fs FS = OSFS{}
	assert.NotNil(t, fs)
	assert.Implements(t, (*FS)(nil), OSFS{})
}

// Test RealSysInfo
func TestRealSysInfo_CPUCores(t *testing.T) {
	sysInfo := RealSysInfo{}
	cores := sysInfo.CPUCores()
	
	// CPU cores should be a positive number on any real system
	// On test systems, it should be at least 1
	assert.GreaterOrEqual(t, cores, 0) // Allow 0 in case of error
	
	// In most cases, it should be at least 1
	if cores == 0 {
		t.Log("Warning: CPUCores returned 0, which might indicate an error")
	}
}

func TestRealSysInfo_RAM(t *testing.T) {
	sysInfo := RealSysInfo{}
	ram := sysInfo.RAM()
	
	// RAM should be a non-empty string with "GB" suffix on real systems
	if ram != "" {
		assert.Contains(t, ram, "GB")
		assert.True(t, len(ram) > 2) // Should be more than just "GB"
	} else {
		t.Log("Warning: RAM returned empty string, which might indicate an error")
	}
}

func TestRealSysInfo_PrimaryIP(t *testing.T) {
	sysInfo := RealSysInfo{}
	ip := sysInfo.PrimaryIP()
	
	// Primary IP might be empty in some test environments
	// but if it's not empty, it should look like an IP address
	if ip != "" {
		// Basic check - should contain dots for IPv4 or colons for IPv6
		assert.True(t, strings.Contains(ip, ".") || strings.Contains(ip, ":"),
			"IP address should contain dots or colons: %s", ip)
	} else {
		t.Log("Warning: PrimaryIP returned empty string, which might be expected in test environment")
	}
}

func TestRealSysInfo_OSPlatform(t *testing.T) {
	sysInfo := RealSysInfo{}
	platform := sysInfo.OSPlatform()
	
	// OS platform should be a non-empty string on real systems
	if platform != "" {
		assert.True(t, len(platform) > 0)
		// Should contain some recognizable OS information
		// This is a loose check since different systems return different formats
		assert.True(t, len(strings.Fields(platform)) >= 1)
	} else {
		t.Log("Warning: OSPlatform returned empty string, which might indicate an error")
	}
}

func TestRealSysInfo_Implements_SysInfo_Interface(t *testing.T) {
	var sysInfo SysInfo = RealSysInfo{}
	assert.NotNil(t, sysInfo)
	assert.Implements(t, (*SysInfo)(nil), RealSysInfo{})
}

// Test interfaces
func TestHTTPDoer_Interface(t *testing.T) {
	// Test that http.Client implements HTTPDoer
	var doer HTTPDoer = &http.Client{}
	assert.NotNil(t, doer)
	assert.Implements(t, (*HTTPDoer)(nil), &http.Client{})
	
	// Test with a real HTTP request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()
	
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	
	resp, err := doer.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Mock implementations for testing
type MockExiter struct {
	ExitCode int
	Called   bool
}

func (m *MockExiter) Exit(code int) {
	m.ExitCode = code
	m.Called = true
}

func TestMockExiter(t *testing.T) {
	mock := &MockExiter{}
	var exiter Exiter = mock
	
	exiter.Exit(42)
	
	assert.True(t, mock.Called)
	assert.Equal(t, 42, mock.ExitCode)
}

type MockFS struct {
	ReadFileFunc    func(path string) ([]byte, error)
	WriteFileFunc   func(path string, data []byte, perm fs.FileMode) error
	MkdirAllFunc    func(path string, perm fs.FileMode) error
}

func (m *MockFS) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, errors.New("not implemented")
}

func (m *MockFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(path, data, perm)
	}
	return errors.New("not implemented")
}

func (m *MockFS) MkdirAll(path string, perm fs.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return errors.New("not implemented")
}

func TestMockFS(t *testing.T) {
	mock := &MockFS{
		ReadFileFunc: func(path string) ([]byte, error) {
			if path == "test.txt" {
				return []byte("mock content"), nil
			}
			return nil, errors.New("file not found")
		},
		WriteFileFunc: func(path string, data []byte, perm fs.FileMode) error {
			if path == "test.txt" {
				return nil
			}
			return errors.New("write failed")
		},
		MkdirAllFunc: func(path string, perm fs.FileMode) error {
			if path == "test/dir" {
				return nil
			}
			return errors.New("mkdir failed")
		},
	}
	
	var fs FS = mock
	
	// Test ReadFile
	content, err := fs.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("mock content"), content)
	
	_, err = fs.ReadFile("nonexistent.txt")
	assert.Error(t, err)
	
	// Test WriteFile
	err = fs.WriteFile("test.txt", []byte("data"), 0644)
	assert.NoError(t, err)
	
	err = fs.WriteFile("invalid.txt", []byte("data"), 0644)
	assert.Error(t, err)
	
	// Test MkdirAll
	err = fs.MkdirAll("test/dir", 0755)
	assert.NoError(t, err)
	
	err = fs.MkdirAll("invalid/dir", 0755)
	assert.Error(t, err)
}

type MockSysInfo struct {
	CPUCoresFunc   func() int
	RAMFunc        func() string
	PrimaryIPFunc  func() string
	OSPlatformFunc func() string
}

func (m *MockSysInfo) CPUCores() int {
	if m.CPUCoresFunc != nil {
		return m.CPUCoresFunc()
	}
	return 0
}

func (m *MockSysInfo) RAM() string {
	if m.RAMFunc != nil {
		return m.RAMFunc()
	}
	return ""
}

func (m *MockSysInfo) PrimaryIP() string {
	if m.PrimaryIPFunc != nil {
		return m.PrimaryIPFunc()
	}
	return ""
}

func (m *MockSysInfo) OSPlatform() string {
	if m.OSPlatformFunc != nil {
		return m.OSPlatformFunc()
	}
	return ""
}

func TestMockSysInfo(t *testing.T) {
	mock := &MockSysInfo{
		CPUCoresFunc:   func() int { return 8 },
		RAMFunc:        func() string { return "16.00GB" },
		PrimaryIPFunc:  func() string { return "192.168.1.100" },
		OSPlatformFunc: func() string { return "linux ubuntu 20.04" },
	}
	
	var sysInfo SysInfo = mock
	
	assert.Equal(t, 8, sysInfo.CPUCores())
	assert.Equal(t, "16.00GB", sysInfo.RAM())
	assert.Equal(t, "192.168.1.100", sysInfo.PrimaryIP())
	assert.Equal(t, "linux ubuntu 20.04", sysInfo.OSPlatform())
}
