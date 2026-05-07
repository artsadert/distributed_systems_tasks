package main

import (
	"broker-comparison/internal/broker"
	"broker-comparison/internal/metrics"
	"broker-comparison/internal/payload"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	brokerType  = pflag.String("broker", "rabbitmq", "Broker type: rabbitmq or redis")
	brokerURI   = pflag.String("uri", "amqp://guest:guest@localhost:5672/", "Broker connection URI")
	queueName   = pflag.String("queue", "test_queue", "Queue name")
	messageSize = pflag.Int("size", 1024, "Message size in bytes")
	rate        = pflag.Int("rate", 1000, "Target messages per second")
	duration    = pflag.Duration("duration", 30*time.Second, "Test duration")
	metricsFile = pflag.String("metrics-file", "producer_metrics.json", "File to write metrics")
	logLevel    = pflag.String("log-level", "info", "Log level")
)

func main() {
	pflag.Parse()

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logrus.Info("Received interrupt signal, stopping...")
		cancel()
	}()

	var b broker.Broker
	switch *brokerType {
	case "rabbitmq":
		b, err = broker.NewRabbitMQBroker(*brokerURI, *queueName)
	case "redis":
		b, err = broker.NewRedisBroker(*brokerURI, *queueName)
	case "artmq":
		b, err = broker.NewArtMQBroker(*brokerURI, *queueName)
	default:
		logrus.Fatalf("Unknown broker type: %s", *brokerType)
	}

	if err != nil {
		logrus.Fatalf("Failed to create broker: %v", err)
	}
	defer b.Close()

	purgeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.Purge(purgeCtx); err != nil {
		logrus.WithError(err).Warn("Failed to purge queue")
	} else {
		logrus.Info("Queue purged")
	}

	gen := payload.NewGenerator(*messageSize)
	m := metrics.NewProducerMetrics()

	logrus.WithFields(logrus.Fields{
		"broker": *brokerType,
		"queue":  *queueName,
		"size":   *messageSize,
		"rate":   *rate,
	}).Info("Starting producer")

	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			durationActual := time.Since(start)
			logrus.Info("Producer finished")
			saveMetrics(m, durationActual, *metricsFile)
			return
		case <-ticker.C:
			msg := gen.Generate()
			err := b.Publish(context.Background(), msg)
			if err != nil {
				m.RecordError()
				logrus.WithError(err).Error("Failed to publish message")
			} else {
				m.RecordSent()
			}
		}
	}
}

func saveMetrics(m *metrics.ProducerMetrics, duration time.Duration, filename string) {
	data := map[string]interface{}{
		"messages_sent": m.Sent(),
		"errors":        m.Errors(),
		"duration_sec":  duration.Seconds(),
		"actual_rate":   float64(m.Sent()) / duration.Seconds(),
		"target_rate":   *rate,
		"message_size":  *messageSize,
		"broker":        *brokerType,
	}
	file, err := os.Create(filename)
	if err != nil {
		logrus.WithError(err).Error("Failed to create metrics file")
		return
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		logrus.WithError(err).Error("Failed to encode metrics")
	}
	logrus.WithField("file", filename).Info("Metrics saved")
}
