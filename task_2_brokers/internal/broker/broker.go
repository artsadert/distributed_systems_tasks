package broker

import "context"

type Broker interface {
	Publish(ctx context.Context, data []byte) error
	Consume(ctx context.Context, handler func([]byte) error) error
	Close() error
	Purge(ctx context.Context) error
	PublishBatch(ctx context.Context, batch [][]byte) error
}