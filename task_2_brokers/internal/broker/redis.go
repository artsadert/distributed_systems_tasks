package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisBroker struct {
	client *redis.Client
	queue  string
}

func NewRedisBroker(uri, queue string) (*RedisBroker, error) {
	opts, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URI: %w", err)
	}
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisBroker{
		client: client,
		queue:  queue,
	}, nil
}

func (r *RedisBroker) Publish(ctx context.Context, data []byte) error {
	return r.client.LPush(ctx, r.queue, data).Err()
}

func (r *RedisBroker) Consume(ctx context.Context, handler func([]byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			
			result, err := r.client.BLPop(ctx, 1*time.Second, r.queue).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				return err
			}
			if err := handler([]byte(result[1])); err != nil {
				return err
			}
		}
	}
}

func (r *RedisBroker) PublishBatch(ctx context.Context, batch [][]byte) error {
	if len(batch) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, msg := range batch {
		pipe.LPush(ctx, r.queue, msg)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBroker) Purge(ctx context.Context) error {
	return r.client.Del(ctx, r.queue).Err()
}

func (r *RedisBroker) Close() error {
	return r.client.Close()
}