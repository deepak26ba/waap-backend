package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"waap-backend/pkg/config"
	"waap-backend/pkg/db"
)

// Server encapsulates the HTTP server, configuration, database connection, and router.
type Server struct {
	cfg     *config.Config
	log     *zap.Logger
	pool    *pgxpool.Pool
	queries db.Querier
	router  *gin.Engine
	httpSrv *http.Server
}

// New creates and initializes a new Server instance.
func New(cfg *config.Config, pool *pgxpool.Pool, queries db.Querier, log *zap.Logger) *Server {
	if cfg != nil && cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := NewRouter(cfg, pool, queries, log)

	port := "8080"
	if cfg != nil && cfg.Port != "" {
		port = cfg.Port
	}

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		cfg:     cfg,
		log:     log,
		pool:    pool,
		queries: queries,
		router:  router,
		httpSrv: httpSrv,
	}
}

// Router returns the configured Gin engine.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Start runs the HTTP server and blocks until an error occurs or the server is closed.
func (s *Server) Start() error {
	if s.log != nil {
		s.log.Info("Starting HTTP server", zap.String("addr", s.httpSrv.Addr))
	}
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the HTTP server without interrupting active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
