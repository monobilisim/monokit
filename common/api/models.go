package common

// @Description Error response
type ErrorResponse struct {
	Error string `json:"error" example:"Admin access required"`
}

// @Description Group model
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required" example:"developers"`
}

// @Description Host response model
type HostResponse struct {
	ID                 uint   `json:"id" example:"1"`
	Name               string `json:"name" example:"webserver01"`
	CpuCores           int    `json:"cpuCores" example:"8"`
	Ram                string `json:"ram" example:"32GB"`
	MonokitVersion     string `json:"monokitVersion" example:"1.0.0"`
	Os                 string `json:"os" example:"Ubuntu 22.04 LTS"`
	DisabledComponents string `json:"disabledComponents" example:"nginx::mysql"`
	IpAddress          string `json:"ipAddress" example:"192.168.1.100"`
	Status             string `json:"status" example:"Online"`
	WantsUpdateTo      string `json:"wantsUpdateTo" example:"1.1.0"`
	Groups             string `json:"groups" example:"developers,production"`
}

// @Description Group response model
type GroupResponse struct {
	ID    uint           `json:"id" example:"1"`
	Name  string         `json:"name" example:"developers"`
	Hosts []HostResponse `json:"hosts,omitempty"`
}

// @Description User response model
type UserResponse struct {
	ID       uint   `json:"id" example:"1"`
	Username string `json:"username" example:"johndoe"`
	Email    string `json:"email" example:"john.doe@example.com"`
	Role     string `json:"role" example:"admin"`
	Groups   string `json:"groups" example:"developers,admins"`
}

// @Description Login request
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"secretpassword123"`
}

// @Description Login response
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  struct {
		Username string `json:"username" example:"johndoe"`
		Email    string `json:"email" example:"john.doe@example.com"`
		Role     string `json:"role" example:"admin"`
		Groups   string `json:"groups" example:"developers,admins"`
	} `json:"user"`
}

// @Description Register request
type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"secretpassword123"`
	Email    string `json:"email" binding:"required" example:"john.doe@example.com"`
	Role     string `json:"role" binding:"required" example:"user"`
	Groups   string `json:"groups" example:"developers"`
}

// @Description Update user groups request
type UpdateUserGroupsRequest struct {
	Groups string `json:"groups" binding:"required" example:"developers,production"`
}

// @Description Update own user details request
type UpdateMeRequest struct {
	Username string `json:"username,omitempty" example:"johndoe"`
	Password string `json:"password,omitempty" example:"newpassword123"`
	Email    string `json:"email,omitempty" example:"john.doe@example.com"`
}

// @Description Update user request (admin)
type UpdateUserRequest struct {
	Username string `json:"username,omitempty" example:"johndoe"`
	Password string `json:"password,omitempty" example:"newpassword123"`
	Email    string `json:"email,omitempty" example:"john.doe@example.com"`
	Role     string `json:"role,omitempty" example:"admin"`
	Groups   string `json:"groups,omitempty" example:"developers,admins"`
}
