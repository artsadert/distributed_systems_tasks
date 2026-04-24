# Сравнение RabbitMQ и Redis как брокеров сообщений

## Сборка

```bash
go mod tidy
go build -o bin/producer ./cmd/producer
go build -o bin/consumer ./cmd/consumer
```

```bash
./bin/consumer --broker rabbitmq --uri amqp://guest:guest@localhost:5672/ --queue test --duration 35s


./bin/producer --broker rabbitmq --uri amqp://guest:guest@localhost:5672/ --queue test --size 1024 --rate 1000 --duration 30s

```

## Запуск тестов

```bash
chmod +x scripts/run_test.sh
mkdir results
./scripts/run_test.sh rabbitmq amqp://guest:guest@localhost:5672/ test_queue
./scripts/run_test.sh redis redis://localhost:6379/0 test_queue
```