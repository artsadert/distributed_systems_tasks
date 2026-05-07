package broker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	artmq "github.com/artsadert/artmq-client-go"
)

// ArtMQBroker реализует Broker поверх MQTT-брокера artmq.
// Гарантия доставки – QoS 1 (хотя бы один раз).
type ArtMQBroker struct {
	client *artmq.Client
	topic  string

	subOnce sync.Once
	msgs    chan []byte
	subErr  error
}

// NewArtMQBroker подключается к artmq-серверу. uri может быть в форме
// "mqtt://host:port", "host:port" или просто "host" (тогда используется порт 1883).
func NewArtMQBroker(uri, topic string) (*ArtMQBroker, error) {
	addr := strings.TrimPrefix(uri, "mqtt://")
	addr = strings.TrimSuffix(addr, "/")
	if !strings.Contains(addr, ":") {
		addr = addr + ":1883"
	}

	opts := artmq.NewClientOptions().
		SetBrokerAddr(addr).
		SetKeepAlive(30 * time.Second).
		SetCleanStart(true).
		SetConnectTimeout(10 * time.Second)

	c := artmq.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to artmq: %w", err)
	}

	return &ArtMQBroker{
		client: c,
		topic:  topic,
		msgs:   make(chan []byte, 1024),
	}, nil
}

// Publish отправляет одно сообщение с QoS 1 – клиент блокируется до получения PUBACK.
func (b *ArtMQBroker) Publish(ctx context.Context, data []byte) error {
	if err := b.client.Publish(ctx, b.topic, data, artmq.WithQoS(artmq.QoS1)); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}
	return nil
}

// PublishBatch последовательно отправляет несколько сообщений.
func (b *ArtMQBroker) PublishBatch(ctx context.Context, batch [][]byte) error {
	for i, data := range batch {
		if err := b.Publish(ctx, data); err != nil {
			return fmt.Errorf("failed to publish message %d in batch: %w", i, err)
		}
	}
	return nil
}

// Consume подписывается на топик и блокируется до отмены контекста.
// MQTT не имеет понятия "подтверждение обработки" на уровне приложения, поэтому
// ошибки handler'а только логируются: повторная доставка возможна лишь при потере соединения.
func (b *ArtMQBroker) Consume(ctx context.Context, handler func([]byte) error) error {
	b.subOnce.Do(func() {
		b.subErr = b.client.Subscribe(ctx, b.topic, artmq.QoS1, func(_ string, payload []byte) {
			buf := make([]byte, len(payload))
			copy(buf, payload)
			select {
			case b.msgs <- buf:
			case <-ctx.Done():
			}
		})
	})
	if b.subErr != nil {
		return fmt.Errorf("subscribe failed: %w", b.subErr)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-b.msgs:
			func() {
				defer func() { _ = recover() }()
				_ = handler(msg)
			}()
		}
	}
}

// Purge для MQTT не имеет прямого аналога: каждый тестовый прогон использует
// уникальный topic ("queue_<size>_<rate>"), а ретейн-сообщения мы не публикуем.
func (b *ArtMQBroker) Purge(_ context.Context) error {
	return nil
}

// Close закрывает соединение с брокером.
func (b *ArtMQBroker) Close() error {
	return b.client.Disconnect()
}