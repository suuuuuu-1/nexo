package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var (
	ErrOrderNotFound     = repository.ErrOrderNotFound
	ErrOrderConflict     = errors.New("order conflict")
	ErrInvalidOrderInput = errors.New("invalid order input")
	ErrInvalidOrderState = errors.New("invalid order state")
	ErrInsufficientFunds = errors.New("insufficient wallet balance")
	ErrAlreadyOwned      = errors.New("user already owns this entitlement")
)

// OrderService 编排订单用例；事务入口负责原子提交，Repository 提供普通查询。
type OrderService struct {
	repo orderReader
	tx   repository.TransactionRunner
}

// orderReader 限定 Service 需要的普通查询，事务写操作仍通过 TransactionRunner 执行。
type orderReader interface {
	ListPlans(ctx context.Context) ([]model.MembershipPlan, error)
	GetOrder(ctx context.Context, userID, orderID string) (*model.Order, error)
	ListOrders(ctx context.Context, userID string, limit, offset int) ([]model.Order, error)
}

// NewOrderService 创建订单业务服务。
func NewOrderService(repo orderReader, tx repository.TransactionRunner) *OrderService {
	return &OrderService{repo: repo, tx: tx}
}

// ListPlans 查询当前可购买的会员计划。
func (s *OrderService) ListPlans(ctx context.Context) ([]model.MembershipPlan, error) {
	return s.repo.ListPlans(ctx)
}

// CreateOrder 校验购买规则并在事务中创建订单和订单明细。
func (s *OrderService) CreateOrder(ctx context.Context, userID string, input model.OrderCreateInput) (*model.Order, error) {
	if input.ItemID == "" || !validItemType(input.ItemType) {
		return nil, ErrInvalidOrderInput
	}
	if input.PaymentMethod == "" {
		input.PaymentMethod = model.PaymentWallet
	}
	if input.PaymentMethod != model.PaymentWallet && input.PaymentMethod != model.PaymentMock {
		return nil, ErrInvalidOrderInput
	}

	var existing *model.Order
	var createdID string
	err := s.tx.WithinTx(ctx, func(tx repository.Transaction) error {
		if input.IdempotencyKey != "" {
			found, err := tx.FindOrderByIdempotencyKey(ctx, userID, input.IdempotencyKey)
			if err == nil {
				existing = found
				return nil
			}
			if !errors.Is(err, ErrOrderNotFound) {
				return err
			}
		}

		product, err := tx.ResolveProduct(ctx, input.ItemType, input.ItemID)
		if err != nil {
			return err
		}
		if input.ItemType == model.ItemEpisode && (product.AccessType == model.AccessFree || product.AccessType == model.AccessMembership) {
			return ErrInvalidOrderInput
		}
		if product.PriceCents <= 0 {
			return ErrInvalidOrderInput
		}

		owned, err := tx.HasActiveEntitlement(ctx, userID, input.ItemType, input.ItemID)
		if err != nil {
			return err
		}
		if owned {
			return ErrAlreadyOwned
		}

		now := time.Now()
		expiresAt := now.Add(30 * time.Minute)
		order := &model.Order{
			ID:            uuid.NewString(),
			OrderNo:       "NEXO-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:16],
			UserID:        userID,
			Status:        model.StatusPendingPayment,
			PaymentMethod: input.PaymentMethod,
			TotalAmount:   product.PriceCents,
			ExpiresAt:     &expiresAt,
		}
		inserted, err := tx.InsertOrder(ctx, order, input.IdempotencyKey)
		if err != nil {
			return err
		}
		if !inserted {
			if input.IdempotencyKey == "" {
				return ErrOrderConflict
			}
			existing, err = tx.FindOrderByIdempotencyKey(ctx, userID, input.IdempotencyKey)
			return err
		}
		if err := tx.InsertOrderItem(ctx, order.ID, model.OrderItem{
			ItemType:  input.ItemType,
			ItemID:    input.ItemID,
			ItemName:  product.Name,
			UnitPrice: product.PriceCents,
			Quantity:  1,
		}); err != nil {
			return err
		}
		createdID = order.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	return s.repo.GetOrder(ctx, userID, createdID)
}

// GetOrder 查询指定用户自己的订单。
func (s *OrderService) GetOrder(ctx context.Context, userID, orderID string) (*model.Order, error) {
	return s.repo.GetOrder(ctx, userID, orderID)
}

// ListOrders 查询指定用户的订单列表。
func (s *OrderService) ListOrders(ctx context.Context, userID string, limit, offset int) ([]model.Order, error) {
	return s.repo.ListOrders(ctx, userID, limit, offset)
}

// PayWithWallet 编排订单状态校验、钱包扣款和支付事件写入。
func (s *OrderService) PayWithWallet(ctx context.Context, userID, orderID string) (*model.Order, error) {
	err := s.tx.WithinTx(ctx, func(tx repository.Transaction) error {
		order, err := tx.LockOrderForPayment(ctx, userID, orderID)
		if err != nil {
			return err
		}
		if isPaidOrFurther(order.Status) {
			return nil
		}
		if order.Status != model.StatusPendingPayment || order.PaymentMethod != model.PaymentWallet {
			return ErrInvalidOrderState
		}

		wallet, err := tx.LockWalletForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if wallet.Balance < order.TotalAmount {
			return ErrInsufficientFunds
		}
		newBalance := wallet.Balance - order.TotalAmount
		if err := tx.UpdateWalletBalance(ctx, userID, newBalance, wallet.Version+1); err != nil {
			return err
		}
		if err := tx.InsertPurchaseLedger(ctx, userID, orderID, -order.TotalAmount, newBalance); err != nil {
			return err
		}
		if err := tx.MarkOrderPaid(ctx, orderID, "wallet:"+orderID); err != nil {
			return err
		}
		return tx.InsertOrderPaidEvent(ctx, orderID)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetOrder(ctx, userID, orderID)
}

// MockCallback 按订单状态机处理模拟支付平台回调；重复成功回调不会再次写支付事件。
func (s *OrderService) MockCallback(ctx context.Context, orderNo, providerTxnID string) (*model.Order, error) {
	var userID, orderID string
	err := s.tx.WithinTx(ctx, func(tx repository.Transaction) error {
		order, err := tx.LockOrderByNumber(ctx, orderNo)
		if err != nil {
			return err
		}
		userID, orderID = order.UserID, order.ID
		if isPaidOrFurther(order.Status) {
			return nil
		}
		if order.Status != model.StatusPendingPayment {
			return ErrInvalidOrderState
		}
		if err := tx.MarkOrderPaid(ctx, order.ID, providerTxnID); err != nil {
			return err
		}
		return tx.InsertOrderPaidEvent(ctx, order.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetOrder(ctx, userID, orderID)
}

func validItemType(itemType model.ItemType) bool {
	return itemType == model.ItemEpisode || itemType == model.ItemContent || itemType == model.ItemMembership
}

func isPaidOrFurther(status model.OrderStatus) bool {
	return status == model.StatusPaid || status == model.StatusFulfilling || status == model.StatusCompleted
}
