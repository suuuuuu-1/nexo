package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type progressRequest struct {
	PositionSeconds int    `json:"position_seconds"`
	DurationSeconds int    `json:"duration_seconds"`
	ExpectedVersion *int64 `json:"expected_version"`
}

// updateProgress 记录用户观看进度，可携带版本号启用乐观锁。
func (r *Router) updateProgress(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.progress.Upsert(c.Request.Context(), userID, c.Param("id"), req.PositionSeconds, req.DurationSeconds, req.ExpectedVersion)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// listProgress 返回当前用户的观看记录。
func (r *Router) listProgress(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	items, err := r.progress.List(c.Request.Context(), userID, queryInt(c, "limit", 20))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
