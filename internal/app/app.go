package app

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lukman-ss/ballo"
	"github.com/lukman-ss/ballo-todo/internal/config"
	"github.com/lukman-ss/ballo-todo/internal/http/handler"
	"github.com/lukman-ss/ballo-todo/internal/task"
	"github.com/lukman-ss/ballo-todo/internal/todo"
	"github.com/lukman-ss/ballo-todo/internal/web"
	_ "modernc.org/sqlite"
)

// App encapsulates the Ballo engine, configuration, and app components.
type App struct {
	cfg    *config.Config
	engine *ballo.Engine
	ready  *atomic.Bool
	logger *slog.Logger
	db     *sql.DB
}

// New constructs and wires the Ballo application.
func New(cfg *config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger := newLogger(cfg.LogLevel)

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if err := todo.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	todoStore := todo.NewStore(db, loc)

	templateDir := cfg.TemplateDir
	if templateDir == "" {
		templateDir = "internal/web/templates"
	}
	renderer, err := web.NewRenderer(templateDir, loc)
	if err != nil {
		db.Close()
		return nil, err
	}
	webHandler := handler.NewWebHandler(todoStore, renderer, loc, cfg.MaxBodyBytes)

	ready := &atomic.Bool{}
	ready.Store(false)

	engine := ballo.New()

	engine.Use(ballo.RecoveryMiddleware())
	engine.Use(web.CSRFMiddleware())

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

	engine.NotFound(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusNotFound, map[string]string{"error": "resource not found"}), nil
	})
	engine.MethodNotAllowed(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"}), nil
	})

	healthHandler := handler.NewHealthHandler(ready)
	engine.GET("/healthz", healthHandler.Healthz)
	engine.GET("/readyz", healthHandler.Readyz)

	// Web UI routes
	engine.GET("/", webHandler.Index)
	engine.POST("/todos", webHandler.Create)
	engine.GET("/todos/new", webHandler.NewForm)
	engine.GET("/todos/:id/edit", webHandler.EditForm)
	engine.POST("/todos/:id/edit", webHandler.EditSubmit)
	engine.POST("/todos/:id/toggle", webHandler.Toggle)
	engine.POST("/todos/:id/delete", webHandler.Delete)

	// Static file serving via httptest recorder
	staticFS := http.FileServer(http.Dir("internal/web/static"))
	engine.GET("/static/*filepath", func(req *ballo.Request) (*ballo.Response, error) {
		rec := httptest.NewRecorder()
		http.StripPrefix("/static", staticFS).ServeHTTP(rec, req.Request)
		result := rec.Result()
		body, _ := io.ReadAll(result.Body)
		return &ballo.Response{
			StatusCode: result.StatusCode,
			Header:     result.Header,
			Body:       body,
		}, nil
	})

	// Legacy task API
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
		db:     db,
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

// DB returns the database connection.
func (a *App) DB() *sql.DB {
	return a.db
}

// Close shuts down app resources.
func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
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

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}
