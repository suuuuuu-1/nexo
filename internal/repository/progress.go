package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

var ErrProgressVersionConflict = errors.New("watch progress was updated by another request")

type ProgressRepository struct{ db *pgxpool.Pool }

// NewProgressRepository 创建观看进度数据访问对象。
func NewProgressRepository(db *pgxpool.Pool) *ProgressRepository { return &ProgressRepository{db: db} }

// Upsert 创建或更新观看进度。
// expectedVersion 不为空时启用乐观锁，版本不匹配则拒绝覆盖较新的进度。
func (r *ProgressRepository) Upsert(ctx context.Context, userID, episodeID string, position, duration int, expectedVersion *int64) (*model.WatchProgress, error) {
	if position < 0 || duration < 0 {
		return nil, errors.New("invalid progress")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var item model.WatchProgress
	if expectedVersion != nil {
		command, err := tx.Exec(ctx, `
			UPDATE watch_progress
			SET position_seconds = $3, duration_seconds = $4, version = version + 1, last_watched_at = NOW()
			WHERE user_id = $1 AND episode_id = $2 AND version = $5
		`, userID, episodeID, position, duration, *expectedVersion)
		if err != nil {
			return nil, err
		}
		if command.RowsAffected() == 0 {
			return nil, ErrProgressVersionConflict
		}
	} else {
		_, err := tx.Exec(ctx, `
			INSERT INTO watch_progress (user_id, episode_id, position_seconds, duration_seconds)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, episode_id) DO UPDATE SET position_seconds = $3, duration_seconds = $4, version = watch_progress.version + 1, last_watched_at = NOW()
		`, userID, episodeID, position, duration)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.QueryRow(ctx, `SELECT user_id, episode_id, position_seconds, duration_seconds, version, last_watched_at FROM watch_progress WHERE user_id = $1 AND episode_id = $2`, userID, episodeID).Scan(&item.UserID, &item.EpisodeID, &item.PositionSeconds, &item.DurationSeconds, &item.Version, &item.LastWatchedAt); err != nil {
		return nil, fmt.Errorf("read progress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

// List 返回用户最近观看的 Episode 进度。
func (r *ProgressRepository) List(ctx context.Context, userID string, limit int) ([]model.WatchProgress, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id, episode_id, position_seconds, duration_seconds, version, last_watched_at FROM watch_progress WHERE user_id = $1 ORDER BY last_watched_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.WatchProgress, 0)
	for rows.Next() {
		var item model.WatchProgress
		if err := rows.Scan(&item.UserID, &item.EpisodeID, &item.PositionSeconds, &item.DurationSeconds, &item.Version, &item.LastWatchedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
