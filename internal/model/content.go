package model

import "time"

type ContentStatus string

const (
	// StatusDraft 表示内容还未对用户公开。
	StatusDraft ContentStatus = "DRAFT"
	// StatusPublished 表示内容可以出现在用户侧并参与访问校验。
	StatusPublished ContentStatus = "PUBLISHED"
	// StatusOffline 表示内容曾经发布过，但当前不可访问。
	StatusOffline ContentStatus = "OFFLINE"
)

type AccessType string

const (
	// AccessFree 不需要购买或会员权益即可访问。
	AccessFree AccessType = "FREE"
	// AccessPurchase 需要单集或整部购买权益。
	AccessPurchase AccessType = "PURCHASE"
	// AccessMembership 需要有效会员权益。
	AccessMembership AccessType = "MEMBERSHIP"
	// AccessPurchaseOrMembership 购买权益和会员权益任一满足即可访问。
	AccessPurchaseOrMembership AccessType = "PURCHASE_OR_MEMBERSHIP"
)

type Content struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	ContentType    string        `json:"content_type"`
	Description    string        `json:"description"`
	CoverObjectKey *string       `json:"cover_object_key,omitempty"`
	PriceCents     int64         `json:"price_cents"`
	Status         ContentStatus `json:"status"`
	CreatedBy      string        `json:"created_by"`
	UpdatedBy      string        `json:"updated_by"`
	PublishedBy    *string       `json:"published_by,omitempty"`
	PublishedAt    *time.Time    `json:"published_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	Episodes       []Episode     `json:"episodes,omitempty"`
}

// Episode 是内容的可访问单元，资源本身通过对象存储 key 关联。
type Episode struct {
	ID            string        `json:"id"`
	ContentID     string        `json:"content_id"`
	ContentTitle  string        `json:"content_title,omitempty"`
	ContentStatus ContentStatus `json:"content_status,omitempty"`
	EpisodeNo     int           `json:"episode_no"`
	Title         string        `json:"title"`
	Summary       string        `json:"summary"`
	AccessType    AccessType    `json:"access_type"`
	PriceCents    int64         `json:"price_cents"`
	ResourceType  string        `json:"resource_type"`
	BucketName    *string       `json:"bucket_name,omitempty"`
	ObjectKey     *string       `json:"object_key,omitempty"`
	Status        ContentStatus `json:"status"`
	CreatedBy     string        `json:"created_by"`
	UpdatedBy     string        `json:"updated_by"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// ContentInput 是内容创建和更新时使用的字段集合，不包含由服务端维护的状态字段。
type ContentInput struct {
	Title          string
	ContentType    string
	Description    string
	CoverObjectKey *string
	PriceCents     int64
}

// EpisodeInput 是 Episode 创建和更新时使用的字段集合。
type EpisodeInput struct {
	EpisodeNo    int
	Title        string
	Summary      string
	AccessType   AccessType
	PriceCents   int64
	ResourceType string
	BucketName   *string
	ObjectKey    *string
}
