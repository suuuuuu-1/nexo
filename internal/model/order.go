package model

import "time"

type OrderStatus string

const (
	// 订单状态按“待支付 → 已支付 → 履约中 → 已完成”推进。
	StatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	StatusPaymentFailed  OrderStatus = "PAYMENT_FAILED"
	StatusPaid           OrderStatus = "PAID"
	StatusFulfilling     OrderStatus = "FULFILLING"
	StatusCompleted      OrderStatus = "COMPLETED"
	StatusCancelled      OrderStatus = "CANCELLED"
	StatusExpired        OrderStatus = "EXPIRED"
)

type PaymentMethod string

const (
	// PaymentWallet 使用站内虚拟币余额支付。
	PaymentWallet PaymentMethod = "WALLET"
	// PaymentMock 用于模拟第三方支付回调。
	PaymentMock PaymentMethod = "MOCK"
)

type ItemType string

const (
	// ItemEpisode 表示购买单集权益。
	ItemEpisode ItemType = "EPISODE"
	// ItemContent 表示购买整部内容权益。
	ItemContent ItemType = "CONTENT"
	// ItemMembership 表示购买会员订阅。
	ItemMembership ItemType = "MEMBERSHIP"
)

type MembershipPlan struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	PriceCents   int64     `json:"price_cents"`
	DurationDays int       `json:"duration_days"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// OrderProduct 是创建订单时读取的商品快照，不接收客户端提交的价格。
type OrderProduct struct {
	Name       string
	PriceCents int64
	AccessType AccessType
}

type OrderItem struct {
	ID        string   `json:"id"`
	ItemType  ItemType `json:"item_type"`
	ItemID    string   `json:"item_id"`
	ItemName  string   `json:"item_name"`
	UnitPrice int64    `json:"unit_price"`
	Quantity  int      `json:"quantity"`
}

type Order struct {
	ID            string        `json:"id"`
	OrderNo       string        `json:"order_no"`
	UserID        string        `json:"user_id"`
	Status        OrderStatus   `json:"status"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	ProviderTxnID *string       `json:"provider_txn_id,omitempty"`
	TotalAmount   int64         `json:"total_amount"`
	Currency      string        `json:"currency"`
	ExpiresAt     *time.Time    `json:"expires_at,omitempty"`
	PaidAt        *time.Time    `json:"paid_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Item          OrderItem     `json:"item"`
}

type OrderCreateInput struct {
	// IdempotencyKey 用于客户端重试时复用同一订单结果。
	ItemType       ItemType
	ItemID         string
	PaymentMethod  PaymentMethod
	IdempotencyKey string
}
