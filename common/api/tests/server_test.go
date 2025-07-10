//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixDuplicateHosts_NoDuplicates(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	// Create hosts with unique names
	SetupTestHost(t, db, "alpha")
	SetupTestHost(t, db, "beta")

	models.HostsList = nil
	models.FixDuplicateHosts(db)

	// Ensure no renames
	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 2, len(hosts))
	assert.ElementsMatch(t, []string{"alpha", "beta"}, []string{hosts[0].Name, hosts[1].Name})

	// Ensure HostsList matches DB
	assert.Equal(t, 2, len(models.HostsList))
	hostNames := make(map[string]bool)
	for _, h := range models.HostsList {
		hostNames[h.Name] = true
	}
	assert.True(t, hostNames["alpha"])
	assert.True(t, hostNames["beta"])
}

func TestFixDuplicateHosts_TwoDuplicates(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	db.Exec("DROP INDEX IF EXISTS idx_hosts_name")

	SetupTestHost(t, db, "dupehost")
	SetupTestHost(t, db, "dupehost") // duplicate name

	models.HostsList = nil
	models.FixDuplicateHosts(db)

	// Fetch from DB and check names
	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 2, len(hosts))
	var foundBase, foundRenamed bool
	for _, h := range hosts {
		if h.Name == "dupehost" {
			foundBase = true
		} else if h.Name == "dupehost-1" {
			foundRenamed = true
		}
	}
	assert.True(t, foundBase)
	assert.True(t, foundRenamed)

	// Ensure HostsList matches DB
	assert.ElementsMatch(t, hosts, models.HostsList)
}

func TestFixDuplicateHosts_ThreeDuplicates(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	db.Exec("DROP INDEX IF EXISTS idx_hosts_name")

	SetupTestHost(t, db, "clash")
	SetupTestHost(t, db, "clash")
	SetupTestHost(t, db, "clash")

	models.HostsList = nil
	models.FixDuplicateHosts(db)

	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 3, len(hosts))
	counts := map[string]bool{"clash": false, "clash-1": false, "clash-2": false}
	for _, h := range hosts {
		counts[h.Name] = true
	}
	assert.True(t, counts["clash"])
	assert.True(t, counts["clash-1"])
	assert.True(t, counts["clash-2"])

	// Ensure names are unique
	nameSet := map[string]bool{}
	for _, h := range hosts {
		assert.False(t, nameSet[h.Name], "duplicate name found")
		nameSet[h.Name] = true
	}
	assert.Equal(t, 3, len(nameSet))
}

func TestFixDuplicateHosts_Mixed(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	db.Exec("DROP INDEX IF EXISTS idx_hosts_name")

	// M: 2 dups; N: 3 dups; O: unique
	SetupTestHost(t, db, "M")
	SetupTestHost(t, db, "M")
	SetupTestHost(t, db, "N")
	SetupTestHost(t, db, "N")
	SetupTestHost(t, db, "N")
	SetupTestHost(t, db, "O")

	models.HostsList = nil
	models.FixDuplicateHosts(db)

	var hosts []models.Host
	db.Find(&hosts)
	require.Equal(t, 6, len(hosts))
	expected := map[string]int{"M": 1, "M-1": 1, "N": 1, "N-1": 1, "N-2": 1, "O": 1}
	for _, h := range hosts {
		expected[h.Name]--
	}
	for k, v := range expected {
		assert.Equal(t, 0, v, "mismatch for host %s", k)
	}

	// Ensure HostsList matches DB and has only unique names
	assert.Equal(t, 6, len(models.HostsList))
	nameSet := map[string]bool{}
	for _, h := range models.HostsList {
		assert.False(t, nameSet[h.Name], "HostsList not unique")
		nameSet[h.Name] = true
	}
}
