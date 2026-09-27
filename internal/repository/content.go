package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

var (
	ErrContentNotFound = errors.New("content not found")
	ErrContentConflict = errors.New("content conflict")
)

type ContentRepository struct{ db *pgxpool.Pool }

// NewContentRepository 创建内容数据访问对象。
func NewContentRepository(db *pgxpool.Pool) *ContentRepository { return &ContentRepository{db: db} }

// CreateContent 创建草稿内容，发布动作由单独的状态接口完成。
func (r *ContentRepository) CreateContent(ctx context.Context, actorID string, input model.ContentInput) (*model.Content, error) {
	return r.findContent(ctx, `
		INSERT INTO contents (title, content_type, description, cover_object_key, price_cents, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, title, content_type, description, cover_object_key, price_cents, status, created_by, updated_by, published_by, published_at, created_at, updated_at
	`, input.Title, input.ContentType, input.Description, input.CoverObjectKey, input.PriceCents, actorID)
}

// UpdateContent 更新内容元数据，不在这里隐式改变发布状态。
func (r *ContentRepository) UpdateContent(ctx context.Context, id, actorID string, input model.ContentInput) (*model.Content, error) {
	return r.findContent(ctx, `
		UPDATE contents
		SET title = $2, content_type = $3, description = $4, cover_object_key = $5, price_cents = $6, updated_by = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, content_type, description, cover_object_key, price_cents, status, created_by, updated_by, published_by, published_at, created_at, updated_at
	`, id, input.Title, input.ContentType, input.Description, input.CoverObjectKey, input.PriceCents, actorID)
}

// ListContents 根据调用方决定是否过滤为已发布内容。
func (r *ContentRepository) ListContents(ctx context.Context, publishedOnly bool, limit, offset int) ([]model.Content, error) {
	query := `SELECT id, title, content_type, description, cover_object_key, price_cents, status, created_by, updated_by, published_by, published_at, created_at, updated_at FROM contents`
	if publishedOnly {
		query += ` WHERE status = 'PUBLISHED'`
	}
	query += ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list contents: %w", err)
	}
	defer rows.Close()
	result := make([]model.Content, 0)
	for rows.Next() {
		item, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contents: %w", err)
	}
	return result, nil
}

// GetContent 查询内容及其 Episode 列表。
func (r *ContentRepository) GetContent(ctx context.Context, id string, publishedOnly bool) (*model.Content, error) {
	query := `SELECT id, title, content_type, description, cover_object_key, price_cents, status, created_by, updated_by, published_by, published_at, created_at, updated_at FROM contents WHERE id = $1`
	if publishedOnly {
		query += ` AND status = 'PUBLISHED'`
	}
	content, err := r.findContent(ctx, query, id)
	if err != nil {
		return nil, err
	}
	episodes, err := r.ListEpisodes(ctx, id, publishedOnly)
	if err != nil {
		return nil, err
	}
	content.Episodes = episodes
	return content, nil
}

// PublishContent 实际承担发布和下线两种状态变更，并记录操作人。
func (r *ContentRepository) PublishContent(ctx context.Context, id, actorID string, status model.ContentStatus) (*model.Content, error) {
	return r.findContent(ctx, `
		UPDATE contents
		SET status = $2::varchar, updated_by = $3,
			published_by = CASE WHEN $2::varchar = 'PUBLISHED' THEN $3 ELSE published_by END,
			published_at = CASE WHEN $2::varchar = 'PUBLISHED' THEN NOW() ELSE published_at END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, content_type, description, cover_object_key, price_cents, status, created_by, updated_by, published_by, published_at, created_at, updated_at
	`, id, status, actorID)
}

// CreateEpisode 创建 Episode，并保存对象存储的 bucket 和 object key 元数据。
func (r *ContentRepository) CreateEpisode(ctx context.Context, actorID, contentID string, input model.EpisodeInput) (*model.Episode, error) {
	return r.findEpisode(ctx, `
		INSERT INTO episodes (content_id, episode_no, title, summary, access_type, price_cents, resource_type, bucket_name, object_key, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		RETURNING id, content_id, episode_no, title, summary, access_type, price_cents, resource_type, bucket_name, object_key, status, created_by, updated_by, created_at, updated_at
	`, contentID, input.EpisodeNo, input.Title, input.Summary, input.AccessType, input.PriceCents, input.ResourceType, input.BucketName, input.ObjectKey, actorID)
}

// UpdateEpisode 更新 Episode 的展示字段、访问规则和资源定位信息。
func (r *ContentRepository) UpdateEpisode(ctx context.Context, id, actorID string, input model.EpisodeInput) (*model.Episode, error) {
	return r.findEpisode(ctx, `
		UPDATE episodes
		SET episode_no = $2, title = $3, summary = $4, access_type = $5, price_cents = $6, resource_type = $7, bucket_name = $8, object_key = $9, updated_by = $10, updated_at = NOW()
		WHERE id = $1
		RETURNING id, content_id, episode_no, title, summary, access_type, price_cents, resource_type, bucket_name, object_key, status, created_by, updated_by, created_at, updated_at
	`, id, input.EpisodeNo, input.Title, input.Summary, input.AccessType, input.PriceCents, input.ResourceType, input.BucketName, input.ObjectKey, actorID)
}

