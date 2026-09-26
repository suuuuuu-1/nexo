package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// getPlayURL 先查 Episode，再统一校验权益，最后生成短期对象存储访问地址。
func (r *Router) getPlayURL(c *gin.Context) {
	userID, _ := CurrentUserID(c)
	episode, err := r.content.GetEpisode(c.Request.Context(), c.Param("id"), true)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	access, err := r.entitlement.CheckEpisodeAccess(c.Request.Context(), userID, episode.ID)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	if !access.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "episode access denied", "reason": access.Reason})
		return
	}
	if episode.ObjectKey == nil || strings.TrimSpace(*episode.ObjectKey) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "episode resource is not configured"})
		return
	}
	bucket := r.config.R2Bucket
	if episode.BucketName != nil && *episode.BucketName != "" {
		bucket = *episode.BucketName
	}
	playURL, err := r.storage.PresignGet(c.Request.Context(), bucket, *episode.ObjectKey, 10*time.Minute)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"play_url": playURL, "expires_at": time.Now().Add(10 * time.Minute)})
}
