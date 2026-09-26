package service

import (
	"context"
	"testing"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

type fakeEntitlementRunner struct {
	tx *fakeEntitlementTx
}

func (r *fakeEntitlementRunner) WithinEntitlementTx(ctx context.Context, fn func(repository.EntitlementTx) error) error {
	return fn(r.tx)
}

type fakeEntitlementTx struct {
	order               model.Order
	createdSubscription int
	createdEntitlement  int
	lastScope           model.EntitlementScope
	lastSource          model.EntitlementSource
}

func (tx *fakeEntitlementTx) LockOrderForFulfillment(context.Context, string) (*model.Order, error) {
	return &tx.order, nil
}

func (tx *fakeEntitlementTx) UpdateOrderStatus(_ context.Context, _ string, status model.OrderStatus) error {
	tx.order.Status = status
	return nil
}

func (*fakeEntitlementTx) GetMembershipPlanDuration(context.Context, string) (int, error) {
	return 30, nil
}

func (tx *fakeEntitlementTx) CreateSubscription(context.Context, string, string, string, model.TimeRange) (string, error) {
	tx.createdSubscription++
	return "subscription-test", nil
}

func (tx *fakeEntitlementTx) CreateEntitlement(_ context.Context, _ string, scope model.EntitlementScope, _ string, source model.EntitlementSource, _ string, _ model.TimeRange) error {
	tx.createdEntitlement++
	tx.lastScope = scope
	tx.lastSource = source
	return nil
}

func TestFulfillOrderIsIdempotentAfterCompletion(t *testing.T) {
	tests := []struct {
		name                 string
		itemType             model.ItemType
		wantScope            model.EntitlementScope
		wantSource           model.EntitlementSource
		wantSubscriptionRows int
	}{
		{
			name:       "episode purchase",
			itemType:   model.ItemEpisode,
			wantScope:  model.ScopeEpisode,
			wantSource: model.SourceOrder,
		},
		{
			name:                 "membership subscription",
			itemType:             model.ItemMembership,
			wantScope:            model.ScopeMembership,
			wantSource:           model.SourceSubscription,
			wantSubscriptionRows: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeEntitlementTx{order: model.Order{
				ID:     "order-test",
				UserID: "user-test",
				Status: model.StatusPaid,
				Item: model.OrderItem{
					ItemType: tt.itemType,
					ItemID:   "item-test",
				},
			}}
			service := NewEntitlementService(nil, &fakeEntitlementRunner{tx: tx})

			for i := 0; i < 2; i++ {
				if err := service.FulfillOrder(context.Background(), "order-test"); err != nil {
					t.Fatalf("FulfillOrder() attempt %d: %v", i+1, err)
				}
			}

			if tx.order.Status != model.StatusCompleted {
				t.Fatalf("order status = %q, want %q", tx.order.Status, model.StatusCompleted)
			}
			if tx.createdEntitlement != 1 {
				t.Errorf("created entitlements = %d, want 1", tx.createdEntitlement)
			}
			if tx.createdSubscription != tt.wantSubscriptionRows {
				t.Errorf("created subscriptions = %d, want %d", tx.createdSubscription, tt.wantSubscriptionRows)
			}
			if tx.lastScope != tt.wantScope || tx.lastSource != tt.wantSource {
				t.Errorf("grant scope/source = %s/%s, want %s/%s", tx.lastScope, tx.lastSource, tt.wantScope, tt.wantSource)
			}
		})
	}
}
