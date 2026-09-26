package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/suuuuu/nexo/internal/model"
)

func (t *postgresTransaction) FindOrderByIdempotencyKey(ctx context.Context, userID, key string) (*model.Order, error) {
	return getOrderByIdempotencyTx(ctx, t.tx, userID, key)
}

func (t *postgresTransaction) ResolveProduct(ctx context.Context, itemType model.ItemType, itemID string) (model.OrderProduct, error) {
	var product model.OrderProduct
	var err error
	switch itemType {
	case model.ItemEpisode:
		err = t.tx.QueryRow(ctx, `
			SELECT e.title, e.price_cents, e.access_type
			FROM episodes e JOIN contents c ON c.id = e.content_id
			WHERE e.id = $1 AND e.status = 'PUBLISHED' AND c.status = 'PUBLISHED'
		`, itemID).Scan(&product.Name, &product.PriceCents, &product.AccessType)
	case model.ItemContent:
		err = t.tx.QueryRow(ctx, `SELECT title, price_cents FROM contents WHERE id = $1 AND status = 'PUBLISHED'`, itemID).Scan(&product.Name, &product.PriceCents)
	case model.ItemMembership:
		err = t.tx.QueryRow(ctx, `SELECT name, price_cents FROM membership_plans WHERE id = $1 AND status = 'ACTIVE'`, itemID).Scan(&product.Name, &product.PriceCents)
	default:
		return model.OrderProduct{}, fmt.Errorf("unsupported item type %q", itemType)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.OrderProduct{}, ErrOrderNotFound
	}
	if err != nil {
		return model.OrderProduct{}, err
	}
	return product, nil
}

func (t *postgresTransaction) HasActiveEntitlement(ctx context.Context, userID string, itemType model.ItemType, itemID string) (bool, error) {
	var exists bool
	switch itemType {
	case model.ItemEpisode:
		err := t.tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM entitlements e
				WHERE e.user_id = $1 AND e.status = 'ACTIVE'
				  AND e.starts_at <= NOW() AND (e.ends_at IS NULL OR e.ends_at > NOW())
				  AND (
					(e.scope_type = 'EPISODE' AND e.scope_id = $2)
					OR (e.scope_type = 'CONTENT' AND e.scope_id = (SELECT content_id FROM episodes WHERE id = $2))
				  )
			)
		`, userID, itemID).Scan(&exists)
		return exists, err
	case model.ItemContent:
		err := t.tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM entitlements
				WHERE user_id = $1 AND scope_type = 'CONTENT' AND scope_id = $2
				  AND status = 'ACTIVE' AND starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())
			)
		`, userID, itemID).Scan(&exists)
		return exists, err
	case model.ItemMembership:
		err := t.tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM entitlements
				WHERE user_id = $1 AND scope_type = 'MEMBERSHIP'
				  AND status = 'ACTIVE' AND starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())
			)
		`, userID).Scan(&exists)
		return exists, err
	default:
		return false, fmt.Errorf("unsupported item type %q", itemType)
	}
}

func (t *postgresTransaction) InsertOrder(ctx context.Context, order *model.Order, idempotencyKey string) (bool, error) {
	var key any
	if idempotencyKey != "" {
		key = idempotencyKey
	}
	var insertedID string
	err := t.tx.QueryRow(ctx, `
		INSERT INTO orders (id, order_no, user_id, status, payment_method, total_amount, idempotency_key, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING id
	`, order.ID, order.OrderNo, order.UserID, order.Status, order.PaymentMethod, order.TotalAmount, key, order.ExpiresAt).Scan(&insertedID)
	if errors.Is(err, pgx.ErrNoRows) && idempotencyKey != "" {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert order: %w", err)
	}
	return true, nil
}

func (t *postgresTransaction) InsertOrderItem(ctx context.Context, orderID string, item model.OrderItem) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO order_items (order_id, item_type, item_id, item_name, unit_price)
		VALUES ($1, $2, $3, $4, $5)
	`, orderID, item.ItemType, item.ItemID, item.ItemName, item.UnitPrice)
	if err != nil {
		return fmt.Errorf("insert order item: %w", err)
	}
	return nil
}

func (t *postgresTransaction) LockOrderForPayment(ctx context.Context, userID, orderID string) (*model.Order, error) {
	var item model.Order
	err := t.tx.QueryRow(ctx, `
		SELECT id, user_id, status, total_amount, payment_method
		FROM orders WHERE id = $1 AND user_id = $2 FOR UPDATE
	`, orderID, userID).Scan(&item.ID, &item.UserID, &item.Status, &item.TotalAmount, &item.PaymentMethod)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	return &item, err
}

func (t *postgresTransaction) LockOrderByNumber(ctx context.Context, orderNo string) (*model.Order, error) {
	var item model.Order
	err := t.tx.QueryRow(ctx, `
		SELECT id, order_no, user_id, status
		FROM orders WHERE order_no = $1 FOR UPDATE
	`, orderNo).Scan(&item.ID, &item.OrderNo, &item.UserID, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	return &item, err
}

func (t *postgresTransaction) InsertPurchaseLedger(ctx context.Context, userID, orderID string, amount, balanceAfter int64) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO wallet_ledger (user_id, order_id, transaction_type, amount, balance_after, idempotency_key)
		VALUES ($1, $2, 'PURCHASE', $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, userID, orderID, amount, balanceAfter, "purchase:"+orderID)
	return err
}

func (t *postgresTransaction) MarkOrderPaid(ctx context.Context, orderID, providerTxnID string) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE orders SET status = 'PAID', provider_txn_id = $2, paid_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, orderID, providerTxnID)
	return err
}

func (t *postgresTransaction) InsertOrderPaidEvent(ctx context.Context, orderID string) error {
	payload, err := json.Marshal(map[string]string{"order_id": orderID})
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(ctx, `
		INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload)
		VALUES ('order.paid', 'ORDER', $1, $2)
	`, orderID, payload)
	return err
}
