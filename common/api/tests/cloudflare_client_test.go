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
