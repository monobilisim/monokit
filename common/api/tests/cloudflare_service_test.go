//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{
		Enabled:   true,
		APIToken:  "test-token",
		Timeout:   30,
		VerifySSL: true,
	}

	service := cloudflare.NewService(db, config)

	assert.NotNil(t, service)
}

func TestNewService_Disabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{
		Enabled: false,
	}

	service := cloudflare.NewService(db, config)

	assert.NotNil(t, service)
}

func TestService_CreateCloudflareDomain_Success(t *testing.T) {
	// Skip this test as it requires real Cloudflare API integration
	t.Skip("Skipping CreateCloudflareDomain test - requires real Cloudflare API integration")
}

func TestService_CreateCloudflareDomain_Disabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")

	config := models.CloudflareConfig{
		Enabled: false,
	}

	service := cloudflare.NewService(db, config)

	req := models.CreateCloudflareDomainRequest{
		ZoneName: "example.com",
		ZoneID:   "zone123",
	}

	cfDomain, err := service.CreateCloudflareDomain(domain.ID, req)
	assert.NoError(t, err)
	assert.NotNil(t, cfDomain)
	assert.Equal(t, "example.com", cfDomain.ZoneName)
	assert.Equal(t, "zone123", cfDomain.ZoneID)
}

func TestService_GetCloudflareDomains_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")

	// Create test Cloudflare domains
	cfDomain1 := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	cfDomain2 := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "test.com",
		ZoneID:       "zone2",
		ProxyEnabled: false,
		Active:       true,
	}

	require.NoError(t, db.Create(&cfDomain1).Error)
	require.NoError(t, db.Create(&cfDomain2).Error)

	config := models.CloudflareConfig{Enabled: true}
	service := cloudflare.NewService(db, config)

	cfDomains, err := service.GetCloudflareDomains(domain.ID)
	require.NoError(t, err)

	assert.Len(t, cfDomains, 2)
	assert.Equal(t, "example.com", cfDomains[0].ZoneName)
	assert.Equal(t, "test.com", cfDomains[1].ZoneName)
}

func TestService_GetCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	config := models.CloudflareConfig{Enabled: true}
	service := cloudflare.NewService(db, config)

	retrievedCfDomain, err := service.GetCloudflareDomain(cfDomain.ID)
	require.NoError(t, err)

	assert.Equal(t, cfDomain.ID, retrievedCfDomain.ID)
	assert.Equal(t, "example.com", retrievedCfDomain.ZoneName)
	assert.Equal(t, "zone1", retrievedCfDomain.ZoneID)
}

func TestService_GetCloudflareDomain_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{Enabled: true}
	service := cloudflare.NewService(db, config)

	_, err := service.GetCloudflareDomain(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cloudflare domain not found")
}

func TestService_UpdateCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	config := models.CloudflareConfig{Enabled: true}
	service := cloudflare.NewService(db, config)

	req := models.UpdateCloudflareDomainRequest{
		ZoneName:     "updated.com",
		ProxyEnabled: &[]bool{false}[0],
		Active:       &[]bool{false}[0],
	}

	updatedCfDomain, err := service.UpdateCloudflareDomain(cfDomain.ID, req)
	require.NoError(t, err)

	assert.Equal(t, cfDomain.ID, updatedCfDomain.ID)
	assert.Equal(t, "updated.com", updatedCfDomain.ZoneName)
	assert.Equal(t, "zone1", updatedCfDomain.ZoneID) // Should remain unchanged
	assert.False(t, updatedCfDomain.ProxyEnabled)
	assert.False(t, updatedCfDomain.Active)
}

func TestService_DeleteCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	config := models.CloudflareConfig{Enabled: true}
	service := cloudflare.NewService(db, config)

	err := service.DeleteCloudflareDomain(cfDomain.ID)
	require.NoError(t, err)

	// Verify it was deleted from database
	var deletedCfDomain models.CloudflareDomain
	err = db.First(&deletedCfDomain, cfDomain.ID).Error
	assert.Error(t, err) // Should not be found
}

func TestService_GetClient_DomainToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{
		Enabled:  true,
		APIToken: "global-token",
		Timeout:  30,
	}

	service := cloudflare.NewService(db, config)

	cfDomain := &models.CloudflareDomain{
		APIToken: "domain-specific-token",
	}

	client, err := service.GetClient(cfDomain)
	require.NoError(t, err)

	assert.Equal(t, "domain-specific-token", client.APIToken)
}

func TestService_GetClient_GlobalToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{
		Enabled:  true,
		APIToken: "global-token",
		Timeout:  30,
	}

	service := cloudflare.NewService(db, config)

	cfDomain := &models.CloudflareDomain{
		APIToken: "", // No domain-specific token
	}

	client, err := service.GetClient(cfDomain)
	require.NoError(t, err)

	assert.Equal(t, "global-token", client.APIToken)
}

func TestService_GetClient_NoToken(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	config := models.CloudflareConfig{
		Enabled:  true,
		APIToken: "", // No global token
		Timeout:  30,
	}

	service := cloudflare.NewService(db, config)

	cfDomain := &models.CloudflareDomain{
		APIToken: "", // No domain-specific token
	}

	_, err := service.GetClient(cfDomain)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no global Cloudflare client available")
}
