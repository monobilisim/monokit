//go:build with_api

package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/monobilisim/monokit/common/api/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServerMain_OpenDBFailure(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	deps := server.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return nil, errors.New("failed to connect to database")
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB, ctx context.Context) *gin.Engine {
			routerBuilt = true
			return gin.New()
		},
		RunRouter: func(r *gin.Engine) error {
			routerRan = true
			return nil
		},
	}

	// Test that the function panics with the expected message
	assert.PanicsWithValue(t, "failed to connect database", func() {
		server.ServerMainWithDeps(deps)
	})

	// Verify that only LoadConfig was called before the panic
	assert.True(t, configLoaded, "LoadConfig should have been called")
	assert.False(t, dbSetup, "SetupDB should not have been called")
	assert.False(t, routerBuilt, "BuildRouter should not have been called")
	assert.False(t, routerRan, "RunRouter should not have been called")
}

func TestServerMain_RunRouterFailure(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	// Use in-memory sqlite DB
	memdb, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}

	deps := server.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return memdb, nil
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB, ctx context.Context) *gin.Engine {
			routerBuilt = true
			return gin.New()
		},
		RunRouter: func(r *gin.Engine) error {
			routerRan = true
			return errors.New("failed to start server")
		},
	}

	// Test that the function panics with the expected message
	assert.PanicsWithValue(t, "failed to run router", func() {
		server.ServerMainWithDeps(deps)
	})

	// Verify that all steps were called except RunRouter completed successfully
	assert.True(t, configLoaded, "LoadConfig should have been called")
	assert.True(t, dbSetup, "SetupDB should have been called")
	assert.True(t, routerBuilt, "BuildRouter should have been called")
	assert.True(t, routerRan, "RunRouter should have been called (but failed)")
}

// Test FixDuplicateHosts with different domain scenarios
func TestFixDuplicateHosts_DifferentDomains(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create two domains
	domain1 := SetupTestDomain(t, db, "domain1")
	domain2 := SetupTestDomain(t, db, "domain2")

	// Temporarily drop the unique constraint to allow creating duplicates for testing
	db.Exec("DROP INDEX IF EXISTS idx_host_name_domain")

	// Create hosts with same name in different domains (should be allowed)
	host1 := models.Host{
		Name:     "samename",
		DomainID: domain1.ID,
		Status:   "Online",
	}
	host2 := models.Host{
		Name:     "samename",
		DomainID: domain2.ID,
		Status:   "Online",
	}
	require.NoError(t, db.Create(&host1).Error)
	require.NoError(t, db.Create(&host2).Error)

	// Create duplicates within domain1
	host3 := models.Host{
		Name:     "samename",
		DomainID: domain1.ID,
		Status:   "Online",
	}
	require.NoError(t, db.Create(&host3).Error)

	models.HostsList = nil
	server.FixDuplicateHosts(db)

	// Verify results
	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 3, len(hosts))

	// Count hosts by domain
	domain1Hosts := 0
	domain2Hosts := 0
	for _, h := range hosts {
		if h.DomainID == domain1.ID {
			domain1Hosts++
		} else if h.DomainID == domain2.ID {
			domain2Hosts++
		}
	}

	assert.Equal(t, 2, domain1Hosts, "Domain1 should have 2 hosts (original + renamed)")
	assert.Equal(t, 1, domain2Hosts, "Domain2 should have 1 host (unchanged)")

	// Verify that domain2 host name is unchanged
	var domain2Host models.Host
	db.Where("domain_id = ?", domain2.ID).First(&domain2Host)
	assert.Equal(t, "samename", domain2Host.Name)

	// Verify that domain1 has one "samename" and one "samename-1"
	var domain1HostsList []models.Host
	db.Where("domain_id = ?", domain1.ID).Find(&domain1HostsList)
	names := make([]string, len(domain1HostsList))
	for i, h := range domain1HostsList {
		names[i] = h.Name
	}
	assert.ElementsMatch(t, []string{"samename", "samename-1"}, names)
}

// Test FixDuplicateHosts with empty database
func TestFixDuplicateHosts_EmptyDatabase(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	models.HostsList = nil
	server.FixDuplicateHosts(db)

	// Should not panic and should result in empty HostsList
	assert.Empty(t, models.HostsList)
}

// Test FixDuplicateHosts with many duplicates
func TestFixDuplicateHosts_ManyDuplicates(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Temporarily drop the unique constraint to allow creating duplicates for testing
	db.Exec("DROP INDEX IF EXISTS idx_host_name_domain")

	// Create 5 hosts with the same name
	for i := 0; i < 5; i++ {
		SetupTestHost(t, db, "manydupes")
	}

	models.HostsList = nil
	server.FixDuplicateHosts(db)

	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 5, len(hosts))

	// Verify all names are unique
	nameSet := make(map[string]bool)
	for _, h := range hosts {
		assert.False(t, nameSet[h.Name], "Duplicate name found: %s", h.Name)
		nameSet[h.Name] = true
	}

	// Verify expected names exist
	expectedNames := []string{"manydupes", "manydupes-1", "manydupes-2", "manydupes-3", "manydupes-4"}
	actualNames := make([]string, len(hosts))
	for i, h := range hosts {
		actualNames[i] = h.Name
	}
	assert.ElementsMatch(t, expectedNames, actualNames)
}

// Test FixDuplicateHosts with special characters in names
func TestFixDuplicateHosts_SpecialCharacters(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Temporarily drop the unique constraint to allow creating duplicates for testing
	db.Exec("DROP INDEX IF EXISTS idx_host_name_domain")

	// Create hosts with special characters
	specialName := "host-with.special_chars@domain"
	SetupTestHost(t, db, specialName)
	SetupTestHost(t, db, specialName)

	models.HostsList = nil
	server.FixDuplicateHosts(db)

	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 2, len(hosts))

	// Verify names
	names := make([]string, len(hosts))
	for i, h := range hosts {
		names[i] = h.Name
	}
	expectedNames := []string{specialName, specialName + "-1"}
	assert.ElementsMatch(t, expectedNames, names)
}

func TestServerMain_SuccessfulFlow(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	// Use in-memory sqlite DB
	memdb, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}

	deps := server.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return memdb, nil
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB, ctx context.Context) *gin.Engine {
			routerBuilt = true
			return gin.New()
		},
		RunRouter: func(r *gin.Engine) error {
			routerRan = true
			return nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Unexpected panic: %v", r)
			}
			close(done)
		}()
		server.ServerMainWithDeps(deps)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("serverMainWithDeps did not finish within timeout")
	}

	// Verify that all steps were executed
	assert.True(t, configLoaded, "LoadConfig should have been called")
	assert.True(t, dbSetup, "SetupDB should have been called")
	assert.True(t, routerBuilt, "BuildRouter should have been called")
	assert.True(t, routerRan, "RunRouter should have been called")
}
