package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/suuuuu/nexo/internal/model"
)

func (t *postgresTransaction) LockOrderForFulfillment(ctx context.Context, orderID string) (*model.Order, error) {
	var item model.Order
	err := t.tx.QueryRow(ctx, `
		SELECT o.id, o.user_id, o.status, oi.item_type, oi.item_id
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.id = $1 FOR UPDATE OF o
	`, orderID).Scan(&item.ID, &item.UserID, &item.Status, &item.Item.ItemType, &item.Item.ItemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	return &item, err
}

func (t *postgresTransaction) UpdateOrderStatus(ctx context.Context, orderID string, status model.OrderStatus) error {
	_, err := t.tx.Exec(ctx, `UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, orderID, status)
	return err
}

func (t *postgresTransaction) GetMembershipPlanDuration(ctx context.Context, planID string) (int, error) {
	var durationDays int
	err := t.tx.QueryRow(ctx, `
		SELECT duration_days FROM membership_plans WHERE id = $1 AND status = 'ACTIVE'
	`, planID).Scan(&durationDays)
	return durationDays, err
}

func (t *postgresTransaction) CreateSubscription(ctx context.Context, userID, planID, orderID string, validity model.TimeRange) (string, error) {
	var subscriptionID string
	err := t.tx.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, order_id, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (order_id) DO NOTHING
		RETURNING id
	`, userID, planID, orderID, validity.StartsAt, validity.EndsAt).Scan(&subscriptionID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = t.tx.QueryRow(ctx, `SELECT id FROM subscriptions WHERE order_id = $1`, orderID).Scan(&subscriptionID)
	}
	return subscriptionID, err
}

func (t *postgresTransaction) CreateEntitlement(ctx context.Context, userID string, scopeType model.EntitlementScope, scopeID string, sourceType model.EntitlementSource, sourceID string, validity model.TimeRange) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO entitlements (user_id, scope_type, scope_id, source_type, source_id, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamptz, NOW()), $7)
		ON CONFLICT DO NOTHING
	`, userID, scopeType, scopeID, sourceType, sourceID, validity.StartsAt, validity.EndsAt)
	return err
}
