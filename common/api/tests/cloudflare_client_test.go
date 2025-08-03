//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "test-token",
		APIKey:    "test-key",
		Email:     "test@example.com",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClientWithToken(t *testing.T) {
	client, err := cloudflare.NewClientWithToken("test-token", 30)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClient_TestConnection_Success(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping TestConnection test - requires real Cloudflare API token")
}

func TestClient_TestConnection_Failure(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping TestConnection test - requires real Cloudflare API token")
}

func TestClient_GetZones_Success(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping GetZones test - requires real Cloudflare API token")
}

func TestClient_GetZone_Success(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping GetZone test - requires real Cloudflare API token")
}

func TestClient_GetZoneByName_Success(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping GetZoneByName test - requires real Cloudflare API token")
}

func TestClient_GetZoneByName_NotFound(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping GetZoneByName test - requires real Cloudflare API token")
}

func TestClient_VerifyZone_Success(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping VerifyZone test - requires real Cloudflare API token")
}

func TestClient_VerifyZone_Failure(t *testing.T) {
	// Skip this test as it requires a real Cloudflare API token
	t.Skip("Skipping VerifyZone test - requires real Cloudflare API token")
}

func TestClient_NoAuthentication(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "",
		APIKey:    "",
		Email:     "",
		Timeout:   30,
		VerifySSL: true,
	}

	_, err := cloudflare.NewClient(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestNewClient_WithAPIKey(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "",
		APIKey:    "test-api-key",
		Email:     "test@example.com",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithAPIKeyNoEmail(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "",
		APIKey:    "test-api-key",
		Email:     "",
		Timeout:   30,
		VerifySSL: true,
	}

	_, err := cloudflare.NewClient(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid authentication method provided")
}

func TestNewClient_InvalidAPIToken(t *testing.T) {
	// Test with invalid token format
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "invalid-token",
		Timeout:   30,
		VerifySSL: true,
	}

	// This should still create a client, but fail on actual API calls
	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_TimeoutConfiguration(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "test-token",
		Timeout:   60,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClientWithToken_ZeroTimeout(t *testing.T) {
	client, err := cloudflare.NewClientWithToken("test-token", 0)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClientWithToken_NegativeTimeout(t *testing.T) {
	client, err := cloudflare.NewClientWithToken("test-token", -10)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_EmptyToken(t *testing.T) {
	_, err := cloudflare.NewClientWithToken("", 30)
	// This might succeed or fail depending on the cloudflare-go library implementation
	// We just test that it doesn't panic
	_ = err
}

func TestClient_StructureValidation(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "test-token",
		Timeout:   30,
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test that the client has the expected structure
	// We can't access private fields, but we can verify the client was created
	assert.IsType(t, &cloudflare.Client{}, client)
}

func TestClient_ConfigurationVariations(t *testing.T) {
	testCases := []struct {
		name     string
		config   models.CloudflareConfig
		hasError bool
	}{
		{
			name: "Valid API Token",
			config: models.CloudflareConfig{
				Enabled:   true,
				APIToken:  "valid-token",
				Timeout:   30,
				VerifySSL: true,
			},
			hasError: false,
		},
		{
			name: "Valid API Key with Email",
			config: models.CloudflareConfig{
				Enabled:   true,
				APIKey:    "valid-key",
				Email:     "test@example.com",
				Timeout:   30,
				VerifySSL: true,
			},
			hasError: false,
		},
		{
			name: "Both API Token and Key",
			config: models.CloudflareConfig{
				Enabled:   true,
				APIToken:  "token",
				APIKey:    "key",
				Email:     "test@example.com",
				Timeout:   30,
				VerifySSL: true,
			},
			hasError: false, // Should prefer API Token
		},
		{
			name: "No Authentication",
			config: models.CloudflareConfig{
				Enabled:   true,
				Timeout:   30,
				VerifySSL: true,
			},
			hasError: true,
		},
		{
			name: "API Key without Email",
			config: models.CloudflareConfig{
				Enabled:   true,
				APIKey:    "key",
				Timeout:   30,
				VerifySSL: true,
			},
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := cloudflare.NewClient(tc.config)
			if tc.hasError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
