package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suuuuu/nexo/internal/model"
)

type WalletRepository struct{ db *pgxpool.Pool }

// NewWalletRepository 创建钱包数据访问对象。
func NewWalletRepository(db *pgxpool.Pool) *WalletRepository { return &WalletRepository{db: db} }

// Get 获取钱包；首次访问用户时延迟创建钱包记录。
func (r *WalletRepository) Get(ctx context.Context, userID string) (*model.Wallet, error) {
	if _, err := r.db.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return nil, err
	}
	item := &model.Wallet{}
	err := r.db.QueryRow(ctx, `SELECT user_id, balance, version, updated_at FROM wallets WHERE user_id = $1`, userID).Scan(&item.UserID, &item.Balance, &item.Version, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}
