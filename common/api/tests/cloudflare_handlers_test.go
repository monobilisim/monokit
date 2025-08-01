//go:build with_api

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/monobilisim/monokit/common/api/cloudflare"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// MockCloudflareClient is a mock implementation of the Cloudflare client for testing
type MockCloudflareClient struct{}

func (m *MockCloudflareClient) VerifyZone(zoneID string) error {
	// Mock implementation - always succeeds
	return nil
}

func (m *MockCloudflareClient) TestConnection() error {
	// Mock implementation - always succeeds
	return nil
}

// Helper function to create a mock Cloudflare service for testing
func createMockCloudflareService(t *testing.T, db *gorm.DB) *cloudflare.Service {
	config := models.CloudflareConfig{
		Enabled:   false, // Disable to avoid real API calls during testing
		APIToken:  "test-token",
		Timeout:   30,
		VerifySSL: true,
	}
	service := cloudflare.NewService(db, config)
	// With Enabled: false, the service will skip API verification
	// but still allow database operations for testing
	return service
}

func TestCreateCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test domain and admin user
	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

	// Mock Cloudflare API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/zones/zone123" {
			zone := cloudflare.Zone{
				ID:     "zone123",
				Name:   "example.com",
				Status: "active",
				Type:   "full",
			}
			response := cloudflare.CloudflareResponse{
				Success: true,
				Result:  zone,
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	cfService := createMockCloudflareService(t, db)

	req := models.CreateCloudflareDomainRequest{
		ZoneName:     "example.com",
		ZoneID:       "zone123",
		APIToken:     "test-token",
		ProxyEnabled: &[]bool{true}[0],
	}

	c, w := CreateRequestContext("POST", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare", req)
	SetPathParams(c, map[string]string{"domain_id": strconv.Itoa(int(domain.ID))})
	AuthorizeContext(c, admin)

	handler := cloudflare.CreateCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.CloudflareDomainResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, domain.ID, response.DomainID)
	assert.Equal(t, "example.com", response.ZoneName)
	assert.Equal(t, "zone123", response.ZoneID)
	assert.True(t, response.ProxyEnabled)
	assert.True(t, response.Active)
}

func TestCreateCloudflareDomain_NonAdmin(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	user := SetupTestUser(t, db, "regular-user") // Not an admin

	cfService := createMockCloudflareService(t, db)

	req := models.CreateCloudflareDomainRequest{
		ZoneName: "example.com",
		ZoneID:   "zone123",
	}

	c, w := CreateRequestContext("POST", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare", req)
	SetPathParams(c, map[string]string{"domain_id": strconv.Itoa(int(domain.ID))})
	AuthorizeContext(c, user)

	handler := cloudflare.CreateCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateCloudflareDomain_InvalidDomainID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)
	cfService := createMockCloudflareService(t, db)

	req := models.CreateCloudflareDomainRequest{
		ZoneName: "example.com",
		ZoneID:   "zone123",
	}

	c, w := CreateRequestContext("POST", "/api/v1/domains/invalid/cloudflare", req)
	SetPathParams(c, map[string]string{"domain_id": "invalid"})
	AuthorizeContext(c, admin)

	handler := cloudflare.CreateCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCloudflareDomain_DomainNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)
	cfService := createMockCloudflareService(t, db)

	req := models.CreateCloudflareDomainRequest{
		ZoneName: "example.com",
		ZoneID:   "zone123",
	}

	c, w := CreateRequestContext("POST", "/api/v1/domains/999/cloudflare", req)
	SetPathParams(c, map[string]string{"domain_id": "999"})
	AuthorizeContext(c, admin)

	handler := cloudflare.CreateCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCloudflareDomains_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

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

	cfService := createMockCloudflareService(t, db)

	c, w := CreateRequestContext("GET", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare", nil)
	SetPathParams(c, map[string]string{"domain_id": strconv.Itoa(int(domain.ID))})
	AuthorizeContext(c, admin)

	handler := cloudflare.GetCloudflareDomains(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.CloudflareDomainResponse
	ExtractJSONResponse(t, w, &response)

	assert.Len(t, response, 2)
	assert.Equal(t, "example.com", response[0].ZoneName)
	assert.Equal(t, "test.com", response[1].ZoneName)
}

func TestGetCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	cfService := createMockCloudflareService(t, db)

	c, w := CreateRequestContext("GET", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare/"+strconv.Itoa(int(cfDomain.ID)), nil)
	SetPathParams(c, map[string]string{
		"domain_id":    strconv.Itoa(int(domain.ID)),
		"cf_domain_id": strconv.Itoa(int(cfDomain.ID)),
	})
	AuthorizeContext(c, admin)

	handler := cloudflare.GetCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.CloudflareDomainResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, cfDomain.ID, response.ID)
	assert.Equal(t, "example.com", response.ZoneName)
	assert.Equal(t, "zone1", response.ZoneID)
}

func TestUpdateCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	cfService := createMockCloudflareService(t, db)

	req := models.UpdateCloudflareDomainRequest{
		ZoneName:     "updated.com",
		ProxyEnabled: &[]bool{false}[0],
	}

	c, w := CreateRequestContext("PUT", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare/"+strconv.Itoa(int(cfDomain.ID)), req)
	SetPathParams(c, map[string]string{
		"domain_id":    strconv.Itoa(int(domain.ID)),
		"cf_domain_id": strconv.Itoa(int(cfDomain.ID)),
	})
	AuthorizeContext(c, admin)

	handler := cloudflare.UpdateCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.CloudflareDomainResponse
	ExtractJSONResponse(t, w, &response)

	assert.Equal(t, cfDomain.ID, response.ID)
	assert.Equal(t, "updated.com", response.ZoneName)
	assert.False(t, response.ProxyEnabled)
}

func TestDeleteCloudflareDomain_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	cfService := createMockCloudflareService(t, db)

	c, w := CreateRequestContext("DELETE", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare/"+strconv.Itoa(int(cfDomain.ID)), nil)
	SetPathParams(c, map[string]string{
		"domain_id":    strconv.Itoa(int(domain.ID)),
		"cf_domain_id": strconv.Itoa(int(cfDomain.ID)),
	})
	AuthorizeContext(c, admin)

	handler := cloudflare.DeleteCloudflareDomain(db, cfService)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response contains success message
	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "Cloudflare domain deleted successfully", response["message"])

	// Verify it was soft deleted from database (should not be found in normal queries)
	var deletedCfDomain models.CloudflareDomain
	err := db.First(&deletedCfDomain, cfDomain.ID).Error
	assert.Error(t, err) // Should not be found in normal query due to soft delete

	// Verify it still exists but is soft deleted
	var softDeletedCfDomain models.CloudflareDomain
	err = db.Unscoped().First(&softDeletedCfDomain, cfDomain.ID).Error
	assert.NoError(t, err)                          // Should be found with Unscoped
	assert.NotNil(t, softDeletedCfDomain.DeletedAt) // Should have deleted_at timestamp
}

func TestTestCloudflareConnection_Disabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	domain := SetupTestDomain(t, db, "test-domain")
	admin := SetupTestAdmin(t, db)

	cfDomain := models.CloudflareDomain{
		DomainID:     domain.ID,
		ZoneName:     "example.com",
		ZoneID:       "zone1",
		APIToken:     "test-token",
		ProxyEnabled: true,
		Active:       true,
	}
	require.NoError(t, db.Create(&cfDomain).Error)

	// Use disabled Cloudflare service for testing
	cfService := createMockCloudflareService(t, db)

	c, w := CreateRequestContext("POST", "/api/v1/domains/"+strconv.Itoa(int(domain.ID))+"/cloudflare/"+strconv.Itoa(int(cfDomain.ID))+"/test", nil)
	SetPathParams(c, map[string]string{
		"domain_id":    strconv.Itoa(int(domain.ID)),
		"cf_domain_id": strconv.Itoa(int(cfDomain.ID)),
	})
	AuthorizeContext(c, admin)

	handler := cloudflare.TestCloudflareConnection(db, cfService)
	handler(c)

	// When Cloudflare is disabled, the connection test should fail
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Cloudflare integration is not enabled")
}
