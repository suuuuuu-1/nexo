package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName = "nexo.events"
	QueueName    = "nexo.entitlement"
	RoutingKey   = "order.paid"
	DeadLetter   = "nexo.entitlement.dlq"
)

type Client struct {
	// RabbitMQ Channel 不是并发安全的，因此 Publish 使用 mu 串行化写入。
	conn *amqp091.Connection
	ch   *amqp091.Channel
	mu   sync.Mutex
}

// Event 是 Outbox 事件投递到 RabbitMQ 后的统一消息格式。
type Event struct {
	ID            string `json:"id"`
	EventType     string `json:"event_type"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	Payload       []byte `json:"payload"`
}

// Connect 建立 RabbitMQ 连接，并声明本项目所需的交换机、队列和绑定关系。
func Connect(url string) (*Client, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := declareTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	return &Client{conn: conn, ch: ch}, nil
}

// Close 按 Channel → Connection 的顺序释放 RabbitMQ 资源。
func (c *Client) Close() error {
	if err := c.ch.Close(); err != nil {
		_ = c.conn.Close()
		return err
	}
	return c.conn.Close()
}

// Check reports whether the AMQP connection and channel are still open.
// RabbitMQ heartbeats update this state when a broker connection is lost.
func (c *Client) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.conn == nil || c.ch == nil || c.conn.IsClosed() || c.ch.IsClosed() {
		return fmt.Errorf("rabbitmq connection or channel is closed")
	}
	return nil
}

// Publish 发布持久化消息，并携带重试次数供消费者处理失败重试。
func (c *Client) Publish(ctx context.Context, routingKey string, event Event, retryCount int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.ch.PublishWithContext(ctx, ExchangeName, routingKey, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         body,
		Headers: amqp091.Table{
			"x-retry-count": retryCount,
		},
	})
}

// Consume 注册手动 ACK 的消费者。
// 只有权益履约成功后才确认消息，失败消息由 Worker 决定重试或进入死信队列。
func (c *Client) Consume(ctx context.Context) (<-chan amqp091.Delivery, error) {
	deliveries, err := c.ch.Consume(QueueName, "nexo-entitlement-worker", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("consume %s: %w", QueueName, err)
	}
	return deliveries, nil
}

// declareTopology 幂等声明消息基础设施，RabbitMQ 重启或服务重启时可重复执行。
func declareTopology(ch *amqp091.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(DeadLetter, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, amqp091.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": DeadLetter,
	}); err != nil {
		return err
	}
	return ch.QueueBind(QueueName, RoutingKey, ExchangeName, false, nil)
}
