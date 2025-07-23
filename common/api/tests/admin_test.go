//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/monobilisim/monokit/common/api/admin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestListGroups(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestGroup(t, db, "testgroup")
	SetupTestGroup(t, db, "anothergroup")

	// Test: Successful request as admin
	c, w := CreateRequestContext("GET", "/api/v1/admin/groups", nil)
	AuthorizeContext(c, adminUser)

	// Create the handler and call it directly
	handler := admin.ExportListGroups(db)
	handler(c)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	var groups []models.Group
	ExtractJSONResponse(t, w, &groups)
	assert.Len(t, groups, 2)
	assert.Contains(t, []string{"testgroup", "anothergroup"}, groups[0].Name)
	assert.Contains(t, []string{"testgroup", "anothergroup"}, groups[1].Name)

	// Test: Unauthorized request
	user := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("GET", "/api/v1/admin/groups", nil)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateGroup(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)

	// Test: Successful group creation
	groupReq := map[string]string{"name": "newgroup"}
	c, w := CreateRequestContext("POST", "/api/v1/admin/groups", groupReq)
	AuthorizeContext(c, adminUser)

	handler := admin.ExportCreateGroup(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var count int64
	db.Model(&models.Group{}).Where("name = ?", "newgroup").Count(&count)
	assert.Equal(t, int64(1), count)

	// Test: Create existing group
	c, w = CreateRequestContext("POST", "/api/v1/admin/groups", groupReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)

	// Test: Unauthorized request
	user := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("POST", "/api/v1/admin/groups", groupReq)
	AuthorizeContext(c, user)

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteGroup(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestGroup(t, db, "testgroup")

	// Add a host to the group for testing removal
	host := SetupTestHost(t, db, "testhost")
	host.Groups = "testgroup,othergroup"
	db.Save(&host)

	// Test: Successful delete
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/testgroup", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "testgroup"})

	handler := admin.ExportDeleteGroup(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify group is gone
	var count int64
	db.Model(&models.Group{}).Where("name = ?", "testgroup").Count(&count)
	assert.Equal(t, int64(0), count)

	// Verify host's group reference was updated
	db.First(&host, host.ID)
	assert.Equal(t, "othergroup", host.Groups)

	// Test: Delete with withHosts=true
	SetupTestGroup(t, db, "deletegroup")
	host2 := SetupTestHost(t, db, "hostfordeletion")
	host2.Groups = "deletegroup"
	db.Save(&host2)

	c, w = CreateRequestContext("DELETE", "/api/v1/admin/groups/deletegroup?withHosts=true", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "deletegroup"})
	SetQueryParams(c, map[string]string{"withHosts": "true"})

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was deleted
	var host2Count int64
	db.Model(&models.Host{}).Where("name = ?", "hostfordeletion").Count(&host2Count)
	assert.Equal(t, int64(0), host2Count)

	// Test: Unauthorized request
	user := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("DELETE", "/api/v1/admin/groups/testgroup", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"name": "testgroup"})

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddHostToGroup(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestGroup(t, db, "testgroup")
	SetupTestHost(t, db, "testhost")

	// Test: Successful add
	c, w := CreateRequestContext("POST", "/api/v1/admin/groups/testgroup/hosts/testhost", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"name":     "testgroup",
		"hostname": "testhost",
	})

	handler := admin.ExportAddHostToGroup(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was added to group
	var updatedHost models.Host
	db.Where("name = ?", "testhost").First(&updatedHost)
	assert.Contains(t, updatedHost.Groups, "testgroup")

	// Test: Group not found
	c, w = CreateRequestContext("POST", "/api/v1/admin/groups/nonexistent/hosts/testhost", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"name":     "nonexistent",
		"hostname": "testhost",
	})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test: Host not found
	c, w = CreateRequestContext("POST", "/api/v1/admin/groups/testgroup/hosts/nonexistent", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"name":     "testgroup",
		"hostname": "nonexistent",
	})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveHostFromGroup(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestGroup(t, db, "testgroup")
	host := SetupTestHost(t, db, "testhost")
	host.Groups = "testgroup,othergroup"
	db.Save(&host)

	// Test: Successful remove
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/testgroup/hosts/testhost", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"name":     "testgroup",
		"hostname": "testhost",
	})

	handler := admin.ExportRemoveHostFromGroup(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was removed from group
	var updatedHost models.Host
	db.Where("name = ?", "testhost").First(&updatedHost)
	assert.NotContains(t, updatedHost.Groups, "testgroup")
	assert.Contains(t, updatedHost.Groups, "othergroup")

	// Test: Removing the last group should set to "nil"
	// Create the lastgroup group first
	SetupTestGroup(t, db, "lastgroup")
	host.Groups = "lastgroup"
	db.Save(&host)

	c, w = CreateRequestContext("DELETE", "/api/v1/admin/groups/lastgroup/hosts/testhost", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"name":     "lastgroup",
		"hostname": "testhost",
	})

	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	db.Where("name = ?", "testhost").First(&updatedHost)
	assert.Equal(t, "nil", updatedHost.Groups)
}

