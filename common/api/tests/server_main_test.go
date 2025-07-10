//go:build with_api

package tests

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServerMain_WithStubs(t *testing.T) {
	var configLoaded, dbSetup, routerBuilt, routerRan bool

	// Use in-memory sqlite DB
	memdb, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}

	deps := models.ServerDeps{
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
		models.ServerMainWithDeps(deps)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("serverMainWithDeps with stubs did not finish")
	}

	if !configLoaded || !dbSetup || !routerBuilt || !routerRan {
		t.Fatalf("Not all dependency steps were executed")
	}
}
