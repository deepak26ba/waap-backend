package routes

import (
	"github.com/gin-gonic/gin"

	"waap-backend/pkg/response"
)

// adminPingHandler handles administrative connectivity verification requests.
func adminPingHandler(c *gin.Context) {
	response.OK(c, gin.H{"status": "admin access granted"})
}
