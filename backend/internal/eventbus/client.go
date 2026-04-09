package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis client
type Client struct {
	Redis *redis.Client
}

// New client
func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("eventbus: parse redis URL %q: %w", redisURL, err)
	}

	// Performance tuning
	opts.PoolSize = 20
	opts.MinIdleConns = 5
	opts.ConnMaxIdleTime = 5 * time.Minute

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("eventbus: connect to redis: %w", err)
	}

	return &Client{Redis: rdb}, nil
}

// Close client
func (c *Client) Close() error {
	return c.Redis.Close()
}
