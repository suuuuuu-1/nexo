package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/health"
)

const readinessTimeout = 2 * time.Second

type ReadinessChecker interface {
	Check(context.Context) health.Report
}

// health reports process liveness without depending on external services.
func (r *Router) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ready checks required runtime dependencies with one bounded request deadline.
func (r *Router) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
	defer cancel()

	if r.readiness == nil {
		c.JSON(http.StatusServiceUnavailable, health.Report{
			Status:       "not_ready",
			Dependencies: map[string]string{"checker": "unavailable"},
		})
		return
	}

	report := r.readiness.Check(ctx)
	status := http.StatusOK
	if report.Status != "ready" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, report)
}
