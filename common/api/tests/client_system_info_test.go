//go:build with_api

package tests

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/api/client"
	"github.com/stretchr/testify/assert"
)

func TestGetCPUCores_Success(t *testing.T) {
	cores := client.GetCPUCores()

	// Should return a positive number of cores
	assert.Greater(t, cores, 0)
	assert.LessOrEqual(t, cores, 256) // Reasonable upper bound for testing

	// Should be consistent with runtime
	runtimeCores := runtime.NumCPU()
	assert.Equal(t, runtimeCores, cores)
}

func TestGetCPUCores_RepeatedCalls(t *testing.T) {
	// Multiple calls should return consistent results
	cores1 := client.GetCPUCores()
	cores2 := client.GetCPUCores()
	cores3 := client.GetCPUCores()

	assert.Equal(t, cores1, cores2)
	assert.Equal(t, cores2, cores3)
}

func TestGetRAM_Success(t *testing.T) {
	ram := client.GetRAM()

	// RAM might be empty in some test environments
	if ram != "" {
		// Should end with "GB"
		assert.True(t, strings.HasSuffix(ram, "GB"))

		// Should be parseable as a number followed by "GB"
		assert.Regexp(t, `^\d+GB$`, ram)

		// Extract numeric part and verify it's reasonable
		numPart := strings.TrimSuffix(ram, "GB")
		assert.Regexp(t, `^\d+$`, numPart)
	}
}

func TestGetRAM_RepeatedCalls(t *testing.T) {
	// Multiple calls should return consistent results
	ram1 := client.GetRAM()
	ram2 := client.GetRAM()
	ram3 := client.GetRAM()

	assert.Equal(t, ram1, ram2)
	assert.Equal(t, ram2, ram3)
}

func TestGetRAM_ReasonableValue(t *testing.T) {
	ram := client.GetRAM()

	// Extract numeric value
	numStr := strings.TrimSuffix(ram, "GB")

	// Convert to int for validation (basic parsing)
	var ramGB int
	_, err := fmt.Sscanf(numStr, "%d", &ramGB)
	assert.NoError(t, err)

	// Should be a reasonable amount of RAM (1GB to 1TB)
	assert.GreaterOrEqual(t, ramGB, 1)
	assert.LessOrEqual(t, ramGB, 1024)
}

func TestGetOS_Success(t *testing.T) {
	osInfo := client.GetOS()

	// OS info might be empty in some test environments
	if osInfo != "" {
		// Should contain reasonable OS information
		// The format should be: platform + " " + platformVersion + " " + kernelVersion
		parts := strings.Fields(osInfo)
		assert.GreaterOrEqual(t, len(parts), 1) // At least the platform name
	}
}

func TestGetOS_RepeatedCalls(t *testing.T) {
	// Multiple calls should return consistent results
	os1 := client.GetOS()
	os2 := client.GetOS()
	os3 := client.GetOS()

	assert.Equal(t, os1, os2)
	assert.Equal(t, os2, os3)
}

func TestGetOS_ContainsExpectedInfo(t *testing.T) {
	osInfo := client.GetOS()

	// Should contain some expected OS-related keywords
	osLower := strings.ToLower(osInfo)

	// Check for common OS indicators
	hasOSInfo := strings.Contains(osLower, "linux") ||
		strings.Contains(osLower, "darwin") ||
		strings.Contains(osLower, "windows") ||
		strings.Contains(osLower, "freebsd") ||
		strings.Contains(osLower, "ubuntu") ||
		strings.Contains(osLower, "centos") ||
		strings.Contains(osLower, "macos")

	// If none of the above, at least it should not be empty
	if !hasOSInfo {
		assert.NotEmpty(t, osInfo)
	}
}

func TestGetIP_Success(t *testing.T) {
	ip := client.GetIP()

	// Should return a non-empty string (unless no network interfaces)
	// Note: This might be empty in some test environments
	if ip != "" {
		// Should be a valid IP format
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			// IPv4 format validation
			for _, part := range parts {
				assert.Regexp(t, `^\d+$`, part)
			}
		} else {
			// Might be IPv6 or other format, just check it's not obviously invalid
			assert.NotContains(t, ip, " ")
		}
	}
}

func TestGetIP_RepeatedCalls(t *testing.T) {
	// Multiple calls should return consistent results
	ip1 := client.GetIP()
	ip2 := client.GetIP()
	ip3 := client.GetIP()

	assert.Equal(t, ip1, ip2)
	assert.Equal(t, ip2, ip3)
}

func TestGetIP_ExcludesLoopback(t *testing.T) {
	ip := client.GetIP()

	// Should not return loopback addresses
	assert.NotEqual(t, "127.0.0.1", ip)
	assert.NotEqual(t, "::1", ip)
	assert.NotEqual(t, "localhost", ip)
}

func TestGetIP_ValidFormat(t *testing.T) {
	ip := client.GetIP()

	if ip != "" {
		// Should not contain CIDR notation (no slash)
		assert.NotContains(t, ip, "/")

		// Should not contain port numbers (basic check)
		colonCount := strings.Count(ip, ":")
		if colonCount <= 1 { // IPv4 should have no colons, IPv6 can have many
			assert.NotContains(t, ip, ":")
		}
	}
}

