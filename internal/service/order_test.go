package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

type memoryOrderRunner struct {
	repository.Transaction
	mu      sync.Mutex
	orders  map[string]*model.Order
	balance int64
	ledger  int
	events  int
}

func (r *memoryOrderRunner) WithinTx(_ context.Context, fn func(repository.Transaction) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fn(r)
}

func (r *memoryOrderRunner) FindOrderByIdempotencyKey(context.Context, string, string) (*model.Order, error) {
	return nil, ErrOrderNotFound
}

func (r *memoryOrderRunner) ResolveProduct(context.Context, model.ItemType, string) (model.OrderProduct, error) {
	return model.OrderProduct{}, errors.New("unexpected ResolveProduct call")
}

func (r *memoryOrderRunner) HasActiveEntitlement(context.Context, string, model.ItemType, string) (bool, error) {
	return false, nil
}

func (r *memoryOrderRunner) InsertOrder(context.Context, *model.Order, string) (bool, error) {
	return false, errors.New("unexpected InsertOrder call")
}

func (r *memoryOrderRunner) InsertOrderItem(context.Context, string, model.OrderItem) error {
	return errors.New("unexpected InsertOrderItem call")
}

func (r *memoryOrderRunner) LockOrderForPayment(_ context.Context, userID, orderID string) (*model.Order, error) {
	order, ok := r.orders[orderID]
	if !ok || order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	copy := *order
	return &copy, nil
}

func (r *memoryOrderRunner) LockOrderByNumber(context.Context, string) (*model.Order, error) {
	return nil, errors.New("unexpected LockOrderByNumber call")
}

func (r *memoryOrderRunner) LockWalletForUpdate(_ context.Context, userID string) (*model.Wallet, error) {
	return &model.Wallet{UserID: userID, Balance: r.balance}, nil
}

func (r *memoryOrderRunner) UpdateWalletBalance(_ context.Context, _ string, balance, _ int64) error {
	r.balance = balance
	return nil
}

func (r *memoryOrderRunner) InsertPurchaseLedger(context.Context, string, string, int64, int64) error {
	r.ledger++
	return nil
}

func (r *memoryOrderRunner) MarkOrderPaid(_ context.Context, orderID, providerTxnID string) error {
	order := r.orders[orderID]
	order.Status = model.StatusPaid
	order.ProviderTxnID = &providerTxnID
	return nil
}

func (r *memoryOrderRunner) InsertOrderPaidEvent(context.Context, string) error {
	r.events++
	return nil
}

func (r *memoryOrderRunner) GetOrder(_ context.Context, userID, orderID string) (*model.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok || order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	copy := *order
	return &copy, nil
}

func (*memoryOrderRunner) ListPlans(context.Context) ([]model.MembershipPlan, error) {
	return nil, nil
}

func (*memoryOrderRunner) ListOrders(context.Context, string, int, int) ([]model.Order, error) {
	return nil, nil
}

func TestConcurrentWalletPaymentsCannotOverspend(t *testing.T) {
	runner := &memoryOrderRunner{
		balance: 100,
		orders: map[string]*model.Order{
			"order-1": {ID: "order-1", UserID: "user-1", Status: model.StatusPendingPayment, PaymentMethod: model.PaymentWallet, TotalAmount: 80},
			"order-2": {ID: "order-2", UserID: "user-1", Status: model.StatusPendingPayment, PaymentMethod: model.PaymentWallet, TotalAmount: 80},
		},
	}
	service := NewOrderService(runner, runner)

	var wait sync.WaitGroup
	errs := make(chan error, len(runner.orders))
	for orderID := range runner.orders {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			_, err := service.PayWithWallet(context.Background(), "user-1", id)
			errs <- err
		}(orderID)
	}
	wait.Wait()
	close(errs)

	succeeded, insufficient := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrInsufficientFunds):
			insufficient++
		default:
			t.Fatalf("PayWithWallet() error = %v", err)
		}
	}
	if succeeded != 1 || insufficient != 1 {
		t.Fatalf("payment results: success=%d insufficient=%d, want 1 each", succeeded, insufficient)
	}
	if runner.balance != 20 {
		t.Fatalf("wallet balance = %d, want 20", runner.balance)
	}
	if runner.ledger != 1 || runner.events != 1 {
		t.Fatalf("ledger/events = %d/%d, want 1/1", runner.ledger, runner.events)
	}
}