func TestUpdateUserGroups(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "testuser")

	// Test: Successful update
	updateReq := models.UpdateUserGroupsRequest{
		Groups: "group1,group2",
	}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/testuser/groups", updateReq)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "testuser"})

	handler := admin.ExportUpdateUserGroups(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify groups were updated
	var updatedUser models.User
	db.Where("username = ?", "testuser").First(&updatedUser)
	assert.Equal(t, "group1,group2", updatedUser.Groups)

	// Test: User not found
	c, w = CreateRequestContext("PUT", "/api/v1/admin/users/nonexistent/groups", updateReq)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "nonexistent"})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)

	// Test: Successful user creation
	userReq := models.RegisterRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
		Role:     "user",
		Groups:   "group1,group2",
	}
	c, w := CreateRequestContext("POST", "/api/v1/admin/users", userReq)
	AuthorizeContext(c, adminUser)

	handler := admin.ExportCreateUser(db)
	handler(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify user was created
	var count int64
	db.Model(&models.User{}).Where("username = ?", "newuser").Count(&count)
	assert.Equal(t, int64(1), count)

	// Test: Create duplicate user
	c, w = CreateRequestContext("POST", "/api/v1/admin/users", userReq)
	AuthorizeContext(c, adminUser)

	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "usertoremove")

	// Test: Successful delete
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/users/usertoremove", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "usertoremove"})

	handler := admin.ExportDeleteUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was deleted
	var count int64
	db.Model(&models.User{}).Where("username = ?", "usertoremove").Count(&count)
	assert.Equal(t, int64(0), count)

	// Test: Deleting own account
	c, w = CreateRequestContext("DELETE", "/api/v1/admin/users/admin", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "admin"})

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "usertoupdate")

	// Test: Successful update
	updateReq := models.UpdateUserRequest{
		Email:  "updated@example.com",
		Role:   "admin",
		Groups: "newgroup1,newgroup2",
	}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/usertoupdate", updateReq)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "usertoupdate"})

	handler := admin.ExportUpdateUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify user was updated
	var updatedUser models.User
	db.Where("username = ?", "usertoupdate").First(&updatedUser)
	assert.Equal(t, "updated@example.com", updatedUser.Email)
	assert.Equal(t, "admin", updatedUser.Role)
	assert.Equal(t, "newgroup1,newgroup2", updatedUser.Groups)

	// Test: Update username
	updateReq2 := models.UpdateUserRequest{
		Username: "newusername",
	}
	c, w = CreateRequestContext("PUT", "/api/v1/admin/users/usertoupdate", updateReq2)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "usertoupdate"})

	handler(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify username was changed
	var count int64
	db.Model(&models.User{}).Where("username = ?", "newusername").Count(&count)
	assert.Equal(t, int64(1), count)

	// Test: User not found
	c, w = CreateRequestContext("PUT", "/api/v1/admin/users/nonexistent", updateReq)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "nonexistent"})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAllUsers(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "user1")
	SetupTestUser(t, db, "user2")

	// Test: Successful request
	c, w := CreateRequestContext("GET", "/api/v1/admin/users", nil)
	AuthorizeContext(c, adminUser)

	handler := admin.ExportGetAllUsers(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var users []models.UserResponse
	ExtractJSONResponse(t, w, &users)
	assert.GreaterOrEqual(t, len(users), 3) // Admin + at least 2 users

	// Check for expected usernames
	usernames := make([]string, len(users))
	for i, u := range users {
		usernames[i] = u.Username
	}
	assert.Contains(t, usernames, "admin")
	assert.Contains(t, usernames, "user1")
	assert.Contains(t, usernames, "user2")

	// Test: Unauthorized
	regularUser := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("GET", "/api/v1/admin/users", nil)
	AuthorizeContext(c, regularUser)

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestScheduleHostDeletion(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	host := SetupTestHost(t, db, "hosttodelete")
	models.HostsList = []models.Host{host}

	// Test: Successful request
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/hosts/hosttodelete", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"hostname": "hosttodelete"})

	handler := admin.ExportScheduleHostDeletion(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host is marked for deletion
	var updatedHost models.Host
	db.Where("name = ?", "hosttodelete").First(&updatedHost)
	assert.True(t, updatedHost.UpForDeletion)

	// Test: Host not found
	c, w = CreateRequestContext("DELETE", "/api/v1/admin/hosts/nonexistent", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"hostname": "nonexistent"})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetUser(t *testing.T) {
	// Setup
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	adminUser := SetupTestAdmin(t, db)
	user := SetupTestUser(t, db, "usertofetch")
	user.Email = "fetch@example.com"
	db.Save(&user)

	// Test: Successful request
	c, w := CreateRequestContext("GET", "/api/v1/admin/users/usertofetch", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "usertofetch"})

	handler := admin.ExportGetUser(db)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var response models.UserResponse
	ExtractJSONResponse(t, w, &response)
	assert.Equal(t, "usertofetch", response.Username)
	assert.Equal(t, "fetch@example.com", response.Email)
	assert.Equal(t, "user", response.Role)

	// Test: User not found
	c, w = CreateRequestContext("GET", "/api/v1/admin/users/nonexistent", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "nonexistent"})

	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test: Unauthorized
	regularUser := SetupTestUser(t, db, "regularuser")
	c, w = CreateRequestContext("GET", "/api/v1/admin/users/usertofetch", nil)
	AuthorizeContext(c, regularUser)
	SetPathParams(c, map[string]string{"username": "usertofetch"})

	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
