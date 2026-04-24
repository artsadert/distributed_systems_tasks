#!/bin/bash

# Запуск всех тестов параллельно
set -e

BROKERS=("rabbitmq" "redis")
SIZES=("128" "1024" "10240" "102400")
RATES=("1000" "5000" "10000")
DURATION_SEC=30
SLEEP_DELAY=1
DRAIN_SEC=5
PRODUCER_DURATION="${DURATION_SEC}s"
# Consumer runs longer: covers the sleep delay before producer starts + drain time after producer stops
CONSUMER_DURATION="$((DURATION_SEC + SLEEP_DELAY + DRAIN_SEC))s"

# URI для брокеров
declare -A URIS
URIS["rabbitmq"]="amqp://guest:guest@localhost:5672/"
URIS["redis"]="redis://localhost:6379/0"

# Сборка бинарников
echo "Building binaries..."
go build -o bin/producer ./cmd/producer
go build -o bin/consumer ./cmd/consumer
go build -o bin/report ./cmd/report

# Очистка старых результатов
rm -f results/*.json
mkdir -p results

# Массив для хранения PID фоновых процессов
pids=()

# Запуск тестов
for BROKER in "${BROKERS[@]}"; do
  URI=${URIS[$BROKER]}
  for SIZE in "${SIZES[@]}"; do
    for RATE in "${RATES[@]}"; do
      QUEUE="queue_${BROKER}_${SIZE}_${RATE}"
      METRICS_PREFIX="${BROKER}_size${SIZE}_rate${RATE}"

      echo "Launching test: Broker=$BROKER, Size=$SIZE, Rate=$RATE"

      # Запускаем consumer в фоне
      ./bin/consumer \
        --broker "$BROKER" \
        --uri "$URI" \
        --queue "$QUEUE" \
        --duration "$CONSUMER_DURATION" \
        --metrics-file "results/${METRICS_PREFIX}_consumer.json" \
        --log-level warn &
      consumer_pid=$!

      # Даём consumer время на подключение
      sleep $SLEEP_DELAY

      # Запускаем producer в фоне
      ./bin/producer \
        --broker "$BROKER" \
        --uri "$URI" \
        --queue "$QUEUE" \
        --size "$SIZE" \
        --rate "$RATE" \
        --duration "$PRODUCER_DURATION" \
        --metrics-file "results/${METRICS_PREFIX}_producer.json" \
        --log-level info &
      producer_pid=$!

      # Сохраняем PID обоих процессов для ожидания
      pids+=($consumer_pid $producer_pid)

      # Небольшая пауза, чтобы не перегрузить брокер одновременным стартом
      sleep 0.2
    done
  done
done

echo "All tests launched. Waiting for completion..."

# Ожидаем завершения всех процессов
for pid in "${pids[@]}"; do
  wait $pid
done

echo "All tests completed."

# Генерация отчёта
./bin/report --results results --output report.md
echo "Report generated: report.md"

