//go:build with_api

package tests

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monobilisim/monokit/common/api/admin"
	"github.com/monobilisim/monokit/common/api/models"
	"github.com/stretchr/testify/assert"
)

func TestSetupAdminRoutes_CoversAll(t *testing.T) {
	r := gin.New()
	db := SetupTestDB(t)
	defer CleanupTestDB(db)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetupAdminRoutes panicked: %v", r)
		}
	}()

	admin.SetupAdminRoutes(r, db)

	want := []string{
		"/api/v1/admin/groups",
		"/api/v1/admin/groups/:name",
		"/api/v1/admin/groups/:name/hosts/:hostname",
		"/api/v1/admin/users/:username/groups",
		"/api/v1/admin/users",
		"/api/v1/admin/users/:username",
		"/api/v1/admin/hosts/:hostname",
		"/api/v1/admin/hosts/:hostname/move/:inventory",
		"/api/v1/admin/users/:username",
	}
	var got []string
	for _, route := range r.Routes() {
		got = append(got, route.Path)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected route %s is not registered", w)
		}
	}
}

func setupMockDB(t *testing.T) *MockGormDB {
	db := SetupTestDB(t)
	return &MockGormDB{DB: db}
}

func TestDeleteGroup_FindHostsError(t *testing.T) {
	m := setupMockDB(t)
	_ = SetupTestGroup(t, m.DB, "g")
	adminUser := SetupTestAdmin(t, m.DB)
	m.ErrorOnFindHosts = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/g", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "g"})
	handler := admin.ExportDeleteGroup(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to fetch hosts")
}

func TestDeleteGroup_SaveHostError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestGroup(t, m.DB, "g3")
	h2 := SetupTestHost(t, m.DB, "h2")
	h2.Groups = "g3"
	m.DB.Save(&h2)
	m.ErrorOnSaveHost = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/g3", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "g3"})
	handler := admin.ExportDeleteGroup(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to update host groups")
}

func TestDeleteGroup_FindUsersError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestGroup(t, m.DB, "g4")
	m.ErrorOnFindUsers = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/g4", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "g4"})
	handler := admin.ExportDeleteGroup(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to fetch users")
}

func TestDeleteGroup_SaveUserError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestGroup(t, m.DB, "g5")
	u := SetupTestUser(t, m.DB, "u5")
	u.Groups = "g5"
	m.DB.Save(&u)
	m.ErrorOnSaveUser = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/g5", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "g5"})
	handler := admin.ExportDeleteGroup(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to update user groups")
}

func TestDeleteGroup_GroupDeleteError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestGroup(t, m.DB, "g6")
	m.ErrorOnDeleteGroup = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/groups/g6", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"name": "g6"})
	handler := admin.ExportDeleteGroup(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to delete group")
}

// -- 4. UpdateUserGroups DB error

// -- 5. UpdateUser with HashPassword failure
func TestUpdateUser_HashPasswordError(t *testing.T) {
	db := SetupTestDB(t)
	adminUser := SetupTestAdmin(t, db)
	_ = SetupTestUser(t, db, "target")
	// We cannot monkey-patch HashPassword in Go; skipping this test until dependency injection is available.
	t.Skip("Cannot monkey-patch HashPassword in Go; skipping test.")

	req := models.UpdateUserRequest{Password: "testx"}
	c, w := CreateRequestContext("PUT", "/api/v1/admin/users/target", req)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"username": "target"})
	handler := admin.ExportUpdateUser(db)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to hash password")
}

// -- 6. ScheduleHostDeletion Save host error
func TestScheduleHostDeletion_SaveHostError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestHost(t, m.DB, "hostxx")
	m.ErrorOnSaveHost = true
	c, w := CreateRequestContext("DELETE", "/api/v1/admin/hosts/hostxx", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{"hostname": "hostxx"})
	handler := admin.ExportScheduleHostDeletion(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to schedule host for deletion")
}

// -- 7. MoveHostToInventory Save host error
func TestMoveHostToInventory_SaveHostError(t *testing.T) {
	m := setupMockDB(t)
	adminUser := SetupTestAdmin(t, m.DB)
	_ = SetupTestHost(t, m.DB, "hostyy")
	i := &models.Inventory{Name: "inv2"}
	m.DB.Create(i)
	c, w := CreateRequestContext("POST", "/api/v1/admin/hosts/hostyy/move/inv2", nil)
	AuthorizeContext(c, adminUser)
	SetPathParams(c, map[string]string{
		"hostname":  "hostyy",
		"inventory": "inv2",
	})
	m.ErrorOnSaveHost = true
	handler := admin.ExportMoveHostToInventory(m)
	handler(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	AssertErrorResponse(t, w, "Failed to move host")
}
