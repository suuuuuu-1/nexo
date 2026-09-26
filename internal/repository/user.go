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

var ErrUserNotFound = errors.New("user not found")
var ErrUserEmailExists = errors.New("email already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository 创建用户数据访问对象。
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create 使用默认 USER 角色创建公开注册用户。
func (r *UserRepository) Create(ctx context.Context, email, passwordHash, nickname string) (*model.User, error) {
	return r.CreateWithRole(ctx, email, passwordHash, nickname, model.RoleUser)
}

// CreateWithRole 供管理员初始化等受控场景创建指定角色的用户。
func (r *UserRepository) CreateWithRole(ctx context.Context, email, passwordHash, nickname string, role model.Role) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, nickname, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
	`, email, passwordHash, nickname, role).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL,
		&user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUserEmailExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// FindByEmail 用于登录时查找用户和密码哈希。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.findOne(ctx, `
		SELECT id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
		FROM users WHERE email = $1
	`, email)
}

// FindByID 用于鉴权后重新确认用户当前状态和角色。
func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	return r.findOne(ctx, `
		SELECT id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
		FROM users WHERE id = $1
	`, id)
}

// UpdateLastLogin 记录最近一次成功登录时间。
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

// UpdateProfile 更新用户可编辑的资料字段。
func (r *UserRepository) UpdateProfile(ctx context.Context, id, nickname string, avatarURL *string) (*model.User, error) {
	return r.findOne(ctx, `
		UPDATE users
		SET nickname = $2, avatar_url = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
	`, id, nickname, avatarURL)
}

// List 分页查询用户，管理端使用。
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		var item model.User
		if err := rows.Scan(&item.ID, &item.Email, &item.PasswordHash, &item.Nickname, &item.AvatarURL, &item.Role, &item.Status, &item.LastLoginAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// UpdateRole 修改用户角色，具体角色合法性由 Service 层校验。
func (r *UserRepository) UpdateRole(ctx context.Context, id string, role model.Role) (*model.User, error) {
	return r.findOne(ctx, `
		UPDATE users
		SET role = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
	`, id, role)
}

// UpdateStatus 修改用户启用状态。
func (r *UserRepository) UpdateStatus(ctx context.Context, id string, status model.UserStatus) (*model.User, error) {
	return r.findOne(ctx, `
		UPDATE users
		SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, password_hash, nickname, avatar_url, role, status, last_login_at, created_at, updated_at
	`, id, status)
}

// findOne 统一处理用户查询、字段扫描和“未找到”错误转换。
func (r *UserRepository) findOne(ctx context.Context, query string, args ...any) (*model.User, error) {
	var user model.User
	err := r.db.QueryRow(ctx, strings.TrimSpace(query), args...).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL,
		&user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &user, nil
}
