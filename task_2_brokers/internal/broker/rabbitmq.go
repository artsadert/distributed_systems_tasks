package broker

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQBroker реализует публикацию и потребление сообщений с гарантией доставки (publisher confirms).
type RabbitMQBroker struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	queue    string
	pubMu    sync.Mutex               // сериализует публикации для корректной работы confirms
	confirms <-chan amqp.Confirmation // канал подтверждений от брокера
}

// NewRabbitMQBroker создаёт новое подключение к RabbitMQ, объявляет durable очередь
// и включает publisher confirms. Очередь НЕ удаляется автоматически (исправлено).
func NewRabbitMQBroker(uri, queue string) (*RabbitMQBroker, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Включаем publisher confirms (один раз)
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	// Объявляем durable очередь БЕЗ автоматического удаления (x-expires удалён)
	_, err = ch.QueueDeclare(
		queue, // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments – больше нет x-expires, очередь не исчезнет через 60 секунд
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Канал для получения подтверждений публикации
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return &RabbitMQBroker{
		conn:     conn,
		channel:  ch,
		queue:    queue,
		confirms: confirms,
	}, nil
}

// Publish отправляет одно сообщение и ждёт подтверждения от RabbitMQ.
// Возвращает ошибку, если брокер не подтвердил приём (Nack) или истёк контекст.
func (r *RabbitMQBroker) Publish(ctx context.Context, data []byte) error {
	r.pubMu.Lock()
	defer r.pubMu.Unlock()

	err := r.channel.PublishWithContext(ctx,
		"",      // exchange
		r.queue, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/octet-stream",
			Body:         data,
		})
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	// Ждём подтверждение именно для этого сообщения (по delivery tag)
	select {
	case confirm := <-r.confirms:
		if confirm.Ack {
			return nil
		}
		return fmt.Errorf("publish nacked for delivery tag %d", confirm.DeliveryTag)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishBatch отправляет несколько сообщений, дожидаясь подтверждения каждого.
// При ошибке на любом сообщении дальнейшая отправка прерывается.
func (r *RabbitMQBroker) PublishBatch(ctx context.Context, batch [][]byte) error {
	if len(batch) == 0 {
		return nil
	}
	for i, data := range batch {
		if err := r.Publish(ctx, data); err != nil {
			return fmt.Errorf("failed to publish message %d in batch: %w", i, err)
		}
	}
	return nil
}

// Consume запускает потребителя. Для каждого сообщения вызывается handler.
// Если handler возвращает ошибку или паникует – сообщение не подтверждается и возвращается в очередь (requeue).
// При успешной обработке – подтверждается.
func (r *RabbitMQBroker) Consume(ctx context.Context, handler func([]byte) error) error {
	msgs, err := r.channel.ConsumeWithContext(ctx,
		r.queue, // queue
		"",      // consumer tag
		false,   // auto-ack (false – управляем подтверждением вручную)
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return nil // канал сообщений закрыт
			}
			// Обрабатываем сообщение с защитой от паники
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						// При панике – возвращаем сообщение в очередь
						msg.Nack(false, true)
					}
				}()
				if err := handler(msg.Body); err != nil {
					msg.Nack(false, true) // возвращаем в очередь
					return
				}
				msg.Ack(false) // успешно обработано
			}()
		}
	}
}

// Purge удаляет все сообщения из очереди.
func (r *RabbitMQBroker) Purge(ctx context.Context) error {
	_, err := r.channel.QueuePurge(r.queue, false)
	return err
}

// Close закрывает канал и соединение.
// ВАЖНО: перед вызовом Close убедитесь, что все публикации завершены (все Publish вернули nil),
// иначе неподтверждённые сообщения могут быть потеряны.
func (r *RabbitMQBroker) Close() error {
	var errs []error
	if err := r.channel.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := r.conn.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
