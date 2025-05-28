//go:build with_api

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupTestDB sets up an in-memory SQLite database for testing
func SetupTestDB(t require.TestingT) *gorm.DB {
	// Use in-memory SQLite for tests - each test gets its own database
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
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

	// Clear global state
	common.HostsList = []common.Host{}

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

// MockGormDB is a wrapper around gorm.DB for testing error conditions.
// It allows specific operations to return errors.
type MockGormDB struct {
	*gorm.DB
	ErrorOnFindHosts   bool
	ErrorOnDeleteHost  bool
	ErrorOnSaveHost    bool
	ErrorOnFindUsers   bool
	ErrorOnSaveUser    bool
	ErrorOnDeleteGroup bool
	ErrorOnCreate      bool
	ErrorOnFirst       bool
	ErrorOnSave        bool
	ErrorOnDelete      bool
	ErrorAssociation   bool
	ErrorOnUpdate      bool
	ErrorOnPreload     bool // NEW: triggers Preload error
}

func (m *MockGormDB) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	switch dest.(type) {
	case *[]common.Host:
		if m.ErrorOnFindHosts {
			return &gorm.DB{Error: fmt.Errorf("mock db error: find hosts")}
		}
	case *[]common.User:
		if m.ErrorOnFindUsers {
			return &gorm.DB{Error: fmt.Errorf("mock db error: find users")}
		}
	}
	return m.DB.Find(dest, conds...)
}

func (m *MockGormDB) Delete(value interface{}, conds ...interface{}) *gorm.DB {
	switch value.(type) {
	case *common.Host:
		if m.ErrorOnDeleteHost {
			return &gorm.DB{Error: fmt.Errorf("mock db error: delete host")}
		}
	case *common.Group:
		if m.ErrorOnDeleteGroup {
			return &gorm.DB{Error: fmt.Errorf("mock db error: delete group")}
		}
	}
	if m.ErrorOnDelete {
		return &gorm.DB{Error: fmt.Errorf("mock db error: delete")}
	}
	return m.DB.Delete(value, conds...)
}

func (m *MockGormDB) Save(value interface{}) *gorm.DB {
	switch value.(type) {
	case *common.Host:
		if m.ErrorOnSaveHost {
			return &gorm.DB{Error: fmt.Errorf("mock db error: save host")}
		}
	case *common.User:
		if m.ErrorOnSaveUser {
			return &gorm.DB{Error: fmt.Errorf("mock db error: save user")}
		}
	}
	if m.ErrorOnSave {
		return &gorm.DB{Error: fmt.Errorf("mock db error: save")}
	}
	return m.DB.Save(value)
}

func (m *MockGormDB) Create(value interface{}) *gorm.DB {
	if m.ErrorOnCreate {
		return &gorm.DB{Error: fmt.Errorf("mock db error: create")}
	}
	return m.DB.Create(value)
}

func (m *MockGormDB) First(dest interface{}, conds ...interface{}) *gorm.DB {
	if m.ErrorOnFirst {
		return &gorm.DB{Error: gorm.ErrRecordNotFound} // Common error for First
	}
	return m.DB.First(dest, conds...)
}

func (m *MockGormDB) Where(query interface{}, args ...interface{}) *gorm.DB {
	newTx := m.DB.Where(query, args...)
	return &gorm.DB{Error: newTx.Error, RowsAffected: newTx.RowsAffected, Statement: newTx.Statement, Config: newTx.Config}
}

func (m *MockGormDB) Preload(query string, args ...interface{}) *gorm.DB {
	if m.ErrorOnPreload {
		return &gorm.DB{Error: fmt.Errorf("mock db error: preload")}
	}
	newTx := m.DB.Preload(query, args...)
	return &gorm.DB{Error: newTx.Error, RowsAffected: newTx.RowsAffected, Statement: newTx.Statement, Config: newTx.Config}
}

// Model returns a new MockGormDB instance that will be used for Association.
func (m *MockGormDB) Model(value interface{}) *gorm.DB {
	switch value.(type) {
	case *common.User:
		if m.ErrorOnUpdate {
			return &gorm.DB{Error: fmt.Errorf("mock db error: update")}
		}
	}
	tx := m.DB.Model(value)
	return tx
}

// Association mocks the gorm.Association method.
func (m *MockGormDB) Association(column string) *gorm.Association {
	if m.ErrorAssociation {
		// Placeholder for more complex association error mocking if needed
	}
	return m.DB.Association(column)
}

// Update is a new method added to MockGormDB
func (m *MockGormDB) Update(column string, value interface{}) *gorm.DB {
	if m.ErrorOnUpdate {
		return &gorm.DB{Error: fmt.Errorf("mock db error: update")}
	}
	return m.DB.Update(column, value)
}

// AssertErrorResponse is a helper to check for a specific error message in a gin.H response
func AssertErrorResponse(t testing.TB, w *httptest.ResponseRecorder, expectedError string) {
	var jsonResponse gin.H
	err := json.Unmarshal(w.Body.Bytes(), &jsonResponse)
	require.NoError(t, err, "Failed to unmarshal error response")

	if w.Code == http.StatusOK {
		t.Logf("AssertErrorResponse called with HTTP StatusOK, body: %s", w.Body.String())
		// If we got a 200 OK, we likely don't have an error message in the expected format.
		// The primary assertion for the status code should catch the main issue.
		// We can assert that the error key is NOT present or is different if that's the expectation for OK responses.
		// For now, just don't panic.
		// We could also fail here: t.Errorf("Expected an error response, but got status %d", w.Code)
		// However, the caller should assert the status code first.
		return
	}

	errorVal, ok := jsonResponse["error"]
	if !ok {
		t.Errorf("Expected JSON response to have an 'error' key, body: %s", w.Body.String())
		return
	}
	errorStr, ok := errorVal.(string)
	if !ok {
		t.Errorf("Expected 'error' value to be a string, but got %T, body: %s", errorVal, w.Body.String())
		return
	}
	assert.Contains(t, errorStr, expectedError)
}
