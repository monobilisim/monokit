// Code in this file relies on test utilities defined in testutils.go and helpers in admin_test.go.
// Ensure these are present in the same package ('tests'), or import them if split across files.
package tests

import (
	"net/http"
	"testing"

	common "github.com/monobilisim/monokit/common/api"
	"github.com/stretchr/testify/assert"
)

// Covers: listGroups 403 (non-admin)
func TestListGroups_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	c, w := CreateRequestContext("GET", "/api/v1/admin/groups", nil)
	AuthorizeContext(c, user)
	handler := common.ExportListGroups(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: deleteGroup 404 (group not found), 403 (non-admin)
func TestDeleteGroup_GroupNotFound(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	admin := SetupTestAdmin(t, db)
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/missing", nil)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{"name": "missing"})
	handler := common.ExportDeleteGroup(db)
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
	handler := common.ExportDeleteGroup(db)
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
	handler := common.ExportAddHostToGroup(db)
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
	handler := common.ExportRemoveHostFromGroup(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: updateUserGroups 403 (non-admin)
func TestUpdateUserGroups_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "testuser")
	updateReq := common.UpdateUserGroupsRequest{Groups: "gr1"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/testuser/groups", updateReq)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "testuser"})
	handler := common.ExportUpdateUserGroups(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: createUser 403 (non-admin)
func TestCreateUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	userReq := common.RegisterRequest{
		Username:  "newuser2",
		Password:  "pw",
		Email:     "e@x.com",
		Role:      "user",
		Groups:    "g",
		Inventory: "def",
	}
	c, w := CreateRequestContext("POST", "/api/v1/admin/users", userReq)
	AuthorizeContext(c, user)
	handler := common.ExportCreateUser(db)
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
	handler := common.ExportDeleteUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: updateUser 403 (non-admin) + 409 conflict (username exists)
func TestUpdateUser_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestUser(t, db, "victim")
	updateReq := common.UpdateUserRequest{Email: "a@b.com"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/victim", updateReq)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"username": "victim"})
	handler := common.ExportUpdateUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_UsernameConflict(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	admin := SetupTestAdmin(t, db)
	SetupTestUser(t, db, "victim")
	SetupTestUser(t, db, "existing")
	updateReq := common.UpdateUserRequest{Username: "existing"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/victim", updateReq)
	AuthorizeContext(c, admin)
	SetPathParams(c, map[string]string{"username": "victim"})
	handler := common.ExportUpdateUser(db)
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
	handler := common.ExportGetAllUsers(db)
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
	handler := common.ExportScheduleHostDeletion(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Covers: moveHostToInventory 403 (non-admin)
func TestMoveHostToInventory_Unauthorized(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(db)
	user := SetupTestUser(t, db, "user403")
	SetupTestHost(t, db, "hosttomove")
	c, w := CreateRequestContext("POST", "/api/v1/admin/hosts/hosttomove/move/newinventory", nil)
	AuthorizeContext(c, user)
	SetPathParams(c, map[string]string{"hostname": "hosttomove", "inventory": "newinventory"})
	handler := common.ExportMoveHostToInventory(db)
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
	handler := common.ExportGetUser(db)
	handler(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
