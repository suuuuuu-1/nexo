package service

import (
	"context"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var ErrProgressVersionConflict = repository.ErrProgressVersionConflict

type ProgressService struct {
	repo *repository.ProgressRepository
}

// NewProgressService 创建观看进度业务服务。
func NewProgressService(repo *repository.ProgressRepository) *ProgressService {
	return &ProgressService{repo: repo}
}

// Upsert 更新观看进度并保留乐观锁语义。
func (s *ProgressService) Upsert(ctx context.Context, userID, episodeID string, position, duration int, expectedVersion *int64) (*model.WatchProgress, error) {
	return s.repo.Upsert(ctx, userID, episodeID, position, duration, expectedVersion)
}

// List 返回观看历史，并统一限制查询数量。
func (s *ProgressService) List(ctx context.Context, userID string, limit int) ([]model.WatchProgress, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, userID, limit)
}
