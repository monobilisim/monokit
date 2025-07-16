//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterHost_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	hostData := models.Host{
		Name:                "new-host",
		CpuCores:            8,
		Ram:                 "16GB",
		MonokitVersion:      "2.0.0",
		Os:                  "Ubuntu 22.04",
		DisabledComponents:  "nil",
		InstalledComponents: "mysql,redis",
		IpAddress:           "192.168.1.100",
		Status:              "online",
		Groups:              "web-servers",
		Inventory:           "default",
	}

	c, w := CreateRequestContext("POST", "/api/v1/hosts", hostData)

	handler := ExportRegisterHost(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify host was created in database
	var createdHost models.Host
	err := db.Where("name = ?", "new-host").First(&createdHost).Error
	require.NoError(t, err)
	assert.Equal(t, "new-host", createdHost.Name)
	assert.Equal(t, 8, createdHost.CpuCores)
	assert.Equal(t, "16GB", createdHost.Ram)
}

func TestRegisterHost_UpdateExisting(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create existing host
	existingHost := SetupTestHost(t, db, "existing-host")
	originalVersion := existingHost.MonokitVersion

	// Create host key for authentication
	hostKey := SetupTestHostKey(t, db, existingHost, "update_host_key_123")

	// Update with new data
	updateData := models.Host{
		Name:                "existing-host",
		CpuCores:            16,
		Ram:                 "32GB",
		MonokitVersion:      "2.1.0",
		Os:                  "Ubuntu 22.04 LTS",
		DisabledComponents:  "backup",
		InstalledComponents: "mysql,redis,nginx",
		IpAddress:           "192.168.1.200",
		Status:              "online",
		Groups:              "web-servers,db-servers",
		Inventory:           "default",
	}

	c, w := CreateRequestContext("POST", "/api/v1/hosts", updateData)
	c.Request.Header.Set("Authorization", hostKey.Token)

	handler := ExportRegisterHost(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was updated
	var updatedHost models.Host
	err := db.Where("name = ?", "existing-host").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, 16, updatedHost.CpuCores)
	assert.Equal(t, "32GB", updatedHost.Ram)
	assert.Equal(t, "2.1.0", updatedHost.MonokitVersion)
	assert.NotEqual(t, originalVersion, updatedHost.MonokitVersion)
}

