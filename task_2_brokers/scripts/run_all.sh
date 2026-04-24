#!/bin/bash

# Start brokers if not running (optional)
# docker-compose up -d
docker exec rabbitmq rabbitmqctl delete_queue test_queue
docker exec redis redis-cli FLUSHDB


# Build binaries
go build -o bin/producer ./cmd/producer
go build -o bin/consumer ./cmd/consumer
go build -o bin/report ./cmd/report

# Clear old results
rm -f results/*.json

# Run RabbitMQ tests
./scripts/run_test.sh rabbitmq amqp://guest:guest@localhost:5672/ test_queue

# Run Redis tests
./scripts/run_test.sh redis redis://localhost:6379/0 test_queue

# Generate report
./bin/report --results results --output report.md

echo "Report generated: report.md"