// PublishEpisode 实际承担 Episode 发布和下线两种状态变更。
func (r *ContentRepository) PublishEpisode(ctx context.Context, id, actorID string, status model.ContentStatus) (*model.Episode, error) {
	return r.findEpisode(ctx, `
		UPDATE episodes SET status = $2, updated_by = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, content_id, episode_no, title, summary, access_type, price_cents, resource_type, bucket_name, object_key, status, created_by, updated_by, created_at, updated_at
	`, id, status, actorID)
}

// GetEpisode 查询单个 Episode；用户侧调用时必须开启 publishedOnly。
func (r *ContentRepository) GetEpisode(ctx context.Context, id string, publishedOnly bool) (*model.Episode, error) {
	query := `
		SELECT e.id, e.content_id, c.title, c.status, e.episode_no, e.title, e.summary, e.access_type, e.price_cents, e.resource_type, e.bucket_name, e.object_key, e.status, e.created_by, e.updated_by, e.created_at, e.updated_at
		FROM episodes e JOIN contents c ON c.id = e.content_id WHERE e.id = $1`
	if publishedOnly {
		query += ` AND c.status = 'PUBLISHED' AND e.status = 'PUBLISHED'`
	}
	return r.findEpisode(ctx, query, id)
}

// ListEpisodes 按 EpisodeNo 顺序返回内容下的 Episode。
func (r *ContentRepository) ListEpisodes(ctx context.Context, contentID string, publishedOnly bool) ([]model.Episode, error) {
	query := `
		SELECT e.id, e.content_id, c.title, c.status, e.episode_no, e.title, e.summary, e.access_type, e.price_cents, e.resource_type, e.bucket_name, e.object_key, e.status, e.created_by, e.updated_by, e.created_at, e.updated_at
		FROM episodes e JOIN contents c ON c.id = e.content_id WHERE e.content_id = $1`
	if publishedOnly {
		query += ` AND c.status = 'PUBLISHED' AND e.status = 'PUBLISHED'`
	}
	query += ` ORDER BY e.episode_no ASC`
	rows, err := r.db.Query(ctx, query, contentID)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer rows.Close()
	result := make([]model.Episode, 0)
	for rows.Next() {
		item, err := scanEpisode(rows, true)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

// findContent 统一处理内容写入后的字段扫描和数据库错误转换。
func (r *ContentRepository) findContent(ctx context.Context, query string, args ...any) (*model.Content, error) {
	var item model.Content
	err := r.db.QueryRow(ctx, strings.TrimSpace(query), args...).Scan(
		&item.ID, &item.Title, &item.ContentType, &item.Description, &item.CoverObjectKey, &item.PriceCents, &item.Status,
		&item.CreatedBy, &item.UpdatedBy, &item.PublishedBy, &item.PublishedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrContentNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrContentConflict
		}
		return nil, fmt.Errorf("content query: %w", err)
	}
	return &item, nil
}

// findEpisode 兼容“带内容信息”和“不带内容信息”的两类查询结果。
func (r *ContentRepository) findEpisode(ctx context.Context, query string, args ...any) (*model.Episode, error) {
	var item model.Episode
	withContent := strings.Contains(query, "c.title")
	var err error
	if withContent {
		err = r.db.QueryRow(ctx, strings.TrimSpace(query), args...).Scan(
			&item.ID, &item.ContentID, &item.ContentTitle, &item.ContentStatus, &item.EpisodeNo, &item.Title, &item.Summary,
			&item.AccessType, &item.PriceCents, &item.ResourceType, &item.BucketName, &item.ObjectKey,
			&item.Status, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
		)
	} else {
		err = r.db.QueryRow(ctx, strings.TrimSpace(query), args...).Scan(
			&item.ID, &item.ContentID, &item.EpisodeNo, &item.Title, &item.Summary, &item.AccessType, &item.PriceCents,
			&item.ResourceType, &item.BucketName, &item.ObjectKey, &item.Status, &item.CreatedBy,
			&item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrContentNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrContentConflict
		}
		return nil, fmt.Errorf("episode query: %w", err)
	}
	return &item, nil
}

// scanContent 将列表查询的一行转换为内容模型。
func scanContent(rows pgx.Rows) (*model.Content, error) {
	var item model.Content
	if err := rows.Scan(&item.ID, &item.Title, &item.ContentType, &item.Description, &item.CoverObjectKey, &item.PriceCents, &item.Status, &item.CreatedBy, &item.UpdatedBy, &item.PublishedBy, &item.PublishedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan content: %w", err)
	}
	return &item, nil
}

// scanEpisode 根据查询是否包含父内容字段完成 Episode 扫描。
func scanEpisode(rows pgx.Rows, withContent bool) (*model.Episode, error) {
	var item model.Episode
	var err error
	if withContent {
		err = rows.Scan(&item.ID, &item.ContentID, &item.ContentTitle, &item.ContentStatus, &item.EpisodeNo, &item.Title, &item.Summary, &item.AccessType, &item.PriceCents, &item.ResourceType, &item.BucketName, &item.ObjectKey, &item.Status, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	} else {
		err = rows.Scan(&item.ID, &item.ContentID, &item.EpisodeNo, &item.Title, &item.Summary, &item.AccessType, &item.PriceCents, &item.ResourceType, &item.BucketName, &item.ObjectKey, &item.Status, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("scan episode: %w", err)
	}
	return &item, nil
}
