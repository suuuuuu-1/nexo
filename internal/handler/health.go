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

// health 返回进程存活状态，不检查外部依赖。
func (r *Router) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ready 在单个请求超时时间内检查服务运行所需的依赖。
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
