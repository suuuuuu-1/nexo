package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/suuuuu/nexo/internal/health"
)

func TestHealthAndReadyRoutesHaveSeparateSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &Router{readiness: health.NewChecker(map[string]health.Probe{
		"postgres": func(context.Context) error { return nil },
	})}
	router := gin.New()
	router.GET("/health", api.health)
	router.GET("/ready", api.ready)

	liveness := httptest.NewRecorder()
	router.ServeHTTP(liveness, httptest.NewRequest(http.MethodGet, "/health", nil))
	if liveness.Code != http.StatusOK || !strings.Contains(liveness.Body.String(), `"status":"ok"`) {
		t.Fatalf("liveness response = %d %s", liveness.Code, liveness.Body.String())
	}

	readiness := httptest.NewRecorder()
	router.ServeHTTP(readiness, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if readiness.Code != http.StatusOK || !strings.Contains(readiness.Body.String(), `"status":"ready"`) {
		t.Fatalf("readiness response = %d %s", readiness.Code, readiness.Body.String())
	}
}

func TestReadyRouteReturnsServiceUnavailableWhenDependencyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &Router{readiness: health.NewChecker(map[string]health.Probe{
		"postgres": func(context.Context) error { return errors.New("connection details must not leak") },
	})}
	router := gin.New()
	router.GET("/ready", api.ready)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(response.Body.String(), `"postgres":"unavailable"`) || strings.Contains(response.Body.String(), "connection details") {
		t.Fatalf("readiness response leaks details or misses dependency status: %s", response.Body.String())
	}
}
