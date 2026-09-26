package model

import "time"

type Role string

const (
	// RoleUser 只能访问用户侧功能。
	RoleUser Role = "USER"
	// RoleOperator 可以管理内容，但不能管理系统用户和角色。
	RoleOperator Role = "OPERATOR"
	// RoleAdmin 拥有运营能力，并可管理用户角色和状态。
	RoleAdmin Role = "ADMIN"
)

type UserStatus string

const (
	// StatusActive 表示用户可以登录并访问接口。
	StatusActive UserStatus = "ACTIVE"
	// StatusDisabled 表示用户被禁止继续使用系统。
	StatusDisabled UserStatus = "DISABLED"
)

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Nickname     string     `json:"nickname"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	Role         Role       `json:"role"`
	Status       UserStatus `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PublicUser 是可以返回给前端的用户信息，刻意不包含密码哈希。
type PublicUser struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Nickname  string     `json:"nickname"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	Role      Role       `json:"role"`
	Status    UserStatus `json:"status"`
}

// Public 将内部用户模型转换为安全的公开模型。
func (u *User) Public() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		Role:      u.Role,
		Status:    u.Status,
	}
}
