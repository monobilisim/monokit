//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainCreation(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a global admin user
	admin := SetupTestAdmin(t, db)

	// Test: Create a new domain
	domainRequest := models.CreateDomainRequest{
		Name:        "test-domain",
		Description: "Test domain for unit tests",
		Settings:    `{"theme": "dark"}`,
	}

	c, _ := CreateRequestContext("POST", "/api/v1/domains", domainRequest)
	AuthorizeContextWithDomain(c, admin, db)

	// Mock domain creation handler (would need to be implemented)
	// handler := domains.CreateDomain(db)
	// handler(c)

	// For now, just test the domain model creation directly
	domain := models.Domain{
		Name:        domainRequest.Name,
		Description: domainRequest.Description,
		Settings:    domainRequest.Settings,
		Active:      true,
	}
	result := db.Create(&domain)
	require.NoError(t, result.Error)

	// Verify domain was created
	var dbDomain models.Domain
	result = db.Where("name = ?", "test-domain").First(&dbDomain)
	require.NoError(t, result.Error)
	assert.Equal(t, "test-domain", dbDomain.Name)
	assert.Equal(t, "Test domain for unit tests", dbDomain.Description)
	assert.True(t, dbDomain.Active)
}

func TestDomainUserAssignment(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create test domain and user
	domain := SetupTestDomain(t, db, "test-domain")
	user := SetupTestUser(t, db, "testuser")

	// Test: Assign user to domain as domain admin
	domainUser := SetupTestDomainUser(t, db, user, domain, "domain_admin")

	// Verify assignment
	assert.Equal(t, user.ID, domainUser.UserID)
	assert.Equal(t, domain.ID, domainUser.DomainID)
	assert.Equal(t, "domain_admin", domainUser.Role)

	// Test: Verify user can access domain
	var userDomains []models.DomainUser
	result := db.Preload("Domain").Where("user_id = ?", user.ID).Find(&userDomains)
	require.NoError(t, result.Error)
	require.Len(t, userDomains, 1)
	assert.Equal(t, "test-domain", userDomains[0].Domain.Name)
}

func TestDomainScopedResources(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create two domains
	domain1 := SetupTestDomain(t, db, "domain1")
	domain2 := SetupTestDomain(t, db, "domain2")

	// Create hosts in different domains
	host1 := SetupTestHostInDomain(t, db, "host1", domain1)
	host2 := SetupTestHostInDomain(t, db, "host2", domain2)

	// Test: Verify hosts are domain-scoped
	assert.Equal(t, domain1.ID, host1.DomainID)
	assert.Equal(t, domain2.ID, host2.DomainID)

	// Test: Query hosts by domain
	var domain1Hosts []models.Host
	result := db.Where("domain_id = ?", domain1.ID).Find(&domain1Hosts)
	require.NoError(t, result.Error)
	require.Len(t, domain1Hosts, 1)
	assert.Equal(t, "host1", domain1Hosts[0].Name)

	var domain2Hosts []models.Host
	result = db.Where("domain_id = ?", domain2.ID).Find(&domain2Hosts)
	require.NoError(t, result.Error)
	require.Len(t, domain2Hosts, 1)
	assert.Equal(t, "host2", domain2Hosts[0].Name)
}

func TestDomainUniqueConstraints(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create a domain
	domain := SetupTestDomain(t, db, "test-domain")

	// Test: Create host with unique name within domain
	host1 := SetupTestHostInDomain(t, db, "unique-host", domain)
	assert.Equal(t, "unique-host", host1.Name)

	// Test: Try to create another host with same name in same domain (should fail)
	host2 := models.Host{
		Name:     "unique-host",
		DomainID: domain.ID,
		CpuCores: 2,
		Ram:      "4GB",
	}
	result := db.Create(&host2)
	assert.Error(t, result.Error) // Should fail due to unique constraint

	// Test: Create host with same name in different domain (should succeed)
	domain2 := SetupTestDomain(t, db, "domain2")
	host3 := SetupTestHostInDomain(t, db, "unique-host", domain2)
	assert.Equal(t, "unique-host", host3.Name)
	assert.Equal(t, domain2.ID, host3.DomainID)
}

func TestGlobalAdminAccess(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create global admin and regular user
	admin := SetupTestAdmin(t, db)
	user := SetupTestUser(t, db, "regularuser")

	// Create domains
	domain1 := SetupTestDomain(t, db, "domain1")
	domain2 := SetupTestDomain(t, db, "domain2")

	// Assign regular user to only domain1
	SetupTestDomainUser(t, db, user, domain1, "domain_user")

	// Test: Global admin should have access to all domains
	assert.Equal(t, "global_admin", admin.Role)

	// Test: Regular user should only have access to assigned domains
	assert.Equal(t, "", user.Role) // Empty role means domain-scoped

	var userDomains []models.DomainUser
	result := db.Where("user_id = ?", user.ID).Find(&userDomains)
	require.NoError(t, result.Error)
	require.Len(t, userDomains, 1)
	assert.Equal(t, domain1.ID, userDomains[0].DomainID)

	// Test: User should not have access to domain2
	assert.NotEqual(t, domain2.ID, userDomains[0].DomainID)
}

func TestDomainInventoryScoping(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create domains
	domain1 := SetupTestDomain(t, db, "domain1")
	domain2 := SetupTestDomain(t, db, "domain2")

	// Create inventories in different domains
	inventory1 := SetupTestInventoryInDomain(t, db, "inventory1", domain1)
	inventory2 := SetupTestInventoryInDomain(t, db, "inventory2", domain2)

	// Test: Verify inventories are domain-scoped
	assert.Equal(t, domain1.ID, inventory1.DomainID)
	assert.Equal(t, domain2.ID, inventory2.DomainID)

	// Test: Same inventory name can exist in different domains
	inventory3 := SetupTestInventoryInDomain(t, db, "inventory1", domain2)
	assert.Equal(t, "inventory1", inventory3.Name)
	assert.Equal(t, domain2.ID, inventory3.DomainID)

	// Test: Cannot create duplicate inventory name in same domain
	duplicateInventory := models.Inventory{
		Name:     "inventory1",
		DomainID: domain1.ID,
	}
	result := db.Create(&duplicateInventory)
	assert.Error(t, result.Error) // Should fail due to unique constraint
}
