//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/domains"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRedmineTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&models.Domain{}, &models.User{}, &models.DomainUser{})
	require.NoError(t, err)

	return db
}

func setupRedmineTestData(t *testing.T, db *gorm.DB) (models.User, models.Domain) {
	// Create test user
	user := models.User{
		Username: "testuser",
		Role:     "user",
	}
	require.NoError(t, db.Create(&user).Error)

	// Create test domain
	domain := models.Domain{
		Name:             "test-domain",
		RedmineProjectID: "test-project",
	}
	require.NoError(t, db.Create(&domain).Error)

	// Create domain user association
	domainUser := models.DomainUser{
		UserID:   user.ID,
		DomainID: domain.ID,
		Role:     "domain_user",
	}
	require.NoError(t, db.Create(&domainUser).Error)

	return user, domain
}

// Simple test to verify the handler exists and can be called
func TestGetDomainRedmineProject_HandlerExists(t *testing.T) {
	db := setupRedmineTestDB(t)

	// Test that the handler function exists and can be created
	handler := domains.GetDomainRedmineProject(db)
	assert.NotNil(t, handler)
}

// Additional tests can be added here as needed

// End of file
