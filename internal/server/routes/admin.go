package routes

import (
	"github.com/gin-gonic/gin"

	"waap-backend/pkg/response"
)

// RegisterAdminRoutes mounts administrative endpoints on the provided router group.
func RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.GET("/ping", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "admin access granted"})
	})
}