func TestSystemInfoIntegration_AllFunctions(t *testing.T) {
	// Test all system info functions together to ensure they work in combination
	cores := client.GetCPUCores()
	ram := client.GetRAM()
	osInfo := client.GetOS()
	ip := client.GetIP()

	// All should return valid values
	assert.Greater(t, cores, 0)

	// RAM and OS might be empty in some test environments
	if ram != "" {
		assert.Contains(t, ram, "GB")
	}

	// OS info might be empty in some test environments
	if osInfo != "" {
		assert.NotContains(t, osInfo, "error")
		assert.NotContains(t, osInfo, "Error")
	}

	// IP might be empty in some environments, so we don't assert on it

	// Log the values for debugging (will show in verbose test output)
	t.Logf("System Info - CPU Cores: %d, RAM: %s, OS: %s, IP: %s", cores, ram, osInfo, ip)
}

func TestSystemInfoStability_MultipleRuns(t *testing.T) {
	// Run multiple iterations to ensure stability
	const iterations = 10

	var cores []int
	var rams []string
	var oses []string
	var ips []string

	for i := 0; i < iterations; i++ {
		cores = append(cores, client.GetCPUCores())
		rams = append(rams, client.GetRAM())
		oses = append(oses, client.GetOS())
		ips = append(ips, client.GetIP())
	}

	// All values should be consistent across iterations
	for i := 1; i < iterations; i++ {
		assert.Equal(t, cores[0], cores[i], "CPU cores should be consistent")
		assert.Equal(t, rams[0], rams[i], "RAM should be consistent")
		assert.Equal(t, oses[0], oses[i], "OS info should be consistent")
		assert.Equal(t, ips[0], ips[i], "IP should be consistent")
	}

	// Log first values for debugging
	t.Logf("Stability test - CPU: %d, RAM: %s, OS: %s, IP: %s", cores[0], rams[0], oses[0], ips[0])
}

func TestSystemInfo_ConcurrentCalls(t *testing.T) {
	// Test concurrent calls to ensure thread safety
	const numGoroutines = 10
	const callsPerGoroutine = 5

	results := make(chan map[string]interface{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in goroutine %d: %v", goroutineID, r)
				}
			}()

			result := make(map[string]interface{})
			for j := 0; j < callsPerGoroutine; j++ {
				result["cores"] = client.GetCPUCores()
				result["ram"] = client.GetRAM()
				result["os"] = client.GetOS()
				result["ip"] = client.GetIP()
			}
			results <- result
		}(i)
	}

	// Collect all results with timeout
	var allResults []map[string]interface{}
	for i := 0; i < numGoroutines; i++ {
		select {
		case result := <-results:
			allResults = append(allResults, result)
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for goroutine %d to complete", i)
		}
	}

	// Verify all results are consistent
	firstResult := allResults[0]
	for i := 1; i < len(allResults); i++ {
		assert.Equal(t, firstResult["cores"], allResults[i]["cores"])
		assert.Equal(t, firstResult["ram"], allResults[i]["ram"])
		assert.Equal(t, firstResult["os"], allResults[i]["os"])
		assert.Equal(t, firstResult["ip"], allResults[i]["ip"])
	}
}

func TestSystemInfo_ErrorHandling(t *testing.T) {
	// These functions should handle errors gracefully and return sensible defaults

	// GetCPUCores should return 0 on error (though this is unlikely)
	cores := client.GetCPUCores()
	assert.GreaterOrEqual(t, cores, 0)

	// GetRAM should return empty string on error
	ram := client.GetRAM()
	// If not empty, should be properly formatted
	if ram != "" {
		assert.Contains(t, ram, "GB")
	}

	// GetOS should return empty string on error
	osInfo := client.GetOS()
	// If not empty, should be reasonable
	if osInfo != "" {
		assert.NotContains(t, osInfo, "error")
		assert.NotContains(t, osInfo, "Error")
	}

	// GetIP should return empty string on error
	ip := client.GetIP()
	// If not empty, should not contain error messages
	if ip != "" {
		assert.NotContains(t, ip, "error")
		assert.NotContains(t, ip, "Error")
		assert.NotContains(t, ip, "fail")
	}
}

func TestSystemInfo_EdgeCases(t *testing.T) {
	// Test some edge cases and boundary conditions

	// CPU cores should be reasonable
	cores := client.GetCPUCores()
	assert.LessOrEqual(t, cores, 1000) // Extremely high upper bound

	// RAM should not be impossibly high
	ram := client.GetRAM()
	if ram != "" {
		ramStr := strings.TrimSuffix(ram, "GB")
		var ramVal int
		if _, err := fmt.Sscanf(ramStr, "%d", &ramVal); err == nil {
			assert.LessOrEqual(t, ramVal, 10000) // 10TB upper bound
		}
	}

	// OS string should not be excessively long
	osInfo := client.GetOS()
	assert.LessOrEqual(t, len(osInfo), 1000)

	// IP should not be excessively long
	ip := client.GetIP()
	assert.LessOrEqual(t, len(ip), 100)
}
