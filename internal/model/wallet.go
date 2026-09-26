package model

import "time"

type Wallet struct {
	// Balance 使用最小货币单位整数保存，避免浮点数精度问题。
	UserID    string    `json:"user_id"`
	Balance   int64     `json:"balance"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}
