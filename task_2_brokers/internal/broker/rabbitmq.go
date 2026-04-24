package broker

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQBroker struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

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
	// Enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		return nil, err
	}

	// Declare a durable queue
	_, err = ch.QueueDeclare(
		queue,                          // name
		true,                           // durable
		false,                          // delete when unused
		false,                          // exclusive
		false,                          // no-wait
		amqp.Table{"x-expires": 60000}, // автоудаление через 60 сек неиспользования
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	return &RabbitMQBroker{
		conn:    conn,
		channel: ch,
		queue:   queue,
	}, nil
}

func (r *RabbitMQBroker) Publish(ctx context.Context, data []byte) error {
	return r.channel.PublishWithContext(ctx,
		"",      // exchange
		r.queue, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/octet-stream",
			Body:         data,
		})
}

func (r *RabbitMQBroker) Consume(ctx context.Context, handler func([]byte) error) error {
	msgs, err := r.channel.ConsumeWithContext(ctx,
		r.queue, // queue
		"",      // consumer tag
		false,   // auto-ack
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
				return nil
			}
			if err := handler(msg.Body); err != nil {
				msg.Nack(false, true)
				return err
			}
			msg.Ack(false)
		}
	}
}

func (r *RabbitMQBroker) PublishBatch(ctx context.Context, batch [][]byte) error {
    return nil 
}

func (r *RabbitMQBroker) Purge(ctx context.Context) error {
	_, err := r.channel.QueuePurge(r.queue, false)
	return err
}

func (r *RabbitMQBroker) Close() error {
	if err := r.channel.Close(); err != nil {
		r.conn.Close()
		return err
	}
	return r.conn.Close()
}
