//go:build !with_api

package common

import (
	"github.com/gin-gonic/gin"
)

// SetupFrontend is a no-op when building without API
func SetupFrontend(r *gin.Engine) {
	// Do nothing when API (and therefore frontend) is not included
}
