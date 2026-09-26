package model

// OutboxEvent 是数据库 Outbox 表中等待发布到消息队列的事件。
type OutboxEvent struct {
	ID            string
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       []byte
	Attempts      int
}
