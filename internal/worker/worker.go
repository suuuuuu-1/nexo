package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/suuuuu/nexo/internal/mq"
	"github.com/suuuuu/nexo/internal/repository"
	"github.com/suuuuu/nexo/internal/service"
)

func StartOutboxPublisher(ctx context.Context, repo *repository.OutboxRepository, broker *mq.Client) {
	// Publisher 采用轻量轮询，适合当前单体 v1；后续可替换成更复杂的调度器。
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishOutboxBatch(ctx, repo, broker)
			}
		}
	}()
}

// StartEntitlementWorker 启动订单支付事件消费者。
// 消费采用手动 ACK，确保权益发放成功后才确认原始消息。
func StartEntitlementWorker(ctx context.Context, broker *mq.Client, entitlementService *service.EntitlementService) error {
	deliveries, err := broker.Consume(ctx)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}
				handleDelivery(ctx, broker, entitlementService, delivery)
			}
		}
	}()
	return nil
}

// publishOutboxBatch 完成“领取事件 → 投递 MQ → 更新投递状态”的一轮处理。
func publishOutboxBatch(ctx context.Context, repo *repository.OutboxRepository, broker *mq.Client) {
	events, err := repo.ClaimPending(ctx, 20)
	if err != nil {
		log.Printf("outbox claim failed: %v", err)
		return
	}
	for _, event := range events {
		publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := broker.Publish(publishCtx, mq.RoutingKey, mq.Event{ID: event.ID, EventType: event.EventType, AggregateType: event.AggregateType, AggregateID: event.AggregateID, Payload: event.Payload}, 0)
		cancel()
		if err != nil {
			log.Printf("outbox publish failed event=%s: %v", event.ID, err)
			_ = repo.MarkFailed(ctx, event.ID, event.Attempts+1)
			continue
		}
		if err := repo.MarkPublished(ctx, event.ID); err != nil {
			log.Printf("outbox mark published failed event=%s: %v", event.ID, err)
		}
	}
}

// handleDelivery 解析支付事件并执行权益履约。
// 履约服务本身具有幂等约束，因此 RabbitMQ 重复投递不会重复创建权益。
func handleDelivery(ctx context.Context, broker *mq.Client, entitlementService *service.EntitlementService, delivery amqp091.Delivery) {
	var event mq.Event
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		_ = delivery.Nack(false, false)
		return
	}
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.OrderID == "" {
		_ = delivery.Nack(false, false)
		return
	}
	httpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := entitlementService.FulfillOrder(httpCtx, payload.OrderID)
	cancel()
	if err == nil {
		_ = delivery.Ack(false)
		return
	}

	retryCount := 0
	if value, ok := delivery.Headers["x-retry-count"].(int); ok {
		retryCount = value
	}
	if retryCount >= 3 {
		_ = delivery.Nack(false, false)
		return
	}
	if err := broker.Publish(ctx, mq.RoutingKey, event, retryCount+1); err != nil {
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}
