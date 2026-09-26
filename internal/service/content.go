package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/suuuuu/nexo/internal/cache"
	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var (
	ErrInvalidContentInput = errors.New("invalid content input")
	ErrContentNotFound     = repository.ErrContentNotFound
	ErrContentConflict     = repository.ErrContentConflict
)

type ContentService struct {
	repo  *repository.ContentRepository
	cache *cache.Client
}

// NewContentService 创建内容服务，并注入内容 Repository 和可选缓存客户端。
func NewContentService(repo *repository.ContentRepository, cacheClient *cache.Client) *ContentService {
	return &ContentService{repo: repo, cache: cacheClient}
}

// ListPublished 返回用户侧可见的已发布内容。
func (s *ContentService) ListPublished(ctx context.Context, limit, offset int) ([]model.Content, error) {
	limit, offset = normalizePage(limit, offset)
	return s.repo.ListContents(ctx, true, limit, offset)
}

// ListForOperator 返回运营侧内容列表，包含草稿和下线内容。
func (s *ContentService) ListForOperator(ctx context.Context, limit, offset int) ([]model.Content, error) {
	limit, offset = normalizePage(limit, offset)
	return s.repo.ListContents(ctx, false, limit, offset)
}

// GetPublished 优先读取 Redis 热点缓存，缓存未命中时回源数据库并写入缓存。
func (s *ContentService) GetPublished(ctx context.Context, id string) (*model.Content, error) {
	var cached model.Content
	key := "content:detail:" + id
	if s.cache != nil {
		if ok, err := s.cache.GetJSON(ctx, key, &cached); err == nil && ok {
			return &cached, nil
		}
	}
	item, err := s.repo.GetContent(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SetJSON(ctx, key, item, cacheTTL)
	}
	return item, nil
}

// GetForOperator 获取运营侧完整内容，不受发布状态过滤。
func (s *ContentService) GetForOperator(ctx context.Context, id string) (*model.Content, error) {
	return s.repo.GetContent(ctx, id, false)
}

// CreateContent 校验并创建内容。
func (s *ContentService) CreateContent(ctx context.Context, actorID string, input model.ContentInput) (*model.Content, error) {
	input = normalizeContentInput(input)
	if err := validateContent(input); err != nil {
		return nil, err
	}
	return s.repo.CreateContent(ctx, actorID, input)
}

// UpdateContent 更新内容后主动删除详情缓存，避免旧数据继续被读取。
func (s *ContentService) UpdateContent(ctx context.Context, id, actorID string, input model.ContentInput) (*model.Content, error) {
	input = normalizeContentInput(input)
	if err := validateContent(input); err != nil {
		return nil, err
	}
	item, err := s.repo.UpdateContent(ctx, id, actorID, input)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "content:detail:"+id)
	}
	return item, err
}

// ChangeContentStatus 修改发布状态，并同步失效用户侧详情缓存。
func (s *ContentService) ChangeContentStatus(ctx context.Context, id, actorID string, status model.ContentStatus) (*model.Content, error) {
	if status != model.StatusPublished && status != model.StatusOffline {
		return nil, ErrInvalidContentInput
	}
	item, err := s.repo.PublishContent(ctx, id, actorID, status)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "content:detail:"+id)
	}
	return item, err
}

// CreateEpisode 创建内容下的 Episode，并在成功后失效父内容缓存。
func (s *ContentService) CreateEpisode(ctx context.Context, actorID, contentID string, input model.EpisodeInput) (*model.Episode, error) {
	input = normalizeEpisodeInput(input)
	if err := validateEpisode(input); err != nil {
		return nil, err
	}
	item, err := s.repo.CreateEpisode(ctx, actorID, contentID, input)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "content:detail:"+contentID)
	}
	return item, err
}

// UpdateEpisode 更新 Episode 的元数据和资源 key。
func (s *ContentService) UpdateEpisode(ctx context.Context, id, actorID string, input model.EpisodeInput) (*model.Episode, error) {
	input = normalizeEpisodeInput(input)
	if err := validateEpisode(input); err != nil {
		return nil, err
	}
	item, err := s.repo.UpdateEpisode(ctx, id, actorID, input)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "content:detail:"+item.ContentID)
	}
	return item, err
}

// ChangeEpisodeStatus 修改 Episode 发布状态，并失效父内容详情缓存。
func (s *ContentService) ChangeEpisodeStatus(ctx context.Context, id, actorID string, status model.ContentStatus) (*model.Episode, error) {
	if status != model.StatusPublished && status != model.StatusOffline {
		return nil, ErrInvalidContentInput
	}
	item, err := s.repo.PublishEpisode(ctx, id, actorID, status)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "content:detail:"+item.ContentID)
	}
	return item, err
}

// GetEpisode 根据访问场景决定是否只返回已发布 Episode。
func (s *ContentService) GetEpisode(ctx context.Context, id string, publishedOnly bool) (*model.Episode, error) {
	return s.repo.GetEpisode(ctx, id, publishedOnly)
}

// validateContent 负责内容字段和内容类型的业务校验。
func validateContent(input model.ContentInput) error {
	if strings.TrimSpace(input.Title) == "" || len([]rune(input.Title)) > 200 || input.PriceCents < 0 {
		return ErrInvalidContentInput
	}
	if input.ContentType != "VIDEO" && input.ContentType != "NOVEL" {
		return ErrInvalidContentInput
	}
	return nil
}

// validateEpisode 负责访问类型、资源类型和价格等字段校验。
func validateEpisode(input model.EpisodeInput) error {
	if input.EpisodeNo <= 0 || strings.TrimSpace(input.Title) == "" || input.PriceCents < 0 {
		return ErrInvalidContentInput
	}
	if input.AccessType != model.AccessFree && input.AccessType != model.AccessPurchase && input.AccessType != model.AccessMembership && input.AccessType != model.AccessPurchaseOrMembership {
		return ErrInvalidContentInput
	}
	if input.ResourceType != "VIDEO" && input.ResourceType != "TEXT" {
		return ErrInvalidContentInput
	}
	return nil
}

// normalizePage 统一限制分页大小，避免无界查询。
func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// normalizeContentInput 填充内容创建时的默认类型。
func normalizeContentInput(input model.ContentInput) model.ContentInput {
	if input.ContentType == "" {
		input.ContentType = "VIDEO"
	}
	return input
}

// normalizeEpisodeInput 填充 Episode 的访问类型、资源类型和存储提供方默认值。
func normalizeEpisodeInput(input model.EpisodeInput) model.EpisodeInput {
	if input.AccessType == "" {
		input.AccessType = model.AccessFree
	}
	if input.ResourceType == "" {
		input.ResourceType = "VIDEO"
	}
	if input.StorageProvider == "" {
		input.StorageProvider = "R2"
	}
	return input
}

const cacheTTL = 5 * time.Minute
