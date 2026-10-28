package handler

import (
	"net/http"
	"sync/atomic"

	"github.com/lukman-ss/ballo"
)

// HealthHandler manages liveness and readiness probe responses.
type HealthHandler struct {
	ready *atomic.Bool
}

// NewHealthHandler creates a HealthHandler with readiness flag.
func NewHealthHandler(ready *atomic.Bool) *HealthHandler {
	return &HealthHandler{ready: ready}
}

// Healthz responds to liveness probes without exposing sensitive details.
func (h *HealthHandler) Healthz(req *ballo.Request) (*ballo.Response, error) {
	return ballo.JSON(http.StatusOK, map[string]string{
		"status": "pass",
	}), nil
}

// Readyz responds to readiness probes.
func (h *HealthHandler) Readyz(req *ballo.Request) (*ballo.Response, error) {
	if h.ready != nil && !h.ready.Load() {
		return ballo.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		}), nil
	}
	return ballo.JSON(http.StatusOK, map[string]string{
		"status": "ready",
	}), nil
}
