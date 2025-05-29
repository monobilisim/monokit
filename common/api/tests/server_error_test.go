//go:build with_api

package tests

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServerMain_OpenDBFailure(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	deps := common.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return nil, errors.New("failed to connect to database")
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB) *gin.Engine {
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
		common.ServerMainWithDeps(deps)
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

	deps := common.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return memdb, nil
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB) *gin.Engine {
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
		common.ServerMainWithDeps(deps)
	})

	// Verify that all steps were called except RunRouter completed successfully
	assert.True(t, configLoaded, "LoadConfig should have been called")
	assert.True(t, dbSetup, "SetupDB should have been called")
	assert.True(t, routerBuilt, "BuildRouter should have been called")
	assert.True(t, routerRan, "RunRouter should have been called (but failed)")
}

func TestServerMain_SuccessfulFlow(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	// Use in-memory sqlite DB
	memdb, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}

	deps := common.ServerDeps{
		LoadConfig: func() {
			configLoaded = true
		},
		OpenDB: func() (*gorm.DB, error) {
			return memdb, nil
		},
		SetupDB: func(db *gorm.DB) {
			dbSetup = true
		},
		BuildRouter: func(db *gorm.DB) *gin.Engine {
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
		common.ServerMainWithDeps(deps)
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
