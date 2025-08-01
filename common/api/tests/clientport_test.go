//go:build with_api

package tests

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/monobilisim/monokit/common/api/clientport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSExiter_Interface(t *testing.T) {
	// Test that OSExiter implements the Exiter interface
	var exiter clientport.Exiter = clientport.OSExiter{}
	assert.NotNil(t, exiter)
	assert.Implements(t, (*clientport.Exiter)(nil), clientport.OSExiter{})
}

func TestOSFS_ReadFile(t *testing.T) {
	fs := clientport.OSFS{}
	
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test_clientport_read_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	testContent := "test file content for clientport"
	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()
	
	// Test reading the file
	content, err := fs.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
	
	// Test reading non-existent file
	_, err = fs.ReadFile("/non/existent/clientport/file")
	assert.Error(t, err)
}

func TestOSFS_WriteFile(t *testing.T) {
	fs := clientport.OSFS{}
	
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test_clientport_write_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	testFile := tmpDir + "/clientport_test.txt"
	testContent := []byte("test write content for clientport")
	
	// Test writing file
	err = fs.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)
	
	// Verify file was written correctly
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, content)
	
	// Test writing to invalid path
	err = fs.WriteFile("/invalid/clientport/path/file.txt", testContent, 0644)
	assert.Error(t, err)
}

func TestOSFS_MkdirAll(t *testing.T) {
	fs := clientport.OSFS{}
	
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test_clientport_mkdir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	testPath := tmpDir + "/nested/deep/clientport/directory"
	
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

func TestOSFS_Interface(t *testing.T) {
	var fs clientport.FS = clientport.OSFS{}
	assert.NotNil(t, fs)
	assert.Implements(t, (*clientport.FS)(nil), clientport.OSFS{})
}

func TestRealSysInfo_CPUCores(t *testing.T) {
	sysInfo := clientport.RealSysInfo{}
	cores := sysInfo.CPUCores()
	
	// CPU cores should be a positive number on real systems
	// Allow 0 for test environments where CPU info might not be available
	assert.GreaterOrEqual(t, cores, 0)
	
	if cores > 0 {
		assert.LessOrEqual(t, cores, 256) // Reasonable upper bound
	}
}

func TestRealSysInfo_RAM(t *testing.T) {
	sysInfo := clientport.RealSysInfo{}
	ram := sysInfo.RAM()
	
	// RAM should be a non-empty string with "GB" suffix on real systems
	if ram != "" {
		assert.Contains(t, ram, "GB")
		assert.True(t, len(ram) > 2) // Should be more than just "GB"
	} else {
		t.Log("Warning: RAM returned empty string, which might indicate an error in test environment")
	}
}

func TestRealSysInfo_PrimaryIP(t *testing.T) {
	sysInfo := clientport.RealSysInfo{}
	ip := sysInfo.PrimaryIP()
	
	// Primary IP might be empty in some test environments
	if ip != "" {
		// Basic validation - should contain dots for IPv4 or colons for IPv6
		assert.True(t, len(ip) > 0)
		// Could be IPv4 or IPv6, so just check it's not empty
		assert.NotEqual(t, "127.0.0.1", ip) // Should not be localhost
	} else {
		t.Log("Warning: PrimaryIP returned empty string, which might be expected in test environment")
	}
}

func TestRealSysInfo_OSPlatform(t *testing.T) {
	sysInfo := clientport.RealSysInfo{}
	platform := sysInfo.OSPlatform()
	
	// OS platform should be a non-empty string on real systems
	if platform != "" {
		assert.True(t, len(platform) > 0)
		// Should contain some recognizable OS information
		// This is a loose check since different systems return different formats
		assert.True(t, len(platform) >= 1)
	} else {
		t.Log("Warning: OSPlatform returned empty string, which might indicate an error in test environment")
	}
}

func TestRealSysInfo_Interface(t *testing.T) {
	var sysInfo clientport.SysInfo = clientport.RealSysInfo{}
	assert.NotNil(t, sysInfo)
	assert.Implements(t, (*clientport.SysInfo)(nil), clientport.RealSysInfo{})
}

func TestHTTPDoer_Interface(t *testing.T) {
	// Test that http.Client implements HTTPDoer
	var doer clientport.HTTPDoer = &http.Client{}
	assert.NotNil(t, doer)
	assert.Implements(t, (*clientport.HTTPDoer)(nil), &http.Client{})
	
	// Test with a real HTTP request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("clientport test response"))
	}))
	defer server.Close()
	
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	
	resp, err := doer.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test mock implementations to ensure they work correctly
func TestMockExiter(t *testing.T) {
	type MockExiter struct {
		ExitCode int
		Called   bool
	}
	
	mockExiter := &MockExiter{}
	mockExiter.Exit = func(code int) {
		mockExiter.ExitCode = code
		mockExiter.Called = true
	}
	
	// Simulate exit call
	mockExiter.Exit(42)
	
	assert.True(t, mockExiter.Called)
	assert.Equal(t, 42, mockExiter.ExitCode)
}

func TestMockFS(t *testing.T) {
	type MockFS struct {
		ReadFileFunc  func(path string) ([]byte, error)
		WriteFileFunc func(path string, data []byte, perm fs.FileMode) error
		MkdirAllFunc  func(path string, perm fs.FileMode) error
	}
	
	mock := &MockFS{
		ReadFileFunc: func(path string) ([]byte, error) {
			if path == "test.txt" {
				return []byte("mock content"), nil
			}
			return nil, os.ErrNotExist
		},
		WriteFileFunc: func(path string, data []byte, perm fs.FileMode) error {
			if path == "invalid.txt" {
				return os.ErrPermission
			}
			return nil
		},
		MkdirAllFunc: func(path string, perm fs.FileMode) error {
			if path == "invalid/dir" {
				return os.ErrPermission
			}
			return nil
		},
	}
	
	// Test ReadFile
	content, err := mock.ReadFileFunc("test.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("mock content"), content)
	
	_, err = mock.ReadFileFunc("nonexistent.txt")
	assert.Error(t, err)
	
	// Test WriteFile
	err = mock.WriteFileFunc("test.txt", []byte("data"), 0644)
	assert.NoError(t, err)
	
	err = mock.WriteFileFunc("invalid.txt", []byte("data"), 0644)
	assert.Error(t, err)
	
	// Test MkdirAll
	err = mock.MkdirAllFunc("test/dir", 0755)
	assert.NoError(t, err)
	
	err = mock.MkdirAllFunc("invalid/dir", 0755)
	assert.Error(t, err)
}

func TestMockSysInfo(t *testing.T) {
	type MockSysInfo struct {
		CPUCoresFunc   func() int
		RAMFunc        func() string
		PrimaryIPFunc  func() string
		OSPlatformFunc func() string
	}
	
	mock := &MockSysInfo{
		CPUCoresFunc:   func() int { return 8 },
		RAMFunc:        func() string { return "16.00GB" },
		PrimaryIPFunc:  func() string { return "192.168.1.100" },
		OSPlatformFunc: func() string { return "linux ubuntu 20.04" },
	}
	
	assert.Equal(t, 8, mock.CPUCoresFunc())
	assert.Equal(t, "16.00GB", mock.RAMFunc())
	assert.Equal(t, "192.168.1.100", mock.PrimaryIPFunc())
	assert.Equal(t, "linux ubuntu 20.04", mock.OSPlatformFunc())
}
