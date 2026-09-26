package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

type OutboxRepository struct{ db *pgxpool.Pool }

// NewOutboxRepository 创建 Outbox 数据访问对象。
func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository { return &OutboxRepository{db: db} }

// ClaimPending 使用 FOR UPDATE SKIP LOCKED 抢占一批待投递事件。
// 多个 Publisher 实例并行运行时不会重复领取同一批记录。
func (r *OutboxRepository) ClaimPending(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT id, event_type, aggregate_type, aggregate_id, payload, attempts
		FROM outbox_events
		WHERE status = 'PENDING' AND next_retry_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, err
	}
	deferred := make([]model.OutboxEvent, 0)
	for rows.Next() {
		var item model.OutboxEvent
		if err := rows.Scan(&item.ID, &item.EventType, &item.AggregateType, &item.AggregateID, &item.Payload, &item.Attempts); err != nil {
			rows.Close()
			return nil, err
		}
		deferred = append(deferred, item)
	}
	rows.Close()
	for _, item := range deferred {
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1 WHERE id = $1`, item.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return deferred, nil
}

// MarkPublished 将已经成功投递到 RabbitMQ 的事件标记为已发布。
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE outbox_events SET status = 'PUBLISHED', published_at = NOW() WHERE id = $1`, id)
	return err
}

// MarkFailed 根据尝试次数安排下一次重试，超过阈值后进入 FAILED 状态。
func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, attempts int) error {
	status := "PENDING"
	if attempts >= 5 {
		status = "FAILED"
	}
	_, err := r.db.Exec(ctx, `UPDATE outbox_events SET status = $2, next_retry_at = $3 WHERE id = $1`, id, status, time.Now().Add(time.Duration(attempts+1)*time.Second))
	return err
}
