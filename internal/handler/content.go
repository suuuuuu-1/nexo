package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/suuuuu/nexo/internal/model"
)

type contentRequest struct {
	Title          string  `json:"title" binding:"required"`
	ContentType    string  `json:"content_type"`
	Description    string  `json:"description"`
	CoverObjectKey *string `json:"cover_object_key"`
	PriceCents     int64   `json:"price_cents"`
}

type episodeRequest struct {
	EpisodeNo    int              `json:"episode_no" binding:"required"`
	Title        string           `json:"title" binding:"required"`
	Summary      string           `json:"summary"`
	AccessType   model.AccessType `json:"access_type"`
	PriceCents   int64            `json:"price_cents"`
	ResourceType string           `json:"resource_type"`
	BucketName   *string          `json:"bucket_name"`
	ObjectKey    *string          `json:"object_key"`
}

type episodeUploadRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes" binding:"required"`
}

const (
	maxEpisodeUploadBytes = int64(1 << 30) // v1 单个视频上限 1 GiB。
	episodeUploadTTL      = 15 * time.Minute
)

// listPublishedContents 返回用户侧可浏览的已发布内容。
func (r *Router) listPublishedContents(c *gin.Context) {
	items, err := r.content.ListPublished(c.Request.Context(), queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// getPublishedContent 返回内容详情及已发布 Episode。
func (r *Router) getPublishedContent(c *gin.Context) {
	item, err := r.content.GetPublished(c.Request.Context(), c.Param("id"))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// getPublishedEpisode 返回用户侧可见的单个 Episode 元数据。
func (r *Router) getPublishedEpisode(c *gin.Context) {
	item, err := r.content.GetEpisode(c.Request.Context(), c.Param("id"), true)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// listOperatorContents 返回运营侧包含草稿和下线内容的列表。
func (r *Router) listOperatorContents(c *gin.Context) {
	items, err := r.content.ListForOperator(c.Request.Context(), queryInt(c, "limit", 50), queryInt(c, "offset", 0))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// getOperatorContent 返回运营侧完整内容详情。
func (r *Router) getOperatorContent(c *gin.Context) {
	item, err := r.content.GetForOperator(c.Request.Context(), c.Param("id"))
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// createContent 创建草稿内容，发布由独立状态接口完成。
func (r *Router) createContent(c *gin.Context) {
	actorID, ok := CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var req contentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.content.CreateContent(c.Request.Context(), actorID, model.ContentInput{Title: req.Title, ContentType: req.ContentType, Description: req.Description, CoverObjectKey: req.CoverObjectKey, PriceCents: req.PriceCents})
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

// updateContent 更新内容元数据。
func (r *Router) updateContent(c *gin.Context) {
	actorID, _ := CurrentUserID(c)
	var req contentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.content.UpdateContent(c.Request.Context(), c.Param("id"), actorID, model.ContentInput{Title: req.Title, ContentType: req.ContentType, Description: req.Description, CoverObjectKey: req.CoverObjectKey, PriceCents: req.PriceCents})
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// publishContent 将内容发布到用户侧。
func (r *Router) publishContent(c *gin.Context) { r.changeContentStatus(c, model.StatusPublished) }

// offlineContent 将内容下线，但保留历史数据和订单关系。
func (r *Router) offlineContent(c *gin.Context) { r.changeContentStatus(c, model.StatusOffline) }

// changeContentStatus 复用内容发布和下线的公共处理逻辑。
func (r *Router) changeContentStatus(c *gin.Context, status model.ContentStatus) {
	actorID, _ := CurrentUserID(c)
	item, err := r.content.ChangeContentStatus(c.Request.Context(), c.Param("id"), actorID, status)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// presignEpisodeUpload 为已存在的运营内容生成服务端命名的 R2 对象 key 和短期 PUT 地址。
func (r *Router) presignEpisodeUpload(c *gin.Context) {
	var req episodeUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload request"})
		return
	}
	if !validEpisodeUpload(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only MP4 videos up to 1 GiB are allowed"})
		return
	}
	contentID := c.Param("id")
	if _, err := r.content.GetForOperator(c.Request.Context(), contentID); err != nil {
		r.writeBusinessError(c, err)
		return
	}

	objectKey := fmt.Sprintf("nexo/contents/%s/episodes/%s.mp4", contentID, uuid.NewString())
	url, err := r.storage.PresignPut(c.Request.Context(), r.config.R2Bucket, objectKey, req.ContentType, episodeUploadTTL)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"upload_url":   url,
		"object_key":   objectKey,
		"expires_at":   time.Now().Add(episodeUploadTTL),
		"content_type": req.ContentType,
	})
}

func validEpisodeUpload(req episodeUploadRequest) bool {
	return strings.TrimSpace(req.Filename) != "" &&
		strings.EqualFold(filepath.Ext(req.Filename), ".mp4") &&
		strings.EqualFold(strings.TrimSpace(req.ContentType), "video/mp4") &&
		req.SizeBytes > 0 && req.SizeBytes <= maxEpisodeUploadBytes
}

// createEpisode 创建内容下的 Episode，并保存其资源对象 key。
func (r *Router) createEpisode(c *gin.Context) {
	actorID, _ := CurrentUserID(c)
	var req episodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.content.CreateEpisode(c.Request.Context(), actorID, c.Param("id"), model.EpisodeInput{EpisodeNo: req.EpisodeNo, Title: req.Title, Summary: req.Summary, AccessType: req.AccessType, PriceCents: req.PriceCents, ResourceType: req.ResourceType, BucketName: req.BucketName, ObjectKey: req.ObjectKey})
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

// updateEpisode 更新 Episode 元数据及对象存储信息。
func (r *Router) updateEpisode(c *gin.Context) {
	actorID, _ := CurrentUserID(c)
	var req episodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := r.content.UpdateEpisode(c.Request.Context(), c.Param("id"), actorID, model.EpisodeInput{EpisodeNo: req.EpisodeNo, Title: req.Title, Summary: req.Summary, AccessType: req.AccessType, PriceCents: req.PriceCents, ResourceType: req.ResourceType, BucketName: req.BucketName, ObjectKey: req.ObjectKey})
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// publishEpisode 确认视频对象存在且元数据合规后，再将 Episode 发布到用户侧。
func (r *Router) publishEpisode(c *gin.Context) {
	episode, err := r.content.GetEpisode(c.Request.Context(), c.Param("id"), false)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	if episode.ObjectKey == nil || strings.TrimSpace(*episode.ObjectKey) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "episode video has not been uploaded"})
		return
	}
	bucket := r.config.R2Bucket
	if episode.BucketName != nil && strings.TrimSpace(*episode.BucketName) != "" {
		bucket = *episode.BucketName
	}
	info, err := r.storage.HeadObject(c.Request.Context(), bucket, *episode.ObjectKey)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "episode object was not found in R2; upload it before publishing"})
		return
	}
	if info.Size <= 0 || info.Size > maxEpisodeUploadBytes || !strings.EqualFold(strings.TrimSpace(strings.Split(info.ContentType, ";")[0]), "video/mp4") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "episode object must be an MP4 video no larger than 1 GiB"})
		return
	}
	r.changeEpisodeStatus(c, model.StatusPublished)
}

// offlineEpisode 将 Episode 下线。
func (r *Router) offlineEpisode(c *gin.Context) { r.changeEpisodeStatus(c, model.StatusOffline) }

// changeEpisodeStatus 复用 Episode 发布和下线的公共处理逻辑。
func (r *Router) changeEpisodeStatus(c *gin.Context, status model.ContentStatus) {
	actorID, _ := CurrentUserID(c)
	item, err := r.content.ChangeEpisodeStatus(c.Request.Context(), c.Param("id"), actorID, status)
	if err != nil {
		r.writeBusinessError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
