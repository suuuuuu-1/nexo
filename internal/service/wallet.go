package service

import (
	"context"
	"errors"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var (
	ErrInvalidWalletAmount    = errors.New("invalid wallet amount")
	errRechargeAlreadyApplied = errors.New("wallet recharge already applied")
)

type WalletService struct {
	repo *repository.WalletRepository
	tx   repository.TransactionRunner
}

// NewWalletService 创建钱包业务服务。
func NewWalletService(repo *repository.WalletRepository, tx repository.TransactionRunner) *WalletService {
	return &WalletService{repo: repo, tx: tx}
}

// Get 返回当前用户钱包。
func (s *WalletService) Get(ctx context.Context, userID string) (*model.Wallet, error) {
	return s.repo.Get(ctx, userID)
}

// Recharge 校验充值请求，并在锁定的钱包行上幂等地更新余额和流水。
func (s *WalletService) Recharge(ctx context.Context, userID string, amount int64, idempotencyKey string) (*model.Wallet, error) {
	if amount <= 0 || idempotencyKey == "" {
		return nil, ErrInvalidWalletAmount
	}
	err := s.tx.WithinTx(ctx, func(tx repository.Transaction) error {
		wallet, err := tx.LockWalletForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		alreadyApplied, err := tx.HasLedgerIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return err
		}
		if alreadyApplied {
			return errRechargeAlreadyApplied
		}
		newBalance := wallet.Balance + amount
		if err := tx.UpdateWalletBalance(ctx, userID, newBalance, wallet.Version+1); err != nil {
			return err
		}
		return tx.InsertRechargeLedger(ctx, userID, amount, newBalance, idempotencyKey)
	})
	if errors.Is(err, errRechargeAlreadyApplied) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, userID)
}
