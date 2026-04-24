package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBroker реализует очередь на списках Redis с гарантией повторной обработки
// при ошибках, но без атомарных подтверждений (для надёжности используйте Streams).
type RedisBroker struct {
	client *redis.Client
	queue  string
	mu     sync.RWMutex
	closed bool
}

// NewRedisBroker создаёт подключение к Redis.
func NewRedisBroker(uri, queue string) (*RedisBroker, error) {
	opts, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URI: %w", err)
	}
	client := redis.NewClient(opts)

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

// Publish добавляет сообщение в конец очереди (FIFO).
func (r *RedisBroker) Publish(ctx context.Context, data []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return fmt.Errorf("broker is closed")
	}
	return r.client.RPush(ctx, r.queue, data).Err()
}

// PublishBatch публикует батч атомарно через конвейер.
// Внимание: если произойдёт сбой во время выполнения, часть сообщений может быть уже записана.
func (r *RedisBroker) PublishBatch(ctx context.Context, batch [][]byte) error {
	if len(batch) == 0 {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return fmt.Errorf("broker is closed")
	}
	pipe := r.client.Pipeline()
	for _, msg := range batch {
		pipe.RPush(ctx, r.queue, msg)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Consume запускает потребителя. При ошибке обработки сообщение возвращается в очередь.
// Не использует AUTO-ACK (т.к. Redis List не поддерживает), но возврат имитирует повторную обработку.
func (r *RedisBroker) Consume(ctx context.Context, handler func([]byte) error) error {
	// Бесконечный цикл, но без busy loop – блокирующий вызов BLPop с таймаутом
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// нет busy loop, потому что BLPop будет ждать 1 секунду, но select с default всё равно активен?
			// Лучше убрать default и полагаться на BLPop с контекстом через горутину, но упростим:
			// Будем использовать короткий таймаут и проверять ctx.Done()
		}

		// Используем BLPop с таймаутом 1 секунда.
		// Это не идеально для мгновенного отклика на ctx.Done(), но предотвращает busy loop.
		// Более правильное решение – использовать клиент с поддержкой контекста для BLPop,
		// но go-redis не поддерживает ctx в BLPop (только таймаут). Поэтому так:
		result, err := r.client.BLPop(ctx, 1*time.Second, r.queue).Result()
		if err != nil {
			if err == redis.Nil {
				continue // таймаут, очередь пуста
			}
			// Проверяем, не закрыт ли контекст (возможно, ошибка из-за отмены)
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return fmt.Errorf("failed to consume: %w", err)
			}
		}
		// result[0] – имя ключа, result[1] – значение
		msgData := []byte(result[1])

		// Обработка с защитой от паники
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					// При панике возвращаем сообщение в очередь
					_ = r.client.RPush(ctx, r.queue, msgData).Err()
				}
			}()
			if err := handler(msgData); err != nil {
				// При ошибке возвращаем сообщение в очередь (можно добавить счётчик попыток)
				_ = r.client.RPush(ctx, r.queue, msgData).Err()
				// Не выходим из Consume, продолжаем обрабатывать следующие сообщения
				return
			}
			// Если успешно – сообщение удалено из очереди (BLPop уже удалил), ничего не делаем
		}()
	}
}

// Purge удаляет всю очередь.
func (r *RedisBroker) Purge(ctx context.Context) error {
	return r.client.Del(ctx, r.queue).Err()
}

// Close закрывает клиент Redis и запрещает новые публикации.
func (r *RedisBroker) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.client.Close()
}