func TestRegisterHost_InvalidJSON(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("POST", "/api/v1/hosts", "invalid json")

	handler := ExportRegisterHost(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegisterHost_MissingRequiredFields(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Host with missing name
	incompleteHost := models.Host{
		CpuCores: 4,
		Ram:      "8GB",
	}

	c, w := CreateRequestContext("POST", "/api/v1/hosts", incompleteHost)

	handler := ExportRegisterHost(db)
	handler(c)

	// Should still succeed but with default values (creates new host)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetAllHosts_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test user for authentication
	user := SetupTestUser(t, db, "hostviewer")

	// Create test hosts
	host1 := SetupTestHost(t, db, "host1")
	host2 := SetupTestHost(t, db, "host2")
	host3 := SetupTestHost(t, db, "host3")

	// Populate global HostsList that getAllHosts uses
	models.HostsList = []models.Host{host1, host2, host3}

	c, w := CreateRequestContext("GET", "/api/v1/hosts", nil)
	AuthorizeContext(c, user)

	handler := ExportGetAllHosts(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var hosts []models.Host
	ExtractJSONResponse(t, w, &hosts)
	assert.GreaterOrEqual(t, len(hosts), 3)

	// Check that our test hosts are included
	hostNames := make(map[string]bool)
	for _, host := range hosts {
		hostNames[host.Name] = true
	}
	assert.True(t, hostNames["host1"])
	assert.True(t, hostNames["host2"])
	assert.True(t, hostNames["host3"])
}

func TestGetAllHosts_EmptyDatabase(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test user for authentication
	user := SetupTestUser(t, db, "hostviewer")

	// Ensure global HostsList is empty
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("GET", "/api/v1/hosts", nil)
	AuthorizeContext(c, user)

	handler := ExportGetAllHosts(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var hosts []models.Host
	ExtractJSONResponse(t, w, &hosts)
	assert.Equal(t, 0, len(hosts))
}

func TestGetHostByName_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	testHost := SetupTestHost(t, db, "target-host")

	// Add host to global list (as the actual handler does)
	models.HostsList = []models.Host{testHost}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/target-host", nil)
	SetPathParams(c, map[string]string{"name": "target-host"})

	handler := ExportGetHostByName()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var returnedHost models.Host
	ExtractJSONResponse(t, w, &returnedHost)
	assert.Equal(t, "target-host", returnedHost.Name)
}

func TestGetHostByName_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/nonexistent", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent"})

	handler := ExportGetHostByName()
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteHost_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	testHost := SetupTestHost(t, db, "host-to-delete")
	models.HostsList = []models.Host{testHost}

	c, w := CreateRequestContext("DELETE", "/api/v1/hosts/host-to-delete", nil)
	SetPathParams(c, map[string]string{"name": "host-to-delete"})

	handler := ExportDeleteHost(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host is soft deleted (no longer accessible through normal queries)
	var hostCount int64
	db.Model(&models.Host{}).Where("name = ?", "host-to-delete").Count(&hostCount)
	assert.Equal(t, int64(0), hostCount)

	// Verify host still exists but is soft deleted
	var deletedHost models.Host
	err := db.Unscoped().Where("name = ?", "host-to-delete").First(&deletedHost).Error
	require.NoError(t, err)
	assert.NotNil(t, deletedHost.DeletedAt)
}

func TestDeleteHost_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("DELETE", "/api/v1/hosts/nonexistent", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent"})

	handler := ExportDeleteHost(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestForceDeleteHost_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	testHost := SetupTestHost(t, db, "host-to-force-delete")
	models.HostsList = []models.Host{testHost}

	c, w := CreateRequestContext("DELETE", "/api/v1/hosts/host-to-force-delete/force", nil)
	SetPathParams(c, map[string]string{"name": "host-to-force-delete"})

	handler := ExportForceDeleteHost(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host is completely deleted
	var hostCount int64
	db.Model(&models.Host{}).Where("name = ?", "host-to-force-delete").Count(&hostCount)
	assert.Equal(t, int64(0), hostCount)
}

func TestForceDeleteHost_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("DELETE", "/api/v1/hosts/nonexistent/force", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent"})

	handler := ExportForceDeleteHost(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateHost_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	testHost := SetupTestHost(t, db, "host-to-update")
	models.HostsList = []models.Host{testHost}

	updateData := map[string]interface{}{
		"wantsUpdateTo": "3.0.0",
		"groups":        "new-group",
	}

	c, w := CreateRequestContext("PUT", "/api/v1/hosts/host-to-update", updateData)
	SetPathParams(c, map[string]string{"name": "host-to-update"})

	handler := ExportUpdateHost(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was updated
	var updatedHost models.Host
	err := db.Where("name = ?", "host-to-update").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", updatedHost.WantsUpdateTo)
	assert.Equal(t, "new-group", updatedHost.Groups)
}

func TestUpdateHost_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	models.HostsList = []models.Host{}

	updateData := map[string]interface{}{
		"wantsUpdateTo": "3.0.0",
	}

	c, w := CreateRequestContext("PUT", "/api/v1/hosts/nonexistent", updateData)
	SetPathParams(c, map[string]string{"name": "nonexistent"})

	handler := ExportUpdateHost(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAssignedHosts_WithUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create user with specific inventories (filtering is by inventory, not groups)
	user := SetupTestUser(t, db, "testuser")
	user.Inventories = "dev,staging"
	db.Save(&user)

	// Create hosts with different inventories
	host1 := SetupTestHost(t, db, "dev-host")
	host1.Inventory = "dev"
	require.NoError(t, db.Save(&host1).Error)

	host2 := SetupTestHost(t, db, "staging-host")
	host2.Inventory = "staging"
	require.NoError(t, db.Save(&host2).Error)

	host3 := SetupTestHost(t, db, "prod-host")
	host3.Inventory = "production"
	require.NoError(t, db.Save(&host3).Error)

	// Update global HostsList that the function uses
	models.HostsList = []models.Host{host1, host2, host3}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/assigned", nil)
	AuthorizeContext(c, user)

	handler := ExportGetAssignedHosts(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var hosts []models.Host
	ExtractJSONResponse(t, w, &hosts)

	// Should return hosts that match user's inventories (dev, staging)
	assert.GreaterOrEqual(t, len(hosts), 2)

	hostNames := make(map[string]bool)
	for _, host := range hosts {
		hostNames[host.Name] = true
	}
	assert.True(t, hostNames["dev-host"])
	assert.True(t, hostNames["staging-host"])
	assert.False(t, hostNames["prod-host"]) // Should not include production host
}

func TestGetAssignedHosts_AdminUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	admin := SetupTestAdmin(t, db)

	// Create test hosts
	host1 := SetupTestHost(t, db, "host1")
	host2 := SetupTestHost(t, db, "host2")

	// Update global HostsList that the function uses
	models.HostsList = []models.Host{host1, host2}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/assigned", nil)
	AuthorizeContext(c, admin)

	handler := ExportGetAssignedHosts(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var hosts []models.Host
	ExtractJSONResponse(t, w, &hosts)

	// Admin should see all hosts
	assert.GreaterOrEqual(t, len(hosts), 2)
}

func TestGetAssignedHosts_NoUser(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	c, w := CreateRequestContext("GET", "/api/v1/hosts/assigned", nil)
	// No user in context

	handler := ExportGetAssignedHosts(db)
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHostRegistration_ConcurrentUpdates(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	hostData := models.Host{
		Name:           "concurrent-host",
		CpuCores:       4,
		Ram:            "8GB",
		MonokitVersion: "1.0.0",
		Os:             "Test OS",
		Status:         "online",
		Inventory:      "default",
	}

	handler := ExportRegisterHost(db)

	// First registration - creates new host
	c1, w1 := CreateRequestContext("POST", "/api/v1/hosts", hostData)
	handler(c1)
	assert.Equal(t, http.StatusCreated, w1.Code) // First registration creates host

	// Extract the API key from the first response to use for the second request
	var firstResponse map[string]interface{}
	ExtractJSONResponse(t, w1, &firstResponse)
	apiKey := firstResponse["apiKey"].(string)

	// Second registration - updates existing host with authentication
	c2, w2 := CreateRequestContext("POST", "/api/v1/hosts", hostData)
	c2.Request.Header.Set("Authorization", apiKey)
	handler(c2)
	assert.Equal(t, http.StatusOK, w2.Code) // Second registration updates host

	// Should only have one host in database
	var hostCount int64
	db.Model(&models.Host{}).Where("name = ?", "concurrent-host").Count(&hostCount)
	assert.Equal(t, int64(1), hostCount)
}
