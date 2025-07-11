//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============ INVENTORY MANAGEMENT TESTS ============

func TestGetAllInventories_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test user for authentication
	user := SetupTestUser(t, db, "inventoryviewer")

	// Create test inventories in database
	inventory1 := models.Inventory{Name: "production"}
	inventory2 := models.Inventory{Name: "staging"}
	inventory3 := models.Inventory{Name: "development"}

	require.NoError(t, db.Create(&inventory1).Error)
	require.NoError(t, db.Create(&inventory2).Error)
	require.NoError(t, db.Create(&inventory3).Error)

	c, w := CreateRequestContext("GET", "/api/v1/inventory", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetAllInventories(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var inventories []models.Inventory
	ExtractJSONResponse(t, w, &inventories)

	// Should return at least our 3 test inventories + the default one
	assert.GreaterOrEqual(t, len(inventories), 3)

	// Check for our test inventories
	inventoryNames := make(map[string]bool)
	for _, inv := range inventories {
		inventoryNames[inv.Name] = true
	}
	assert.True(t, inventoryNames["production"])
	assert.True(t, inventoryNames["staging"])
	assert.True(t, inventoryNames["development"])
}

func TestCreateInventory_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "inventorycreator")

	inventoryData := models.Inventory{
		Name: "testing",
	}

	c, w := CreateRequestContext("POST", "/api/v1/inventory", inventoryData)
	AuthorizeContext(c, user)

	handler := server.ExportCreateInventory(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createdInventory models.Inventory
	ExtractJSONResponse(t, w, &createdInventory)
	assert.Equal(t, "testing", createdInventory.Name)
	assert.NotZero(t, createdInventory.ID)

	// Verify it was actually created in the database
	var dbInventory models.Inventory
	err := db.Where("name = ?", "testing").First(&dbInventory).Error
	require.NoError(t, err)
	assert.Equal(t, "testing", dbInventory.Name)
}

func TestCreateInventory_InvalidJSON(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "inventorycreator")

	c, w := CreateRequestContext("POST", "/api/v1/inventory", "invalid json")
	AuthorizeContext(c, user)

	handler := server.ExportCreateInventory(db)
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteInventory_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "inventorydeleter")

	// Create test inventory
	inventory := models.Inventory{Name: "to-be-deleted"}
	require.NoError(t, db.Create(&inventory).Error)

	c, w := CreateRequestContext("DELETE", "/api/v1/inventory/to-be-deleted", nil)
	SetPathParams(c, map[string]string{"name": "to-be-deleted"})
	AuthorizeContext(c, user)

	handler := server.ExportDeleteInventory(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "deleted", response["status"])

	// Verify it was actually deleted from the database
	var deletedInventory models.Inventory
	err := db.Where("name = ?", "to-be-deleted").First(&deletedInventory).Error
	assert.Error(t, err) // Should not be found
}

func TestDeleteInventory_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "inventorydeleter")

	c, w := CreateRequestContext("DELETE", "/api/v1/inventory/nonexistent", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent"})
	AuthorizeContext(c, user)

	handler := server.ExportDeleteInventory(db)
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "Inventory not found", response["error"])
}

// ============ COMPONENT MANAGEMENT TESTS ============

func TestEnableComponent_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Create test host with disabled components
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "mysql::redis::nginx"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/component-test-host/enable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportEnableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "enabled", response["status"])

	// Verify mysql was removed from disabled components
	var updatedHost models.Host
	err := db.Where("name = ?", "component-test-host").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, "redis::nginx", updatedHost.DisabledComponents)
}

func TestEnableComponent_AlreadyEnabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Create test host with some disabled components (but not mysql)
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "redis::nginx"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/component-test-host/enable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportEnableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "already enabled", response["status"])
}

func TestEnableComponent_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/nonexistent/enable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportEnableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "not found", response["status"])
}

func TestDisableComponent_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Create test host with some disabled components
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "redis"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/component-test-host/disable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportDisableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "disabled", response["status"])

	// Verify mysql was added to disabled components
	var updatedHost models.Host
	err := db.Where("name = ?", "component-test-host").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, "redis::mysql", updatedHost.DisabledComponents)
}

func TestDisableComponent_AlreadyDisabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Create test host with mysql already disabled
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "mysql::redis"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/component-test-host/disable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportDisableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "already disabled", response["status"])
}

func TestDisableComponent_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentmanager")

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/nonexistent/disable/mysql", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportDisableComponent(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "not found", response["status"])
}

func TestGetComponentStatus_Enabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentviewer")

	// Create test host with some disabled components (but not mysql)
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "redis::nginx"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/component-test-host/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportGetComponentStatus()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "component-test-host", response["name"])
	assert.Equal(t, "mysql", response["service"])
	assert.Equal(t, false, response["disabled"])
	assert.Equal(t, "enabled", response["status"])
}

