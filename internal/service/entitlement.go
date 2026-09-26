package service

import (
	"context"
	"errors"
	"time"

	"github.com/suuuuu/nexo/internal/model"
	"github.com/suuuuu/nexo/internal/repository"
)

var ErrInvalidFulfillmentState = errors.New("order is not ready for fulfillment")

type EntitlementService struct {
	repo *repository.EntitlementRepository
	tx   repository.EntitlementTransactionRunner
}

// NewEntitlementService 创建权益业务服务。
func NewEntitlementService(repo *repository.EntitlementRepository, tx repository.EntitlementTransactionRunner) *EntitlementService {
	return &EntitlementService{repo: repo, tx: tx}
}

// CheckEpisodeAccess 根据 Episode 的访问规则组合购买权益和会员权益。
func (s *EntitlementService) CheckEpisodeAccess(ctx context.Context, userID, episodeID string) (model.AccessResult, error) {
	contentID, accessType, err := s.repo.GetEpisodePolicy(ctx, episodeID)
	if err != nil {
		return model.AccessResult{}, err
	}
	if accessType == model.AccessFree {
		return model.AccessResult{Allowed: true, Reason: string(model.AccessFree)}, nil
	}

	if accessType == model.AccessPurchase || accessType == model.AccessPurchaseOrMembership {
		sourceID, found, err := s.repo.FindActivePurchaseEntitlement(ctx, userID, episodeID, contentID)
		if err != nil {
			return model.AccessResult{}, err
		}
		if found {
			return model.AccessResult{Allowed: true, Reason: "ENTITLEMENT", SourceID: sourceID}, nil
		}
	}
	if accessType == model.AccessMembership || accessType == model.AccessPurchaseOrMembership {
		sourceID, found, err := s.repo.FindActiveMembershipEntitlement(ctx, userID)
		if err != nil {
			return model.AccessResult{}, err
		}
		if found {
			return model.AccessResult{Allowed: true, Reason: "ENTITLEMENT", SourceID: sourceID}, nil
		}
	}
	return model.AccessResult{Allowed: false, Reason: string(accessType)}, nil
}

// FulfillOrder 推进支付订单状态机，并在同一事务中创建对应的订阅和权益。
func (s *EntitlementService) FulfillOrder(ctx context.Context, orderID string) error {
	return s.tx.WithinEntitlementTx(ctx, func(tx repository.EntitlementTx) error {
		order, err := tx.LockOrderForFulfillment(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status == model.StatusCompleted {
			return nil
		}
		if order.Status != model.StatusPaid && order.Status != model.StatusFulfilling {
			return ErrInvalidFulfillmentState
		}
		if order.Status == model.StatusPaid {
			if err := tx.UpdateOrderStatus(ctx, order.ID, model.StatusFulfilling); err != nil {
				return err
			}
		}

		validity := model.TimeRange{}
		scopeType := model.ScopeEpisode
		scopeID := order.Item.ItemID
		sourceType := model.SourceOrder
		sourceID := order.ID
		switch order.Item.ItemType {
		case model.ItemEpisode:
			scopeType = model.ScopeEpisode
		case model.ItemContent:
			scopeType = model.ScopeContent
		case model.ItemMembership:
			now := time.Now()
			durationDays, err := tx.GetMembershipPlanDuration(ctx, order.Item.ItemID)
			if err != nil {
				return err
			}
			validity.StartsAt = &now
			endsAt := now.Add(time.Duration(durationDays) * 24 * time.Hour)
			validity.EndsAt = &endsAt
			subscriptionID, err := tx.CreateSubscription(ctx, order.UserID, order.Item.ItemID, order.ID, validity)
			if err != nil {
				return err
			}
			scopeType = model.ScopeMembership
			scopeID = order.Item.ItemID
			sourceType = model.SourceSubscription
			sourceID = subscriptionID
		default:
			return ErrInvalidFulfillmentState
		}

		if err := tx.CreateEntitlement(ctx, order.UserID, scopeType, scopeID, sourceType, sourceID, validity); err != nil {
			return err
		}
		return tx.UpdateOrderStatus(ctx, order.ID, model.StatusCompleted)
	})
}
