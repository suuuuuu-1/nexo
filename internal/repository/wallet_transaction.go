package repository

import (
	"context"

	"github.com/suuuuu/nexo/internal/model"
)

func (t *postgresTransaction) LockWalletForUpdate(ctx context.Context, userID string) (*model.Wallet, error) {
	if _, err := t.tx.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return nil, err
	}
	var item model.Wallet
	err := t.tx.QueryRow(ctx, `
		SELECT user_id, balance, version, updated_at
		FROM wallets WHERE user_id = $1 FOR UPDATE
	`, userID).Scan(&item.UserID, &item.Balance, &item.Version, &item.UpdatedAt)
	return &item, err
}

func (t *postgresTransaction) HasLedgerIdempotencyKey(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := t.tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM wallet_ledger WHERE idempotency_key = $1)`, key).Scan(&exists)
	return exists, err
}

func (t *postgresTransaction) UpdateWalletBalance(ctx context.Context, userID string, balance, version int64) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE wallets SET balance = $2, version = $3, updated_at = NOW()
		WHERE user_id = $1
	`, userID, balance, version)
	return err
}

func (t *postgresTransaction) InsertRechargeLedger(ctx context.Context, userID string, amount, balanceAfter int64, key string) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO wallet_ledger (user_id, transaction_type, amount, balance_after, idempotency_key)
		VALUES ($1, 'RECHARGE', $2, $3, $4)
	`, userID, amount, balanceAfter, key)
	return err
}