func TestGetComponentStatus_Disabled(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentviewer")

	// Create test host with mysql disabled
	host := SetupTestHost(t, db, "component-test-host")
	host.DisabledComponents = "mysql::redis"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/component-test-host/mysql", nil)
	SetPathParams(c, map[string]string{"name": "component-test-host", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportGetComponentStatus()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "component-test-host", response["name"])
	assert.Equal(t, "mysql", response["service"])
	assert.Equal(t, true, response["disabled"])
	assert.Equal(t, "disabled", response["status"])
}

func TestGetComponentStatus_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentviewer")

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("GET", "/api/v1/hosts/nonexistent/mysql", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportGetComponentStatus()
	handler(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "Host not found", response["error"])
}

func TestGetComponentStatus_MissingParams(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "componentviewer")

	c, w := CreateRequestContext("GET", "/api/v1/hosts//mysql", nil)
	SetPathParams(c, map[string]string{"name": "", "service": "mysql"})
	AuthorizeContext(c, user)

	handler := server.ExportGetComponentStatus()
	handler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "Missing host name or service name", response["error"])
}

// ============ GROUP MANAGEMENT TESTS ============

func TestGetAllGroups_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "groupviewer")

	// Create test hosts with different groups
	host1 := SetupTestHost(t, db, "host1")
	host1.Groups = "web-servers,database-servers"
	require.NoError(t, db.Save(&host1).Error)

	host2 := SetupTestHost(t, db, "host2")
	host2.Groups = "web-servers,load-balancers"
	require.NoError(t, db.Save(&host2).Error)

	host3 := SetupTestHost(t, db, "host3")
	host3.Groups = "database-servers"
	require.NoError(t, db.Save(&host3).Error)

	host4 := SetupTestHost(t, db, "host4")
	host4.Groups = "nil"
	require.NoError(t, db.Save(&host4).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host1, host2, host3, host4}

	c, w := CreateRequestContext("GET", "/api/v1/groups", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetAllGroups(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var groups []string
	ExtractJSONResponse(t, w, &groups)

	// Should contain unique groups from all hosts
	assert.Contains(t, groups, "web-servers")
	assert.Contains(t, groups, "database-servers")
	assert.Contains(t, groups, "load-balancers")

	// Should not contain "nil" or duplicates
	assert.NotContains(t, groups, "nil")

	// Check for uniqueness
	groupMap := make(map[string]bool)
	for _, group := range groups {
		assert.False(t, groupMap[group], "Group should not be duplicated: %s", group)
		groupMap[group] = true
	}
}

func TestGetAllGroups_EmptyHostsList(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "groupviewer")

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("GET", "/api/v1/groups", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetAllGroups(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var groups []string
	ExtractJSONResponse(t, w, &groups)
	assert.Equal(t, 0, len(groups))
}

func TestGetAllGroups_OnlyNilGroups(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "groupviewer")

	// Create hosts with only "nil" groups
	host1 := SetupTestHost(t, db, "host1")
	host1.Groups = "nil"
	host2 := SetupTestHost(t, db, "host2")
	host2.Groups = "nil"

	// Update global HostsList
	models.HostsList = []models.Host{host1, host2}

	c, w := CreateRequestContext("GET", "/api/v1/groups", nil)
	AuthorizeContext(c, user)

	handler := server.ExportGetAllGroups(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var groups []string
	ExtractJSONResponse(t, w, &groups)
	assert.Equal(t, 0, len(groups))
}

// ============ HOST VERSION MANAGEMENT TESTS ============

func TestUpdateHostVersion_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "versionmanager")

	// Create test host
	host := SetupTestHost(t, db, "version-test-host")
	host.WantsUpdateTo = ""
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/version-test-host/updateTo/2.0.0", nil)
	SetPathParams(c, map[string]string{"name": "version-test-host", "version": "2.0.0"})
	AuthorizeContext(c, user)

	handler := server.ExportUpdateHostVersion(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the version was updated in the database
	var updatedHost models.Host
	err := db.Where("name = ?", "version-test-host").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", updatedHost.WantsUpdateTo)
}

func TestUpdateHostVersion_HostNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "versionmanager")

	// Empty hosts list
	models.HostsList = []models.Host{}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/nonexistent/updateTo/2.0.0", nil)
	SetPathParams(c, map[string]string{"name": "nonexistent", "version": "2.0.0"})
	AuthorizeContext(c, user)

	handler := server.ExportUpdateHostVersion(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "not found", response["status"])
}

func TestUpdateHostVersion_EmptyVersion(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	user := SetupTestUser(t, db, "versionmanager")

	// Create test host
	host := SetupTestHost(t, db, "version-test-host")
	host.WantsUpdateTo = "1.0.0"
	require.NoError(t, db.Save(&host).Error)

	// Update global HostsList
	models.HostsList = []models.Host{host}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/version-test-host/updateTo/", nil)
	SetPathParams(c, map[string]string{"name": "version-test-host", "version": ""})
	AuthorizeContext(c, user)

	handler := server.ExportUpdateHostVersion(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the version was cleared (set to empty)
	var updatedHost models.Host
	err := db.Where("name = ?", "version-test-host").First(&updatedHost).Error
	require.NoError(t, err)
	assert.Equal(t, "", updatedHost.WantsUpdateTo)
}
