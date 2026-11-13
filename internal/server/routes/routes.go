package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
)

// Dependencies holds shared resources required to initialize and wire application routes.
type Dependencies struct {
	Config  *config.Config
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Log     *zap.Logger
}

// Register mounts all route hierarchies onto the Gin engine.
func Register(r *gin.Engine, deps Dependencies) {
	RegisterObservability(r, deps.Pool, deps.Log)
	RegisterV1(r, deps)
}
