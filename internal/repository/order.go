package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

var ErrOrderNotFound = errors.New("order not found")

type OrderRepository struct{ db *pgxpool.Pool }

// NewOrderRepository 创建订单数据访问对象。
func NewOrderRepository(db *pgxpool.Pool) *OrderRepository { return &OrderRepository{db: db} }

// ListPlans 查询当前有效的会员计划。
func (r *OrderRepository) ListPlans(ctx context.Context) ([]model.MembershipPlan, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, description, price_cents, duration_days, status, created_at FROM membership_plans WHERE status = 'ACTIVE' ORDER BY price_cents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]model.MembershipPlan, 0)
	for rows.Next() {
		var item model.MembershipPlan
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.PriceCents, &item.DurationDays, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, item)
	}
	return plans, rows.Err()
}

// GetOrder 只允许用户读取自己的订单。
func (r *OrderRepository) GetOrder(ctx context.Context, userID, orderID string) (*model.Order, error) {
	return r.queryOrder(ctx, `
		SELECT o.id, o.order_no, o.user_id, o.status, o.payment_method, o.provider_txn_id, o.total_amount, o.currency, o.expires_at, o.paid_at, o.created_at, o.updated_at,
		       oi.id, oi.item_type, oi.item_id, oi.item_name, oi.unit_price, oi.quantity
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.id = $1 AND o.user_id = $2
	`, orderID, userID)
}

// ListOrders 返回用户自己的订单列表。
func (r *OrderRepository) ListOrders(ctx context.Context, userID string, limit, offset int) ([]model.Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.order_no, o.user_id, o.status, o.payment_method, o.provider_txn_id, o.total_amount, o.currency, o.expires_at, o.paid_at, o.created_at, o.updated_at,
		       oi.id, oi.item_type, oi.item_id, oi.item_name, oi.unit_price, oi.quantity
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.user_id = $1 ORDER BY o.created_at DESC LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Order, 0)
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (r *OrderRepository) queryOrder(ctx context.Context, query string, args ...any) (*model.Order, error) {
	return scanOrderRow(r.db.QueryRow(ctx, strings.TrimSpace(query), args...))
}

func getOrderByIdempotencyTx(ctx context.Context, tx pgx.Tx, userID, key string) (*model.Order, error) {
	return queryOrderTx(ctx, tx, `
		SELECT o.id, o.order_no, o.user_id, o.status, o.payment_method, o.provider_txn_id, o.total_amount, o.currency, o.expires_at, o.paid_at, o.created_at, o.updated_at,
		       oi.id, oi.item_type, oi.item_id, oi.item_name, oi.unit_price, oi.quantity
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.user_id = $1 AND o.idempotency_key = $2
	`, userID, key)
}

func queryOrderTx(ctx context.Context, tx pgx.Tx, query string, args ...any) (*model.Order, error) {
	return scanOrderRow(tx.QueryRow(ctx, strings.TrimSpace(query), args...))
}

type rowScanner interface{ Scan(dest ...any) error }

func scanOrderRow(row rowScanner) (*model.Order, error) {
	item := &model.Order{}
	err := row.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.Status, &item.PaymentMethod, &item.ProviderTxnID, &item.TotalAmount, &item.Currency, &item.ExpiresAt, &item.PaidAt, &item.CreatedAt, &item.UpdatedAt, &item.Item.ID, &item.Item.ItemType, &item.Item.ItemID, &item.Item.ItemName, &item.Item.UnitPrice, &item.Item.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func scanOrder(rows pgx.Rows) (*model.Order, error) {
	item := &model.Order{}
	if err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.Status, &item.PaymentMethod, &item.ProviderTxnID, &item.TotalAmount, &item.Currency, &item.ExpiresAt, &item.PaidAt, &item.CreatedAt, &item.UpdatedAt, &item.Item.ID, &item.Item.ItemType, &item.Item.ItemID, &item.Item.ItemName, &item.Item.UnitPrice, &item.Item.Quantity); err != nil {
		return nil, err
	}
	return item, nil
}
