package app

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lukman-ss/ballo"
	"github.com/lukman-ss/ballo-todo/internal/config"
	"github.com/lukman-ss/ballo-todo/internal/http/handler"
	"github.com/lukman-ss/ballo-todo/internal/task"
)

// App encapsulates the Ballo engine, configuration, and app components.
type App struct {
	cfg    *config.Config
	engine *ballo.Engine
	ready  *atomic.Bool
	logger *slog.Logger
}

// New constructs and wires the Ballo application.
func New(cfg *config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger := newLogger(cfg.LogLevel)

	ready := &atomic.Bool{}
	ready.Store(false)

	engine := ballo.New()

	// Global recovery middleware
	engine.Use(ballo.RecoveryMiddleware())

	// Access logging middleware with timing (never logs credentials/headers/body)
	engine.Use(func(next ballo.Handler) ballo.Handler {
		return func(req *ballo.Request) (*ballo.Response, error) {
			start := time.Now()
			resp, err := next(req)
			duration := time.Since(start)

			status := http.StatusOK
			if resp != nil && resp.StatusCode != 0 {
				status = resp.StatusCode
			}

			logger.Info("http request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", req.RemoteAddr,
			)

			return resp, err
		}
	})

	// Custom 404 handler
	engine.NotFound(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusNotFound, map[string]string{
			"error": "resource not found",
		}), nil
	})

	// Custom 405 handler
	engine.MethodNotAllowed(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		}), nil
	})

	// Register health handlers
	healthHandler := handler.NewHealthHandler(ready)
	engine.GET("/healthz", healthHandler.Healthz)
	engine.GET("/readyz", healthHandler.Readyz)

	// Register task handlers
	taskStore := task.NewStore()
	taskHandler := handler.NewTaskHandler(taskStore, cfg.MaxBodyBytes)

	apiV1 := engine.Group("/api/v1")
	apiV1.POST("/tasks", taskHandler.Create)
	apiV1.GET("/tasks", taskHandler.List)
	apiV1.GET("/tasks/:id", taskHandler.GetByID)
	apiV1.PATCH("/tasks/:id", taskHandler.Update)
	apiV1.DELETE("/tasks/:id", taskHandler.Delete)

	return &App{
		cfg:    cfg,
		engine: engine,
		ready:  ready,
		logger: logger,
	}, nil
}

// Engine returns the underlying Ballo engine (implements http.Handler).
func (a *App) Engine() *ballo.Engine {
	return a.engine
}

// SetReady sets the application readiness state.
func (a *App) SetReady(ready bool) {
	a.ready.Store(ready)
}

// Logger returns the application structured logger.
func (a *App) Logger() *slog.Logger {
	return a.logger
}

func newLogger(levelStr string) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}
