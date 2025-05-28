//go:build with_api

package tests

import (
	"net/http"
	"testing"

	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
)

// Setup AWX configuration for testing
func setupAwxConfig() {
	common.ServerConfig.Awx = common.AwxConfig{
		Enabled:            true,
		Url:                "https://awx.example.com",
		Username:           "admin",
		Password:           "password",
		Timeout:            30,
		DefaultInventoryID: 1,
		VerifySSL:          false,
	}
}

func TestCreateAwxHost(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	admin := SetupTestAdmin(t, db)
	SetupTestHost(t, db, "awxhost")

	// Test: AWX host creation (will fail due to unreachable AWX server)
	requestData := map[string]interface{}{
		"name":       "awxhost",
		"ip_address": "192.168.1.100",
		"extra_vars": map[string]interface{}{
			"test_var": "test_value",
		},
		"awx_only": false,
	}

	// Set the default inventory ID in config
	common.ServerConfig.Awx.DefaultInventoryID = 12

	c, w := CreateRequestContext("POST", "/api/v1/hosts/awx", requestData)
	AuthorizeContext(c, admin)

	handler := common.ExportCreateAwxHost(db)
	handler(c)

	// Expect error since AWX server is not reachable
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Failed to execute request")

	// Test: AWX disabled
	common.ServerConfig.Awx.Enabled = false
	c, w = CreateRequestContext("POST", "/api/v1/hosts/awx", requestData)
	AuthorizeContext(c, admin)

	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "AWX integration is not enabled", response["error"])

	// Test: Missing required fields
	common.ServerConfig.Awx.Enabled = true
	invalidData := map[string]interface{}{
		"name": "test", // Missing ip_address
	}
	c, w = CreateRequestContext("POST", "/api/v1/hosts/awx", invalidData)
	AuthorizeContext(c, admin)

	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Invalid request")
}

func TestDeleteAwxHost(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	admin := SetupTestAdmin(t, db)
	host := SetupTestHost(t, db, "awxhosttodelete")
	host.AwxHostId = "456"
	db.Save(&host)

	// Test: AWX host deletion (will fail due to unreachable AWX server)
	c, w := CreateRequestContext("DELETE", "/api/v1/hosts/awxhosttodelete/awx", nil)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{"name": "awxhosttodelete"})

	handler := common.ExportDeleteAwxHost(db)
	handler(c)

	// Expect error since AWX server is not reachable
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Failed to execute request")

	// Test: Host not in AWX
	SetupTestHost(t, db, "hostnotinawx")
	c, w = CreateRequestContext("DELETE", "/api/v1/hosts/hostnotinawx/awx", nil)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{"name": "hostnotinawx"})

	handler(c)
	// The endpoint still tries to delete even if host not in AWX
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Test: AWX disabled
	common.ServerConfig.Awx.Enabled = false
	c, w = CreateRequestContext("DELETE", "/api/v1/hosts/awxhosttodelete/awx", nil)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{"name": "awxhosttodelete"})

	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAwxTemplatesGlobal(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	user := SetupTestUser(t, db, "testuser")

	// Test: Get all AWX templates (will fail due to unreachable AWX server)
	c, w := CreateRequestContext("GET", "/api/v1/awx/templates", nil)
	AuthorizeContext(c, user)

	handler := common.ExportGetAwxTemplatesGlobal(db)
	handler(c)

	// Expect error since AWX server is not reachable
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Failed to execute request")

	// Test: AWX disabled
	common.ServerConfig.Awx.Enabled = false
	c, w = CreateRequestContext("GET", "/api/v1/awx/templates", nil)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "AWX integration is not enabled", response["error"])
}

func TestExecuteAwxJob(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	admin := SetupTestAdmin(t, db)
	host := SetupTestHost(t, db, "testhost")
	host.AwxHostId = "789"
	db.Save(&host)

	// Test: Execute AWX job (will fail due to unreachable AWX server)
	requestData := map[string]interface{}{
		"extra_vars": map[string]string{
			"test_var": "test_value",
		},
	}

	c, w := CreateRequestContext("POST", "/api/v1/hosts/testhost/awx/jobs/10", requestData)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{
		"name":       "testhost",
		"templateID": "10",
	})

	handler := common.ExportExecuteAwxJob(db)
	handler(c)

	// Expect error since AWX server is not reachable - but it fails earlier due to missing template ID
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "No default template ID configured")

	// Test: Host not found
	c, w = CreateRequestContext("POST", "/api/v1/hosts/nonexistent/awx/jobs/10", requestData)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{
		"name":       "nonexistent",
		"templateID": "10",
	})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test: Host not in AWX
	SetupTestHost(t, db, "hostnotinawx")
	c, w = CreateRequestContext("POST", "/api/v1/hosts/hostnotinawx/awx/jobs/10", requestData)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{
		"name":       "hostnotinawx",
		"templateID": "10",
	})

	handler(c)
	// Same error as above - missing template ID
	assert.Equal(t, http.StatusBadRequest, w.Code)
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "No default template ID configured")
}

func TestGetAwxJobStatus(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	user := SetupTestUser(t, db, "testuser")

	// Test: Get job status (will fail due to unreachable AWX server)
	c, w := CreateRequestContext("GET", "/api/v1/awx/jobs/200/status", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"jobID": "200"})

	handler := common.ExportGetAwxJobStatus(db)
	handler(c)

	// Expect error since AWX server is not reachable
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]interface{}
	ExtractJSONResponse(t, w, &response)
	assert.Contains(t, response["error"], "Failed to execute request")

	// Test: Invalid job ID - endpoint doesn't validate, still tries to call AWX
	c, w = CreateRequestContext("GET", "/api/v1/awx/jobs/invalid/status", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"jobID": "invalid"})

	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEnsureHostInAwx(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	setupAwxConfig()

	host := SetupTestHost(t, db, "testhost")

	// Test case 1: Host already has AWX ID - but function still checks AWX
	host.AwxHostId = "existing-123"
	awxId, err := common.ExportEnsureHostInAwx(db, host)
	// Will fail due to unreachable AWX
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute search request")

	// Test case 2: AWX disabled
	common.ServerConfig.Awx.Enabled = false
	host.AwxHostId = ""
	awxId, err = common.ExportEnsureHostInAwx(db, host)
	assert.EqualError(t, err, "AWX integration is not enabled")
	assert.Empty(t, awxId)

	// Test case 3: Host needs to be created in AWX (will fail due to unreachable server)
	common.ServerConfig.Awx.Enabled = true
	host.AwxHostId = ""
	awxId, err = common.ExportEnsureHostInAwx(db, host)
	assert.Error(t, err)
	assert.Empty(t, awxId)
}
