package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	// redis 客户端由本包统一封装，业务层只依赖 JSON 和删除缓存等必要操作。
	redis *redis.Client
}

// Connect 创建 Redis 客户端并确认 Redis 当前可用。
func Connect(ctx context.Context, redisURL string) (*Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &Client{redis: client}, nil
}

// Close 释放 Redis 连接池。
func (c *Client) Close() error {
	return c.redis.Close()
}

// Ping verifies Redis is reachable within the caller's context deadline.
func (c *Client) Ping(ctx context.Context) error {
	return c.redis.Ping(ctx).Err()
}

// GetJSON 读取并反序列化 JSON 缓存；缓存不存在时返回 false，而不是错误。
func (c *Client) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	value, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON 序列化对象并写入带过期时间的缓存。
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, key, data, ttl).Err()
}

// Delete 删除指定缓存键，用于内容变更后的主动失效。
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.redis.Del(ctx, keys...).Err()
}
