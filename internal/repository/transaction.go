package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

// Transaction 汇总业务事务内需要的数据操作，避免订单、钱包和权益各自重复定义事务接口。
// v1 使用 PostgreSQL 实现；Service 单元测试可用内存实现替代这些操作。
type Transaction interface {
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
	HasLedgerIdempotencyKey(ctx context.Context, key string) (bool, error)
	InsertRechargeLedger(ctx context.Context, userID string, amount, balanceAfter int64, key string) error
	MarkOrderPaid(ctx context.Context, orderID, providerTxnID string) error
	InsertOrderPaidEvent(ctx context.Context, orderID string) error
	LockOrderForFulfillment(ctx context.Context, orderID string) (*model.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID string, status model.OrderStatus) error
	GetMembershipPlanDuration(ctx context.Context, planID string) (int, error)
	CreateSubscription(ctx context.Context, userID, planID, orderID string, validity model.TimeRange) (string, error)
	CreateEntitlement(ctx context.Context, userID string, scopeType model.EntitlementScope, scopeID string, sourceType model.EntitlementSource, sourceID string, validity model.TimeRange) error
}

// TransactionRunner 在一个数据库事务中运行回调：回调返回 nil 时提交，返回错误时回滚。
type TransactionRunner interface {
	WithinTx(ctx context.Context, fn func(Transaction) error) error
}

// TransactionManager 创建并提交 PostgreSQL 事务。
// Service 负责决定事务内的业务步骤，Repository 负责执行具体 SQL。
type TransactionManager struct{ db *pgxpool.Pool }

var _ TransactionRunner = (*TransactionManager)(nil)

func NewTransactionManager(db *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{db: db}
}

func (m *TransactionManager) WithinTx(ctx context.Context, fn func(Transaction) error) error {
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
