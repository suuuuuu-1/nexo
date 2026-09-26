package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

// OrderTx 暴露订单用例需要的事务内数据操作，不暴露 pgx.Tx 给 Service。
type OrderTx interface {
	FindOrderByIdempotencyKey(ctx context.Context, userID, key string) (*model.Order, error)
	ResolveProduct(ctx context.Context, itemType model.ItemType, itemID string) (model.OrderProduct, error)
	HasActiveEntitlement(ctx context.Context, userID string, itemType model.ItemType, itemID string) (bool, error)
	InsertOrder(ctx context.Context, order *model.Order, idempotencyKey string) (bool, error)
	InsertOrderItem(ctx context.Context, orderID string, item model.OrderItem) error
	LockOrderForPayment(ctx context.Context, userID, orderID string) (*model.Order, error)
	LockOrderByNumber(ctx context.Context, orderNo string) (*model.Order, error)
	LockWalletForUpdate(ctx context.Context, userID string) (*model.Wallet, error)
	UpdateWalletBalance(ctx context.Context, userID string, balance, version int64) error
	InsertPurchaseLedger(ctx context.Context, userID, orderID string, amount, balanceAfter int64) error
	MarkOrderPaid(ctx context.Context, orderID, providerTxnID string) error
	InsertOrderPaidEvent(ctx context.Context, orderID string) error
}

// WalletTx 暴露钱包充值事务需要的数据操作。
type WalletTx interface {
	LockWalletForUpdate(ctx context.Context, userID string) (*model.Wallet, error)
	HasLedgerIdempotencyKey(ctx context.Context, key string) (bool, error)
	UpdateWalletBalance(ctx context.Context, userID string, balance, version int64) error
	InsertRechargeLedger(ctx context.Context, userID string, amount, balanceAfter int64, key string) error
}

// EntitlementTx 暴露订单履约事务需要的数据操作。
type EntitlementTx interface {
	LockOrderForFulfillment(ctx context.Context, orderID string) (*model.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID string, status model.OrderStatus) error
	GetMembershipPlanDuration(ctx context.Context, planID string) (int, error)
	CreateSubscription(ctx context.Context, userID, planID, orderID string, validity model.TimeRange) (string, error)
	CreateEntitlement(ctx context.Context, userID string, scopeType model.EntitlementScope, scopeID string, sourceType model.EntitlementSource, sourceID string, validity model.TimeRange) error
}

// OrderTransactionRunner 是订单用例所依赖的事务边界。
type OrderTransactionRunner interface {
	WithinOrderTx(ctx context.Context, fn func(OrderTx) error) error
}

// WalletTransactionRunner 是钱包用例所依赖的事务边界。
type WalletTransactionRunner interface {
	WithinWalletTx(ctx context.Context, fn func(WalletTx) error) error
}

// EntitlementTransactionRunner 是权益履约用例所依赖的事务边界。
type EntitlementTransactionRunner interface {
	WithinEntitlementTx(ctx context.Context, fn func(EntitlementTx) error) error
}

// TransactionManager 创建并提交各业务用例所需的 PostgreSQL 事务。
// Service 负责决定事务内的业务步骤，Repository 负责执行具体 SQL。
type TransactionManager struct{ db *pgxpool.Pool }

var (
	_ OrderTransactionRunner       = (*TransactionManager)(nil)
	_ WalletTransactionRunner      = (*TransactionManager)(nil)
	_ EntitlementTransactionRunner = (*TransactionManager)(nil)
)

func NewTransactionManager(db *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{db: db}
}

func (m *TransactionManager) WithinOrderTx(ctx context.Context, fn func(OrderTx) error) error {
	return m.within(ctx, func(tx pgx.Tx) error { return fn(&postgresTransaction{tx: tx}) })
}

func (m *TransactionManager) WithinWalletTx(ctx context.Context, fn func(WalletTx) error) error {
	return m.within(ctx, func(tx pgx.Tx) error { return fn(&postgresTransaction{tx: tx}) })
}

func (m *TransactionManager) WithinEntitlementTx(ctx context.Context, fn func(EntitlementTx) error) error {
	return m.within(ctx, func(tx pgx.Tx) error { return fn(&postgresTransaction{tx: tx}) })
}

func (m *TransactionManager) within(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type postgresTransaction struct{ tx pgx.Tx }
