package model

import "time"

type EntitlementScope string

const (
	ScopeEpisode    EntitlementScope = "EPISODE"
	ScopeContent    EntitlementScope = "CONTENT"
	ScopeMembership EntitlementScope = "MEMBERSHIP"
)

type EntitlementSource string

const (
	SourceOrder        EntitlementSource = "ORDER"
	SourceSubscription EntitlementSource = "SUBSCRIPTION"
)

// TimeRange 描述权益或订阅的有效期；EndsAt 为空表示没有固定到期时间。
type TimeRange struct {
	StartsAt *time.Time
	EndsAt   *time.Time
}

type AccessResult struct {
	// Reason 用于向前端说明是免费访问、会员权益还是购买权益。
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
	SourceID string `json:"source_id,omitempty"`
}
