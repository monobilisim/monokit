//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/monobilisim/monokit/common/api/admin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

// Covers: listGroups 403 (non-admin)
func TestListGroups_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	c, w := CreateRequestContext("GET", "/api/v1/admin/groups", nil)
	AuthorizeContext(c, user)
	handler := admin.ExportListGroups(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: deleteGroup 404 (group not found), 403 (non-admin)
func TestDeleteGroup_GroupNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	adminUser := SetupTestAdmin(t, db)
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/missing", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "missing"})
	handler := admin.ExportDeleteGroup(db)
	handler(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteGroup_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestGroup(t, db, "testgroup")
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/testgroup", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"name": "testgroup"})
	handler := admin.ExportDeleteGroup(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: addHostToGroup 403 (non-admin)
func TestAddHostToGroup_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestGroup(t, db, "testgroup")
	SetupTestHost(t, db, "testhost")
	c, w := CreateRequestContext("POST", "/api/v1/admin/groups/testgroup/hosts/testhost", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"name": "testgroup", "hostname": "testhost"})
	handler := admin.ExportAddHostToGroup(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: removeHostFromGroup 403 (non-admin)
func TestRemoveHostFromGroup_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestGroup(t, db, "testgroup")
	SetupTestHost(t, db, "testhost")
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/testgroup/hosts/testhost", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"name": "testgroup", "hostname": "testhost"})
	handler := admin.ExportRemoveHostFromGroup(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: updateUserGroups 403 (non-admin)
func TestUpdateUserGroups_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "testuser")
	updateReq := models.UpdateUserGroupsRequest{Groups: "gr1"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/testuser/groups", updateReq)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "testuser"})
	handler := admin.ExportUpdateUserGroups(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: createUser 403 (non-admin)
func TestCreateUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	userReq := models.RegisterRequest{
		Username: "newuser2",
		Password: "pw",
		Email:    "e@x.com",
		Role:     "user",
		Groups:   "g",
	}
	c, w := CreateRequestContext("POST", "/api/v1/admin/users", userReq)
	AuthorizeContext(c, user)
	handler := admin.ExportCreateUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: deleteUser 403 (non-admin)
func TestDeleteUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "victim")
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/users/victim", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "victim"})
	handler := admin.ExportDeleteUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: updateUser 403 (non-admin) + 409 conflict (username exists)
func TestUpdateUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "victim")
	updateReq := models.UpdateUserRequest{Email: "a@b.com"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/victim", updateReq)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "victim"})
	handler := admin.ExportUpdateUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_UsernameConflict(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	adminUser := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "victim")
	SetupTestUser(t, db, "existing")
	updateReq := models.UpdateUserRequest{Username: "existing"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/victim", updateReq)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "victim"})
	handler := admin.ExportUpdateUser(db)
	handler(c)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// Covers: getAllUsers 403 (non-admin)
func TestGetAllUsers_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	c, w := CreateRequestContext("GET", "/api/v1/admin/users", nil)
	AuthorizeContext(c, user)
	handler := admin.ExportGetAllUsers(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: scheduleHostDeletion 403 (non-admin)
func TestScheduleHostDeletion_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestHost(t, db, "hosttodelete")
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/hosts/hosttodelete", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"hostname": "hosttodelete"})
	handler := admin.ExportScheduleHostDeletion(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: getUser 403 (non-admin)
func TestGetUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "usertofetch")
	c, w := CreateRequestContext("GET", "/api/v1/admin/users/usertofetch", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "usertofetch"})
	handler := admin.ExportGetUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
