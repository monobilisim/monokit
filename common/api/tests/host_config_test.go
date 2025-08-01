//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/host"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetHostConfig_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test host configurations
	configs := []models.HostFileConfig{
		{
			HostName:  "test-host",
			FileName:  "config1.yml",
			Content:   "key1: value1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			HostName:  "test-host",
			FileName:  "config2.yml",
			Content:   "key2: value2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, config := range configs {
		require.NoError(t, db.Create(&config).Error)
	}

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/hosts/:name/config", host.HandleGetHostConfig(db))

	// Make request
	req, err := http.NewRequest("GET", "/hosts/test-host/config", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "key1: value1", response["config1.yml"])
	assert.Equal(t, "key2: value2", response["config2.yml"])
	assert.Len(t, response, 2)
}

func TestHandleGetHostConfig_EmptyHostName(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/hosts/:name/config", host.HandleGetHostConfig(db))

	// Make request with empty host name
	req, err := http.NewRequest("GET", "/hosts//config", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "host name is required", response["error"])
}

func TestHandleGetHostConfig_NoConfigs(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/hosts/:name/config", host.HandleGetHostConfig(db))

	// Make request for host with no configs
	req, err := http.NewRequest("GET", "/hosts/nonexistent-host/config", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response)
}

func TestHandlePostHostConfig_CreateNew(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/hosts/:name/config", host.HandlePostHostConfig(db))

	// Test data
	configMap := map[string]string{
		"app.yml":    "app_config: value1",
		"server.yml": "server_config: value2",
	}

	jsonData, err := json.Marshal(configMap)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/hosts/test-host/config", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "created", response["status"])
	assert.Equal(t, "configurations updated", response["message"])

	// Verify configs were created in database
	var configs []models.HostFileConfig
	err = db.Where("host_name = ?", "test-host").Find(&configs).Error
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Check specific configs
	configsByName := make(map[string]models.HostFileConfig)
	for _, config := range configs {
		configsByName[config.FileName] = config
	}

	assert.Equal(t, "app_config: value1", configsByName["app.yml"].Content)
	assert.Equal(t, "server_config: value2", configsByName["server.yml"].Content)
}

func TestHandlePostHostConfig_UpdateExisting(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create existing config
	existingConfig := models.HostFileConfig{
		HostName:  "test-host",
		FileName:  "app.yml",
		Content:   "old_config: old_value",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&existingConfig).Error)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/hosts/:name/config", host.HandlePostHostConfig(db))

	// Test data with updated content
	configMap := map[string]string{
		"app.yml": "new_config: new_value",
	}

	jsonData, err := json.Marshal(configMap)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/hosts/test-host/config", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify config was updated in database
	var updatedConfig models.HostFileConfig
	err = db.Where("host_name = ? AND file_name = ?", "test-host", "app.yml").First(&updatedConfig).Error
	require.NoError(t, err)
	assert.Equal(t, "new_config: new_value", updatedConfig.Content)
	assert.Equal(t, existingConfig.ID, updatedConfig.ID) // Should preserve ID
}

func TestHandlePostHostConfig_EmptyHostName(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/hosts/:name/config", host.HandlePostHostConfig(db))

	configMap := map[string]string{
		"app.yml": "config: value",
	}

	jsonData, err := json.Marshal(configMap)
	require.NoError(t, err)

	// Make request with empty host name
	req, err := http.NewRequest("POST", "/hosts//config", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "host name is required", response["error"])
}

func TestHandlePostHostConfig_InvalidJSON(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/hosts/:name/config", host.HandlePostHostConfig(db))

	// Invalid JSON
	invalidJSON := `{"app.yml": "config: value"`

	// Make request
	req, err := http.NewRequest("POST", "/hosts/test-host/config", bytes.NewBufferString(invalidJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlePutHostConfig(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/hosts/:name/config", host.HandlePutHostConfig(db))

	// Test data
	configMap := map[string]string{
		"app.yml": "put_config: put_value",
	}

	jsonData, err := json.Marshal(configMap)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("PUT", "/hosts/test-host/config", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions - PUT should behave the same as POST
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "created", response["status"])
	assert.Equal(t, "configurations updated", response["message"])
}

func TestHandleDeleteHostConfig_Success(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test config
	config := models.HostFileConfig{
		HostName:  "test-host",
		FileName:  "app.yml",
		Content:   "config: value",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&config).Error)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/hosts/:name/config/:filename", host.HandleDeleteHostConfig(db))

	// Make request
	req, err := http.NewRequest("DELETE", "/hosts/test-host/config/app.yml", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "deleted", response["status"])
	assert.Equal(t, "configuration file deleted: app.yml", response["message"])

	// Verify config was deleted from database
	var deletedConfig models.HostFileConfig
	err = db.Where("host_name = ? AND file_name = ?", "test-host", "app.yml").First(&deletedConfig).Error
	assert.Error(t, err) // Should not find the deleted config
}

func TestHandleDeleteHostConfig_NotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/hosts/:name/config/:filename", host.HandleDeleteHostConfig(db))

	// Make request for non-existent config
	req, err := http.NewRequest("DELETE", "/hosts/test-host/config/nonexistent.yml", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "configuration not found for file: nonexistent.yml", response["error"])
}

func TestHandleDeleteHostConfig_MissingParameters(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/hosts/:name/config/:filename", host.HandleDeleteHostConfig(db))

	// Test missing host name
	req, err := http.NewRequest("DELETE", "/hosts//config/app.yml", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "host name and filename are required", response["error"])

	// Test missing filename
	req2, err := http.NewRequest("DELETE", "/hosts/test-host/config/", nil)
	require.NoError(t, err)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusBadRequest, w2.Code)

	var response2 map[string]string
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)
	assert.Equal(t, "host name and filename are required", response2["error"])
}
