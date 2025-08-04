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

// Test DNS record structure and validation
func TestDNSRecord_Structure(t *testing.T) {
	// Test that we can create DNS records with various configurations
	testCases := []struct {
		name   string
		record cloudflare.DNSRecord
	}{
		{
			name: "A Record",
			record: cloudflare.DNSRecord{
				Type:    "A",
				Name:    "test.example.com",
				Content: "192.168.1.1",
				TTL:     300,
			},
		},
		{
			name: "CNAME Record",
			record: cloudflare.DNSRecord{
				Type:    "CNAME",
				Name:    "www.example.com",
				Content: "example.com",
				TTL:     3600,
			},
		},
		{
			name: "MX Record",
			record: cloudflare.DNSRecord{
				Type:     "MX",
				Name:     "example.com",
				Content:  "mail.example.com",
				TTL:      3600,
				Priority: &[]uint16{10}[0],
			},
		},
		{
			name: "TXT Record",
			record: cloudflare.DNSRecord{
				Type:    "TXT",
				Name:    "example.com",
				Content: "v=spf1 include:_spf.google.com ~all",
				TTL:     300,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify record structure
			assert.NotEmpty(t, tc.record.Type)
			assert.NotEmpty(t, tc.record.Name)
			assert.NotEmpty(t, tc.record.Content)
			assert.Greater(t, tc.record.TTL, 0)
		})
	}
}

// Test client method signatures and error handling
func TestClient_MethodSignatures(t *testing.T) {
	// Create a client that will fail API calls but has valid structure
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "invalid-token-for-testing",
		Timeout:   1, // Short timeout to fail quickly
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test method signatures by calling them (they will fail but we test the interface)
	t.Run("GetZones", func(t *testing.T) {
		zones, err := client.GetZones()
		assert.Error(t, err) // Expected to fail with invalid token
		assert.Nil(t, zones)
		assert.Contains(t, err.Error(), "failed to list zones")
	})

	t.Run("GetZone", func(t *testing.T) {
		zone, err := client.GetZone("invalid-zone-id")
		assert.Error(t, err) // Expected to fail
		assert.Nil(t, zone)
		assert.Contains(t, err.Error(), "failed to get zone details")
	})

	t.Run("GetZoneByName", func(t *testing.T) {
		zone, err := client.GetZoneByName("nonexistent.example.com")
		assert.Error(t, err) // Expected to fail
		assert.Nil(t, zone)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("VerifyZone", func(t *testing.T) {
		err := client.VerifyZone("invalid-zone-id")
		assert.Error(t, err) // Expected to fail
	})

	t.Run("TestConnection", func(t *testing.T) {
		err := client.TestConnection()
		assert.Error(t, err) // Expected to fail with invalid token
		assert.Contains(t, err.Error(), "failed to verify Cloudflare API connection")
	})
}

// Test DNS operations method signatures
func TestClient_DNSOperations(t *testing.T) {
	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "invalid-token-for-testing",
		Timeout:   1, // Short timeout
		VerifySSL: true,
	}

	client, err := cloudflare.NewClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	testZoneID := "invalid-zone-id"
	testRecordID := "invalid-record-id"

	t.Run("GetDNSRecords", func(t *testing.T) {
		records, err := client.GetDNSRecords(testZoneID)
		assert.Error(t, err) // Expected to fail
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "failed to list DNS records")
	})

	t.Run("CreateDNSRecord", func(t *testing.T) {
		record := cloudflare.DNSRecord{
			Type:    "A",
			Name:    "test.example.com",
			Content: "192.168.1.1",
			TTL:     300,
		}

		createdRecord, err := client.CreateDNSRecord(testZoneID, record)
		assert.Error(t, err) // Expected to fail
		assert.Nil(t, createdRecord)
		assert.Contains(t, err.Error(), "failed to create DNS record")
	})

	t.Run("UpdateDNSRecord", func(t *testing.T) {
		record := cloudflare.DNSRecord{
			Type:    "A",
			Name:    "test.example.com",
			Content: "192.168.1.2",
			TTL:     600,
		}

		updatedRecord, err := client.UpdateDNSRecord(testZoneID, testRecordID, record)
		assert.Error(t, err) // Expected to fail
		assert.Nil(t, updatedRecord)
		assert.Contains(t, err.Error(), "failed to update DNS record")
	})

	t.Run("DeleteDNSRecord", func(t *testing.T) {
		err := client.DeleteDNSRecord(testZoneID, testRecordID)
		assert.Error(t, err) // Expected to fail
		assert.Contains(t, err.Error(), "failed to delete DNS record")
	})
}
