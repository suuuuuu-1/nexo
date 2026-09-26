package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

type EntitlementRepository struct{ db *pgxpool.Pool }

// NewEntitlementRepository 创建权益数据访问对象。
func NewEntitlementRepository(db *pgxpool.Pool) *EntitlementRepository {
	return &EntitlementRepository{db: db}
}

// GetEpisodePolicy 查询已发布 Episode 的访问类型和所属内容。
func (r *EntitlementRepository) GetEpisodePolicy(ctx context.Context, episodeID string) (string, model.AccessType, error) {
	var contentID string
	var accessType model.AccessType
	err := r.db.QueryRow(ctx, `
		SELECT content_id, access_type
		FROM episodes WHERE id = $1 AND status = 'PUBLISHED'
	`, episodeID).Scan(&contentID, &accessType)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrContentNotFound
	}
	return contentID, accessType, err
}

// FindActivePurchaseEntitlement 查询针对 Episode 或所属 Content 的有效购买权益。
func (r *EntitlementRepository) FindActivePurchaseEntitlement(ctx context.Context, userID, episodeID, contentID string) (string, bool, error) {
	var sourceID string
	err := r.db.QueryRow(ctx, `
		SELECT source_id FROM entitlements
		WHERE user_id = $1 AND status = 'ACTIVE'
		  AND starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())
		  AND (
			(scope_type = 'EPISODE' AND scope_id = $2)
			OR (scope_type = 'CONTENT' AND scope_id = $3)
		  )
		ORDER BY CASE WHEN scope_type = 'EPISODE' THEN 1 ELSE 2 END
		LIMIT 1
	`, userID, episodeID, contentID).Scan(&sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return sourceID, err == nil, err
}

// FindActiveMembershipEntitlement 查询用户当前有效的会员权益。
func (r *EntitlementRepository) FindActiveMembershipEntitlement(ctx context.Context, userID string) (string, bool, error) {
	var sourceID string
	err := r.db.QueryRow(ctx, `
		SELECT source_id FROM entitlements
		WHERE user_id = $1 AND scope_type = 'MEMBERSHIP' AND status = 'ACTIVE'
		  AND starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())
		LIMIT 1
	`, userID).Scan(&sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return sourceID, err == nil, err
}
