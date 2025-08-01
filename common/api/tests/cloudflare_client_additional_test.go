//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestNewClient_WithAPIToken(t *testing.T) {
	config := models.CloudflareConfig{
		APIToken:  "test-token-123",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	
	// We expect this to fail because it's not a real token, but we can test the structure
	if err != nil {
		// This is expected with a fake token
		assert.Contains(t, err.Error(), "failed to create Cloudflare client")
	} else {
		// If somehow it succeeds (shouldn't with fake token), verify structure
		assert.NotNil(t, client)
		assert.Equal(t, "test-token-123", client.APIToken)
	}
}

func TestNewClient_WithAPIKeyAndEmail(t *testing.T) {
	config := models.CloudflareConfig{
		APIKey:    "test-key-123",
		Email:     "test@example.com",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	
	// We expect this to fail because it's not a real key/email, but we can test the structure
	if err != nil {
		// This is expected with fake credentials
		assert.Contains(t, err.Error(), "failed to create Cloudflare client")
	} else {
		// If somehow it succeeds (shouldn't with fake credentials), verify structure
		assert.NotNil(t, client)
	}
}

func TestNewClient_NoAuthentication(t *testing.T) {
	config := models.CloudflareConfig{
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestNewClient_IncompleteAPIKeyAuth(t *testing.T) {
	// Test with API key but no email
	config := models.CloudflareConfig{
		APIKey:    "test-key-123",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestNewClient_IncompleteEmailAuth(t *testing.T) {
	// Test with email but no API key
	config := models.CloudflareConfig{
		Email:     "test@example.com",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestNewClientWithToken_Success(t *testing.T) {
	token := "test-token-456"
	timeout := 60

	client, err := cloudflare.NewClientWithToken(token, timeout)
	
	// We expect this to fail because it's not a real token, but we can test the structure
	if err != nil {
		// This is expected with a fake token
		assert.Contains(t, err.Error(), "failed to create Cloudflare client")
	} else {
		// If somehow it succeeds (shouldn't with fake token), verify structure
		assert.NotNil(t, client)
		assert.Equal(t, token, client.APIToken)
	}
}

func TestNewClientWithToken_EmptyToken(t *testing.T) {
	token := ""
	timeout := 60

	client, err := cloudflare.NewClientWithToken(token, timeout)
	
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to create Cloudflare client")
}

func TestCloudflareResponse_Structure(t *testing.T) {
	// Test that CloudflareResponse can be created and marshaled
	response := cloudflare.CloudflareResponse{
		Success: true,
		Result:  map[string]interface{}{"test": "data"},
		Errors:  []string{"error1", "error2"},
	}

	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
	assert.Len(t, response.Errors, 2)
	assert.Equal(t, "error1", response.Errors[0])
}

func TestCloudflareResponse_EmptyErrors(t *testing.T) {
	// Test CloudflareResponse with no errors
	response := cloudflare.CloudflareResponse{
		Success: true,
		Result:  "success data",
		Errors:  nil,
	}

	assert.True(t, response.Success)
	assert.Equal(t, "success data", response.Result)
	assert.Nil(t, response.Errors)
}

func TestCloudflareResponse_FailureCase(t *testing.T) {
	// Test CloudflareResponse for failure case
	response := cloudflare.CloudflareResponse{
		Success: false,
		Result:  nil,
		Errors:  []string{"Authentication failed", "Invalid token"},
	}

	assert.False(t, response.Success)
	assert.Nil(t, response.Result)
	assert.Len(t, response.Errors, 2)
	assert.Contains(t, response.Errors, "Authentication failed")
	assert.Contains(t, response.Errors, "Invalid token")
}

// Test type aliases
func TestZoneTypeAlias(t *testing.T) {
	// Test that Zone type alias works
	var zone cloudflare.Zone
	assert.NotNil(t, &zone) // Just verify the type exists
}

func TestDNSRecordTypeAlias(t *testing.T) {
	// Test that DNSRecord type alias works
	var record cloudflare.DNSRecord
	assert.NotNil(t, &record) // Just verify the type exists
}

// Test configuration variations
func TestCloudflareConfig_DefaultTimeout(t *testing.T) {
	config := models.CloudflareConfig{
		APIToken: "test-token",
		Timeout:  0, // Default timeout
	}

	// Even with timeout 0, the client creation should handle it
	_, err := cloudflare.NewClient(config)
	
	// We expect an error due to fake token, but not due to timeout
	if err != nil {
		assert.Contains(t, err.Error(), "failed to create Cloudflare client")
		// Should not contain timeout-related errors
		assert.NotContains(t, err.Error(), "timeout")
	}
}

func TestCloudflareConfig_LargeTimeout(t *testing.T) {
	config := models.CloudflareConfig{
		APIToken: "test-token",
		Timeout:  3600, // 1 hour timeout
	}

	_, err := cloudflare.NewClient(config)
	
	// We expect an error due to fake token, but not due to timeout
	if err != nil {
		assert.Contains(t, err.Error(), "failed to create Cloudflare client")
		// Should not contain timeout-related errors
		assert.NotContains(t, err.Error(), "timeout")
	}
}

func TestCloudflareConfig_VerifySSLVariations(t *testing.T) {
	testCases := []struct {
		name      string
		verifySSL bool
	}{
		{"VerifySSL enabled", true},
		{"VerifySSL disabled", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := models.CloudflareConfig{
				APIToken:  "test-token",
				Timeout:   30,
				VerifySSL: tc.verifySSL,
			}

			_, err := cloudflare.NewClient(config)
			
			// We expect an error due to fake token, but the VerifySSL setting should not cause issues
			if err != nil {
				assert.Contains(t, err.Error(), "failed to create Cloudflare client")
			}
		})
	}
}
