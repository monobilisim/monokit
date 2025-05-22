//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupTestDB sets up an in-memory SQLite database for testing
func SetupTestDB(t require.TestingT) *gorm.DB {
	// Use in-memory SQLite for tests
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to in-memory database")

	// Migrate the schema in the correct order to avoid foreign key issues
	err = db.AutoMigrate(
		&common.APILogEntry{},
		&common.Inventory{},
		&common.Host{},
		&common.User{},
		&common.HostKey{},
		&common.Session{},
		&common.Group{},
		&common.HostLog{},
		&common.HostFileConfig{},
	)
	require.NoError(t, err, "Failed to migrate test database schema")

	// Create default inventory required by many tests
	db.Create(&common.Inventory{Name: "default"})

	return db
}

// CleanupTestDB cleans up the test database
func CleanupTestDB(db *gorm.DB) {
	// Get the underlying *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}

// SetupTestAdmin creates an admin user for testing
func SetupTestAdmin(t require.TestingT, db *gorm.DB) common.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("adminpass"), 14)
	require.NoError(t, err)

	admin := common.User{
		Username:    "admin",
		Password:    string(hashedPassword),
		Email:       "admin@example.com",
		Role:        "admin",
		Groups:      "nil",
		Inventories: "default",
		AuthMethod:  "local",
	}
	result := db.Create(&admin)
	require.NoError(t, result.Error)
	return admin
}

// SetupTestUser creates a regular user for testing
func SetupTestUser(t require.TestingT, db *gorm.DB, username string) common.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("userpass"), 14)
	require.NoError(t, err)

	user := common.User{
		Username:    username,
		Password:    string(hashedPassword),
		Email:       fmt.Sprintf("%s@example.com", username),
		Role:        "user",
		Groups:      "nil",
		Inventories: "default",
		AuthMethod:  "local",
	}
	result := db.Create(&user)
	require.NoError(t, result.Error)
	return user
}

// SetupTestHost creates a host for testing
func SetupTestHost(t require.TestingT, db *gorm.DB, hostname string) common.Host {
	host := common.Host{
		Name:                hostname,
		CpuCores:            4,
		Ram:                 "8GB",
		MonokitVersion:      "1.0.0",
		Os:                  "Test OS",
		DisabledComponents:  "nil",
		InstalledComponents: "test-component",
		IpAddress:           "127.0.0.1",
		Status:              "online",
		Groups:              "nil",
		Inventory:           "default",
		UpForDeletion:       false,
	}
	result := db.Create(&host)
	require.NoError(t, result.Error)
	return host
}

// SetupTestGroup creates a group for testing
func SetupTestGroup(t require.TestingT, db *gorm.DB, name string) common.Group {
	group := common.Group{
		Name: name,
	}
	result := db.Create(&group)
	require.NoError(t, result.Error)
	return group
}

// CreateTestContext creates a gin context for testing
func CreateTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// AuthorizeContext sets up context with authenticated user
func AuthorizeContext(c *gin.Context, user common.User) {
	c.Set("user", user)
}

// CreateRequestContext creates a gin context with a request for testing
func CreateRequestContext(method, url string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}

	c.Request = req
	return c, w
}

// SetupSession creates a session for testing
func SetupSession(t require.TestingT, db *gorm.DB, user common.User) common.Session {
	session := common.Session{
		Token:     "test-session-token",
		UserID:    user.ID,
		User:      user,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Timeout:   time.Now().Add(24 * time.Hour),
	}
	result := db.Create(&session)
	require.NoError(t, result.Error)
	return session
}

// ExtractJSONResponse extracts a response from the recorder and unmarshals it
func ExtractJSONResponse(t require.TestingT, w *httptest.ResponseRecorder, out interface{}) {
	err := json.Unmarshal(w.Body.Bytes(), out)
	require.NoError(t, err)
}

// SetPathParams sets path parameters for a gin context
func SetPathParams(c *gin.Context, params map[string]string) {
	// Create the param key-value pairs
	pairs := []gin.Param{}
	for k, v := range params {
		pairs = append(pairs, gin.Param{Key: k, Value: v})
	}

	// Set the params
	c.Params = pairs
}

// SetQueryParams sets query parameters for a gin context
func SetQueryParams(c *gin.Context, params map[string]string) {
	// Create query string
	queryParts := []string{}
	for k, v := range params {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
	}
	queryString := strings.Join(queryParts, "&")

	// Set up request with query string
	c.Request.URL.RawQuery = queryString
}
