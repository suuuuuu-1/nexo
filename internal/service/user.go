package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/suuuuu/nexo/internal/auth"
	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var (
	ErrInvalidUserInput   = errors.New("invalid user input")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrInvalidRole        = errors.New("invalid role")
	ErrInvalidUserStatus  = errors.New("invalid user status")
	ErrUserNotFound       = repository.ErrUserNotFound
	ErrUserEmailExists    = repository.ErrUserEmailExists
)

type UserService struct {
	repo      *repository.UserRepository
	jwtSecret string
	tokenTTL  time.Duration
}

// NewUserService 创建用户业务服务，并注入 JWT 签发所需配置。
func NewUserService(repo *repository.UserRepository, jwtSecret string, tokenTTL time.Duration) *UserService {
	return &UserService{repo: repo, jwtSecret: jwtSecret, tokenTTL: tokenTTL}
}

type RegisterInput struct {
	Email    string
	Password string
	Nickname string
}

type LoginInput struct {
	Email    string
	Password string
}

type UpdateProfileInput struct {
	Nickname  string
	AvatarURL *string
}

type UserAuthResult struct {
	Token string           `json:"token"`
	User  model.PublicUser `json:"user"`
}

// Register 校验注册输入、哈希密码、创建 USER 并立即签发登录 Token。
func (s *UserService) Register(ctx context.Context, input RegisterInput) (*UserAuthResult, error) {
	email := normalizeEmail(input.Email)
	if !validEmail(email) || len(input.Password) < 8 {
		return nil, ErrInvalidUserInput
	}
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = strings.Split(email, "@")[0]
	}
	if len([]rune(nickname)) > 64 {
		return nil, ErrInvalidUserInput
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	created, err := s.repo.Create(ctx, email, passwordHash, nickname)
	if err != nil {
		return nil, err
	}
	return s.issueToken(created)
}

// BootstrapAdmin 用于部署初始化管理员，不开放给普通 HTTP 注册接口。
func (s *UserService) BootstrapAdmin(ctx context.Context, email, password, nickname string) (*model.User, error) {
	email = normalizeEmail(email)
	nickname = strings.TrimSpace(nickname)
	if !validEmail(email) || len(password) < 8 || nickname == "" || len([]rune(nickname)) > 64 {
		return nil, ErrInvalidUserInput
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return s.repo.CreateWithRole(ctx, email, passwordHash, nickname, model.RoleAdmin)
}

// Login 验证密码和用户状态，成功后更新登录时间并签发 Token。
func (s *UserService) Login(ctx context.Context, input LoginInput) (*UserAuthResult, error) {
	email := normalizeEmail(input.Email)
	found, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if auth.ComparePassword(found.PasswordHash, input.Password) != nil {
		return nil, ErrInvalidCredentials
	}
	if found.Status != model.StatusActive {
		return nil, ErrUserDisabled
	}
	if err := s.repo.UpdateLastLogin(ctx, found.ID); err != nil {
		return nil, fmt.Errorf("update last login: %w", err)
	}
	now := time.Now()
	found.LastLoginAt = &now
	return s.issueToken(found)
}

// GetByID 根据用户 ID 获取当前用户，用于鉴权中间件和个人中心。
func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.repo.FindByID(ctx, id)
}

// UpdateProfile 更新用户资料，不允许通过该接口修改角色和状态。
func (s *UserService) UpdateProfile(ctx context.Context, id string, input UpdateProfileInput) (*model.User, error) {
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" || len([]rune(nickname)) > 64 {
		return nil, ErrInvalidUserInput
	}
	return s.repo.UpdateProfile(ctx, id, nickname, input.AvatarURL)
}

// List 返回管理端用户列表，并在 Service 层统一限制分页参数。
func (s *UserService) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}

// UpdateRole 修改角色前先进行白名单校验。
func (s *UserService) UpdateRole(ctx context.Context, id string, role model.Role) (*model.User, error) {
	if role != model.RoleUser && role != model.RoleOperator && role != model.RoleAdmin {
		return nil, ErrInvalidRole
	}
	return s.repo.UpdateRole(ctx, id, role)
}

// UpdateStatus 修改用户状态前先进行白名单校验。
func (s *UserService) UpdateStatus(ctx context.Context, id string, status model.UserStatus) (*model.User, error) {
	if status != model.StatusActive && status != model.StatusDisabled {
		return nil, ErrInvalidUserStatus
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

// ParseToken 将 JWT 校验逻辑封装在用户服务中，供 HTTP 中间件调用。
func (s *UserService) ParseToken(token string) (*auth.Claims, error) {
	return auth.ParseToken(token, s.jwtSecret)
}

// issueToken 将内部 User 转换为公开用户信息，并生成登录响应。
func (s *UserService) issueToken(user *model.User) (*UserAuthResult, error) {
	token, err := auth.NewToken(user.ID, string(user.Role), s.jwtSecret, s.tokenTTL)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return &UserAuthResult{Token: token, User: user.Public()}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && strings.Contains(email, "@")
}
